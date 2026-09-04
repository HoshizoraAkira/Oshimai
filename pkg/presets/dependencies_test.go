package presets

import (
	"testing"
	"time"
)

func TestListDependencies(t *testing.T) {
	deps := ListDependencies()
	if len(deps) == 0 {
		t.Fatal("expected at least one dependency preset")
	}
}

func TestFindDependency(t *testing.T) {
	d, ok := FindDependency("midtrans")
	if !ok {
		t.Fatal("expected to find midtrans preset")
	}
	if len(d.Domains) == 0 {
		t.Error("expected midtrans preset to declare domains")
	}
	if _, ok := FindDependency("nonexistent"); ok {
		t.Error("expected Find to fail for unknown dependency")
	}
}

func TestBuildOutageFaultScopesToDomainsOnly(t *testing.T) {
	d, _ := FindDependency("xendit")
	fault := d.BuildOutageFault(30 * time.Second)
	if err := fault.Validate(); err != nil {
		t.Fatalf("generated fault should be valid, got: %v", err)
	}
	if len(fault.Filter.TargetDomains) == 0 {
		t.Error("expected outage fault to be domain-scoped")
	}
	if fault.LossPercent <= 0 || fault.Latency <= 0 {
		t.Error("expected a meaningful degraded fault, not a no-op")
	}
}
