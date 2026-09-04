package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

// simulateAgent behaves like cmd/agent's main loop, but in-process: register, poll once, execute
// with a trivial fixed summary (skipping a real loadengine run keeps this test fast and avoids
// spinning up yet another httptest target on top of the ones already in this package's suite).
func simulateAgent(t *testing.T, serverURL, agentID, region string, fakeSummary *loadengine.ExecutionSummary) {
	t.Helper()
	regBody, _ := json.Marshal(map[string]string{"id": agentID, "region": region})
	resp, err := http.Post(serverURL+"/api/v1/agents/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("agent %s register failed: %v", agentID, err)
	}
	resp.Body.Close()

	go func() {
		pollResp, err := http.Get(serverURL + "/api/v1/agents/" + agentID + "/poll")
		if err != nil {
			return
		}
		defer pollResp.Body.Close()
		if pollResp.StatusCode != http.StatusOK {
			return
		}
		var a AgentAssignment
		json.NewDecoder(pollResp.Body).Decode(&a)

		resultBody, _ := json.Marshal(AgentResult{RunID: a.RunID, AgentID: agentID, Summary: fakeSummary})
		http.Post(serverURL+"/api/v1/agents/"+agentID+"/results", "application/json", bytes.NewReader(resultBody))
	}()
}

func TestMultiRegionRunEndToEnd(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	simulateAgent(t, ts.URL, "agent-jkt", "jakarta", &loadengine.ExecutionSummary{
		TotalRequests: 100, TotalErrors: 1, TotalDuration: 5 * time.Second,
		StatusCodes: map[int]int64{200: 99, 500: 1},
		Latency:     loadengine.LatencyStats{Min: 10 * time.Millisecond, Mean: 40 * time.Millisecond, P99: 90 * time.Millisecond, Max: 120 * time.Millisecond},
	})
	simulateAgent(t, ts.URL, "agent-sg", "singapore", &loadengine.ExecutionSummary{
		TotalRequests: 200, TotalErrors: 0, TotalDuration: 5 * time.Second,
		StatusCodes: map[int]int64{200: 200},
		Latency:     loadengine.LatencyStats{Min: 5 * time.Millisecond, Mean: 20 * time.Millisecond, P99: 60 * time.Millisecond, Max: 80 * time.Millisecond},
	})

	// Give both agents a moment to finish registering before dispatch.
	time.Sleep(100 * time.Millisecond)

	reqBody := MultiRegionRequest{
		Base: CreateRunRequest{
			ScenarioYAML: `
id: multiregion_test
base_url: http://127.0.0.1:1
initial_step_id: ping
steps:
  ping:
    id: ping
    request:
      path: /ping
`,
			LoadConfig: loadengine.EngineConfig{Profile: loadengine.ProfileFlatVU, VUs: 20, Duration: 5 * time.Second},
		},
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/api/v1/runs/multiregion", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("multiregion request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var errBody map[string]string
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, errBody)
	}

	var result MultiRegionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if result.AgentsUsed != 2 {
		t.Errorf("expected 2 agents used, got %d", result.AgentsUsed)
	}
	if result.Merged.TotalRequests != 300 {
		t.Errorf("expected 300 merged total requests, got %d", result.Merged.TotalRequests)
	}
	if len(result.PerAgentRegion) != 2 {
		t.Errorf("expected per-agent region breakdown for both agents, got %+v", result.PerAgentRegion)
	}
}
