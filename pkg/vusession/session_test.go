package vusession

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestMultiStepStatePassing verifies that dynamic values extracted from Step 1 (e.g. JWT token, user_id)
// are successfully interpolated into subsequent step URL path and HTTP request headers.
func TestMultiStepStatePassing(t *testing.T) {
	var step1Hit, step2Hit, step3Hit int32
	var receivedAuthHeader string
	var receivedProfilePath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			atomic.AddInt32(&step1Hit, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"code": 200,
				"data": {
					"token": "bearer-token-abc-999",
					"user_id": 108,
					"roles": ["shopper", "beta_user"]
				}
			}`))

		case "/api/v1/users/108/profile":
			atomic.AddInt32(&step2Hit, 1)
			receivedAuthHeader = r.Header.Get("Authorization")
			receivedProfilePath = r.URL.Path

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"code": 200,
				"data": {
					"cart_id": "cart-777",
					"balance": 250000
				}
			}`))

		case "/api/v1/cart/cart-777/checkout":
			atomic.AddInt32(&step3Hit, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code": 200, "status": "completed"}`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	scenario := &Scenario{
		ID:            "ecommerce_flow",
		Name:          "E-Commerce User Journey",
		BaseURL:       server.URL,
		InitialStepID: "step_login",
		Steps: map[string]*Step{
			"step_login": {
				ID:   "step_login",
				Name: "User Login",
				Request: RequestConfig{
					Method: http.MethodPost,
					Path:   "/api/v1/auth/login",
					Body:   `{"username": "shopper", "password": "password123"}`,
				},
				Extractors: []ExtractorConfig{
					{
						Source:    ExtractorSourceBodyJSON,
						Path:      "data.token",
						TargetVar: "jwt_token",
					},
					{
						Source:    ExtractorSourceBodyJSON,
						Path:      "data.user_id",
						TargetVar: "uid",
					},
				},
				Transitions: []Transition{
					{TargetStepID: "step_profile", Probability: 1.0},
				},
			},
			"step_profile": {
				ID:   "step_profile",
				Name: "Get Profile",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/api/v1/users/${uid}/profile",
					Headers: map[string]string{
						"Authorization": "Bearer ${jwt_token}",
					},
				},
				Extractors: []ExtractorConfig{
					{
						Source:    ExtractorSourceBodyJSON,
						Path:      "data.cart_id",
						TargetVar: "cart_id",
					},
				},
				Transitions: []Transition{
					{TargetStepID: "step_checkout", Probability: 1.0},
				},
			},
			"step_checkout": {
				ID:   "step_checkout",
				Name: "Checkout Cart",
				Request: RequestConfig{
					Method: http.MethodPost,
					Path:   "/api/v1/cart/{{.cart_id}}/checkout",
					Headers: map[string]string{
						"Authorization": "Bearer {{.jwt_token}}",
					},
				},
				Transitions: []Transition{
					{TargetStepID: "END"},
				},
			},
		},
	}

	vu, err := NewVirtualUser(Config{
		ID:       "vu-test-01",
		Scenario: scenario,
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create VirtualUser: %v", err)
	}

	report, err := vu.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	if !report.Success {
		t.Fatalf("expected report.Success to be true, got false. Reason: %s", report.FailureReason)
	}

	if atomic.LoadInt32(&step1Hit) != 1 || atomic.LoadInt32(&step2Hit) != 1 || atomic.LoadInt32(&step3Hit) != 1 {
		t.Errorf("step hits mismatch: step1=%d, step2=%d, step3=%d", step1Hit, step2Hit, step3Hit)
	}

	expectedAuth := "Bearer bearer-token-abc-999"
	if receivedAuthHeader != expectedAuth {
		t.Errorf("expected header %q, got %q", expectedAuth, receivedAuthHeader)
	}

	expectedProfilePath := "/api/v1/users/108/profile"
	if receivedProfilePath != expectedProfilePath {
		t.Errorf("expected path %q, got %q", expectedProfilePath, receivedProfilePath)
	}

	// Verify final state snapshot
	if report.FinalState["jwt_token"] != "bearer-token-abc-999" {
		t.Errorf("expected state jwt_token to be preserved, got %v", report.FinalState["jwt_token"])
	}
	if report.FinalState["cart_id"] != "cart-777" {
		t.Errorf("expected state cart_id to be preserved, got %v", report.FinalState["cart_id"])
	}

	if len(report.StepResults) != 3 {
		t.Fatalf("expected 3 step results, got %d", len(report.StepResults))
	}
}

// TestFailureHandling verifies that if step 1 returns 401 or 500, the session aborts
// and does NOT proceed to subsequent steps.
func TestFailureHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "401_Unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error": "invalid_credentials"}`,
		},
		{
			name:       "500_InternalServerError",
			statusCode: http.StatusInternalServerError,
			body:       `{"error": "database_down"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var step1Hit, step2Hit int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/login" {
					atomic.AddInt32(&step1Hit, 1)
					w.WriteHeader(tc.statusCode)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				if r.URL.Path == "/profile" {
					atomic.AddInt32(&step2Hit, 1)
					w.WriteHeader(http.StatusOK)
					return
				}
			}))
			defer server.Close()

			scenario := &Scenario{
				ID:            "failure_test_scenario",
				Name:          "Failure Handling Test",
				BaseURL:       server.URL,
				InitialStepID: "step_login",
				Steps: map[string]*Step{
					"step_login": {
						ID: "step_login",
						Request: RequestConfig{
							Method: http.MethodPost,
							Path:   "/login",
						},
						Transitions: []Transition{
							{TargetStepID: "step_profile"},
						},
						OnFailure: FailurePolicy{
							Action: FailureActionAbort,
						},
					},
					"step_profile": {
						ID: "step_profile",
						Request: RequestConfig{
							Method: http.MethodGet,
							Path:   "/profile",
						},
					},
				},
			}

			vu, err := NewVirtualUser(Config{
				Scenario: scenario,
				Client:   server.Client(),
			})
			if err != nil {
				t.Fatalf("failed to create VU: %v", err)
			}

			report, err := vu.Execute(context.Background())
			if err != nil {
				t.Fatalf("execution returned unexpected Go error: %v", err)
			}

			if report.Success {
				t.Fatalf("expected report.Success to be false due to HTTP %d", tc.statusCode)
			}

			if atomic.LoadInt32(&step1Hit) != 1 {
				t.Errorf("expected step 1 to be called once, got %d", step1Hit)
			}

			if atomic.LoadInt32(&step2Hit) != 0 {
				t.Errorf("step 2 should NEVER have been called after step 1 failure, but was called %d times", step2Hit)
			}

			if len(report.StepResults) != 1 {
				t.Fatalf("expected 1 step result recorded, got %d", len(report.StepResults))
			}

			if report.StepResults[0].StatusCode != tc.statusCode {
				t.Errorf("expected recorded status code %d, got %d", tc.statusCode, report.StepResults[0].StatusCode)
			}
		})
	}
}

// TestProbabilisticBranching verifies Markov-style probabilistic transitions (80% vs 20%) over 1,000 runs.
func TestProbabilisticBranching(t *testing.T) {
	var stepAHits, stepBHits, stepCHits int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			atomic.AddInt64(&stepAHits, 1)
			w.WriteHeader(http.StatusOK)
		case "/b":
			atomic.AddInt64(&stepBHits, 1)
			w.WriteHeader(http.StatusOK)
		case "/c":
			atomic.AddInt64(&stepCHits, 1)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	scenario := &Scenario{
		ID:            "markov_branching",
		InitialStepID: "step_a",
		BaseURL:       server.URL,
		Steps: map[string]*Step{
			"step_a": {
				ID: "step_a",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/a",
				},
				Transitions: []Transition{
					{TargetStepID: "step_b", Probability: 0.80},
					{TargetStepID: "step_c", Probability: 0.20},
				},
			},
			"step_b": {
				ID: "step_b",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/b",
				},
				Transitions: []Transition{
					{TargetStepID: "END"},
				},
			},
			"step_c": {
				ID: "step_c",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/c",
				},
				Transitions: []Transition{
					{TargetStepID: "END"},
				},
			},
		},
	}

	const iterations = 1000
	randSrc := NewDefaultRandSource(42) // Fixed seed for reproducible test

	for i := 0; i < iterations; i++ {
		vu, err := NewVirtualUser(Config{
			Scenario: scenario,
			Client:   server.Client(),
			RandSrc:  randSrc,
		})
		if err != nil {
			t.Fatalf("failed to init VU: %v", err)
		}

		report, err := vu.Execute(context.Background())
		if err != nil || !report.Success {
			t.Fatalf("iteration %d failed: %v, reason: %s", i, err, report.FailureReason)
		}
	}

	bRatio := float64(stepBHits) / float64(iterations)
	cRatio := float64(stepCHits) / float64(iterations)

	t.Logf("Iterations: %d | Step B: %d (%.2f%%) | Step C: %d (%.2f%%)",
		iterations, stepBHits, bRatio*100, stepCHits, cRatio*100)

	// Tolerance of 4% for 1000 iterations: B should be 76%-84%, C should be 16%-24%
	if bRatio < 0.76 || bRatio > 0.84 {
		t.Errorf("Step B ratio %.4f out of expected tolerance [0.76, 0.84]", bRatio)
	}
	if cRatio < 0.16 || cRatio > 0.24 {
		t.Errorf("Step C ratio %.4f out of expected tolerance [0.16, 0.24]", cRatio)
	}
}

// TestConcurrencyAndRaceSafety verifies thread-safe execution of multiple concurrent VUs sharing the same scenario.
func TestConcurrencyAndRaceSafety(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok", "token": "session-token-xyz"}`))
	}))
	defer server.Close()

	scenario := &Scenario{
		ID:            "concurrent_scenario",
		InitialStepID: "step_1",
		BaseURL:       server.URL,
		Steps: map[string]*Step{
			"step_1": {
				ID: "step_1",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/status",
				},
				Extractors: []ExtractorConfig{
					{
						Source:    ExtractorSourceBodyJSON,
						Path:      "token",
						TargetVar: "tok",
					},
				},
				Transitions: []Transition{
					{TargetStepID: "END"},
				},
			},
		},
	}

	const concurrentUsers = 50
	var wg sync.WaitGroup
	errCh := make(chan error, concurrentUsers)
	sharedClient := server.Client()

	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			vu, err := NewVirtualUser(Config{
				ID:       fmt.Sprintf("concurrent-vu-%d", id),
				Scenario: scenario,
				Client:   sharedClient,
			})
			if err != nil {
				errCh <- fmt.Errorf("vu init error: %w", err)
				return
			}

			report, err := vu.Execute(context.Background())
			if err != nil {
				errCh <- fmt.Errorf("vu execute error: %w", err)
				return
			}
			if !report.Success {
				errCh <- fmt.Errorf("vu failed: %s", report.FailureReason)
				return
			}
			if report.FinalState["tok"] != "session-token-xyz" {
				errCh <- fmt.Errorf("state token mismatch: %v", report.FinalState["tok"])
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent test worker failed: %v", err)
	}
}

// TestScenarioParser verifies JSON and YAML serialization and graph validation.
func TestScenarioParser(t *testing.T) {
	yamlData := `
id: yaml_test_scenario
name: YAML Scenario
base_url: https://example.com
initial_step_id: login
timeout: 5s
steps:
  login:
    id: login
    name: Login Step
    request:
      method: POST
      path: /login
      headers:
        Content-Type: application/json
      body: '{"user": "test"}'
      timeout: 2s
    transitions:
      - target_step_id: END
        probability: 1.0
`

	sc, err := ParseScenarioYAML([]byte(yamlData))
	if err != nil {
		t.Fatalf("failed to parse YAML scenario: %v", err)
	}

	if sc.ID != "yaml_test_scenario" {
		t.Errorf("expected ID 'yaml_test_scenario', got %q", sc.ID)
	}
	if sc.Timeout.AsDuration() != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", sc.Timeout)
	}
	loginStep, ok := sc.Steps["login"]
	if !ok {
		t.Fatalf("expected 'login' step to exist")
	}
	if loginStep.Request.Timeout.AsDuration() != 2*time.Second {
		t.Errorf("expected step timeout 2s, got %v", loginStep.Request.Timeout)
	}

	// Test invalid graph validation (dangling target)
	badYaml := `
id: bad_scenario
initial_step_id: step_one
steps:
  step_one:
    id: step_one
    request:
      path: /test
    transitions:
      - target_step_id: non_existent_step
`
	_, err = ParseScenarioYAML([]byte(badYaml))
	if err == nil {
		t.Fatalf("expected error for dangling step reference, got nil")
	}
	if !strings.Contains(err.Error(), "non-existent step") {
		t.Errorf("expected error message to mention non-existent step, got %v", err)
	}
}

// TestExtractorEdgeCases tests array indexing, header extraction, and fallback defaults.
func TestExtractorEdgeCases(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	resp.Header.Set("X-Custom-Trace-ID", "trace-98765")
	resp.Header.Set("Set-Cookie", "session=abcde")

	body := []byte(`{
		"users": [
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"}
		],
		"meta": {
			"version": "v2.1"
		}
	}`)

	// 1. Array index extraction
	val, err := Extract(ExtractorConfig{
		Source:    ExtractorSourceBodyJSON,
		Path:      "users.1.name",
		TargetVar: "user_name",
	}, resp, body)
	if err != nil || val != "Bob" {
		t.Errorf("expected 'Bob', got %v (err: %v)", val, err)
	}

	// 2. Header extraction
	val, err = Extract(ExtractorConfig{
		Source:    ExtractorSourceHeader,
		Path:      "X-Custom-Trace-ID",
		TargetVar: "trace_id",
	}, resp, body)
	if err != nil || val != "trace-98765" {
		t.Errorf("expected 'trace-98765', got %v (err: %v)", val, err)
	}

	// 3. Fallback default value on missing key
	val, err = Extract(ExtractorConfig{
		Source:    ExtractorSourceBodyJSON,
		Path:      "non.existent.path",
		TargetVar: "fallback_var",
		Default:   "default_value",
	}, resp, body)
	if err != nil || val != "default_value" {
		t.Errorf("expected fallback 'default_value', got %v (err: %v)", val, err)
	}

	// 4. Regex extraction
	regexBody := []byte(`Session token: [TOK_99887766] successfully registered`)
	val, err = Extract(ExtractorConfig{
		Source:    ExtractorSourceRegex,
		Regex:     `\[(TOK_[0-9]+)\]`,
		TargetVar: "token_from_regex",
	}, resp, regexBody)
	if err != nil || val != "TOK_99887766" {
		t.Errorf("expected 'TOK_99887766', got %v (err: %v)", val, err)
	}
}

// BenchmarkStateInterpolation measures variable substitution throughput and memory allocation.
func BenchmarkStateInterpolation(b *testing.B) {
	state := NewSessionState()
	state.Set("token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0")
	state.Set("user_id", "user-1002")
	state.Set("action", "checkout")

	templateStr := "https://api.internal/v1/users/${user_id}/actions/{{.action}}?auth=Bearer ${token}"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		res, err := state.Interpolate(templateStr)
		if err != nil {
			b.Fatalf("interpolation error: %v", err)
		}
		if len(res) == 0 {
			b.Fatalf("empty result")
		}
	}
}

// TestConditionalTransitions verifies that transitions only activate when their condition evaluates to true.
func TestConditionalTransitions(t *testing.T) {
	eval := NewTransitionEvaluator(NewDefaultRandSource(123))
	state := NewSessionState()
	state.Set("role", "admin")

	step := &Step{
		ID: "step_check",
		Transitions: []Transition{
			{TargetStepID: "step_admin", Condition: "state.role == admin"},
			{TargetStepID: "step_user", Condition: "state.role == user"},
		},
	}

	result := &StepResult{
		StatusCode: 200,
		Success:    true,
	}

	selected, err := eval.SelectNextTransition(step, result, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected == nil || selected.TargetStepID != "step_admin" {
		t.Fatalf("expected step_admin to be chosen, got %+v", selected)
	}

	// Test condition with status code
	stepStatus := &Step{
		ID: "step_status",
		Transitions: []Transition{
			{TargetStepID: "on_error", Condition: "status >= 400"},
			{TargetStepID: "on_success", Condition: "status == 200"},
		},
	}

	errResult := &StepResult{StatusCode: 500, Success: false}
	selectedErr, err := eval.SelectNextTransition(stepStatus, errResult, state)
	if err != nil || selectedErr == nil || selectedErr.TargetStepID != "on_error" {
		t.Fatalf("expected on_error, got %+v", selectedErr)
	}

	succResult := &StepResult{StatusCode: 200, Success: true}
	selectedSucc, err := eval.SelectNextTransition(stepStatus, succResult, state)
	if err != nil || selectedSucc == nil || selectedSucc.TargetStepID != "on_success" {
		t.Fatalf("expected on_success, got %+v", selectedSucc)
	}
}

// TestFailureRetryPolicy verifies that a step configured with FailureActionRetry retries up to MaxRetries.
func TestFailureRetryPolicy(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := atomic.AddInt32(&attempts, 1)
		if att < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "recovered"}`))
	}))
	defer server.Close()

	scenario := &Scenario{
		ID:            "retry_scenario",
		InitialStepID: "flake_step",
		BaseURL:       server.URL,
		Steps: map[string]*Step{
			"flake_step": {
				ID: "flake_step",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/flake",
				},
				OnFailure: FailurePolicy{
					Action:     FailureActionRetry,
					MaxRetries: 3,
				},
				Transitions: []Transition{
					{TargetStepID: "END"},
				},
			},
		},
	}

	vu, err := NewVirtualUser(Config{
		Scenario: scenario,
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to init VU: %v", err)
	}

	report, err := vu.Execute(context.Background())
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if !report.Success {
		t.Fatalf("expected retry to succeed on 3rd attempt, got failure: %s", report.FailureReason)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

// TestFailureFallbackTransition verifies that FailureActionTransition routes to FallbackStepID.
func TestFailureFallbackTransition(t *testing.T) {
	var fallbackHit int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/primary" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/fallback" {
			atomic.AddInt32(&fallbackHit, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"recovered": true}`))
			return
		}
	}))
	defer server.Close()

	scenario := &Scenario{
		ID:            "fallback_scenario",
		InitialStepID: "step_primary",
		BaseURL:       server.URL,
		Steps: map[string]*Step{
			"step_primary": {
				ID: "step_primary",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/primary",
				},
				OnFailure: FailurePolicy{
					Action:         FailureActionTransition,
					FallbackStepID: "step_fallback",
				},
			},
			"step_fallback": {
				ID: "step_fallback",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/fallback",
				},
				Transitions: []Transition{
					{TargetStepID: "END"},
				},
			},
		},
	}

	vu, err := NewVirtualUser(Config{
		Scenario: scenario,
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to init VU: %v", err)
	}

	report, err := vu.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Success {
		t.Fatalf("expected fallback flow to succeed, failed with: %s", report.FailureReason)
	}
	if atomic.LoadInt32(&fallbackHit) != 1 {
		t.Errorf("expected fallback step to be executed once, got %d", fallbackHit)
	}
}

// TestExplicitAssertions verifies AssertionConfig behaviors (status range, body contains, header equals).
func TestExplicitAssertions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Status", "verified")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"result": "success_created"}`))
	}))
	defer server.Close()

	step := &Step{
		ID: "step_assertions",
		Request: RequestConfig{
			Method: http.MethodPost,
			Path:   "/test",
		},
		Assertions: []AssertionConfig{
			{Type: AssertStatusCodeInRange, MinCode: 200, MaxCode: 204},
			{Type: AssertBodyContains, Expected: "success_created"},
			{Type: AssertHeaderEquals, Target: "X-Custom-Status", Expected: "verified"},
		},
	}

	scenario := &Scenario{
		ID:            "assert_scen",
		InitialStepID: "step_assertions",
		BaseURL:       server.URL,
		Steps:         map[string]*Step{"step_assertions": step},
	}

	vu, err := NewVirtualUser(Config{Scenario: scenario, Client: server.Client()})
	if err != nil {
		t.Fatalf("failed to init VU: %v", err)
	}

	report, err := vu.Execute(context.Background())
	if err != nil || !report.Success {
		t.Fatalf("expected assertions to pass, report: %+v, err: %v", report, err)
	}
}

// TestSessionStateOperations tests helper methods on SessionState.
func TestSessionStateOperations(t *testing.T) {
	state := NewSessionStateWith(map[string]any{
		"env":  "production",
		"port": 8080,
	})

	if val, ok := state.GetString("env"); !ok || val != "production" {
		t.Errorf("expected 'production', got %q", val)
	}
	if val, ok := state.GetString("port"); !ok || val != "8080" {
		t.Errorf("expected '8080', got %q", val)
	}

	state.SetMultiple(map[string]any{
		"token": "tok-123",
		"tier":  "gold",
	})
	if _, ok := state.Get("token"); !ok {
		t.Errorf("expected 'token' to exist")
	}

	state.Delete("tier")
	if _, ok := state.Get("tier"); ok {
		t.Errorf("expected 'tier' to be deleted")
	}

	// Test missing variable in template interpolation returns clean error
	_, err := state.Interpolate("url/${missing_var}")
	if err == nil {
		t.Fatalf("expected error for missing variable interpolation")
	}
}

// TestJSONParserAndFileLoading verifies scenario loading and validation logic.
func TestJSONParserAndFileLoading(t *testing.T) {
	jsonContent := `{
		"id": "json_scen",
		"name": "JSON Scenario",
		"base_url": "http://localhost:8080",
		"initial_step_id": "start",
		"steps": {
			"start": {
				"id": "start",
				"name": "Start Step",
				"request": {
					"method": "GET",
					"path": "/start"
				}
			}
		}
	}`

	sc, err := ParseScenarioJSON([]byte(jsonContent))
	if err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if sc.ID != "json_scen" {
		t.Errorf("expected ID 'json_scen', got %s", sc.ID)
	}

	// Negative validation: empty ID
	badSc := &Scenario{
		InitialStepID: "start",
		Steps:         sc.Steps,
	}
	if err := ValidateScenario(badSc); err == nil {
		t.Fatalf("expected validation error for empty ID")
	}
}

// TestDurationSerialization tests JSON and YAML marshaling/unmarshaling for Duration.
func TestDurationSerialization(t *testing.T) {
	d := Duration(5 * time.Second)

	// JSON
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("failed to marshal duration to JSON: %v", err)
	}
	var unmarshaledD Duration
	if err := json.Unmarshal(data, &unmarshaledD); err != nil {
		t.Fatalf("failed to unmarshal duration from JSON: %v", err)
	}
	if unmarshaledD.AsDuration() != 5*time.Second {
		t.Errorf("expected 5s, got %v", unmarshaledD.AsDuration())
	}

	// YAML
	yData, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("failed to marshal duration to YAML: %v", err)
	}
	var yUnmarshaled Duration
	if err := yaml.Unmarshal(yData, &yUnmarshaled); err != nil {
		t.Fatalf("failed to unmarshal duration from YAML: %v", err)
	}
	if yUnmarshaled.AsDuration() != 5*time.Second {
		t.Errorf("expected 5s, got %v", yUnmarshaled.AsDuration())
	}
}

// TestLoadScenarioFromFile tests file-based scenario loading for both YAML and JSON files.
func TestLoadScenarioFromFile(t *testing.T) {
	tempDir := t.TempDir()

	yamlPath := filepath.Join(tempDir, "test_scen.yaml")
	yamlData := []byte(`
id: file_yaml
name: File YAML
base_url: https://api.local
initial_step_id: step_one
steps:
  step_one:
    id: step_one
    request:
      path: /status
`)
	if err := os.WriteFile(yamlPath, yamlData, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loadedYAML, err := LoadScenarioFromFile(yamlPath)
	if err != nil || loadedYAML.ID != "file_yaml" {
		t.Fatalf("failed to load YAML file: %v", err)
	}

	jsonPath := filepath.Join(tempDir, "test_scen.json")
	jsonData := []byte(`{
		"id": "file_json",
		"name": "File JSON",
		"base_url": "https://api.local",
		"initial_step_id": "step_one",
		"steps": {
			"step_one": {
				"id": "step_one",
				"request": {"path": "/status"}
			}
		}
	}`)
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	loadedJSON, err := LoadScenarioFromFile(jsonPath)
	if err != nil || loadedJSON.ID != "file_json" {
		t.Fatalf("failed to load JSON file: %v", err)
	}
}

// TestValidationEdgeCases tests topological and configuration errors in Scenario.
func TestValidationEdgeCases(t *testing.T) {
	// Nil scenario
	if err := ValidateScenario(nil); err == nil {
		t.Errorf("expected error on nil scenario")
	}

	// Empty initial step ID
	if err := ValidateScenario(&Scenario{ID: "test"}); err == nil {
		t.Errorf("expected error on empty initial_step_id")
	}

	// Initial step ID not in steps
	if err := ValidateScenario(&Scenario{
		ID:            "test",
		InitialStepID: "missing",
		Steps:         map[string]*Step{"other": {ID: "other"}},
	}); err == nil {
		t.Errorf("expected error when initial step not in steps map")
	}

	// Step has fallback action but no fallback ID
	if err := ValidateScenario(&Scenario{
		ID:            "test",
		InitialStepID: "step1",
		Steps: map[string]*Step{
			"step1": {
				ID: "step1",
				OnFailure: FailurePolicy{
					Action: FailureActionTransition,
				},
			},
		},
	}); err == nil {
		t.Errorf("expected error on missing fallback_step_id")
	}

	// Step has negative probability
	if err := ValidateScenario(&Scenario{
		ID:            "test",
		InitialStepID: "step1",
		Steps: map[string]*Step{
			"step1": {
				ID: "step1",
				Transitions: []Transition{
					{Probability: -0.5},
				},
			},
		},
	}); err == nil {
		t.Errorf("expected error on negative probability")
	}
}

// TestContextCancellationAndMaxSteps verifies session termination on context cancellation and step loop safety.
func TestContextCancellationAndMaxSteps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 1. Context Cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	scenario := &Scenario{
		ID:            "cancel_test",
		InitialStepID: "step1",
		BaseURL:       server.URL,
		Steps: map[string]*Step{
			"step1": {
				ID: "step1",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/test",
				},
			},
		},
	}

	vu, err := NewVirtualUser(Config{
		Scenario: scenario,
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to init VU: %v", err)
	}

	report, err := vu.Execute(ctx)
	if err != nil || report.Success {
		t.Fatalf("expected report.Success=false on canceled context, got report: %+v, err: %v", report, err)
	}

	// 2. Max Steps Loop Guard (Cycle detection)
	cyclicScenario := &Scenario{
		ID:            "cycle_test",
		InitialStepID: "loop_step",
		BaseURL:       server.URL,
		Steps: map[string]*Step{
			"loop_step": {
				ID: "loop_step",
				Request: RequestConfig{
					Method: http.MethodGet,
					Path:   "/loop",
				},
				Transitions: []Transition{
					{TargetStepID: "loop_step", Probability: 1.0},
				},
			},
		},
	}

	vuCyclic, err := NewVirtualUser(Config{
		Scenario: cyclicScenario,
		Client:   server.Client(),
		MaxSteps: 5,
	})
	if err != nil {
		t.Fatalf("failed to init cyclic VU: %v", err)
	}

	reportCyclic, err := vuCyclic.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reportCyclic.Success {
		t.Fatalf("expected cyclic execution to fail due to max steps")
	}
	if !strings.Contains(reportCyclic.FailureReason, "exceeded maximum allowed steps") {
		t.Errorf("expected failure reason to mention max steps, got %q", reportCyclic.FailureReason)
	}
}

// TestTransitionConditionOperators verifies relational comparison operators in conditions.
func TestTransitionConditionOperators(t *testing.T) {
	eval := NewTransitionEvaluator(NewDefaultRandSource(99))

	step := &Step{
		ID: "comp_step",
		Transitions: []Transition{
			{TargetStepID: "lt_step", Condition: "status < 300"},
			{TargetStepID: "lte_step", Condition: "status <= 200"},
			{TargetStepID: "gt_step", Condition: "status > 400"},
			{TargetStepID: "ne_step", Condition: "status != 200"},
		},
	}

	res200 := &StepResult{StatusCode: 200, Success: true}
	trans, err := eval.SelectNextTransition(step, res200, nil)
	if err != nil || trans == nil {
		t.Fatalf("expected valid transition for 200, got: %+v (err: %v)", trans, err)
	}
	if trans.TargetStepID != "lt_step" && trans.TargetStepID != "lte_step" {
		t.Errorf("expected lt_step or lte_step, got %q", trans.TargetStepID)
	}

	res500 := &StepResult{StatusCode: 500, Success: false}
	trans500, err := eval.SelectNextTransition(step, res500, nil)
	if err != nil || trans500 == nil {
		t.Fatalf("expected valid transition for 500, got: %+v", trans500)
	}
	if trans500.TargetStepID != "gt_step" && trans500.TargetStepID != "ne_step" {
		t.Errorf("expected gt_step or ne_step, got %q", trans500.TargetStepID)
	}
}

// TestEcommerceSampleScenarioEndToEnd loads the actual examples/scenario_ecommerce.yaml file
// and runs an end-to-end simulation against a mock server.
func TestEcommerceSampleScenarioEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/post":
			// Echo back posted json
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"json": %s}`, body)
		case "/get":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "catalog_or_cart_retrieved"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sc, err := LoadScenarioFromFile("../../examples/scenario_ecommerce.yaml")
	if err != nil {
		t.Fatalf("failed to load scenario_ecommerce.yaml: %v", err)
	}

	sc.BaseURL = server.URL // Override target to mock server

	vu, err := NewVirtualUser(Config{
		ID:       "vu-e2e-user-1",
		Scenario: sc,
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create VU: %v", err)
	}

	report, err := vu.Execute(context.Background())
	if err != nil {
		t.Fatalf("execution returned unexpected error: %v", err)
	}
	if !report.Success {
		t.Fatalf("expected successful scenario run, got failure: %s", report.FailureReason)
	}

	if report.FinalState["current_user"] != "shopper_01" {
		t.Errorf("expected final state current_user='shopper_01', got %v", report.FinalState["current_user"])
	}

	t.Logf("E2E Run completed successfully with %d steps in %v", len(report.StepResults), report.TotalDuration)
}

