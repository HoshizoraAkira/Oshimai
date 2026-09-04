package loadengine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ErrCircuitBreakerTripped represents an automatic kill-switch safety abort.
type ErrCircuitBreakerTripped struct {
	Reason     string
	ErrorRate  float64
	P99Latency time.Duration
}

func (e ErrCircuitBreakerTripped) Error() string {
	return fmt.Sprintf("circuit breaker tripped: %s (error_rate=%.2f%%, p99=%v)",
		e.Reason, e.ErrorRate*100, e.P99Latency)
}

// CircuitBreakerMonitor evaluates real-time window metrics and triggers an automatic abort
// via context.CancelCauseFunc if target health thresholds are breached.
type CircuitBreakerMonitor struct {
	cfg         CircuitBreakerConfig
	window      *SlidingWindow
	cancelCause context.CancelCauseFunc

	mu                  sync.RWMutex
	tripped             bool
	tripReason          string
	errBreachCount      int
	latencyBreachCount  int
}

// NewCircuitBreakerMonitor creates an initialized monitor.
func NewCircuitBreakerMonitor(cfg CircuitBreakerConfig, window *SlidingWindow, cancelCause context.CancelCauseFunc) *CircuitBreakerMonitor {
	if cfg.EvaluationInterval <= 0 {
		cfg.EvaluationInterval = 1 * time.Second
	}
	if cfg.ConsecutiveBreaches <= 0 {
		cfg.ConsecutiveBreaches = 2
	}
	if cfg.MinRequests <= 0 {
		cfg.MinRequests = 10
	}

	return &CircuitBreakerMonitor{
		cfg:         cfg,
		window:      window,
		cancelCause: cancelCause,
	}
}

// Start launches the background monitoring ticker until context is done.
func (cb *CircuitBreakerMonitor) Start(ctx context.Context) {
	if !cb.cfg.Enabled {
		return
	}

	ticker := time.NewTicker(cb.cfg.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cb.evaluate()
		}
	}
}

// evaluate performs health check on sliding window metrics.
func (cb *CircuitBreakerMonitor) evaluate() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.tripped {
		return
	}

	stats := cb.window.Stats()

	// Guard: require minimum sample size before triggering aborts
	if stats.Requests < int64(cb.cfg.MinRequests) {
		cb.errBreachCount = 0
		cb.latencyBreachCount = 0
		return
	}

	// 1. Check Error Rate
	if cb.cfg.MaxErrorRate > 0 && stats.ErrorRate > cb.cfg.MaxErrorRate {
		cb.errBreachCount++
	} else {
		cb.errBreachCount = 0
	}

	// 2. Check P99 Latency
	if cb.cfg.MaxP99Latency > 0 && stats.P99Latency > cb.cfg.MaxP99Latency {
		cb.latencyBreachCount++
	} else {
		cb.latencyBreachCount = 0
	}

	// Evaluate breach conditions
	var trippedReason string
	if cb.errBreachCount >= cb.cfg.ConsecutiveBreaches {
		trippedReason = fmt.Sprintf("error rate %.2f%% exceeded SLA threshold %.2f%%",
			stats.ErrorRate*100, cb.cfg.MaxErrorRate*100)
	} else if cb.latencyBreachCount >= cb.cfg.ConsecutiveBreaches {
		trippedReason = fmt.Sprintf("p99 latency %v exceeded SLA threshold %v",
			stats.P99Latency, cb.cfg.MaxP99Latency)
	}

	if trippedReason != "" {
		cb.tripped = true
		cb.tripReason = trippedReason

		if cb.cancelCause != nil {
			cb.cancelCause(ErrCircuitBreakerTripped{
				Reason:     trippedReason,
				ErrorRate:  stats.ErrorRate,
				P99Latency: stats.P99Latency,
			})
		}
	}
}

// IsTripped returns whether the circuit breaker was triggered.
func (cb *CircuitBreakerMonitor) IsTripped() (bool, string) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.tripped, cb.tripReason
}
