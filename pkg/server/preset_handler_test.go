package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oshimai/twin/pkg/presets"
)

func TestHandlerCulturalPresetCRUD(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Fresh install: only the 4 built-ins.
	listResp, _ := http.Get(ts.URL + "/api/v1/presets/cultural")
	var list []presets.CulturalPreset
	json.NewDecoder(listResp.Body).Decode(&list)
	listResp.Body.Close()
	if len(list) != 4 {
		t.Fatalf("expected 4 built-in presets, got %d", len(list))
	}

	// Create a custom preset for a non-Indonesian traffic pattern.
	body, _ := json.Marshal(map[string]any{
		"name":                         "Black Friday",
		"description_en":               "A US-style pre-dawn doorbuster spike.",
		"peak_multiplier":              25,
		"suggested_total_duration_sec": 240,
	})
	createResp, err := http.Post(ts.URL+"/api/v1/presets/cultural", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}
	var created presets.CulturalPreset
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	if created.ID == "" || !created.Custom {
		t.Fatalf("expected a non-empty ID and Custom=true, got %+v", created)
	}

	// It should now show up in the list, after the built-ins.
	listResp2, _ := http.Get(ts.URL + "/api/v1/presets/cultural")
	var list2 []presets.CulturalPreset
	json.NewDecoder(listResp2.Body).Decode(&list2)
	listResp2.Body.Close()
	if len(list2) != 5 {
		t.Fatalf("expected 5 presets after creating one, got %d", len(list2))
	}

	// Update it.
	updateBody, _ := json.Marshal(map[string]any{
		"name":                         "Black Friday (Updated)",
		"peak_multiplier":              30,
		"suggested_total_duration_sec": 300,
	})
	updateReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/presets/cultural/"+created.ID, bytes.NewReader(updateBody))
	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from update, got %d", updateResp.StatusCode)
	}
	var updated presets.CulturalPreset
	json.NewDecoder(updateResp.Body).Decode(&updated)
	updateResp.Body.Close()
	if updated.PeakMultiplier != 30 {
		t.Errorf("expected updated peak_multiplier 30, got %d", updated.PeakMultiplier)
	}

	// A built-in preset must reject both update and delete.
	blockedUpdate, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/presets/cultural/harbolnas_1212", bytes.NewReader(updateBody))
	blockedUpdateResp, _ := http.DefaultClient.Do(blockedUpdate)
	if blockedUpdateResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when updating a built-in preset, got %d", blockedUpdateResp.StatusCode)
	}
	blockedUpdateResp.Body.Close()

	blockedDelete, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/presets/cultural/harbolnas_1212", nil)
	blockedDeleteResp, _ := http.DefaultClient.Do(blockedDelete)
	if blockedDeleteResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when deleting a built-in preset, got %d", blockedDeleteResp.StatusCode)
	}
	blockedDeleteResp.Body.Close()

	// Delete the custom one.
	deleteReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/presets/cultural/"+created.ID, nil)
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from delete, got %d", deleteResp.StatusCode)
	}
	deleteResp.Body.Close()

	listResp3, _ := http.Get(ts.URL + "/api/v1/presets/cultural")
	var list3 []presets.CulturalPreset
	json.NewDecoder(listResp3.Body).Decode(&list3)
	listResp3.Body.Close()
	if len(list3) != 4 {
		t.Fatalf("expected back to 4 presets after delete, got %d", len(list3))
	}
}

func TestHandlerCreateCulturalPresetValidation(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{"name": "", "peak_multiplier": 5, "suggested_total_duration_sec": 100})
	resp, err := http.Post(ts.URL+"/api/v1/presets/cultural", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for a preset with no name, got %d", resp.StatusCode)
	}
}

func TestHandlerGetCulturalPresetStagesUsesCustomShape(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(presets.CulturalPreset{
		Name: "Custom Shape", PeakMultiplier: 4, SuggestedTotalS: 100,
		Shape: []presets.ShapeStage{{DurationPercent: 100, VUMultiplier: 4}},
	})
	createResp, _ := http.Post(ts.URL+"/api/v1/presets/cultural", "application/json", bytes.NewReader(body))
	var created presets.CulturalPreset
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	stagesResp, err := http.Get(ts.URL + "/api/v1/presets/cultural/" + created.ID + "?baseline_vus=10")
	if err != nil {
		t.Fatalf("stages request failed: %v", err)
	}
	defer stagesResp.Body.Close()
	if stagesResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", stagesResp.StatusCode)
	}
	var out struct {
		Stages []struct {
			TargetVUs int `json:"target_vus"`
		} `json:"stages"`
	}
	json.NewDecoder(stagesResp.Body).Decode(&out)
	if len(out.Stages) != 1 || out.Stages[0].TargetVUs != 40 {
		t.Errorf("expected the custom shape's single 40-VU stage, got %+v", out.Stages)
	}
}
