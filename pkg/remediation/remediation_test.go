package remediation

import (
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

func TestAnalyze_NilOrEmpty(t *testing.T) {
	report := Analyze(nil, 0, false, nil)
	if report == nil {
		t.Fatalf("expected non-nil report")
	}
	if report.HealthScore <= 0 || report.HealthScore > 100 {
		t.Errorf("health score out of bounds: %d", report.HealthScore)
	}

	emptySummary := &loadengine.ExecutionSummary{
		TotalRequests: 0,
	}
	reportEmpty := Analyze(emptySummary, 20, false, nil)
	if reportEmpty.HealthScore != 50 {
		t.Errorf("expected 50 for empty summary, got %d", reportEmpty.HealthScore)
	}
}

func TestAnalyze_HealthySystem(t *testing.T) {
	summary := &loadengine.ExecutionSummary{
		TotalRequests: 1000,
		TotalErrors:   0,
		ActualRPS:     100.0,
		TotalDuration: 10 * time.Second,
		StatusCodes: map[int]int64{
			200: 1000,
		},
		Latency: loadengine.LatencyStats{
			Min:  2 * time.Millisecond,
			Mean: 15 * time.Millisecond,
			P50:  12 * time.Millisecond,
			P90:  25 * time.Millisecond,
			P95:  35 * time.Millisecond,
			P99:  50 * time.Millisecond,
			Max:  80 * time.Millisecond,
		},
		TerminationStatus: loadengine.StatusCompleted,
	}

	report := Analyze(summary, 50, false, nil)
	if report.HealthScore < 90 {
		t.Errorf("expected healthy score >= 90, got %d", report.HealthScore)
	}
	if report.SafeVUCount != 50 {
		t.Errorf("expected safe VU 50, got %d", report.SafeVUCount)
	}
	if len(report.ActionableFixes) == 0 {
		t.Errorf("expected actionable recommendations for healthy system")
	}
}

func TestAnalyze_Pattern1_GatewayTimeout504(t *testing.T) {
	summary := &loadengine.ExecutionSummary{
		TotalRequests: 500,
		TotalErrors:   100,
		StatusCodes: map[int]int64{
			200: 400,
			504: 100,
		},
		Latency: loadengine.LatencyStats{
			P99:  3500 * time.Millisecond,
			Mean: 1200 * time.Millisecond,
		},
		TerminationStatus: loadengine.StatusCompleted,
	}

	report := Analyze(summary, 100, false, nil)
	found := false
	for _, issue := range report.DetectedIssues {
		if issue.ID == "ISSUE-504-TIMEOUT" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ISSUE-504-TIMEOUT detected")
	}
	if report.HealthScore >= 80 {
		t.Errorf("expected degraded health score for 504 timeouts, got %d", report.HealthScore)
	}
	if report.SuggestedConfigPatch == "" {
		t.Errorf("expected suggested config patch for 504 timeout")
	}
}

func TestAnalyze_Pattern2_ServerCrash502(t *testing.T) {
	summary := &loadengine.ExecutionSummary{
		TotalRequests: 300,
		TotalErrors:   150,
		StatusCodes: map[int]int64{
			200: 150,
			502: 150,
		},
		Latency: loadengine.LatencyStats{
			P99:  500 * time.Millisecond,
			Mean: 100 * time.Millisecond,
		},
		TerminationStatus: loadengine.StatusAbortedByCircuitBreaker,
		AbortReason:       "error rate 50.0% breached threshold",
	}

	report := Analyze(summary, 200, false, nil)
	found := false
	for _, issue := range report.DetectedIssues {
		if issue.ID == "ISSUE-502-CRASH" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ISSUE-502-CRASH detected")
	}
	if report.HealthScore > 50 {
		t.Errorf("expected low health score for server crash, got %d", report.HealthScore)
	}
	if report.SafeVUCount >= 200 {
		t.Errorf("safe VU count should be significantly degraded, got %d", report.SafeVUCount)
	}
}

func TestAnalyze_Pattern3_RateLimiting429(t *testing.T) {
	summary := &loadengine.ExecutionSummary{
		TotalRequests: 1000,
		TotalErrors:   200,
		StatusCodes: map[int]int64{
			200: 800,
			429: 200,
		},
		Latency: loadengine.LatencyStats{
			P99:  50 * time.Millisecond,
			Mean: 20 * time.Millisecond,
		},
		TerminationStatus: loadengine.StatusCompleted,
	}

	report := Analyze(summary, 100, false, nil)
	found := false
	for _, issue := range report.DetectedIssues {
		if issue.ID == "ISSUE-429-RATELIMIT" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ISSUE-429-RATELIMIT detected")
	}
	if report.RootCause == "" {
		t.Errorf("expected root cause identified for rate limit")
	}
}

func TestAnalyze_Pattern4_LatencySpike(t *testing.T) {
	summary := &loadengine.ExecutionSummary{
		TotalRequests: 1000,
		TotalErrors:   0,
		StatusCodes: map[int]int64{
			200: 1000,
		},
		Latency: loadengine.LatencyStats{
			P99:  2500 * time.Millisecond,
			Mean: 900 * time.Millisecond,
		},
		TerminationStatus: loadengine.StatusCompleted,
	}

	report := Analyze(summary, 100, false, nil)
	found := false
	for _, issue := range report.DetectedIssues {
		if issue.ID == "ISSUE-LATENCY-SPIKE" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ISSUE-LATENCY-SPIKE detected")
	}
	if report.HealthScore >= 90 {
		t.Errorf("health score should be penalised for high latency, got %d", report.HealthScore)
	}
}

func TestAnalyze_SimulatedChaos(t *testing.T) {
	summary := &loadengine.ExecutionSummary{
		TotalRequests: 400,
		TotalErrors:   10,
		StatusCodes: map[int]int64{
			200: 390,
			500: 10,
		},
		Latency: loadengine.LatencyStats{
			P99:  300 * time.Millisecond,
			Mean: 100 * time.Millisecond,
		},
		TerminationStatus: loadengine.StatusCompleted,
	}

	report := Analyze(summary, 20, true, nil)
	found := false
	for _, issue := range report.DetectedIssues {
		if issue.ID == "ISSUE-CHAOS-3G" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ISSUE-CHAOS-3G detected when isChaos is true")
	}
}
