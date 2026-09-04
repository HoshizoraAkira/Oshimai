package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oshimai/twin/pkg/loadengine"
)

func newSearchTestServer(t *testing.T) (string, string) {
	t.Helper()
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(target.Close)

	return ts.URL, target.URL
}

func TestHandlerAutoPilot(t *testing.T) {
	serverURL, targetURL := newSearchTestServer(t)

	scenarioYAML := fmt.Sprintf(`
id: autopilot_test
base_url: %s
initial_step_id: ping
steps:
  ping:
    id: ping
    request:
      path: /health
`, targetURL)

	reqBody := AutoPilotRequest{
		Base: CreateRunRequest{
			ScenarioYAML: scenarioYAML,
			LoadConfig:   loadengine.EngineConfig{Profile: loadengine.ProfileFlatVU, VUs: 2, Duration: 200_000_000},
		},
		MinVUs: 1, MaxVUs: 20, TrialDurationSeconds: 1, HealthThreshold: 70, MaxIterations: 2,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(serverURL+"/api/v1/autopilot", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("autopilot request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		SafeMaxVUs int    `json:"safe_max_vus"`
		Verdict    string `json:"verdict"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Verdict == "" {
		t.Error("expected a non-empty verdict")
	}
}

func TestHandlerAutoFuzz(t *testing.T) {
	serverURL, targetURL := newSearchTestServer(t)

	scenarioYAML := fmt.Sprintf(`
id: autofuzz_test
base_url: %s
initial_step_id: ping
steps:
  ping:
    id: ping
    request:
      path: /health
`, targetURL)

	reqBody := AutoFuzzRequest{
		Base: CreateRunRequest{
			ScenarioYAML: scenarioYAML,
			LoadConfig:   loadengine.EngineConfig{Profile: loadengine.ProfileFlatVU, VUs: 2, Duration: 200_000_000},
		},
		TrialDurationSeconds: 1, HealthThreshold: 60, MaxIterations: 2,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(serverURL+"/api/v1/autofuzz", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("autofuzz request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Verdict string `json:"verdict"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Verdict == "" {
		t.Error("expected a non-empty verdict")
	}
}

func TestHandlerAutoFuzzRejectsBlockedTarget(t *testing.T) {
	// A handler with verification enforced, to exercise the safety gate through this path too.
	repo := NewMemoryRepository()
	eb := NewEventBus()
	coord := NewTestCoordinator(repo, eb, CoordinatorConfig{MaxConcurrentRuns: 5, EnforceTargetVerification: true})
	guardedHandler := NewAPIHandler(repo, coord, eb)

	mux := http.NewServeMux()
	guardedHandler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	reqBody := AutoFuzzRequest{
		Base: CreateRunRequest{
			ScenarioYAML: `
id: blocked_test
base_url: https://google.com
initial_step_id: ping
steps:
  ping:
    id: ping
    request:
      path: /
`,
			LoadConfig: loadengine.EngineConfig{Profile: loadengine.ProfileFlatVU, VUs: 2, Duration: 200_000_000},
		},
	}
	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL+"/api/v1/autofuzz", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for blocklisted target, got %d", resp.StatusCode)
	}
}

func TestHandlerDependencyGraph(t *testing.T) {
	serverURL, _ := newSearchTestServer(t)

	traces := `[
		{"trace_id":"t1","span_id":"s1","name":"GET /login","method":"GET","route":"/login","start_time":"2026-01-01T00:00:00Z","end_time":"2026-01-01T00:00:01Z"},
		{"trace_id":"t1","span_id":"s2","name":"GET /browse","method":"GET","route":"/browse","start_time":"2026-01-01T00:00:02Z","end_time":"2026-01-01T00:00:03Z"},
		{"trace_id":"t1","span_id":"s3","name":"POST /checkout","method":"POST","route":"/checkout","start_time":"2026-01-01T00:00:04Z","end_time":"2026-01-01T00:00:05Z"}
	]`
	body, _ := json.Marshal(map[string]string{"otel_traces": traces})

	resp, err := http.Post(serverURL+"/api/v1/traces/dependency-graph", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("dependency-graph request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var graph struct {
		Nodes        []map[string]any `json:"nodes"`
		MostCritical []string         `json:"most_critical"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&graph); err != nil {
		t.Fatalf("failed to decode graph: %v", err)
	}
	if len(graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.Nodes))
	}
}
