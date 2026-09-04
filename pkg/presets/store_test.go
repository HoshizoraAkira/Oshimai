package presets

import "testing"

func TestStoreListIncludesBuiltinsAndCustom(t *testing.T) {
	s := NewStore()
	if got := len(s.List()); got != 4 {
		t.Fatalf("expected 4 built-in presets before any custom preset, got %d", got)
	}

	created, err := s.Create(CulturalPreset{Name: "Black Friday", PeakMultiplier: 15, SuggestedTotalS: 300})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !created.Custom {
		t.Error("expected created preset to be marked Custom")
	}

	list := s.List()
	if len(list) != 5 {
		t.Fatalf("expected 5 presets after one custom preset, got %d", len(list))
	}
	if list[4].ID != created.ID {
		t.Errorf("expected custom preset to be listed after all built-ins, got %+v", list[4])
	}
}

func TestStoreCreateDerivesIDFromName(t *testing.T) {
	s := NewStore()
	p, err := s.Create(CulturalPreset{Name: "Black Friday Rush!", PeakMultiplier: 10, SuggestedTotalS: 120})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if p.ID != "black_friday_rush" {
		t.Errorf("expected derived ID %q, got %q", "black_friday_rush", p.ID)
	}
}

func TestStoreCreateDeduplicatesID(t *testing.T) {
	s := NewStore()
	p1, err := s.Create(CulturalPreset{Name: "Boxing Day", PeakMultiplier: 8, SuggestedTotalS: 200})
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	p2, err := s.Create(CulturalPreset{Name: "Boxing Day", PeakMultiplier: 9, SuggestedTotalS: 200})
	if err != nil {
		t.Fatalf("second Create failed: %v", err)
	}
	if p1.ID == p2.ID {
		t.Errorf("expected distinct IDs for two presets with the same name, both got %q", p1.ID)
	}
}

func TestStoreCreateRejectsBuiltinIDCollision(t *testing.T) {
	s := NewStore()
	p, err := s.Create(CulturalPreset{ID: "harbolnas_1212", Name: "Whatever", PeakMultiplier: 5, SuggestedTotalS: 100})
	if err != nil {
		t.Fatalf("Create should fall back to a derived ID instead of erroring, got: %v", err)
	}
	if p.ID == "harbolnas_1212" {
		t.Error("expected a custom preset to never be assigned a built-in preset's ID")
	}
}

func TestStoreCreateValidation(t *testing.T) {
	s := NewStore()
	cases := []CulturalPreset{
		{Name: "", PeakMultiplier: 5, SuggestedTotalS: 100},
		{Name: "No Peak", PeakMultiplier: 0, SuggestedTotalS: 100},
		{Name: "No Duration", PeakMultiplier: 5, SuggestedTotalS: 0},
		{Name: "Bad Shape", PeakMultiplier: 5, SuggestedTotalS: 100, Shape: []ShapeStage{{DurationPercent: 50, VUMultiplier: 1}}},
	}
	for _, c := range cases {
		if _, err := s.Create(c); err == nil {
			t.Errorf("expected Create to reject invalid preset %+v", c)
		}
	}
}

func TestStoreUpdateOnlyAffectsCustomPresets(t *testing.T) {
	s := NewStore()

	if _, err := s.Update("harbolnas_1212", CulturalPreset{Name: "Hacked", PeakMultiplier: 1, SuggestedTotalS: 1}); err == nil {
		t.Error("expected Update to refuse to modify a built-in preset")
	}

	created, err := s.Create(CulturalPreset{Name: "Ramadan Iftar Rush", PeakMultiplier: 7, SuggestedTotalS: 150})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := s.Update(created.ID, CulturalPreset{Name: "Ramadan Iftar Rush v2", PeakMultiplier: 9, SuggestedTotalS: 180})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("expected Update to preserve the original ID, got %q", updated.ID)
	}
	if updated.PeakMultiplier != 9 {
		t.Errorf("expected updated peak_multiplier 9, got %d", updated.PeakMultiplier)
	}

	if _, err := s.Update("does-not-exist", CulturalPreset{Name: "X", PeakMultiplier: 1, SuggestedTotalS: 1}); err == nil {
		t.Error("expected Update to fail for an unknown preset id")
	}
}

func TestStoreDeleteOnlyAffectsCustomPresets(t *testing.T) {
	s := NewStore()

	if err := s.Delete("gajian_25"); err == nil {
		t.Error("expected Delete to refuse to remove a built-in preset")
	}

	created, err := s.Create(CulturalPreset{Name: "Singles Day 11.11", PeakMultiplier: 14, SuggestedTotalS: 240})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := s.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, ok := s.Find(created.ID); ok {
		t.Error("expected preset to be gone after Delete")
	}
	if err := s.Delete(created.ID); err == nil {
		t.Error("expected a second Delete of the same id to fail")
	}
}

func TestStoreFindChecksBothBuiltinAndCustom(t *testing.T) {
	s := NewStore()
	if _, ok := s.Find("harbolnas_1212"); !ok {
		t.Error("expected Find to see built-in presets through the Store")
	}

	created, _ := s.Create(CulturalPreset{Name: "Diwali Shopping", PeakMultiplier: 11, SuggestedTotalS: 200})
	if _, ok := s.Find(created.ID); !ok {
		t.Error("expected Find to see a just-created custom preset")
	}
}

func TestCustomPresetWithExplicitShapeBuildsMatchingStages(t *testing.T) {
	p := CulturalPreset{
		ID: "custom_twin_peak", Name: "Custom Twin Peak", PeakMultiplier: 4, SuggestedTotalS: 100,
		Shape: []ShapeStage{
			{DurationPercent: 50, VUMultiplier: 4},
			{DurationPercent: 50, VUMultiplier: 1},
		},
	}
	stages := p.BuildRampingStages(10)
	if len(stages) != 2 {
		t.Fatalf("expected exactly 2 stages matching the explicit shape, got %d", len(stages))
	}
	if stages[0].TargetVUs != 40 {
		t.Errorf("expected first stage at 40 VUs (10 baseline * 4x), got %d", stages[0].TargetVUs)
	}
	if stages[1].TargetVUs != 10 {
		t.Errorf("expected second stage at 10 VUs (10 baseline * 1x), got %d", stages[1].TargetVUs)
	}
}
