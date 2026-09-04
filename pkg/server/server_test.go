package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/remediation"
)

func setupTestEnvironment() (*APIHandler, *MemoryRepository, *TestCoordinator, *chaos.MockChaosDriver) {
	repo := NewMemoryRepository()
	eb := NewEventBus()
	mockChaos := chaos.NewMockChaosDriver()
	managedChaos := chaos.NewManagedChaosDriver(mockChaos)

	coord := NewTestCoordinator(repo, eb, CoordinatorConfig{
		MaxConcurrentRuns: 5,
		ChaosDriver:       managedChaos,
	})

	handler := NewAPIHandler(repo, coord, eb)
	return handler, repo, coord, mockChaos
}

// TestAPIRunsLifecycle tests Create Run -> Get Run -> List Runs.
func TestAPIRunsLifecycle(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Mock target API server for the Virtual Users
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer targetServer.Close()

	scenarioYAML := fmt.Sprintf(`
id: api_test_scen
base_url: %s
initial_step_id: step_one
steps:
  step_one:
    id: step_one
    request:
      path: /test
`, targetServer.URL)

	createReq := CreateRunRequest{
		ScenarioYAML: scenarioYAML,
		LoadConfig: loadengine.EngineConfig{
			Profile:  loadengine.ProfileFlatVU,
			VUs:      2,
			Duration: 500 * time.Millisecond,
			CircuitBreaker: loadengine.CircuitBreakerConfig{
				Enabled: false,
			},
		},
	}

	body, _ := json.Marshal(createReq)
	resp, err := http.Post(ts.URL+"/api/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var createdRun TestRun
	if err := json.NewDecoder(resp.Body).Decode(&createdRun); err != nil {
		t.Fatalf("failed to decode created run: %v", err)
	}

	if createdRun.ID == "" {
		t.Fatalf("expected non-empty run ID")
	}

	// Wait for execution to finish
	time.Sleep(800 * time.Millisecond)

	// GET Run details
	getResp, err := http.Get(ts.URL + "/api/v1/runs/" + createdRun.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", getResp.StatusCode)
	}

	var fetchedRun TestRun
	if err := json.NewDecoder(getResp.Body).Decode(&fetchedRun); err != nil {
		t.Fatalf("failed to decode fetched run: %v", err)
	}

	if fetchedRun.Status != RunStatusCompleted {
		t.Errorf("expected status %s, got %s (err: %s)", RunStatusCompleted, fetchedRun.Status, fetchedRun.Error)
	}
	if fetchedRun.Summary == nil || fetchedRun.Summary.TotalRequests == 0 {
		t.Errorf("expected recorded summary with requests")
	}
	if fetchedRun.Diagnostics == nil {
		t.Errorf("expected diagnostics populated on completed run")
	} else if fetchedRun.Diagnostics.HealthScore <= 0 {
		t.Errorf("expected valid health score, got %d", fetchedRun.Diagnostics.HealthScore)
	}

	// Test GET /api/v1/runs/{id}/diagnostics
	diagResp, err := http.Get(ts.URL + "/api/v1/runs/" + createdRun.ID + "/diagnostics")
	if err != nil {
		t.Fatalf("failed to get diagnostics: %v", err)
	}
	defer diagResp.Body.Close()
	if diagResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for diagnostics, got %d", diagResp.StatusCode)
	}
	var diag remediation.DiagnosticReport
	if err := json.NewDecoder(diagResp.Body).Decode(&diag); err != nil {
		t.Fatalf("failed to decode diagnostics response: %v", err)
	}
	if diag.HealthScore <= 0 || diag.StatusLabel == "" {
		t.Errorf("invalid diagnostic response: %+v", diag)
	}

	// GET List Runs
	listResp, err := http.Get(ts.URL + "/api/v1/runs")
	if err != nil {
		t.Fatalf("failed to list runs: %v", err)
	}
	defer listResp.Body.Close()

	var runs []*TestRun
	if err := json.NewDecoder(listResp.Body).Decode(&runs); err != nil {
		t.Fatalf("failed to decode runs list: %v", err)
	}

	if len(runs) != 1 {
		t.Errorf("expected 1 run in history, got %d", len(runs))
	}
}

// TestManualAbort verifies that aborting a run stops load generation and triggers chaos revert.
func TestManualAbort(t *testing.T) {
	handler, _, _, mockChaos := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	scenarioYAML := fmt.Sprintf(`
id: abort_test_scen
base_url: %s
initial_step_id: s1
steps:
  s1:
    id: s1
    request:
      path: /
`, targetServer.URL)

	createReq := CreateRunRequest{
		ScenarioYAML: scenarioYAML,
		LoadConfig: loadengine.EngineConfig{
			Profile:  loadengine.ProfileFlatVU,
			VUs:      5,
			Duration: 10 * time.Second, // Long duration; will abort
		},
		ChaosPlan: ChaosPlan{
			Enabled:       true,
			ScheduleDelay: 0,
			Fault: chaos.FaultSpec{
				ID:       "abort_fault",
				Type:     chaos.FaultLatency,
				Filter:   chaos.FilterConfig{Interface: "lo"},
				Latency:  10 * time.Millisecond,
				Duration: 5 * time.Second,
			},
		},
	}

	client := ts.Client()
	body, _ := json.Marshal(createReq)
	resp, err := client.Post(ts.URL+"/api/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer resp.Body.Close()

	var run TestRun
	_ = json.NewDecoder(resp.Body).Decode(&run)

	// Wait 100ms for run to start
	time.Sleep(100 * time.Millisecond)

	// Send POST /api/v1/runs/{id}/abort
	abortResp, err := client.Post(ts.URL+"/api/v1/runs/"+run.ID+"/abort", "application/json", nil)
	if err != nil {
		t.Fatalf("abort request failed: %v", err)
	}
	defer abortResp.Body.Close()

	if abortResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on abort, got %d", abortResp.StatusCode)
	}

	// Verify status updated to aborted
	time.Sleep(100 * time.Millisecond)
	getResp, err := client.Get(ts.URL + "/api/v1/runs/" + run.ID)
	if err != nil {
		t.Fatalf("get run failed: %v", err)
	}
	defer getResp.Body.Close()
	var abortedRun TestRun
	_ = json.NewDecoder(getResp.Body).Decode(&abortedRun)

	if abortedRun.Status != RunStatusAborted {
		t.Errorf("expected status %s, got %s", RunStatusAborted, abortedRun.Status)
	}

	// Verify Chaos was reverted
	if mockChaos.RevertCount() < 1 {
		t.Errorf("expected chaos to be reverted upon abort")
	}
}

// TestSSEStreaming verifies that Server-Sent Events are streamed to the client during a run.
func TestSSEStreaming(t *testing.T) {
	handler, _, coord, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	scenarioYAML := fmt.Sprintf(`
id: sse_scen
base_url: %s
initial_step_id: s1
steps:
  s1:
    id: s1
    request:
      path: /
`, targetServer.URL)

	run, err := coord.StartRun(context.Background(), CreateRunRequest{
		ScenarioYAML: scenarioYAML,
		LoadConfig: loadengine.EngineConfig{
			Profile:  loadengine.ProfileFlatVU,
			VUs:      2,
			Duration: 2 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("failed to start run: %v", err)
	}

	// Connect to SSE stream
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/runs/"+run.ID+"/stream", nil)
	streamCtx, streamCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer streamCancel()
	req = req.WithContext(streamCtx)

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from SSE, got %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventsReceived int32

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			atomic.AddInt32(&eventsReceived, 1)
			if atomic.LoadInt32(&eventsReceived) >= 1 {
				break
			}
		}
	}

	if atomic.LoadInt32(&eventsReceived) == 0 {
		t.Errorf("expected at least 1 SSE telemetry event, got 0")
	}
}

// TestGenerateScenarioEndpoint verifies on-the-fly OpenAPI scenario synthesis via API.
func TestGenerateScenarioEndpoint(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	openAPISpec := `
openapi: 3.0.0
info:
  title: Micro API
  version: 1.0.0
paths:
  /ping:
    get:
      summary: Ping service
      responses:
        '200':
          description: Pong
`

	genReq := GenerateScenarioRequest{
		OpenAPISpec: openAPISpec,
	}
	body, _ := json.Marshal(genReq)

	resp, err := http.Post(ts.URL+"/api/v1/scenarios/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var sc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&sc); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if sc["steps"] == nil {
		t.Errorf("expected steps in generated scenario")
	}
}

// TestStorageRepositoryConcurrency verifies thread-safety of MemoryRepository under race detector.
func TestStorageRepositoryConcurrency(t *testing.T) {
	repo := NewMemoryRepository()
	const workers = 30
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runID := fmt.Sprintf("run-%d", id)
			_ = repo.Save(context.Background(), &TestRun{ID: runID, Status: RunStatusQueued})
			_, _ = repo.Get(context.Background(), runID)
			_ = repo.Update(context.Background(), &TestRun{ID: runID, Status: RunStatusRunning})
			_, _ = repo.List(context.Background(), 10, 0)
		}(i)
	}

	wg.Wait()
}

// TestNewServerOptionsAndMiddleware verifies CORS, OPTIONS, and middleware handling.
func TestNewServerOptionsAndMiddleware(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	srv := NewServer(ServerConfig{Address: ":0"}, handler)

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	// OPTIONS request for CORS preflight
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/v1/runs", nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("options request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for OPTIONS, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin=*")
	}
}

// TestAPIErrorCases verifies 400 and 404 error responses across handlers.
func TestAPIErrorCases(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Generate Scenario with malformed JSON
	resp, _ := http.Post(ts.URL+"/api/v1/scenarios/generate", "application/json", strings.NewReader(`{invalid_json`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on malformed json, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Generate Scenario with empty spec
	resp, _ = http.Post(ts.URL+"/api/v1/scenarios/generate", "application/json", strings.NewReader(`{"openapi_spec":""}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on empty spec, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Create Run with malformed JSON
	resp, _ = http.Post(ts.URL+"/api/v1/runs", "application/json", strings.NewReader(`bad-json`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on bad create json, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Get Non-existent Run (404)
	resp, _ = http.Get(ts.URL + "/api/v1/runs/non-existent-run-id")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent run, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. Abort Non-existent Run (404)
	resp, _ = http.Post(ts.URL+"/api/v1/runs/non-existent-run-id/abort", "application/json", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 on aborting missing run, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestCircuitBreakerCoordinatedRevert verifies that when the target server fails and trips
// the load engine circuit breaker, the coordinator marks the run as aborted and reverts chaos.
func TestCircuitBreakerCoordinatedRevert(t *testing.T) {
	_, _, coord, mockChaos := setupTestEnvironment()

	// Target server returning 503
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer targetServer.Close()

	scenarioJSON := fmt.Sprintf(`{
		"id": "cb_scen",
		"base_url": "%s",
		"initial_step_id": "s1",
		"steps": {
			"s1": {
				"id": "s1",
				"request": {"path": "/"}
			}
		}
	}`, targetServer.URL)

	run, err := coord.StartRun(context.Background(), CreateRunRequest{
		ScenarioJSON: scenarioJSON,
		LoadConfig: loadengine.EngineConfig{
			Profile:  loadengine.ProfileFlatVU,
			VUs:      5,
			Duration: 5 * time.Second,
			CircuitBreaker: loadengine.CircuitBreakerConfig{
				Enabled:             true,
				EvaluationInterval:  50 * time.Millisecond,
				WindowDuration:      200 * time.Millisecond,
				MinRequests:         5,
				MaxErrorRate:        0.20,
				ConsecutiveBreaches: 1,
			},
		},
		ChaosPlan: ChaosPlan{
			Enabled:       true,
			ScheduleDelay: 0,
			Fault: chaos.FaultSpec{
				ID:       "cb_fault",
				Type:     chaos.FaultLatency,
				Filter:   chaos.FilterConfig{Interface: "lo"},
				Latency:  10 * time.Millisecond,
				Duration: 5 * time.Second,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to start run: %v", err)
	}

	// Wait for circuit breaker to trip and abort run
	time.Sleep(600 * time.Millisecond)

	finalRun, err := coord.repo.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("failed to get final run: %v", err)
	}

	if finalRun.Status != RunStatusAborted {
		t.Errorf("expected status %s, got %s (err: %s)", RunStatusAborted, finalRun.Status, finalRun.Error)
	}

	// Ensure Chaos was reverted
	if mockChaos.RevertCount() < 1 {
		t.Errorf("expected chaos to be reverted when circuit breaker tripped")
	}
}

// TestEventBusClose verifies proper cleanup when EventBus is closed.
func TestEventBusClose(t *testing.T) {
	eb := NewEventBus()
	ch, unsub := eb.Subscribe("run-1", 10)
	defer unsub()

	eb.Close()
	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Errorf("expected channel to be closed after EventBus.Close()")
	}
}

// TestStaticWebDashboardServing verifies that the embedded web frontend is served on root path.
func TestStaticWebDashboardServing(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. GET / (Index HTML)
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("failed to get /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "OSHIMAI") {
		t.Errorf("index.html missing title header, got: %s", string(body))
	}

	// 2. GET /styles.css
	respCss, err := http.Get(ts.URL + "/styles.css")
	if err != nil {
		t.Fatalf("failed to get /styles.css: %v", err)
	}
	respCss.Body.Close()
	if respCss.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /styles.css, got %d", respCss.StatusCode)
	}

	// 3. GET /app.js
	respJs, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("failed to get /app.js: %v", err)
	}
	respJs.Body.Close()
	if respJs.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /app.js, got %d", respJs.StatusCode)
	}
}
