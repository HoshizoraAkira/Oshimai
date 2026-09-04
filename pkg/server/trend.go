package server

import "fmt"

// TrendReport compares a run against a baseline (typically the previous completed run of the
// same scenario) so regressions surface as "P99 got 18% worse than last run" instead of requiring
// the operator to eyeball two separate dashboards.
type TrendReport struct {
	CurrentRunID       string  `json:"current_run_id"`
	BaselineRunID      string  `json:"baseline_run_id"`
	DeltaP99Pct        float64 `json:"delta_p99_pct"`        // Positive = latency got worse.
	DeltaErrorRatePct  float64 `json:"delta_error_rate_pct"` // Positive = error rate got worse (percentage points).
	DeltaHealthScore   int     `json:"delta_health_score"`   // Positive = healthier than baseline.
	DeltaThroughputPct float64 `json:"delta_throughput_pct"` // Positive = higher actual RPS than baseline.
	Verdict            string  `json:"verdict"`              // Human-readable one-line verdict (Bahasa Indonesia).
	Regressed          bool    `json:"regressed"`
}

// CompareRuns computes a TrendReport for current relative to baseline. Both runs must have a
// completed ExecutionSummary; a nil baseline yields a report explaining no comparison is possible.
func CompareRuns(current, baseline *TestRun) (*TrendReport, error) {
	if current == nil || current.Summary == nil {
		return nil, fmt.Errorf("current run has no execution summary to compare")
	}
	if baseline == nil || baseline.Summary == nil {
		return nil, fmt.Errorf("no baseline run with an execution summary was found to compare against")
	}

	report := &TrendReport{CurrentRunID: current.ID, BaselineRunID: baseline.ID}

	curP99 := float64(current.Summary.Latency.P99)
	baseP99 := float64(baseline.Summary.Latency.P99)
	if baseP99 > 0 {
		report.DeltaP99Pct = ((curP99 - baseP99) / baseP99) * 100
	}

	curErrRate := errorRateOf(current)
	baseErrRate := errorRateOf(baseline)
	report.DeltaErrorRatePct = (curErrRate - baseErrRate) * 100

	if current.Summary.ActualRPS > 0 && baseline.Summary.ActualRPS > 0 {
		report.DeltaThroughputPct = ((current.Summary.ActualRPS - baseline.Summary.ActualRPS) / baseline.Summary.ActualRPS) * 100
	}

	curScore, baseScore := 0, 0
	if current.Diagnostics != nil {
		curScore = current.Diagnostics.HealthScore
	}
	if baseline.Diagnostics != nil {
		baseScore = baseline.Diagnostics.HealthScore
	}
	report.DeltaHealthScore = curScore - baseScore

	report.Regressed = report.DeltaHealthScore < -5 || report.DeltaP99Pct > 15 || report.DeltaErrorRatePct > 5

	switch {
	case report.Regressed:
		report.Verdict = fmt.Sprintf("Regresi terdeteksi dibanding run sebelumnya (%s): P99 %+.0f%%, error rate %+.1fpp, skor kesehatan %+d.", baseline.ID, report.DeltaP99Pct, report.DeltaErrorRatePct, report.DeltaHealthScore)
	case report.DeltaHealthScore > 5 || report.DeltaP99Pct < -15:
		report.Verdict = fmt.Sprintf("Membaik dibanding run sebelumnya (%s): P99 %+.0f%%, skor kesehatan %+d.", baseline.ID, report.DeltaP99Pct, report.DeltaHealthScore)
	default:
		report.Verdict = fmt.Sprintf("Stabil dibanding run sebelumnya (%s), tidak ada perubahan signifikan.", baseline.ID)
	}

	return report, nil
}

func errorRateOf(run *TestRun) float64 {
	if run.Summary == nil || run.Summary.TotalRequests == 0 {
		return 0
	}
	return float64(run.Summary.TotalErrors) / float64(run.Summary.TotalRequests)
}

// FindPreviousCompleted scans runs (ordered oldest-to-newest, as returned by StorageRepository.List)
// for the most recent completed run before beforeID that also has an execution summary.
func FindPreviousCompleted(runs []*TestRun, beforeID string) *TestRun {
	beforeIdx := -1
	for i, r := range runs {
		if r.ID == beforeID {
			beforeIdx = i
			break
		}
	}
	if beforeIdx <= 0 {
		return nil
	}
	for i := beforeIdx - 1; i >= 0; i-- {
		if runs[i].Status == RunStatusCompleted && runs[i].Summary != nil {
			return runs[i]
		}
	}
	return nil
}
