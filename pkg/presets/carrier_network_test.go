package presets

import (
	"testing"
	"time"
)

func TestListCarrierProfiles(t *testing.T) {
	profiles := ListCarrierProfiles()
	if len(profiles) < 5 {
		t.Fatalf("expected at least 5 carrier profiles, got %d", len(profiles))
	}
}

func TestFindCarrierProfile(t *testing.T) {
	p, ok := FindCarrierProfile("telkomsel_4g")
	if !ok {
		t.Fatal("expected to find telkomsel_4g profile")
	}
	if p.Latency <= 0 {
		t.Error("expected a positive baseline latency")
	}
	if _, ok := FindCarrierProfile("nonexistent"); ok {
		t.Error("expected Find to fail for unknown profile")
	}
}

func TestCarrierProfileBuildFault(t *testing.T) {
	p, _ := FindCarrierProfile("rural_edge")
	fault := p.BuildFault(10 * time.Second)
	if err := fault.Validate(); err != nil {
		t.Fatalf("generated fault should be valid, got: %v", err)
	}
	// EDGE should clearly be worse than 4G.
	fourG, _ := FindCarrierProfile("telkomsel_4g")
	if p.Latency <= fourG.Latency || p.LossPercent <= fourG.LossPercent {
		t.Error("expected rural EDGE profile to be strictly worse than Telkomsel 4G")
	}
}
