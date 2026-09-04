package chaos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ResourceStressDriver implements ChaosDriver by pressuring CPU, memory, and/or disk I/O on the
// HOST IT RUNS ON — not the network, and not a remote target. It exists for the common real-world
// case where the thing you want to stress-test shares a host with the tester: a local dev machine,
// a same-host sidecar, or an agent deployed onto the target's own container/VM. Think of it as a
// minimal, dependency-free stand-in for `stress-ng`, wired into the same ChaosDriver lifecycle
// (dead-man switch, Apply/Revert) as every other fault in this package.
type ResourceStressDriver struct {
	mu      sync.Mutex
	status  ChaosStatus
	cancel  context.CancelFunc
	tempDir string
}

// NewResourceStressDriver creates an idle stress driver.
func NewResourceStressDriver() *ResourceStressDriver {
	return &ResourceStressDriver{status: ChaosStatus{State: StateIdle}}
}

// Apply starts CPU/memory/disk workers per fault's *LoadPercent/*LoadMB fields, running until
// fault.Duration elapses, Revert is called, or ctx is cancelled — whichever comes first.
func (d *ResourceStressDriver) Apply(ctx context.Context, fault FaultSpec) error {
	if err := fault.Validate(); err != nil {
		return err
	}
	if fault.CPULoadPercent <= 0 && fault.MemoryLoadMB <= 0 && fault.DiskIOLoadMB <= 0 {
		return fmt.Errorf("resource_stress fault requires at least one of cpu_load_percent, memory_load_mb, or disk_io_load_mb")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancel != nil {
		d.cancel() // Idempotently stop any previous stress before starting a new one.
	}

	stressCtx, cancel := context.WithTimeout(context.Background(), fault.Duration)
	d.cancel = cancel

	if fault.CPULoadPercent > 0 {
		for i := 0; i < runtime.NumCPU(); i++ {
			go cpuStressWorker(stressCtx, fault.CPULoadPercent)
		}
	}
	if fault.MemoryLoadMB > 0 {
		go memoryStressWorker(stressCtx, fault.MemoryLoadMB)
	}
	if fault.DiskIOLoadMB > 0 {
		dir, err := os.MkdirTemp("", "oshimai-diskstress-*")
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create disk stress temp dir: %w", err)
		}
		d.tempDir = dir
		go diskStressWorker(stressCtx, dir, fault.DiskIOLoadMB)
	}

	now := time.Now()
	faultCopy := fault
	d.status = ChaosStatus{
		State:        StateInjected,
		CurrentFault: &faultCopy,
		AppliedAt:    now,
		ExpiresAt:    now.Add(fault.Duration),
	}

	go func() {
		<-stressCtx.Done()
		d.mu.Lock()
		if d.status.State == StateInjected && d.status.CurrentFault != nil && d.status.CurrentFault.ID == faultCopy.ID {
			d.status = ChaosStatus{State: StateReverted}
		}
		d.mu.Unlock()
	}()

	return nil
}

// Revert stops all active stress workers immediately and cleans up any temp files.
func (d *ResourceStressDriver) Revert(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	if d.tempDir != "" {
		_ = os.RemoveAll(d.tempDir)
		d.tempDir = ""
	}
	d.status = ChaosStatus{State: StateReverted}
	return nil
}

// Status returns a copy of the live chaos status.
func (d *ResourceStressDriver) Status() ChaosStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}

// cpuStressWorker busy-loops in short duty cycles to approximate percent CPU usage on one core,
// without needing cgroups or any OS-specific throttling API.
func cpuStressWorker(ctx context.Context, percent int) {
	if percent > 100 {
		percent = 100
	}
	const window = 20 * time.Millisecond
	busy := time.Duration(float64(window) * float64(percent) / 100.0)
	idle := window - busy

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		deadline := time.Now().Add(busy)
		for time.Now().Before(deadline) {
			// Deliberately wasted work — the point is to occupy the core, not to compute anything.
		}
		if idle > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(idle):
			}
		}
	}
}

// memoryStressWorker allocates and repeatedly touches sizeMB of memory so the OS actually commits
// real pages (a bare allocation without writes can be lazily backed and never pressure the system).
func memoryStressWorker(ctx context.Context, sizeMB int) {
	const pageStride = 4096
	buf := make([]byte, sizeMB*1024*1024)
	for i := 0; i < len(buf); i += pageStride {
		buf[i] = 1
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			runtime.KeepAlive(buf)
			return
		case <-ticker.C:
			for i := 0; i < len(buf); i += pageStride {
				buf[i]++
			}
		}
	}
}

// diskStressWorker repeatedly writes and deletes a sizeMB file in dir, pressuring disk I/O
// bandwidth without letting unbounded temp data accumulate.
func diskStressWorker(ctx context.Context, dir string, sizeMB int) {
	chunk := make([]byte, 1024*1024) // 1MB chunks
	path := filepath.Join(dir, "stress.bin")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		f, err := os.Create(path)
		if err != nil {
			return
		}
		for i := 0; i < sizeMB; i++ {
			if _, err := f.Write(chunk); err != nil {
				break
			}
		}
		_ = f.Sync()
		f.Close()
		os.Remove(path)
	}
}
