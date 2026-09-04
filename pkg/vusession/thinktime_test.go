package vusession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExecuteSingleStepPureThinkTimeMakesNoHTTPCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sc := &Scenario{
		ID: "think_test", BaseURL: server.URL, InitialStepID: "wait",
		Steps: map[string]*Step{
			"wait": {ID: "wait", ThinkTime: Duration(50 * time.Millisecond)},
		},
	}
	vu, err := NewVirtualUser(Config{Scenario: sc})
	if err != nil {
		t.Fatalf("NewVirtualUser failed: %v", err)
	}

	start := time.Now()
	result, err := vu.ExecuteSingleStep(context.Background(), sc.Steps["wait"])
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ExecuteSingleStep failed: %v", err)
	}
	if !result.Success {
		t.Error("expected a pure think-time step to report success")
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms elapsed, got %v", elapsed)
	}
	if called {
		t.Error("expected no HTTP call for a step with no request path")
	}
}

func TestExecuteSingleStepThinkTimeBeforeRealRequest(t *testing.T) {
	var requestTime time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTime = time.Now()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sc := &Scenario{
		ID: "think_test_2", BaseURL: server.URL, InitialStepID: "delayed_call",
		Steps: map[string]*Step{
			"delayed_call": {ID: "delayed_call", ThinkTime: Duration(40 * time.Millisecond), Request: RequestConfig{Method: "GET", Path: "/ping"}},
		},
	}
	vu, _ := NewVirtualUser(Config{Scenario: sc})

	start := time.Now()
	result, err := vu.ExecuteSingleStep(context.Background(), sc.Steps["delayed_call"])
	if err != nil {
		t.Fatalf("ExecuteSingleStep failed: %v", err)
	}
	if !result.Success || result.StatusCode != 200 {
		t.Errorf("expected a successful real request after the delay, got %+v", result)
	}
	if requestTime.Sub(start) < 40*time.Millisecond {
		t.Error("expected the HTTP call to happen only after ThinkTime elapsed")
	}
}

func TestExecuteSingleStepThinkTimeRespectsContextCancellation(t *testing.T) {
	sc := &Scenario{
		ID: "think_test_3", InitialStepID: "wait",
		Steps: map[string]*Step{"wait": {ID: "wait", ThinkTime: Duration(time.Second)}},
	}
	vu, _ := NewVirtualUser(Config{Scenario: sc})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := vu.ExecuteSingleStep(ctx, sc.Steps["wait"])
	if err == nil {
		t.Error("expected context cancellation to interrupt a long think-time wait")
	}
}
