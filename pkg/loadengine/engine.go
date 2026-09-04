// Package loadengine is Oshimai's virtual-user execution core: it drives the actual HTTP traffic
// against a target for a load test, in one of three profiles (flat_vu, target_rps, ramping — see
// pool.go), while a sliding-window circuit breaker (circuit_breaker.go) watches error rate and P99
// latency in real time and can abort the run early rather than let a broken target get hammered
// for its full configured duration. Each virtual user's session logic (the actual scenario state
// machine) lives in pkg/vusession; this package is purely about *how many* of them run and *how
// fast*, not what any single one of them does.
package loadengine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

var errDurationElapsed = errors.New("load test duration elapsed")

type compositeRecorder struct {
	metrics *MetricsAccumulator
	window  *SlidingWindow
}

func (cr *compositeRecorder) Record(statusCode int, latency time.Duration, isError bool) {
	cr.metrics.Record(statusCode, latency, isError)
	cr.window.Record(statusCode, latency, isError)
}

func (cr *compositeRecorder) RecordStep(stepID string, statusCode int, latency time.Duration, isError bool) {
	cr.metrics.Record(statusCode, latency, isError)
	cr.metrics.RecordStep(stepID, statusCode, latency, isError)
	cr.window.Record(statusCode, latency, isError)
}

// LoadEngine coordinates high-concurrency load generation, real-time metrics, and safety circuit breaking.
type LoadEngine struct {
	cfg          EngineConfig
	activeWindow atomic.Pointer[SlidingWindow]
	activeVUs    atomic.Int32
}

// ActiveVUs returns the current number of active virtual users.
func (e *LoadEngine) ActiveVUs() int {
	return int(e.activeVUs.Load())
}

// CurrentStats returns the active sliding window performance statistics if a run is currently in progress.
func (e *LoadEngine) CurrentStats() (WindowStats, bool) {
	w := e.activeWindow.Load()
	if w == nil {
		return WindowStats{}, false
	}
	return w.Stats(), true
}

// NewLoadEngine creates a LoadEngine with the specified configuration.
func NewLoadEngine(cfg EngineConfig) (*LoadEngine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid engine config: %w", err)
	}

	if cfg.Client == nil {
		cfg.Client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 200,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	return &LoadEngine{cfg: cfg}, nil
}

// Run executes the scenario using the configured load profile and monitors system health in real-time.
func (e *LoadEngine) Run(ctx context.Context, scenario *vusession.Scenario) (*ExecutionSummary, error) {
	if scenario == nil {
		return nil, fmt.Errorf("scenario cannot be nil")
	}

	startTime := time.Now()
	metrics := NewMetricsAccumulator()

	windowDuration := e.cfg.CircuitBreaker.WindowDuration
	if windowDuration <= 0 {
		windowDuration = 5 * time.Second
	}
	window := NewSlidingWindow(e.cfg.CircuitBreaker.EvaluationInterval, windowDuration)
	recorder := &compositeRecorder{metrics: metrics, window: window}

	e.activeWindow.Store(window)
	defer e.activeWindow.Store(nil)

	runCtx, cancelCause := context.WithCancelCause(ctx)
	defer cancelCause(nil)

	// Set up timer for profiles with fixed duration
	totalDuration := e.cfg.Duration
	if e.cfg.Profile == ProfileRamping {
		var rampDuration time.Duration
		for _, s := range e.cfg.Stages {
			rampDuration += s.Duration
		}
		totalDuration = rampDuration
	}

	if totalDuration > 0 {
		timer := time.AfterFunc(totalDuration, func() {
			cancelCause(errDurationElapsed)
		})
		defer timer.Stop()
	}

	// Initialize and launch Circuit Breaker monitor
	cbMonitor := NewCircuitBreakerMonitor(e.cfg.CircuitBreaker, window, cancelCause)
	go cbMonitor.Start(runCtx)

	// Launch load profile generator
	switch e.cfg.Profile {
	case ProfileFlatVU:
		runFlatVU(runCtx, e.cfg, scenario, recorder, &e.activeVUs)
	case ProfileTargetRPS:
		runTargetRPS(runCtx, e.cfg, scenario, recorder, &e.activeVUs)
	case ProfileRamping:
		runRamping(runCtx, e.cfg, scenario, recorder, &e.activeVUs)
	}

	elapsed := time.Since(startTime)

	// Determine termination status
	var status TerminationStatus
	var abortReason string

	if tripped, reason := cbMonitor.IsTripped(); tripped {
		status = StatusAbortedByCircuitBreaker
		abortReason = reason
	} else if cause := context.Cause(runCtx); cause != nil && !errors.Is(cause, errDurationElapsed) && !errors.Is(cause, context.Canceled) {
		var cbErr ErrCircuitBreakerTripped
		if errors.As(cause, &cbErr) {
			status = StatusAbortedByCircuitBreaker
			abortReason = cbErr.Error()
		} else {
			status = StatusContextCancelled
			abortReason = cause.Error()
		}
	} else if errors.Is(ctx.Err(), context.Canceled) {
		status = StatusContextCancelled
		abortReason = "parent context canceled"
	} else {
		status = StatusCompleted
	}

	return metrics.Summary(elapsed, status, abortReason), nil
}
