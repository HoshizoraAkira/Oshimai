package autofuzz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/chaos"
)

func TestScaleFaultClampsAndScales(t *testing.T) {
	r := SeverityRange{MaxLatency: 1000 * time.Millisecond, MaxJitter: 200 * time.Millisecond, MaxLossPercent: 50, MaxCorruptionPercent: 20}

	half := r.ScaleFault("f", 0.5, 5*time.Second, chaos.FilterConfig{Interface: "lo"})
	if half.Latency != 500*time.Millisecond {
		t.Errorf("expected 500ms latency at severity 0.5, got %v", half.Latency)
	}
	if half.LossPercent != 25 {
		t.Errorf("expected 25%% loss at severity 0.5, got %f", half.LossPercent)
	}

	over := r.ScaleFault("f", 1.5, 5*time.Second, chaos.FilterConfig{Interface: "lo"})
	if over.Latency != r.MaxLatency {
		t.Errorf("expected severity to clamp to 1.0, got latency %v", over.Latency)
	}

	under := r.ScaleFault("f", -0.5, 5*time.Second, chaos.FilterConfig{Interface: "lo"})
	if under.Latency != 0 {
		t.Errorf("expected severity to clamp to 0.0, got latency %v", under.Latency)
	}
}

// simulatedHealthDegradation models an application whose health score falls off linearly as
// chaos severity crosses a hidden breaking point — good enough to prove the bisection converges
// without spinning up real HTTP servers for every trial.
func simulatedHealthDegradation(breakingSeverity float64) TrialRunner {
	return func(ctx context.Context, severity float64, fault chaos.FaultSpec) (int, error) {
		if severity < breakingSeverity {
			return 95, nil
		}
		return 30, nil
	}
}

func TestRunFindsBreakingPoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrialDuration = time.Millisecond // Keep the test fast; real orchestration sets this to seconds.
	cfg.MaxIterations = 10
	cfg.Tolerance = 0.02

	result, err := Run(context.Background(), cfg, simulatedHealthDegradation(0.6))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !result.Found {
		t.Fatal("expected a breaking point to be found")
	}
	if diff := result.BreakingSeverity - 0.6; diff < -0.05 || diff > 0.05 {
		t.Errorf("expected breaking severity near 0.6, got %f", result.BreakingSeverity)
	}
	if result.BreakingFault == nil {
		t.Error("expected a concrete breaking fault spec to be reported")
	}
	if len(result.Trials) == 0 {
		t.Error("expected trial history to be recorded")
	}
}

func TestRunReportsNoBreakingPoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrialDuration = time.Millisecond
	// Application never degrades below the health threshold regardless of severity.
	alwaysHealthy := func(ctx context.Context, severity float64, fault chaos.FaultSpec) (int, error) {
		return 100, nil
	}

	result, err := Run(context.Background(), cfg, alwaysHealthy)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Found {
		t.Error("expected no breaking point when the app stays healthy at max severity")
	}
	if result.Verdict == "" {
		t.Error("expected a human-readable verdict even when nothing broke")
	}
}

func TestRunRequiresTrialRunner(t *testing.T) {
	if _, err := Run(context.Background(), DefaultConfig(), nil); err == nil {
		t.Error("expected error for nil TrialRunner")
	}
}

func TestRunPropagatesTrialError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrialDuration = time.Millisecond
	boom := errors.New("target unreachable")
	failing := func(ctx context.Context, severity float64, fault chaos.FaultSpec) (int, error) {
		return 0, boom
	}
	if _, err := Run(context.Background(), cfg, failing); err == nil {
		t.Error("expected trial error to propagate")
	}
}
