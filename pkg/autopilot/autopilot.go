// Package autopilot finds an application's exact safe concurrency capacity by bisecting virtual
// user counts instead of requiring an operator to guess a VU number and eyeball whether the
// result "looks okay". Every other load-testing tool (k6, Gatling, Locust, JMeter) runs the
// profile you hand it; none of them search for the breaking point on your behalf.
package autopilot

import (
	"context"
	"fmt"

	"github.com/oshimai/twin/pkg/search"
)

// TrialRunner executes one short load test at the given VU count and reports the resulting
// health score (see pkg/remediation).
type TrialRunner func(ctx context.Context, vus int) (healthScore int, err error)

// Config parameterizes a capacity search.
type Config struct {
	MinVUs          int
	MaxVUs          int
	HealthThreshold int // Health score strictly below this counts as "broken". Suggested default: 70.
	MaxIterations   int
	ToleranceVUs    int // Stop once the search window narrows to this many VUs, e.g. 2.
}

// DefaultConfig offers sensible defaults for an interactive search between 1 and 500 concurrent
// users, narrowing to within 5 VUs in at most 8 bisection trials.
func DefaultConfig() Config {
	return Config{MinVUs: 1, MaxVUs: 500, HealthThreshold: 70, MaxIterations: 8, ToleranceVUs: 5}
}

// TrialSummary records one VU count tested and its outcome.
type TrialSummary struct {
	VUs         int  `json:"vus"`
	HealthScore int  `json:"health_score"`
	Pass        bool `json:"pass"`
}

// Result is the final capacity-search report.
type Result struct {
	// Found is true when the application broke somewhere within [MinVUs, MaxVUs]. If false, it
	// withstood MaxVUs cleanly — MaxVUs is a lower bound on true capacity, not the real ceiling.
	Found       bool           `json:"found"`
	SafeMaxVUs  int            `json:"safe_max_vus"`
	BreakingVUs int            `json:"breaking_vus,omitempty"`
	Trials      []TrialSummary `json:"trials"`
	Verdict     string         `json:"verdict"`
}

// Run bisects [cfg.MinVUs, cfg.MaxVUs] for the largest concurrency the application serves while
// keeping its health score at or above cfg.HealthThreshold.
func Run(ctx context.Context, cfg Config, runTrial TrialRunner) (*Result, error) {
	if runTrial == nil {
		return nil, fmt.Errorf("runTrial function is required")
	}

	var trials []TrialSummary
	lastPassingVUs := cfg.MinVUs

	evaluate := func(ctx context.Context, x float64) (bool, any, error) {
		vus := int(x + 0.5) // Round to nearest whole VU.
		if vus < 1 {
			vus = 1
		}
		score, err := runTrial(ctx, vus)
		if err != nil {
			return false, nil, err
		}
		pass := score >= cfg.HealthThreshold
		if pass && vus > lastPassingVUs {
			lastPassingVUs = vus
		}
		summary := TrialSummary{VUs: vus, HealthScore: score, Pass: pass}
		trials = append(trials, summary)
		return pass, summary, nil
	}

	searchReport, err := search.FindBreakingPoint(ctx, search.Config{
		Min: float64(cfg.MinVUs), Max: float64(cfg.MaxVUs),
		MaxIterations: cfg.MaxIterations, Tolerance: float64(cfg.ToleranceVUs),
	}, evaluate)
	if err != nil {
		return nil, err
	}

	result := &Result{Found: searchReport.Found, Trials: trials, SafeMaxVUs: lastPassingVUs}

	if !searchReport.Found {
		result.SafeMaxVUs = cfg.MaxVUs
		result.Verdict = fmt.Sprintf("Aplikasi tetap sehat (skor >= %d) bahkan pada %d VU, batas MaxVUs yang diuji. Naikkan MaxVUs untuk mencari batas sesungguhnya.", cfg.HealthThreshold, cfg.MaxVUs)
		return result, nil
	}

	result.BreakingVUs = int(searchReport.BreakingPoint + 0.5)
	result.Verdict = fmt.Sprintf("Kapasitas aman maksimum: ~%d pengguna bersamaan. Mulai dari ~%d pengguna, skor kesehatan turun di bawah %d.", result.SafeMaxVUs, result.BreakingVUs, cfg.HealthThreshold)
	return result, nil
}
