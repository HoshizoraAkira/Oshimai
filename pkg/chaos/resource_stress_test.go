package chaos

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestResourceStressDriverCPU(t *testing.T) {
	d := NewResourceStressDriver()
	err := d.Apply(context.Background(), FaultSpec{
		ID: "cpu-test", Type: FaultResourceStress, Filter: FilterConfig{Interface: "local"},
		CPULoadPercent: 50, Duration: 150 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if got := d.Status().State; got != StateInjected {
		t.Errorf("expected state injected immediately after Apply, got %s", got)
	}

	time.Sleep(300 * time.Millisecond)
	if got := d.Status().State; got != StateReverted {
		t.Errorf("expected state reverted after duration elapsed, got %s", got)
	}
}

func TestResourceStressDriverMemoryAndDisk(t *testing.T) {
	d := NewResourceStressDriver()
	err := d.Apply(context.Background(), FaultSpec{
		ID: "mem-disk-test", Type: FaultResourceStress, Filter: FilterConfig{Interface: "local"},
		MemoryLoadMB: 2, DiskIOLoadMB: 1, Duration: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	d.mu.Lock()
	tempDir := d.tempDir
	d.mu.Unlock()
	if tempDir == "" {
		t.Fatal("expected a temp dir to be created for disk stress")
	}
	if _, err := os.Stat(tempDir); err != nil {
		t.Errorf("expected temp dir to exist while stress is active: %v", err)
	}

	if err := d.Revert(context.Background()); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // Let the disk worker's current write loop notice cancellation.
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("expected temp dir to be cleaned up after Revert")
	}
}

func TestResourceStressDriverRejectsEmptyFault(t *testing.T) {
	d := NewResourceStressDriver()
	err := d.Apply(context.Background(), FaultSpec{
		ID: "empty-test", Filter: FilterConfig{Interface: "local"}, Duration: time.Second,
	})
	if err == nil {
		t.Error("expected error when no CPU/memory/disk load is specified")
	}
}

func TestResourceStressDriverRevertIdempotent(t *testing.T) {
	d := NewResourceStressDriver()
	if err := d.Revert(context.Background()); err != nil {
		t.Errorf("expected Revert on an idle driver to be a no-op, got error: %v", err)
	}
}
