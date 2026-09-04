// Package search provides a generic bisection (binary) search for the smallest input at which a
// monotonic pass/fail evaluator first starts failing. It underpins both the Auto Chaos Fuzzer
// (pkg/autofuzz — searching fault severity for the point that breaks an application) and the
// Auto-Pilot capacity search (pkg/autopilot — searching concurrency for the point past which an
// application is no longer safely served), so the non-trivial convergence logic and edge cases
// are implemented and tested exactly once.
package search

import (
	"context"
	"fmt"
)

// Evaluator runs one trial at input x and reports whether the system still "passes" (is healthy)
// at that input. detail carries evaluator-specific data (e.g. a health score or execution
// summary) forward into the resulting Trial for reporting.
type Evaluator func(ctx context.Context, x float64) (pass bool, detail any, err error)

// Config parameterizes the search. Evaluate is assumed monotonic non-increasing over [Min, Max]:
// once it starts failing at some x, it is assumed to keep failing for all larger inputs. This
// holds for the domains this package is used for (more chaos severity, or more concurrent users,
// only ever makes things harder for a system, never easier).
type Config struct {
	Min           float64
	Max           float64
	MaxIterations int     // Bisection steps after the two boundary probes. Must be > 0.
	Tolerance     float64 // Stop once (hi - lo) <= Tolerance. Must be > 0.
}

// Validate checks the search boundaries are sane before any trial is run.
func (c Config) Validate() error {
	if c.Max <= c.Min {
		return fmt.Errorf("max (%f) must be greater than min (%f)", c.Max, c.Min)
	}
	if c.MaxIterations <= 0 {
		return fmt.Errorf("max_iterations must be > 0")
	}
	if c.Tolerance <= 0 {
		return fmt.Errorf("tolerance must be > 0")
	}
	return nil
}

// Trial records one evaluated input and its outcome.
type Trial struct {
	X      float64
	Pass   bool
	Detail any `json:"detail,omitempty"`
}

// Report summarizes a completed search.
type Report struct {
	// Found is true when a breaking point was located within [Min, Max]. If false, the system
	// passed even at Max — BreakingPoint is meaningless and callers should treat Max as a lower
	// bound on the system's true capacity/tolerance.
	Found         bool
	BreakingPoint float64
	Trials        []Trial
}

// FindBreakingPoint bisects [cfg.Min, cfg.Max] for the smallest x where evaluate(x) first fails.
func FindBreakingPoint(ctx context.Context, cfg Config, evaluate Evaluator) (*Report, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if evaluate == nil {
		return nil, fmt.Errorf("evaluate function is required")
	}

	report := &Report{}

	maxPass, maxDetail, err := evaluate(ctx, cfg.Max)
	if err != nil {
		return nil, fmt.Errorf("trial at max (%f) failed: %w", cfg.Max, err)
	}
	report.Trials = append(report.Trials, Trial{X: cfg.Max, Pass: maxPass, Detail: maxDetail})
	if maxPass {
		// The system survives even the harshest input in range — no breaking point to report.
		return report, nil
	}

	minPass, minDetail, err := evaluate(ctx, cfg.Min)
	if err != nil {
		return nil, fmt.Errorf("trial at min (%f) failed: %w", cfg.Min, err)
	}
	report.Trials = append(report.Trials, Trial{X: cfg.Min, Pass: minPass, Detail: minDetail})
	if !minPass {
		// Already broken at the gentlest input tested — the breaking point is at or below Min.
		report.Found = true
		report.BreakingPoint = cfg.Min
		return report, nil
	}

	lo, hi := cfg.Min, cfg.Max
	for i := 0; i < cfg.MaxIterations && (hi-lo) > cfg.Tolerance; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		mid := lo + (hi-lo)/2
		pass, detail, err := evaluate(ctx, mid)
		if err != nil {
			return nil, fmt.Errorf("trial at %f failed: %w", mid, err)
		}
		report.Trials = append(report.Trials, Trial{X: mid, Pass: pass, Detail: detail})

		if pass {
			lo = mid
		} else {
			hi = mid
		}
	}

	report.Found = true
	report.BreakingPoint = hi
	return report, nil
}
