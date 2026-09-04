package chaos

import (
	"fmt"
	"time"
)

// State represents the current lifecycle phase of a chaos fault injection.
type State string

const (
	StateIdle      State = "idle"
	StateApplying  State = "applying"
	StateInjected  State = "injected"
	StateReverting State = "reverting"
	StateReverted  State = "reverted"
	StateFailed    State = "failed"
)

// FaultType specifies the category of network disruption.
type FaultType string

const (
	FaultLatency        FaultType = "latency"
	FaultPacketLoss     FaultType = "packet_loss"
	FaultCorruption     FaultType = "corruption"
	FaultBandwidthLimit FaultType = "bandwidth_limit"
	FaultComposite      FaultType = "composite"
	FaultDNS            FaultType = "dns"             // Delay or fail DNS resolution for matching domains.
	FaultClockSkew      FaultType = "clock_skew"      // Inject a simulated-time header for cooperating targets.
	FaultResourceStress FaultType = "resource_stress" // Local CPU/memory/disk pressure (see ResourceStressDriver).
)

// FilterConfig restricts chaos fault injection to specific network interfaces, IP CIDRs, ports, or
// hostnames. This prevents blast radius leakage onto control planes, SSH connections, or other
// co-located services — and, via TargetDomains, lets a fault target only a specific third-party
// dependency (e.g. a payment gateway) instead of every outbound request.
type FilterConfig struct {
	Interface     string   `json:"interface" yaml:"interface"`                               // Network interface, e.g. "eth0", "lo"
	TargetIP      string   `json:"target_ip,omitempty" yaml:"target_ip,omitempty"`           // Target IP or CIDR, e.g. "10.0.0.5/32"
	TargetPort    int      `json:"target_port,omitempty" yaml:"target_port,omitempty"`       // Target port (e.g. 8080)
	Protocol      string   `json:"protocol,omitempty" yaml:"protocol,omitempty"`             // "tcp", "udp", or "" (all)
	TargetDomains []string `json:"target_domains,omitempty" yaml:"target_domains,omitempty"` // Hostnames (suffix-matched) — HTTPProxyDriver/DNS fault only.
}

// FaultSpec defines a network disruption task with safety constraints.
type FaultSpec struct {
	ID                string        `json:"id" yaml:"id"`
	Type              FaultType     `json:"type" yaml:"type"`
	Filter            FilterConfig  `json:"filter" yaml:"filter"`
	Latency           time.Duration `json:"latency,omitempty" yaml:"latency,omitempty"`
	Jitter            time.Duration `json:"jitter,omitempty" yaml:"jitter,omitempty"`
	Correlation       float64       `json:"correlation,omitempty" yaml:"correlation,omitempty"`   // 0.0 - 1.0
	LossPercent       float64       `json:"loss_percent,omitempty" yaml:"loss_percent,omitempty"` // 0.0 - 100.0
	CorruptionPercent float64       `json:"corruption_percent,omitempty" yaml:"corruption_percent,omitempty"`
	RateLimitKbps     int           `json:"rate_limit_kbps,omitempty" yaml:"rate_limit_kbps,omitempty"`
	Duration          time.Duration `json:"duration" yaml:"duration"` // Mandatory TTL for Dead-Man Switch

	// DNS fault (FaultDNS / HTTPProxyDriver only).
	DNSFailPercent float64       `json:"dns_fail_percent,omitempty" yaml:"dns_fail_percent,omitempty"` // 0-100: chance resolution fails outright (NXDOMAIN-like).
	DNSDelay       time.Duration `json:"dns_delay,omitempty" yaml:"dns_delay,omitempty"`               // Extra delay before resolution "completes".

	// Clock skew (FaultClockSkew / HTTPProxyDriver only). Since changing the OS clock needs root,
	// this injects an `X-Oshimai-Simulated-Time` header carrying the skewed timestamp for
	// cooperating target apps to read — documented as an app-level simulation, not a host-level one.
	ClockSkewOffset time.Duration `json:"clock_skew_offset,omitempty" yaml:"clock_skew_offset,omitempty"`

	// Resource stress (FaultResourceStress / ResourceStressDriver only). Pressures the HOST the
	// driver runs on — meaningful when Oshimai (or an agent) runs alongside the target, e.g. local
	// dev or a same-host container, not when testing a remote target over the network.
	CPULoadPercent int `json:"cpu_load_percent,omitempty" yaml:"cpu_load_percent,omitempty"` // 0-100 per core.
	MemoryLoadMB   int `json:"memory_load_mb,omitempty" yaml:"memory_load_mb,omitempty"`
	DiskIOLoadMB   int `json:"disk_io_load_mb,omitempty" yaml:"disk_io_load_mb,omitempty"` // Written+deleted per cycle.
}

// Validate checks semantic consistency of FaultSpec.
func (f *FaultSpec) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("fault spec ID cannot be empty")
	}
	if f.Filter.Interface == "" {
		return fmt.Errorf("filter interface must be specified")
	}
	if f.Duration <= 0 {
		return fmt.Errorf("fault duration (TTL) must be > 0 for dead-man switch safety")
	}
	if f.Jitter > f.Latency && f.Latency > 0 {
		return fmt.Errorf("jitter (%v) cannot be greater than base latency (%v)", f.Jitter, f.Latency)
	}
	if f.LossPercent < 0 || f.LossPercent > 100 {
		return fmt.Errorf("loss percentage must be between 0 and 100, got %f", f.LossPercent)
	}
	if f.CorruptionPercent < 0 || f.CorruptionPercent > 100 {
		return fmt.Errorf("corruption percentage must be between 0 and 100, got %f", f.CorruptionPercent)
	}
	if f.Correlation < 0 || f.Correlation > 1.0 {
		return fmt.Errorf("correlation must be between 0.0 and 1.0, got %f", f.Correlation)
	}
	if f.DNSFailPercent < 0 || f.DNSFailPercent > 100 {
		return fmt.Errorf("dns_fail_percent must be between 0 and 100, got %f", f.DNSFailPercent)
	}
	if f.CPULoadPercent < 0 || f.CPULoadPercent > 100 {
		return fmt.Errorf("cpu_load_percent must be between 0 and 100, got %d", f.CPULoadPercent)
	}
	return nil
}

// ChaosStatus captures the live status and execution metadata of the chaos driver.
type ChaosStatus struct {
	State        State      `json:"state"`
	CurrentFault *FaultSpec `json:"current_fault,omitempty"`
	AppliedAt    time.Time  `json:"applied_at,omitempty"`
	ExpiresAt    time.Time  `json:"expires_at,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
}
