package loadengine

import (
	"fmt"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

// ProfileType defines the load generation strategy.
type ProfileType string

const (
	ProfileFlatVU    ProfileType = "flat_vu"
	ProfileTargetRPS ProfileType = "target_rps"
	ProfileRamping   ProfileType = "ramping"
)

// RampingStage represents a discrete step in a ramping load test profile.
type RampingStage struct {
	Duration  time.Duration `json:"duration" yaml:"duration"`
	TargetVUs int           `json:"target_vus" yaml:"target_vus"`
}

// CircuitBreakerConfig configures real-time safety abort thresholds.
type CircuitBreakerConfig struct {
	Enabled             bool          `json:"enabled" yaml:"enabled"`
	EvaluationInterval  time.Duration `json:"evaluation_interval" yaml:"evaluation_interval"`
	WindowDuration      time.Duration `json:"window_duration" yaml:"window_duration"`
	MinRequests         int           `json:"min_requests" yaml:"min_requests"`
	MaxErrorRate        float64       `json:"max_error_rate" yaml:"max_error_rate"` // e.g. 0.30 for 30%
	MaxP99Latency       time.Duration `json:"max_p99_latency" yaml:"max_p99_latency"`
	ConsecutiveBreaches int           `json:"consecutive_breaches" yaml:"consecutive_breaches"`
	DrainTimeout        time.Duration `json:"drain_timeout" yaml:"drain_timeout"`
}

// DefaultCircuitBreakerConfig provides sensible production defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled:             true,
		EvaluationInterval:  1 * time.Second,
		WindowDuration:      5 * time.Second,
		MinRequests:         10,
		MaxErrorRate:        0.30, // 30% error threshold
		MaxP99Latency:       3 * time.Second,
		ConsecutiveBreaches: 2,
		DrainTimeout:        5 * time.Second,
	}
}

// EngineConfig holds configuration parameters for orchestrating load tests.
type EngineConfig struct {
	Profile        ProfileType          `json:"profile" yaml:"profile"`
	VUs            int                  `json:"vus" yaml:"vus"`
	TargetRPS      float64              `json:"target_rps" yaml:"target_rps"`
	Duration       time.Duration        `json:"duration" yaml:"duration"`
	Stages         []RampingStage       `json:"stages,omitempty" yaml:"stages,omitempty"`
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
	Client         vusession.HTTPClient `json:"-" yaml:"-"`
}

// Validate validates the configuration parameters.
func (c *EngineConfig) Validate() error {
	if c.Profile == "" {
		return fmt.Errorf("profile cannot be empty")
	}

	switch c.Profile {
	case ProfileFlatVU:
		if c.VUs <= 0 {
			return fmt.Errorf("flat_vu profile requires VUs > 0")
		}
		if c.Duration <= 0 {
			return fmt.Errorf("duration must be > 0")
		}

	case ProfileTargetRPS:
		if c.TargetRPS <= 0 {
			return fmt.Errorf("target_rps profile requires TargetRPS > 0")
		}
		if c.Duration <= 0 {
			return fmt.Errorf("duration must be > 0")
		}

	case ProfileRamping:
		if len(c.Stages) == 0 {
			return fmt.Errorf("ramping profile requires at least one stage")
		}
		for i, s := range c.Stages {
			if s.Duration <= 0 {
				return fmt.Errorf("stage %d duration must be > 0", i)
			}
			if s.TargetVUs < 0 {
				return fmt.Errorf("stage %d target VUs must be >= 0", i)
			}
		}

	default:
		return fmt.Errorf("unsupported profile: %s", c.Profile)
	}

	return nil
}

// TerminationStatus indicates how the test finished.
type TerminationStatus string

const (
	StatusCompleted               TerminationStatus = "completed"
	StatusAbortedByCircuitBreaker TerminationStatus = "aborted_by_circuit_breaker"
	StatusContextCancelled        TerminationStatus = "context_cancelled"
)

// LatencyStats summarizes request latency quantiles.
type LatencyStats struct {
	Min  time.Duration `json:"min"`
	Mean time.Duration `json:"mean"`
	P50  time.Duration `json:"p50"`
	P90  time.Duration `json:"p90"`
	P95  time.Duration `json:"p95"`
	P99  time.Duration `json:"p99"`
	Max  time.Duration `json:"max"`
}

// StepSummary compiles performance metrics for an individual scenario step.
type StepSummary struct {
	StepID        string        `json:"step_id"`
	TotalRequests int64         `json:"total_requests"`
	TotalErrors   int64         `json:"total_errors"`
	ErrorRate     float64       `json:"error_rate"`
	Latency       LatencyStats  `json:"latency"`
	StatusCodes   map[int]int64 `json:"status_codes"`
}

// ExecutionSummary contains comprehensive execution results.
type ExecutionSummary struct {
	TotalRequests     int64                  `json:"total_requests"`
	TotalErrors       int64                  `json:"total_errors"`
	ActualRPS         float64                `json:"actual_rps"`
	TotalDuration     time.Duration          `json:"total_duration"`
	StatusCodes       map[int]int64          `json:"status_codes"`
	Latency           LatencyStats           `json:"latency"`
	TerminationStatus TerminationStatus      `json:"termination_status"`
	AbortReason       string                 `json:"abort_reason,omitempty"`
	StepMetrics       map[string]StepSummary `json:"step_metrics,omitempty"`
}
