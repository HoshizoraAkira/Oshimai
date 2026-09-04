package autopilot

import (
	"context"
	"errors"
	"testing"
)

// simulatedCapacity models an application that serves traffic cleanly up to a hidden VU ceiling
// and then degrades — enough to prove the bisection converges without real HTTP servers.
func simulatedCapacity(ceiling int) TrialRunner {
	return func(ctx context.Context, vus int) (int, error) {
		if vus <= ceiling {
			return 95, nil
		}
		return 40, nil
	}
}

func TestRunFindsSafeCapacity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 10
	cfg.ToleranceVUs = 2

	result, err := Run(context.Background(), cfg, simulatedCapacity(120))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !result.Found {
		t.Fatal("expected a breaking point to be found")
	}
	if result.SafeMaxVUs < 100 || result.SafeMaxVUs > 120 {
		t.Errorf("expected safe max VUs near 120, got %d", result.SafeMaxVUs)
	}
	if result.BreakingVUs < 120 || result.BreakingVUs > 140 {
		t.Errorf("expected breaking VUs just above the ceiling, got %d", result.BreakingVUs)
	}
}

func TestRunReportsNoBreakingPointWithinRange(t *testing.T) {
	cfg := DefaultConfig()
	alwaysHealthy := func(ctx context.Context, vus int) (int, error) { return 100, nil }

	result, err := Run(context.Background(), cfg, alwaysHealthy)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Found {
		t.Error("expected no breaking point when the app stays healthy through MaxVUs")
	}
	if result.SafeMaxVUs != cfg.MaxVUs {
		t.Errorf("expected SafeMaxVUs to equal MaxVUs (%d) as a lower bound, got %d", cfg.MaxVUs, result.SafeMaxVUs)
	}
}

func TestRunRequiresTrialRunner(t *testing.T) {
	if _, err := Run(context.Background(), DefaultConfig(), nil); err == nil {
		t.Error("expected error for nil TrialRunner")
	}
}

func TestRunPropagatesTrialError(t *testing.T) {
	boom := errors.New("target unreachable")
	failing := func(ctx context.Context, vus int) (int, error) { return 0, boom }
	if _, err := Run(context.Background(), DefaultConfig(), failing); err == nil {
		t.Error("expected trial error to propagate")
	}
}
