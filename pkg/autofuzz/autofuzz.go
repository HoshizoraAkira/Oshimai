// Package autofuzz implements the Auto Chaos Fuzzer: instead of an operator guessing which
// latency/packet-loss/corruption values to inject, it bisects a single "severity" scalar in
// [0, 1] — scaled proportionally across every fault dimension — to find the minimal chaos
// severity that first breaks the application's health score. Existing chaos-engineering tools
// (Chaos Mesh, Gremlin, LitmusChaos) all require the operator to choose the fault parameters;
// none of them search the fault space automatically for the breaking point.
package autofuzz

import (
	"context"
	"fmt"
	"time"

	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/search"
)

// SeverityRange defines the maximum value each fault dimension reaches at severity 1.0; a given
// trial scales every dimension by the same severity so the search remains one-dimensional.
type SeverityRange struct {
	MaxLatency           time.Duration
	MaxJitter            time.Duration
	MaxLossPercent       float64
	MaxCorruptionPercent float64
}

// DefaultSeverityRange offers a reasonable starting range for a typical HTTP API: from perfectly
// healthy network conditions up to a badly degraded mobile connection.
func DefaultSeverityRange() SeverityRange {
	return SeverityRange{
		MaxLatency:           2 * time.Second,
		MaxJitter:            500 * time.Millisecond,
		MaxLossPercent:       40,
		MaxCorruptionPercent: 10,
	}
}

// ScaleFault produces a concrete FaultSpec at severity (clamped to [0, 1]).
func (r SeverityRange) ScaleFault(id string, severity float64, duration time.Duration, filter chaos.FilterConfig) chaos.FaultSpec {
	if severity < 0 {
		severity = 0
	}
	if severity > 1 {
		severity = 1
	}
	return chaos.FaultSpec{
		ID:                id,
		Type:              chaos.FaultComposite,
		Filter:            filter,
		Latency:           time.Duration(float64(r.MaxLatency) * severity),
		Jitter:            time.Duration(float64(r.MaxJitter) * severity),
		LossPercent:       r.MaxLossPercent * severity,
		CorruptionPercent: r.MaxCorruptionPercent * severity,
		Duration:          duration,
	}
}

// TrialRunner executes one short load test with fault injected at the given severity and reports
// back a health score (see pkg/remediation) summarizing how the application coped.
type TrialRunner func(ctx context.Context, severity float64, fault chaos.FaultSpec) (healthScore int, err error)

// Config parameterizes a fuzz search.
type Config struct {
	Range           SeverityRange
	TrialDuration   time.Duration
	HealthThreshold int // Health score strictly below this counts as "broken". Suggested default: 60.
	MaxIterations   int
	Tolerance       float64 // Severity resolution to stop at, e.g. 0.05 (5% steps).
	FaultID         string
	Filter          chaos.FilterConfig
}

// DefaultConfig returns sensible defaults for an interactive "find my breaking point" run: 6
// bisection iterations (8 trials total) is normally enough to narrow the breaking point to within
// 5% severity resolution while keeping total wall-clock time bounded for a synchronous API call.
func DefaultConfig() Config {
	return Config{
		Range:           DefaultSeverityRange(),
		TrialDuration:   5 * time.Second,
		HealthThreshold: 60,
		MaxIterations:   6,
		Tolerance:       0.05,
		FaultID:         "autofuzz_trial",
		Filter:          chaos.FilterConfig{Interface: "lo"},
	}
}

// TrialSummary records one severity level tested and its outcome.
type TrialSummary struct {
	Severity    float64         `json:"severity"`
	HealthScore int             `json:"health_score"`
	Pass        bool            `json:"pass"`
	Fault       chaos.FaultSpec `json:"fault"`
}

// Result is the final fuzz-search report.
type Result struct {
	Found            bool             `json:"found"`
	BreakingSeverity float64          `json:"breaking_severity,omitempty"`
	BreakingFault    *chaos.FaultSpec `json:"breaking_fault,omitempty"`
	Trials           []TrialSummary   `json:"trials"`
	Verdict          string           `json:"verdict"`
}

// Run bisects Config.Range for the minimal chaos severity that drops the application's health
// score below Config.HealthThreshold, calling runTrial once per severity level tested.
func Run(ctx context.Context, cfg Config, runTrial TrialRunner) (*Result, error) {
	if runTrial == nil {
		return nil, fmt.Errorf("runTrial function is required")
	}
	if cfg.TrialDuration <= 0 {
		return nil, fmt.Errorf("trial duration must be > 0")
	}

	var trials []TrialSummary

	evaluate := func(ctx context.Context, severity float64) (bool, any, error) {
		fault := cfg.Range.ScaleFault(cfg.FaultID, severity, cfg.TrialDuration, cfg.Filter)
		score, err := runTrial(ctx, severity, fault)
		if err != nil {
			return false, nil, err
		}
		pass := score >= cfg.HealthThreshold
		summary := TrialSummary{Severity: severity, HealthScore: score, Pass: pass, Fault: fault}
		trials = append(trials, summary)
		return pass, summary, nil
	}

	searchReport, err := search.FindBreakingPoint(ctx, search.Config{
		Min: 0, Max: 1, MaxIterations: cfg.MaxIterations, Tolerance: cfg.Tolerance,
	}, evaluate)
	if err != nil {
		return nil, err
	}

	result := &Result{Found: searchReport.Found, Trials: trials}

	if !searchReport.Found {
		result.Verdict = fmt.Sprintf("Aplikasi tetap sehat (skor >= %d) bahkan pada chaos paling parah yang diuji. Coba naikkan MaxLatency/MaxLossPercent untuk mencari batas sesungguhnya.", cfg.HealthThreshold)
		return result, nil
	}

	result.BreakingSeverity = searchReport.BreakingPoint
	breakingFault := cfg.Range.ScaleFault(cfg.FaultID, searchReport.BreakingPoint, cfg.TrialDuration, cfg.Filter)
	result.BreakingFault = &breakingFault
	result.Verdict = fmt.Sprintf(
		"Aplikasi mulai gagal (skor < %d) pada severity chaos ~%.0f%% — sekitar latency %v, packet loss %.1f%%, korupsi %.1f%%.",
		cfg.HealthThreshold, searchReport.BreakingPoint*100, breakingFault.Latency, breakingFault.LossPercent, breakingFault.CorruptionPercent,
	)
	return result, nil
}
