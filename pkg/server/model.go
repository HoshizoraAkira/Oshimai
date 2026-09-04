package server

import (
	"time"

	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/generator"
	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/remediation"
	"github.com/oshimai/twin/pkg/vusession"
)

// RunStatus denotes the current execution phase of a test run.
type RunStatus string

const (
	RunStatusAwaitingApproval RunStatus = "awaiting_approval"
	RunStatusQueued           RunStatus = "queued"
	RunStatusRunning          RunStatus = "running"
	RunStatusCompleted        RunStatus = "completed"
	RunStatusAborted          RunStatus = "aborted"
	RunStatusFailed           RunStatus = "failed"
)

// ChaosPlan defines scheduled fault injection during load execution.
type ChaosPlan struct {
	Enabled       bool            `json:"enabled" yaml:"enabled"`
	ScheduleDelay time.Duration   `json:"schedule_delay" yaml:"schedule_delay"` // Wait before applying fault
	Fault         chaos.FaultSpec `json:"fault" yaml:"fault"`
}

// CreateRunRequest encapsulates all parameters needed to launch an autonomous test run.
type CreateRunRequest struct {
	ScenarioYAML string                  `json:"scenario_yaml,omitempty" yaml:"scenario_yaml,omitempty"`
	ScenarioJSON string                  `json:"scenario_json,omitempty" yaml:"scenario_json,omitempty"`
	OpenAPISpec  string                  `json:"openapi_spec,omitempty" yaml:"openapi_spec,omitempty"`
	OTelTraces   string                  `json:"otel_traces,omitempty" yaml:"otel_traces,omitempty"`
	LoadConfig   loadengine.EngineConfig `json:"load_config" yaml:"load_config"`
	ChaosPlan    ChaosPlan               `json:"chaos_plan,omitempty" yaml:"chaos_plan,omitempty"`

	// TargetBaseURL declares the target host being exercised, independent of where BaseURL is
	// embedded (scenario YAML/JSON or generator config). Used for target-ownership verification
	// and for the "guarded production" approval gate below.
	TargetBaseURL string `json:"target_base_url,omitempty" yaml:"target_base_url,omitempty"`

	// Environment labels the run so the control plane can require a second approver before
	// executing anything against production (see CoordinatorConfig.RequireApprovalForProduction).
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"` // "staging" | "production"

	// BusinessContext, when supplied, lets the remediation engine translate error rates into an
	// estimated Rupiah revenue-loss figure instead of just technical metrics.
	BusinessContext *remediation.BusinessContext `json:"business_context,omitempty" yaml:"business_context,omitempty"`

	// BenchmarkCategory opts this run into the anonymous cross-customer benchmark: on completion,
	// only health score / P99 / error rate / RPS are submitted to the category's pool — never the
	// target URL, payload, or any other identifying data. Empty (the default) means "don't share".
	BenchmarkCategory string `json:"benchmark_category,omitempty" yaml:"benchmark_category,omitempty"`
}

// TestRun represents a tracked test execution instance.
type TestRun struct {
	ID          string                        `json:"id"`
	Status      RunStatus                     `json:"status"`
	Config      CreateRunRequest              `json:"config"`
	StartTime   time.Time                     `json:"start_time,omitempty"`
	EndTime     time.Time                     `json:"end_time,omitempty"`
	Summary     *loadengine.ExecutionSummary  `json:"summary,omitempty"`
	Diagnostics *remediation.DiagnosticReport `json:"diagnostics,omitempty"`
	Error       string                        `json:"error,omitempty"`

	// Scenario is the resolved step graph this run actually executed (direct YAML/JSON, or the
	// on-the-fly synthesis output) — the dashboard's Flow Heatmap Report renders this alongside
	// Summary.StepMetrics to plot per-step latency/error hotspots on the request flow.
	Scenario *vusession.Scenario `json:"scenario,omitempty"`

	// Guarded Production approval workflow.
	ApprovedBy string    `json:"approved_by,omitempty"`
	ApprovedAt time.Time `json:"approved_at,omitempty"`

	// ShareToken enables a read-only public status link for this run (empty until requested).
	ShareToken string `json:"-"`
}

// TelemetryEvent carries real-time streaming performance metrics for a specific test run.
type TelemetryEvent struct {
	RunID        string        `json:"run_id"`
	Timestamp    time.Time     `json:"timestamp"`
	CurrentRPS   float64       `json:"current_rps"`
	ActiveVUs    int           `json:"active_vus"`
	ErrorRate    float64       `json:"error_rate"`
	ErrorCount   int64         `json:"error_count"`
	LatencyP50   time.Duration `json:"latency_p50"`
	LatencyP90   time.Duration `json:"latency_p90"`
	LatencyP99   time.Duration `json:"latency_p99"`
	ChaosActive  bool          `json:"chaos_active"`
	ChaosFaultID string        `json:"chaos_fault_id,omitempty"`
	Status       string        `json:"status,omitempty"`
}

// GenerateScenarioRequest is the payload for on-the-fly scenario synthesis.
type GenerateScenarioRequest struct {
	OpenAPISpec string                    `json:"openapi_spec"`
	OTelTraces  string                    `json:"otel_traces,omitempty"`
	Config      generator.GeneratorConfig `json:"config"`
}
