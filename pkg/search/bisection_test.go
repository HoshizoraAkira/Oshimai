package search

import (
	"context"
	"errors"
	"testing"
)

func thresholdEvaluator(threshold float64) Evaluator {
	return func(ctx context.Context, x float64) (bool, any, error) {
		return x < threshold, nil, nil
	}
}

func TestFindBreakingPointConverges(t *testing.T) {
	cfg := Config{Min: 0, Max: 100, MaxIterations: 20, Tolerance: 0.5}
	report, err := FindBreakingPoint(context.Background(), cfg, thresholdEvaluator(42.3))
	if err != nil {
		t.Fatalf("FindBreakingPoint failed: %v", err)
	}
	if !report.Found {
		t.Fatal("expected a breaking point to be found")
	}
	if diff := report.BreakingPoint - 42.3; diff < -1 || diff > 1 {
		t.Errorf("expected breaking point near 42.3, got %f", report.BreakingPoint)
	}
}

func TestFindBreakingPointNoneWithinRange(t *testing.T) {
	cfg := Config{Min: 0, Max: 100, MaxIterations: 10, Tolerance: 0.5}
	report, err := FindBreakingPoint(context.Background(), cfg, thresholdEvaluator(500))
	if err != nil {
		t.Fatalf("FindBreakingPoint failed: %v", err)
	}
	if report.Found {
		t.Error("expected no breaking point to be found when the system survives even at Max")
	}
	if len(report.Trials) != 1 {
		t.Errorf("expected exactly one trial (at Max) when nothing breaks, got %d", len(report.Trials))
	}
}

func TestFindBreakingPointAtMinimum(t *testing.T) {
	cfg := Config{Min: 10, Max: 100, MaxIterations: 10, Tolerance: 0.5}
	report, err := FindBreakingPoint(context.Background(), cfg, thresholdEvaluator(5))
	if err != nil {
		t.Fatalf("FindBreakingPoint failed: %v", err)
	}
	if !report.Found || report.BreakingPoint != 10 {
		t.Errorf("expected breaking point pinned at Min (10), got found=%v point=%f", report.Found, report.BreakingPoint)
	}
}

func TestFindBreakingPointRespectsIterationCap(t *testing.T) {
	cfg := Config{Min: 0, Max: 1_000_000, MaxIterations: 3, Tolerance: 0.0001}
	report, err := FindBreakingPoint(context.Background(), cfg, thresholdEvaluator(500_000))
	if err != nil {
		t.Fatalf("FindBreakingPoint failed: %v", err)
	}
	// 2 boundary probes + at most 3 bisection iterations = 5 trials, even though tolerance alone
	// would need far more steps to converge on a search space this wide.
	if len(report.Trials) > 5 {
		t.Errorf("expected iteration cap to limit trials to <= 5, got %d", len(report.Trials))
	}
}

func TestFindBreakingPointValidatesConfig(t *testing.T) {
	cases := []Config{
		{Min: 10, Max: 5, MaxIterations: 5, Tolerance: 0.1}, // Max <= Min
		{Min: 0, Max: 10, MaxIterations: 0, Tolerance: 0.1}, // MaxIterations <= 0
		{Min: 0, Max: 10, MaxIterations: 5, Tolerance: 0},   // Tolerance <= 0
	}
	for _, cfg := range cases {
		if _, err := FindBreakingPoint(context.Background(), cfg, thresholdEvaluator(1)); err == nil {
			t.Errorf("expected validation error for config %+v", cfg)
		}
	}
}

func TestFindBreakingPointPropagatesEvaluatorError(t *testing.T) {
	boom := errors.New("boom")
	evaluator := func(ctx context.Context, x float64) (bool, any, error) { return false, nil, boom }
	cfg := Config{Min: 0, Max: 10, MaxIterations: 5, Tolerance: 0.1}
	if _, err := FindBreakingPoint(context.Background(), cfg, evaluator); err == nil {
		t.Error("expected evaluator error to propagate")
	}
}

func TestFindBreakingPointRequiresEvaluator(t *testing.T) {
	cfg := Config{Min: 0, Max: 10, MaxIterations: 5, Tolerance: 0.1}
	if _, err := FindBreakingPoint(context.Background(), cfg, nil); err == nil {
		t.Error("expected error for nil evaluator")
	}
}
