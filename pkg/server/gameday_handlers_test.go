package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oshimai/twin/pkg/loadengine"
)

func TestHandlerGameDaySchedulesCRUD(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	createBody, _ := json.Marshal(map[string]any{
		"name": "Weekly Checkout GameDay", "weekday": 1, "hour_utc": 9, "minute_utc": 0,
		"run": CreateRunRequest{
			ScenarioYAML: "id: x\nbase_url: http://127.0.0.1:1\ninitial_step_id: a\nsteps:\n  a:\n    id: a\n    request:\n      path: /\n",
			LoadConfig:   loadengine.EngineConfig{Profile: loadengine.ProfileFlatVU, VUs: 5, Duration: 1e9},
		},
	})
	resp, err := http.Post(ts.URL+"/api/v1/gameday/schedules", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]string
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	id := created["id"]
	if id == "" {
		t.Fatal("expected a non-empty schedule ID")
	}

	listResp, err := http.Get(ts.URL + "/api/v1/gameday/schedules")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	var list []map[string]any
	json.NewDecoder(listResp.Body).Decode(&list)
	listResp.Body.Close()
	if len(list) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(list))
	}

	toggleBody, _ := json.Marshal(map[string]bool{"enabled": false})
	toggleResp, err := http.Post(ts.URL+"/api/v1/gameday/schedules/"+id+"/toggle", "application/json", bytes.NewReader(toggleBody))
	if err != nil {
		t.Fatalf("toggle request failed: %v", err)
	}
	if toggleResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from toggle, got %d", toggleResp.StatusCode)
	}
	toggleResp.Body.Close()

	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/gameday/schedules/"+id, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	if delResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from delete, got %d", delResp.StatusCode)
	}
	delResp.Body.Close()

	finalListResp, _ := http.Get(ts.URL + "/api/v1/gameday/schedules")
	var finalList []map[string]any
	json.NewDecoder(finalListResp.Body).Decode(&finalList)
	finalListResp.Body.Close()
	if len(finalList) != 0 {
		t.Errorf("expected 0 schedules after deletion, got %d", len(finalList))
	}
}

func TestHandlerGameDayScheduleRejectsInvalidTime(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{"name": "bad", "weekday": 1, "hour_utc": 99, "minute_utc": 0})
	resp, err := http.Post(ts.URL+"/api/v1/gameday/schedules", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for an invalid hour, got %d", resp.StatusCode)
	}
}
