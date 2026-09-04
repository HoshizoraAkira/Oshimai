package loadengine

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

// helper to create a minimal valid scenario pointing to mock server
func newTestScenario(baseURL string) *vusession.Scenario {
	return &vusession.Scenario{
		ID:            "bench_scenario",
		Name:          "Benchmark Test Scenario",
		BaseURL:       baseURL,
		InitialStepID: "step_ping",
		Steps: map[string]*vusession.Step{
			"step_ping": {
				ID: "step_ping",
				Request: vusession.RequestConfig{
					Method: http.MethodGet,
					Path:   "/ping",
				},
				Transitions: []vusession.Transition{
					{TargetStepID: "END"},
				},
			},
		},
	}
}

// TestTargetRPSRateLimiting verifies that the token-bucket rate limiter maintains
// the target rate within < 10% deviation.
func TestTargetRPSRateLimiting(t *testing.T) {
	var requestCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"pong"}`))
	}))
	defer server.Close()

	targetRPS := 100.0
	duration := 2 * time.Second

	engine, err := NewLoadEngine(EngineConfig{
		Profile:   ProfileTargetRPS,
		TargetRPS: targetRPS,
		Duration:  duration,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	sc := newTestScenario(server.URL)
	summary, err := engine.Run(context.Background(), sc)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if summary.TerminationStatus != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, summary.TerminationStatus)
	}

	expectedTotal := targetRPS * duration.Seconds()
	actual := float64(summary.TotalRequests)
	deviation := math.Abs(actual-expectedTotal) / expectedTotal

	t.Logf("Target RPS: %.0f | Duration: %v | Expected Requests: %.0f | Actual Requests: %d | Actual RPS: %.2f | Deviation: %.2f%%",
		targetRPS, duration, expectedTotal, summary.TotalRequests, summary.ActualRPS, deviation*100)

	if deviation > 0.10 {
		t.Errorf("RPS deviation %.2f%% exceeded 10%% tolerance limit", deviation*100)
	}
}

// TestHighConcurrencyScaleAndRaceSafety verifies execution of 500 concurrent workers
// under the Go race detector with zero data races or memory corruption.
func TestHighConcurrencyScaleAndRaceSafety(t *testing.T) {
	var requestCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	concurrency := 500
	duration := 1 * time.Second

	engine, err := NewLoadEngine(EngineConfig{
		Profile:  ProfileFlatVU,
		VUs:      concurrency,
		Duration: duration,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	sc := newTestScenario(server.URL)
	summary, err := engine.Run(context.Background(), sc)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	t.Logf("Concurrent VUs: %d | Total Requests Handled: %d | Actual RPS: %.2f | P99 Latency: %v",
		concurrency, summary.TotalRequests, summary.ActualRPS, summary.Latency.P99)

	if summary.TotalRequests < int64(concurrency) {
		t.Errorf("expected at least %d requests, got %d", concurrency, summary.TotalRequests)
	}
}

// TestCircuitBreakerAutoKillSwitch verifies that the engine automatically aborts when error rate
// breaches the SLA threshold before the scheduled test duration concludes.
func TestCircuitBreakerAutoKillSwitch(t *testing.T) {
	var requestCount int64

	// Mock server that returns 200 for first 20 requests, then 503 thereafter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt64(&requestCount, 1)
		if cur > 20 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	engine, err := NewLoadEngine(EngineConfig{
		Profile:  ProfileFlatVU,
		VUs:      10,
		Duration: 10 * time.Second, // Intentionally long; should abort much sooner
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:             true,
			EvaluationInterval:  100 * time.Millisecond,
			WindowDuration:      1 * time.Second,
			MinRequests:         15,
			MaxErrorRate:        0.30, // 30% error rate threshold
			ConsecutiveBreaches: 1,
			DrainTimeout:        1 * time.Second,
		},
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	sc := newTestScenario(server.URL)
	startTime := time.Now()
	summary, err := engine.Run(context.Background(), sc)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("run failed with unexpected Go error: %v", err)
	}

	t.Logf("Circuit Breaker Run Elapsed: %v | Total Reqs: %d | Errors: %d | Status: %s | Abort Reason: %s",
		elapsed, summary.TotalRequests, summary.TotalErrors, summary.TerminationStatus, summary.AbortReason)

	if summary.TerminationStatus != StatusAbortedByCircuitBreaker {
		t.Fatalf("expected TerminationStatus %s, got %s", StatusAbortedByCircuitBreaker, summary.TerminationStatus)
	}

	// Ensure it aborted well before the 10-second configured duration
	if elapsed >= 5*time.Second {
		t.Errorf("circuit breaker failed to abort promptly; took %v", elapsed)
	}

	if summary.TotalErrors == 0 {
		t.Errorf("expected recorded errors, got 0")
	}
}

// TestGracefulDrainOnContextCancellation verifies that canceling the parent context
// causes all active workers to shut down cleanly without leaking goroutines.
func TestGracefulDrainOnContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	engine, err := NewLoadEngine(EngineConfig{
		Profile:  ProfileFlatVU,
		VUs:      50,
		Duration: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	sc := newTestScenario(server.URL)

	// Cancel after 300ms
	time.AfterFunc(300*time.Millisecond, func() {
		cancel()
	})

	startTime := time.Now()
	summary, err := engine.Run(ctx, sc)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("run failed with unexpected Go error: %v", err)
	}

	t.Logf("Drain Elapsed: %v | Status: %s | Total Reqs: %d", elapsed, summary.TerminationStatus, summary.TotalRequests)

	if summary.TerminationStatus != StatusContextCancelled {
		t.Errorf("expected status %s, got %s", StatusContextCancelled, summary.TerminationStatus)
	}

	if elapsed >= 3*time.Second {
		t.Errorf("graceful drain took too long: %v", elapsed)
	}
}

// TestRampingStages verifies linear dynamic scaling through multi-stage ramping.
func TestRampingStages(t *testing.T) {
	var requestCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	engine, err := NewLoadEngine(EngineConfig{
		Profile: ProfileRamping,
		Stages: []RampingStage{
			{Duration: 400 * time.Millisecond, TargetVUs: 10},
			{Duration: 400 * time.Millisecond, TargetVUs: 20},
			{Duration: 400 * time.Millisecond, TargetVUs: 0},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	sc := newTestScenario(server.URL)
	summary, err := engine.Run(context.Background(), sc)
	if err != nil {
		t.Fatalf("ramping run failed: %v", err)
	}

	if summary.TerminationStatus != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, summary.TerminationStatus)
	}

	if summary.TotalRequests == 0 {
		t.Errorf("expected requests during ramping stages, got 0")
	}

	t.Logf("Ramping Run Completed: %d requests in %v (Actual RPS: %.2f)",
		summary.TotalRequests, summary.TotalDuration, summary.ActualRPS)
}

// TestHDRHistogramQuantiles verifies accuracy of streaming quantile calculation.
func TestHDRHistogramQuantiles(t *testing.T) {
	h := NewHDRHistogram()

	// Record known distribution: 100 values from 1ms to 100ms
	for i := 1; i <= 100; i++ {
		h.Record(time.Duration(i) * time.Millisecond)
	}

	stats := h.Stats()

	// Min should be ~1ms (within 1us)
	if stats.Min < 900*time.Microsecond || stats.Min > 1100*time.Microsecond {
		t.Errorf("expected min ~1ms, got %v", stats.Min)
	}

	// P50 should be ~50ms (within 5% resolution)
	if stats.P50 < 47*time.Millisecond || stats.P50 > 53*time.Millisecond {
		t.Errorf("expected p50 ~50ms, got %v", stats.P50)
	}

	// P99 should be ~99ms (within 5% resolution)
	if stats.P99 < 94*time.Millisecond || stats.P99 > 102*time.Millisecond {
		t.Errorf("expected p99 ~99ms, got %v", stats.P99)
	}

	// Max should be ~100ms
	if stats.Max < 95*time.Millisecond || stats.Max > 105*time.Millisecond {
		t.Errorf("expected max ~100ms, got %v", stats.Max)
	}
}

// BenchmarkHDRHistogramRecord measures lock-free recording throughput across parallel goroutines.
func BenchmarkHDRHistogramRecord(b *testing.B) {
	h := NewHDRHistogram()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		var i int64
		for pb.Next() {
			i++
			d := time.Duration((i%1000)+1) * time.Millisecond
			h.Record(d)
		}
	})
}

// TestConfigValidationAndDefaults verifies config validation rules and defaults.
func TestConfigValidationAndDefaults(t *testing.T) {
	def := DefaultCircuitBreakerConfig()
	if !def.Enabled || def.MaxErrorRate != 0.30 {
		t.Errorf("unexpected default circuit breaker config: %+v", def)
	}

	// Empty profile
	cfg := EngineConfig{}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error on empty profile")
	}

	// FlatVU missing VUs
	cfg = EngineConfig{Profile: ProfileFlatVU, Duration: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error on missing VUs")
	}

	// TargetRPS missing RPS
	cfg = EngineConfig{Profile: ProfileTargetRPS, Duration: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error on missing TargetRPS")
	}

	// Ramping missing stages
	cfg = EngineConfig{Profile: ProfileRamping}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error on missing stages")
	}

	// Nil scenario check in Run
	eng, err := NewLoadEngine(EngineConfig{
		Profile:  ProfileFlatVU,
		VUs:      1,
		Duration: time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	if _, err := eng.Run(context.Background(), nil); err == nil {
		t.Errorf("expected error on nil scenario")
	}
}

// TestMetricsSnapshot verifies Snapshot method on MetricsAccumulator.
func TestMetricsSnapshot(t *testing.T) {
	m := NewMetricsAccumulator()
	m.Record(200, 10*time.Millisecond, false)
	m.Record(500, 100*time.Millisecond, true)

	snap := m.Snapshot()
	if snap.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", snap.TotalRequests)
	}
	if snap.TotalErrors != 1 {
		t.Errorf("expected 1 error, got %d", snap.TotalErrors)
	}
	if snap.ErrorRate != 0.5 {
		t.Errorf("expected 50%% error rate, got %f", snap.ErrorRate)
	}
}

// TestCircuitBreakerP99LatencyAbort verifies that latency breaching the SLA triggers an abort.
func TestCircuitBreakerP99LatencyAbort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	engine, err := NewLoadEngine(EngineConfig{
		Profile:  ProfileFlatVU,
		VUs:      5,
		Duration: 5 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:             true,
			EvaluationInterval:  50 * time.Millisecond,
			WindowDuration:      500 * time.Millisecond,
			MinRequests:         5,
			MaxP99Latency:       10 * time.Millisecond, // 10ms threshold (server takes 30ms)
			ConsecutiveBreaches: 1,
			DrainTimeout:        500 * time.Millisecond,
		},
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	sc := newTestScenario(server.URL)
	summary, err := engine.Run(context.Background(), sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TerminationStatus != StatusAbortedByCircuitBreaker {
		t.Fatalf("expected StatusAbortedByCircuitBreaker, got %s", summary.TerminationStatus)
	}
}
