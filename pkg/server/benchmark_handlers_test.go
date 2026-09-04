package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

func TestHandlerBenchmarkSubmitAndCompare(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]any{"category": "ecommerce", "health_score": 60, "p99_ms": 500})
		resp, err := http.Post(ts.URL+"/api/v1/benchmark/submit", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("submit request failed: %v", err)
		}
		resp.Body.Close()
	}

	compareBody, _ := json.Marshal(map[string]any{"category": "ecommerce", "health_score": 95, "p99_ms": 50})
	resp, err := http.Post(ts.URL+"/api/v1/benchmark/compare", "application/json", bytes.NewReader(compareBody))
	if err != nil {
		t.Fatalf("compare request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var report map[string]any
	json.NewDecoder(resp.Body).Decode(&report)
	if report["sample_count"].(float64) != 3 {
		t.Errorf("expected 3 samples in the pool, got %v", report["sample_count"])
	}
	if report["health_score_percentile"].(float64) != 100 {
		t.Errorf("expected a run beating all 3 samples to rank at the 100th percentile, got %v", report["health_score_percentile"])
	}
}

func TestRunAutoSubmitsToBenchmarkOnOptIn(t *testing.T) {
	handler, _, coord, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	scenarioYAML := fmt.Sprintf(`
id: benchmark_optin_test
base_url: %s
initial_step_id: ping
steps:
  ping:
    id: ping
    request:
      path: /ping
`, target.URL)

	createReq := CreateRunRequest{
		ScenarioYAML:      scenarioYAML,
		BenchmarkCategory: "saas_b2b",
		LoadConfig:        loadengine.EngineConfig{Profile: loadengine.ProfileFlatVU, VUs: 2, Duration: 200 * time.Millisecond},
	}
	body, _ := json.Marshal(createReq)
	resp, err := http.Post(ts.URL+"/api/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}
	resp.Body.Close()

	// Poll Compare (read-only, doesn't mutate the pool) until the run's own auto-submission shows
	// up as a sample, or time out.
	deadline := time.Now().Add(3 * time.Second)
	var landed bool
	for time.Now().Before(deadline) {
		probe := coord.Benchmark().Compare("saas_b2b", 1, 1)
		if probe.SampleCount >= 1 {
			landed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !landed {
		t.Error("expected the completed run to auto-submit an anonymized result to the benchmark pool")
	}
}
