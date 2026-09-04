// Package benchmark implements the anonymous cross-customer resilience benchmark: "your API is
// faster than 73% of similar e-commerce APIs tested this month" — a network-effect feature no
// other load-testing tool offers, because none of them aggregate results across customers at all.
// A single Oshimai deployment can run this itself (comparing your own runs over time), and any
// number of independent deployments can federate by pointing at one shared instance acting as the
// aggregator — the protocol is the same either way: submit an anonymized Submission, get back a
// PercentileReport. Submissions deliberately carry no URL, payload, or other identifying data —
// only the handful of metrics the comparison actually needs.
package benchmark

import (
	"fmt"
	"sync"
	"time"
)

// Submission is one anonymized run result contributed to a category's benchmark pool.
type Submission struct {
	Category         string    `json:"category"` // e.g. "ecommerce", "fintech", "saas_b2b", "other"
	HealthScore      int       `json:"health_score"`
	P99Ms            float64   `json:"p99_ms"`
	ErrorRatePercent float64   `json:"error_rate_percent"`
	ActualRPS        float64   `json:"actual_rps"`
	SubmittedAt      time.Time `json:"submitted_at"`
}

// PercentileReport tells the submitter how their run stacks up against the category's pool.
type PercentileReport struct {
	Category              string  `json:"category"`
	SampleCount           int     `json:"sample_count"`
	HealthScorePercentile float64 `json:"health_score_percentile"` // % of pool this run's health score beats or ties.
	LatencyPercentile     float64 `json:"latency_percentile"`      // % of pool this run's P99 is faster than or ties.
	Message               string  `json:"message"`
}

const minSampleSizeForComparison = 3

// Aggregator holds submissions in memory, grouped by category. It is intentionally simple (no
// persistence, no cross-process sharing) — a real federated deployment persists this the same way
// it persists TestRuns, which is an operational choice outside this package's concern.
type Aggregator struct {
	mu   sync.Mutex
	data map[string][]Submission
}

// NewAggregator creates an empty aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{data: make(map[string][]Submission)}
}

// Submit adds s to its category's pool and returns the resulting PercentileReport, so the
// submitter and the running percentile calculation always come from a single call.
func (a *Aggregator) Submit(s Submission) PercentileReport {
	if s.SubmittedAt.IsZero() {
		s.SubmittedAt = time.Now()
	}
	if s.Category == "" {
		s.Category = "other"
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	report := a.percentileLocked(s.Category, s.HealthScore, s.P99Ms)
	a.data[s.Category] = append(a.data[s.Category], s)
	return report
}

// Compare reports where the given metrics would rank in category's existing pool, without adding
// a new submission — useful for "how would I compare" without committing to sharing this run.
func (a *Aggregator) Compare(category string, healthScore int, p99Ms float64) PercentileReport {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.percentileLocked(category, healthScore, p99Ms)
}

func (a *Aggregator) percentileLocked(category string, healthScore int, p99Ms float64) PercentileReport {
	pool := a.data[category]
	report := PercentileReport{Category: category, SampleCount: len(pool)}

	if len(pool) < minSampleSizeForComparison {
		report.Message = "Belum cukup data pembanding di kategori ini — butuh minimal 3 submission sebelum persentil bisa dihitung."
		return report
	}

	healthScores := make([]float64, len(pool))
	latencies := make([]float64, len(pool))
	for i, s := range pool {
		healthScores[i] = float64(s.HealthScore)
		latencies[i] = s.P99Ms
	}

	report.HealthScorePercentile = percentileRank(healthScores, float64(healthScore))
	// Lower latency is better, so a run's latency percentile is "the share of the pool with a
	// strictly higher (worse) P99" — how many other runs this one is faster than.
	report.LatencyPercentile = fasterThanRank(latencies, p99Ms)

	report.Message = fmt.Sprintf(
		"Dibandingkan %d aplikasi lain di kategori %q: skor kesehatanmu lebih baik dari %.0f%%, dan latensi P99-mu lebih cepat dari %.0f%%.",
		report.SampleCount, report.Category, report.HealthScorePercentile, report.LatencyPercentile,
	)
	return report
}

// percentileRank returns the percentage of values in sample that are <= x.
func percentileRank(sample []float64, x float64) float64 {
	count := 0
	for _, v := range sample {
		if v <= x {
			count++
		}
	}
	return float64(count) / float64(len(sample)) * 100
}

// fasterThanRank returns the percentage of values in sample that are strictly greater than x —
// used for "lower is better" metrics like latency, where beating a sample means being faster
// (smaller), not larger.
func fasterThanRank(sample []float64, x float64) float64 {
	count := 0
	for _, v := range sample {
		if v > x {
			count++
		}
	}
	return float64(count) / float64(len(sample)) * 100
}
