package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

// startTestServerWithCompletedRun spins up the full HTTP surface and drives one real run to
// completion against a local mock target, returning the handler's base URL and the completed
// run's ID so individual endpoint tests can exercise reporting/sharing/trend features against it.
func startTestServerWithCompletedRun(t *testing.T) (baseURL, runID string) {
	t.Helper()
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(target.Close)

	scenarioYAML := fmt.Sprintf(`
id: endpoint_test_scen
base_url: %s
initial_step_id: step_one
steps:
  step_one:
    id: step_one
    request:
      path: /test
`, target.URL)

	createReq := CreateRunRequest{
		ScenarioYAML: scenarioYAML,
		LoadConfig: loadengine.EngineConfig{
			Profile:  loadengine.ProfileFlatVU,
			VUs:      1,
			Duration: 300 * time.Millisecond,
		},
	}
	body, _ := json.Marshal(createReq)
	resp, err := http.Post(ts.URL+"/api/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}
	var created TestRun
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := http.Get(ts.URL + "/api/v1/runs/" + created.ID)
		if err == nil {
			var run TestRun
			json.NewDecoder(r.Body).Decode(&run)
			r.Body.Close()
			if run.Status == RunStatusCompleted {
				return ts.URL, created.ID
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("run %s did not complete in time", created.ID)
	return "", ""
}

func TestHandlerBadgeEndpoint(t *testing.T) {
	baseURL, runID := startTestServerWithCompletedRun(t)
	resp, err := http.Get(baseURL + "/api/v1/runs/" + runID + "/badge.svg")
	if err != nil {
		t.Fatalf("badge request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("expected image/svg+xml content type, got %s", ct)
	}
}

func TestHandlerExecutiveReportAndAuthorizationLetter(t *testing.T) {
	baseURL, runID := startTestServerWithCompletedRun(t)

	for _, path := range []string{"/report", "/authorization-letter"} {
		resp, err := http.Get(baseURL + "/api/v1/runs/" + runID + path)
		if err != nil {
			t.Fatalf("%s request failed: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestHandlerShareLinkAndPublicRun(t *testing.T) {
	baseURL, runID := startTestServerWithCompletedRun(t)

	resp, err := http.Post(baseURL+"/api/v1/runs/"+runID+"/share", "application/json", nil)
	if err != nil {
		t.Fatalf("share request failed: %v", err)
	}
	var shareResp struct {
		ShareToken string `json:"share_token"`
	}
	json.NewDecoder(resp.Body).Decode(&shareResp)
	resp.Body.Close()
	if shareResp.ShareToken == "" {
		t.Fatal("expected a non-empty share token")
	}

	pubResp, err := http.Get(baseURL + "/api/v1/public/runs/" + shareResp.ShareToken)
	if err != nil {
		t.Fatalf("public run request failed: %v", err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for public run view, got %d", pubResp.StatusCode)
	}

	if _, err := http.Get(baseURL + "/api/v1/public/runs/not-a-real-token"); err != nil {
		t.Fatalf("request failed: %v", err)
	}
}

func TestHandlerVerifyFlow(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	statusResp, err := http.Get(ts.URL + "/api/v1/verify/status?target_url=https://my-shop.example")
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	var status TargetStatus
	json.NewDecoder(statusResp.Body).Decode(&status)
	statusResp.Body.Close()
	if status.IsVerified {
		t.Error("expected a fresh target to be unverified")
	}

	challengeBody, _ := json.Marshal(map[string]string{"target_url": "https://my-shop.example"})
	chResp, err := http.Post(ts.URL+"/api/v1/verify/challenge", "application/json", bytes.NewReader(challengeBody))
	if err != nil {
		t.Fatalf("challenge request failed: %v", err)
	}
	if chResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from challenge, got %d", chResp.StatusCode)
	}
	chResp.Body.Close()

	blockedBody, _ := json.Marshal(map[string]string{"target_url": "https://google.com"})
	blockedResp, err := http.Post(ts.URL+"/api/v1/verify/challenge", "application/json", bytes.NewReader(blockedBody))
	if err != nil {
		t.Fatalf("blocked challenge request failed: %v", err)
	}
	if blockedResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for blocklisted domain, got %d", blockedResp.StatusCode)
	}
	blockedResp.Body.Close()
}

func TestHandlerCulturalPresets(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/presets/cultural")
	if err != nil {
		t.Fatalf("presets request failed: %v", err)
	}
	defer resp.Body.Close()
	var list []map[string]any
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 4 {
		t.Errorf("expected 4 cultural presets, got %d", len(list))
	}
}

func TestHandlerImportHARAndPostmanAndAutoDiscoverAndFuzz(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte(`<html><body><a href="/pricing">Pricing</a></body></html>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	harBody, _ := json.Marshal(map[string]string{
		"har": fmt.Sprintf(`{"log":{"entries":[{"request":{"method":"GET","url":"%s/items","headers":[]}}]}}`, target.URL),
	})
	harResp, err := http.Post(ts.URL+"/api/v1/scenarios/import/har", "application/json", bytes.NewReader(harBody))
	if err != nil {
		t.Fatalf("HAR import request failed: %v", err)
	}
	if harResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from HAR import, got %d", harResp.StatusCode)
	}
	harResp.Body.Close()

	postmanBody, _ := json.Marshal(map[string]string{
		"collection": fmt.Sprintf(`{"info":{"name":"t"},"item":[{"name":"list","request":{"method":"GET","url":"%s/items"}}]}`, target.URL),
	})
	pmResp, err := http.Post(ts.URL+"/api/v1/scenarios/import/postman", "application/json", bytes.NewReader(postmanBody))
	if err != nil {
		t.Fatalf("Postman import request failed: %v", err)
	}
	if pmResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from Postman import, got %d", pmResp.StatusCode)
	}
	pmResp.Body.Close()

	autoBody, _ := json.Marshal(map[string]string{"target_url": target.URL})
	autoResp, err := http.Post(ts.URL+"/api/v1/scenarios/autodiscover", "application/json", bytes.NewReader(autoBody))
	if err != nil {
		t.Fatalf("autodiscover request failed: %v", err)
	}
	if autoResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from autodiscover, got %d", autoResp.StatusCode)
	}
	autoResp.Body.Close()

	fuzzBody, _ := json.Marshal(map[string]string{
		"scenario_yaml": fmt.Sprintf(`
id: fuzz_test
base_url: %s
initial_step_id: create
steps:
  create:
    id: create
    request:
      method: POST
      path: /orders
      body: '{"item_id": 5}'
`, target.URL),
	})
	fuzzResp, err := http.Post(ts.URL+"/api/v1/scenarios/fuzz", "application/json", bytes.NewReader(fuzzBody))
	if err != nil {
		t.Fatalf("fuzz request failed: %v", err)
	}
	if fuzzResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from fuzz builder, got %d", fuzzResp.StatusCode)
	}
	fuzzResp.Body.Close()
}

func TestHandlerImportInsomniaAndJMeter(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	insomniaBody, _ := json.Marshal(map[string]string{
		"export": fmt.Sprintf(`{"resources":[{"_id":"r1","_type":"request","name":"list","method":"GET","url":"%s/items","metaSortKey":1}]}`, target.URL),
	})
	insResp, err := http.Post(ts.URL+"/api/v1/scenarios/import/insomnia", "application/json", bytes.NewReader(insomniaBody))
	if err != nil {
		t.Fatalf("Insomnia import request failed: %v", err)
	}
	if insResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from Insomnia import, got %d", insResp.StatusCode)
	}
	insResp.Body.Close()

	targetURLParsed, _ := url.Parse(target.URL)
	jmxBody, _ := json.Marshal(map[string]string{
		"jmx": fmt.Sprintf(`<jmeterTestPlan><hashTree><HTTPSamplerProxy testname="ping"><stringProp name="HTTPSampler.domain">%s</stringProp><stringProp name="HTTPSampler.protocol">http</stringProp><stringProp name="HTTPSampler.path">/ping</stringProp><stringProp name="HTTPSampler.method">GET</stringProp></HTTPSamplerProxy></hashTree></jmeterTestPlan>`, targetURLParsed.Host),
	})
	jmxResp, err := http.Post(ts.URL+"/api/v1/scenarios/import/jmeter", "application/json", bytes.NewReader(jmxBody))
	if err != nil {
		t.Fatalf("JMeter import request failed: %v", err)
	}
	if jmxResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from JMeter import, got %d", jmxResp.StatusCode)
	}
	jmxResp.Body.Close()
}

type fakeNLLLMClient struct{ response string }

func (f *fakeNLLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return f.response, nil
}

func TestHandlerGenerateScenarioNL(t *testing.T) {
	const fakeYAML = "id: x\nname: X\ninitial_step_id: a\nsteps:\n  a:\n    id: a\n    request:\n      method: GET\n      path: /ping\n    transitions:\n      - target_step_id: \"END\"\n        probability: 1.0\n"

	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Without an LLM client configured, the endpoint must fail clearly, not silently no-op.
	body, _ := json.Marshal(map[string]string{"description": "toko online sederhana"})
	resp, err := http.Post(ts.URL+"/api/v1/scenarios/generate-nl", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 when no LLM client is configured, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	handler.SetLLMClient(&fakeNLLLMClient{response: fakeYAML})

	// 2. Old 'description' field (API convention) must still work.
	body2, _ := json.Marshal(map[string]string{"description": "toko online sederhana"})
	resp2, err := http.Post(ts.URL+"/api/v1/scenarios/generate-nl", "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatalf("request (description field) failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for 'description' field, got %d", resp2.StatusCode)
	}
	var out2 map[string]any
	json.NewDecoder(resp2.Body).Decode(&out2)
	if _, ok := out2["scenario_yaml"]; !ok {
		t.Error("expected 'scenario_yaml' key in response")
	}
	if _, ok := out2["scenario"]; !ok {
		t.Error("expected 'scenario' key in response")
	}

	// 3. New 'prompt' field (frontend convention) must also work.
	body3, _ := json.Marshal(map[string]string{"prompt": "aplikasi e-commerce dengan login"})
	resp3, err := http.Post(ts.URL+"/api/v1/scenarios/generate-nl", "application/json", bytes.NewReader(body3))
	if err != nil {
		t.Fatalf("request (prompt field) failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for 'prompt' field, got %d", resp3.StatusCode)
	}
	var out3 map[string]any
	json.NewDecoder(resp3.Body).Decode(&out3)
	if _, ok := out3["scenario_yaml"]; !ok {
		t.Error("expected 'scenario_yaml' key in response when using 'prompt' field")
	}
}


func TestHandlerRunTrend(t *testing.T) {
	baseURL, runID := startTestServerWithCompletedRun(t)

	// No baseline exists yet (this is the only completed run) — expect 422, not a crash.
	resp, err := http.Get(baseURL + "/api/v1/runs/" + runID + "/trend")
	if err != nil {
		t.Fatalf("trend request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 with no baseline available, got %d", resp.StatusCode)
	}
}
