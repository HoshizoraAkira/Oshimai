package presets

import "testing"

func TestListReturnsAllPresets(t *testing.T) {
	list := List()
	if len(list) != 4 {
		t.Fatalf("expected 4 cultural presets, got %d", len(list))
	}
}

func TestFind(t *testing.T) {
	p, ok := Find("harbolnas_1212")
	if !ok {
		t.Fatal("expected to find harbolnas_1212 preset")
	}
	if p.PeakMultiplier <= 1 {
		t.Errorf("expected peak multiplier > 1, got %d", p.PeakMultiplier)
	}

	if _, ok := Find("nonexistent"); ok {
		t.Error("expected Find to fail for unknown preset id")
	}
}

func TestBuildRampingStagesScalesFromBaseline(t *testing.T) {
	p, _ := Find("selebgram_viral")
	stages := p.BuildRampingStages(10)
	if len(stages) == 0 {
		t.Fatal("expected non-empty ramping stages")
	}

	var maxVUs int
	var totalDuration int64
	for _, s := range stages {
		if s.TargetVUs > maxVUs {
			maxVUs = s.TargetVUs
		}
		totalDuration += int64(s.Duration)
		if s.Duration <= 0 {
			t.Errorf("stage duration must be positive, got %v", s.Duration)
		}
	}
	if maxVUs != 10*p.PeakMultiplier {
		t.Errorf("expected peak VUs %d, got %d", 10*p.PeakMultiplier, maxVUs)
	}
}

func TestBuildRampingStagesDefaultsBaseline(t *testing.T) {
	p, _ := Find("gajian_25")
	stages := p.BuildRampingStages(0) // Zero/negative should fall back to a sane default.
	if len(stages) == 0 {
		t.Fatal("expected stages even with zero baseline")
	}
}
