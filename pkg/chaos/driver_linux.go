//go:build linux

package chaos

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// LinuxNetemDriver implements ChaosDriver for Linux hosts and containers using netlink & tc netem.
type LinuxNetemDriver struct {
	mu           sync.RWMutex
	status       ChaosStatus
	netnsPath    string
	activeRoot   bool
	appliedFault *FaultSpec
}

// newNetemOrError backs chaos.NewDriverByMode("netem") on Linux builds.
func newNetemOrError() (ChaosDriver, error) {
	return NewLinuxNetemDriver(""), nil
}

// NewLinuxNetemDriver creates a new Linux-native netem driver.
// If netnsPath is non-empty, operations are executed within the target container network namespace.
func NewLinuxNetemDriver(netnsPath string) *LinuxNetemDriver {
	return &LinuxNetemDriver{
		netnsPath: netnsPath,
		status: ChaosStatus{
			State: StateIdle,
		},
	}
}

// withNetns executes fn inside the designated network namespace (if configured).
func (d *LinuxNetemDriver) withNetns(fn func() error) error {
	if d.netnsPath == "" {
		return fn()
	}

	targetNs, err := netns.GetFromPath(d.netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open target netns %s: %w", d.netnsPath, err)
	}
	defer targetNs.Close()

	curNs, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current netns: %w", err)
	}
	defer curNs.Close()

	if err := netns.Set(targetNs); err != nil {
		return fmt.Errorf("failed to enter target netns: %w", err)
	}
	defer netns.Set(curNs)

	return fn()
}

// Apply configures Linux tc netem and optional u32 traffic isolation filters.
func (d *LinuxNetemDriver) Apply(ctx context.Context, fault FaultSpec) error {
	if err := fault.Validate(); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.status.State = StateApplying

	err := d.withNetns(func() error {
		link, err := netlink.LinkByName(fault.Filter.Interface)
		if err != nil {
			return fmt.Errorf("network interface %q not found: %w", fault.Filter.Interface, err)
		}

		// First, cleanly ensure any existing root qdisc is cleared
		_ = netlink.QdiscDel(&netlink.GenericQdisc{
			QdiscAttrs: netlink.QdiscAttrs{
				LinkIndex: link.Attrs().Index,
				Parent:    netlink.HANDLE_ROOT,
			},
		})

		// Case A: Unfiltered (apply netem directly at root)
		if fault.Filter.TargetIP == "" && fault.Filter.TargetPort == 0 {
			netemAttrs := netlink.NetemQdiscAttrs{
				Latency:     uint32(fault.Latency.Microseconds()),
				Jitter:      uint32(fault.Jitter.Microseconds()),
				Loss:        float32(fault.LossPercent),
				CorruptProb: float32(fault.CorruptionPercent),
			}
			qdisc := netlink.NewNetem(
				netlink.QdiscAttrs{
					LinkIndex: link.Attrs().Index,
					Parent:    netlink.HANDLE_ROOT,
					Handle:    netlink.MakeHandle(1, 0),
				},
				netemAttrs,
			)
			if err := netlink.QdiscAdd(qdisc); err != nil {
				return fmt.Errorf("failed to add root netem qdisc: %w", err)
			}
			d.activeRoot = true
			return nil
		}

		// Case B: Filtered by IP and/or Port using prio qdisc and u32 filter
		// Root prio qdisc with 3 bands (1:1, 1:2, 1:3). Default traffic -> 1:1, Fault -> 1:2.
		prioQdisc := netlink.NewPrio(netlink.QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    netlink.HANDLE_ROOT,
			Handle:    netlink.MakeHandle(1, 0),
		})
		if err := netlink.QdiscAdd(prioQdisc); err != nil {
			return fmt.Errorf("failed to add root prio qdisc for filtered traffic: %w", err)
		}

		// Attach netem to band 1:2 (handle 20:)
		netemAttrs := netlink.NetemQdiscAttrs{
			Latency:     uint32(fault.Latency.Microseconds()),
			Jitter:      uint32(fault.Jitter.Microseconds()),
			Loss:        float32(fault.LossPercent),
			CorruptProb: float32(fault.CorruptionPercent),
		}
		faultQdisc := netlink.NewNetem(
			netlink.QdiscAttrs{
				LinkIndex: link.Attrs().Index,
				Parent:    netlink.MakeHandle(1, 2),
				Handle:    netlink.MakeHandle(20, 0),
			},
			netemAttrs,
		)
		if err := netlink.QdiscAdd(faultQdisc); err != nil {
			return fmt.Errorf("failed to add netem qdisc on band 1:2: %w", err)
		}

		// Add u32 filter
		u32Args, err := BuildU32FilterArgs(fault.Filter, "1:", "1:2")
		if err != nil {
			return fmt.Errorf("failed to build u32 filter arguments: %w", err)
		}

		// Execute tc filter rule
		cmd := exec.CommandContext(ctx, "tc", u32Args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to apply u32 filter via tc (%s): %w", string(out), err)
		}

		d.activeRoot = true
		return nil
	})

	if err != nil {
		d.status.State = StateFailed
		d.status.LastError = err.Error()
		return err
	}

	now := time.Now()
	faultCopy := fault
	d.appliedFault = &faultCopy
	d.status = ChaosStatus{
		State:        StateInjected,
		CurrentFault: &faultCopy,
		AppliedAt:    now,
		ExpiresAt:    now.Add(fault.Duration),
		LastError:    "",
	}
	return nil
}

// Revert idempotenly removes any active tc qdisc rules on the target interface.
func (d *LinuxNetemDriver) Revert(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.status.State = StateReverting

	err := d.withNetns(func() error {
		if d.appliedFault == nil {
			return nil
		}
		link, err := netlink.LinkByName(d.appliedFault.Filter.Interface)
		if err != nil {
			// Interface might be gone or renamed, consider already reverted
			return nil
		}

		delErr := netlink.QdiscDel(&netlink.GenericQdisc{
			QdiscAttrs: netlink.QdiscAttrs{
				LinkIndex: link.Attrs().Index,
				Parent:    netlink.HANDLE_ROOT,
			},
		})

		if delErr != nil {
			// Check if already deleted (ENOENT)
			if strings.Contains(delErr.Error(), "no such file") || strings.Contains(delErr.Error(), "cannot find") {
				return nil
			}
			return delErr
		}
		return nil
	})

	if err != nil {
		d.status.State = StateFailed
		d.status.LastError = err.Error()
		return err
	}

	d.activeRoot = false
	d.appliedFault = nil
	d.status = ChaosStatus{
		State:        StateReverted,
		CurrentFault: nil,
	}
	return nil
}

// Status returns current driver state.
func (d *LinuxNetemDriver) Status() ChaosStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.status
}
