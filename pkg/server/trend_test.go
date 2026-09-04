package server

import (
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/remediation"
)

func makeCompletedRun(id string, p99 time.Duration, totalReqs, totalErrors int64, healthScore int) *TestRun {
	return &TestRun{
		ID:     id,
		Status: RunStatusCompleted,
		Summary: &loadengine.ExecutionSummary{
			TotalRequests: totalReqs,
			TotalErrors:   totalErrors,
			ActualRPS:     100,
			Latency:       loadengine.LatencyStats{P99: p99},
		},
		Diagnostics: &remediation.DiagnosticReport{HealthScore: healthScore},
	}
}

func TestCompareRunsDetectsRegression(t *testing.T) {
	baseline := makeCompletedRun("run-1", 200*time.Millisecond, 1000, 5, 92)
	current := makeCompletedRun("run-2", 400*time.Millisecond, 1000, 80, 60)

	report, err := CompareRuns(current, baseline)
	if err != nil {
		t.Fatalf("CompareRuns failed: %v", err)
	}
	if !report.Regressed {
		t.Error("expected regression to be detected")
	}
	if report.DeltaP99Pct <= 0 {
		t.Errorf("expected positive P99 delta (worse), got %f", report.DeltaP99Pct)
	}
	if report.DeltaHealthScore >= 0 {
		t.Errorf("expected negative health score delta, got %d", report.DeltaHealthScore)
	}
}

func TestCompareRunsDetectsImprovement(t *testing.T) {
	baseline := makeCompletedRun("run-1", 400*time.Millisecond, 1000, 80, 60)
	current := makeCompletedRun("run-2", 150*time.Millisecond, 1000, 2, 95)

	report, err := CompareRuns(current, baseline)
	if err != nil {
		t.Fatalf("CompareRuns failed: %v", err)
	}
	if report.Regressed {
		t.Error("did not expect regression for an improved run")
	}
	if report.DeltaHealthScore <= 0 {
		t.Errorf("expected positive health score delta, got %d", report.DeltaHealthScore)
	}
}

func TestCompareRunsRequiresBothSummaries(t *testing.T) {
	current := makeCompletedRun("run-2", 200*time.Millisecond, 1000, 5, 90)
	if _, err := CompareRuns(current, nil); err == nil {
		t.Error("expected error when baseline is nil")
	}
	if _, err := CompareRuns(&TestRun{ID: "no-summary"}, current); err == nil {
		t.Error("expected error when current run has no summary")
	}
}

func TestFindPreviousCompleted(t *testing.T) {
	runs := []*TestRun{
		makeCompletedRun("run-1", 100*time.Millisecond, 500, 1, 95),
		{ID: "run-2", Status: RunStatusFailed}, // Failed run, should be skipped.
		makeCompletedRun("run-3", 120*time.Millisecond, 500, 1, 93),
	}

	prev := FindPreviousCompleted(runs, "run-3")
	if prev == nil || prev.ID != "run-1" {
		t.Fatalf("expected run-1 as previous completed run, got %+v", prev)
	}

	if FindPreviousCompleted(runs, "run-1") != nil {
		t.Error("expected nil when there is no earlier run")
	}
	if FindPreviousCompleted(runs, "does-not-exist") != nil {
		t.Error("expected nil for unknown run id")
	}
}
