package benchmark

import "testing"

func TestAggregatorInsufficientSamples(t *testing.T) {
	agg := NewAggregator()
	report := agg.Submit(Submission{Category: "ecommerce", HealthScore: 90, P99Ms: 100})
	if report.SampleCount != 0 {
		t.Errorf("expected 0 prior samples on the first submission, got %d", report.SampleCount)
	}
	if report.Message == "" {
		t.Error("expected a message explaining insufficient data")
	}
}

func TestAggregatorComputesPercentiles(t *testing.T) {
	agg := NewAggregator()
	// Seed the pool with 4 prior submissions of varying quality.
	agg.Submit(Submission{Category: "ecommerce", HealthScore: 50, P99Ms: 500})
	agg.Submit(Submission{Category: "ecommerce", HealthScore: 60, P99Ms: 400})
	agg.Submit(Submission{Category: "ecommerce", HealthScore: 70, P99Ms: 300})
	agg.Submit(Submission{Category: "ecommerce", HealthScore: 80, P99Ms: 200})

	// A new submission that beats all 4 prior ones on both dimensions.
	report := agg.Submit(Submission{Category: "ecommerce", HealthScore: 95, P99Ms: 50})
	if report.SampleCount != 4 {
		t.Fatalf("expected 4 prior samples, got %d", report.SampleCount)
	}
	if report.HealthScorePercentile != 100 {
		t.Errorf("expected 100%% health score percentile (beats all 4), got %.1f", report.HealthScorePercentile)
	}
	if report.LatencyPercentile != 100 {
		t.Errorf("expected 100%% latency percentile (faster than all 4), got %.1f", report.LatencyPercentile)
	}
}

func TestAggregatorComparesWithoutSubmitting(t *testing.T) {
	agg := NewAggregator()
	agg.Submit(Submission{Category: "fintech", HealthScore: 50, P99Ms: 500})
	agg.Submit(Submission{Category: "fintech", HealthScore: 60, P99Ms: 400})
	agg.Submit(Submission{Category: "fintech", HealthScore: 70, P99Ms: 300})

	before := agg.Compare("fintech", 90, 100)
	after := agg.Compare("fintech", 90, 100)
	if before.SampleCount != after.SampleCount {
		t.Error("expected Compare to not mutate the pool (repeated calls should see the same sample count)")
	}
	if before.SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", before.SampleCount)
	}
}

func TestAggregatorCategoriesAreIsolated(t *testing.T) {
	agg := NewAggregator()
	agg.Submit(Submission{Category: "ecommerce", HealthScore: 90, P99Ms: 100})
	agg.Submit(Submission{Category: "ecommerce", HealthScore: 90, P99Ms: 100})
	agg.Submit(Submission{Category: "ecommerce", HealthScore: 90, P99Ms: 100})

	report := agg.Compare("fintech", 50, 500)
	if report.SampleCount != 0 {
		t.Errorf("expected fintech pool to be unaffected by ecommerce submissions, got %d samples", report.SampleCount)
	}
}

func TestAggregatorDefaultsEmptyCategory(t *testing.T) {
	agg := NewAggregator()
	agg.Submit(Submission{HealthScore: 80, P99Ms: 100})
	report := agg.Compare("other", 80, 100)
	if report.SampleCount != 1 {
		t.Errorf("expected an empty category to fall back to \"other\", got %d samples", report.SampleCount)
	}
}
