package generator

import (
	"encoding/json"
	"testing"

	"github.com/oshimai/twin/pkg/vusession"
)

func TestFuzzVariants(t *testing.T) {
	variants := FuzzVariants(`{"quantity": 2, "note": "hello"}`)
	if len(variants) < 3 {
		t.Fatalf("expected at least 3 fuzz variants, got %d", len(variants))
	}
	for _, v := range variants {
		if v.Name == "empty_object" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v.Body), &decoded); err != nil {
			t.Errorf("variant %q produced invalid JSON: %v", v.Name, err)
		}
	}
}

func TestFuzzVariantsIgnoresNonObjectPayload(t *testing.T) {
	if v := FuzzVariants(`not json`); v != nil {
		t.Errorf("expected nil for unparseable payload, got %v", v)
	}
	if v := FuzzVariants(`{}`); v != nil {
		t.Errorf("expected nil for empty object payload, got %v", v)
	}
}

func TestBuildFuzzScenario(t *testing.T) {
	base := &vusession.Scenario{
		ID:            "checkout",
		Name:          "Checkout Flow",
		BaseURL:       "https://shop.example.co.id",
		InitialStepID: "create_order",
		Steps: map[string]*vusession.Step{
			"create_order": {
				ID:      "create_order",
				Request: vusession.RequestConfig{Method: "POST", Path: "/orders", Body: `{"item_id": 5, "quantity": 1}`},
			},
		},
	}

	fuzzSc, err := BuildFuzzScenario(base)
	if err != nil {
		t.Fatalf("BuildFuzzScenario failed: %v", err)
	}
	if len(fuzzSc.Steps) == 0 {
		t.Fatal("expected generated fuzz steps")
	}
	for _, step := range fuzzSc.Steps {
		if len(step.Assertions) != 1 || step.Assertions[0].MaxCode != 499 {
			t.Errorf("expected every fuzz step to assert non-5xx, got %+v", step.Assertions)
		}
	}
}

func TestBuildFuzzScenarioRejectsNoBodySteps(t *testing.T) {
	base := &vusession.Scenario{
		ID:            "readonly",
		InitialStepID: "list",
		Steps: map[string]*vusession.Step{
			"list": {ID: "list", Request: vusession.RequestConfig{Method: "GET", Path: "/items"}},
		},
	}
	if _, err := BuildFuzzScenario(base); err == nil {
		t.Error("expected error when no POST/PUT/PATCH body steps exist to fuzz")
	}
}
