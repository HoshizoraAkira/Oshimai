package vusession

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a custom wrapper around time.Duration that supports string representations
// like "5s", "500ms" in both JSON and YAML.
type Duration time.Duration

func (d Duration) AsDuration() time.Duration {
	return time.Duration(d)
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		if strings.TrimSpace(value) == "" {
			*d = 0
			return nil
		}
		pd, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration string: %w", err)
		}
		*d = Duration(pd)
		return nil
	default:
		return fmt.Errorf("invalid duration format: expected string or number, got %T", v)
	}
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		if strings.TrimSpace(s) == "" {
			*d = 0
			return nil
		}
		pd, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", s, err)
		}
		*d = Duration(pd)
		return nil
	}

	var n int64
	if err := value.Decode(&n); err == nil {
		*d = Duration(time.Duration(n))
		return nil
	}

	return fmt.Errorf("unable to decode duration from YAML node: %v", value.Value)
}

// ExtractorSource specifies the source location in the HTTP response for value extraction.
type ExtractorSource string

const (
	ExtractorSourceBodyJSON   ExtractorSource = "body_json"
	ExtractorSourceHeader     ExtractorSource = "header"
	ExtractorSourceStatusCode ExtractorSource = "status_code"
	ExtractorSourceRegex      ExtractorSource = "regex"
)

// ExtractorConfig defines a rule to extract dynamic data from an HTTP response into the SessionState.
type ExtractorConfig struct {
	Source    ExtractorSource `json:"source" yaml:"source"`
	Path      string          `json:"path,omitempty" yaml:"path,omitempty"`       // Dot-path for body_json or header name
	Regex     string          `json:"regex,omitempty" yaml:"regex,omitempty"`     // Regex pattern with capture group
	TargetVar string          `json:"target_var" yaml:"target_var"`               // Target key in SessionState
	Default   any             `json:"default,omitempty" yaml:"default,omitempty"` // Fallback value if extraction yields nil
}

// AssertionType represents validation types for response verification.
type AssertionType string

const (
	AssertStatusCodeInRange AssertionType = "status_in_range"
	AssertBodyContains      AssertionType = "body_contains"
	AssertHeaderEquals      AssertionType = "header_equals"
)

// AssertionConfig defines a post-step validation criteria.
type AssertionConfig struct {
	Type     AssertionType `json:"type" yaml:"type"`
	MinCode  int           `json:"min_code,omitempty" yaml:"min_code,omitempty"`
	MaxCode  int           `json:"max_code,omitempty" yaml:"max_code,omitempty"`
	Expected any           `json:"expected,omitempty" yaml:"expected,omitempty"`
	Target   string        `json:"target,omitempty" yaml:"target,omitempty"` // Header key or JSON path
}

// Transition defines a Markov branch transition to another Step or termination.
type Transition struct {
	TargetStepID string  `json:"target_step_id" yaml:"target_step_id"`               // Empty or "END" terminates the session
	Probability  float64 `json:"probability,omitempty" yaml:"probability,omitempty"` // 0.0 - 1.0 (relative or absolute)
	Weight       int     `json:"weight,omitempty" yaml:"weight,omitempty"`           // Integer weight (alternative to Probability)
	Condition    string  `json:"condition,omitempty" yaml:"condition,omitempty"`     // Condition expression, e.g. "status == 200"
}

// FailureAction denotes how a step failure should be handled.
type FailureAction string

const (
	FailureActionAbort      FailureAction = "abort"
	FailureActionRetry      FailureAction = "retry"
	FailureActionTransition FailureAction = "transition"
)

// FailurePolicy defines error handling and recovery options for a Step.
type FailurePolicy struct {
	Action         FailureAction `json:"action" yaml:"action"`
	MaxRetries     int           `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
	FallbackStepID string        `json:"fallback_step_id,omitempty" yaml:"fallback_step_id,omitempty"`
}

// RequestConfig specifies the HTTP call parameters for a Step.
type RequestConfig struct {
	Method  string            `json:"method" yaml:"method"`
	Path    string            `json:"path" yaml:"path"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    string            `json:"body,omitempty" yaml:"body,omitempty"`
	Timeout Duration          `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// Step represents a discrete operation in a Virtual User workflow.
type Step struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Request     RequestConfig     `json:"request" yaml:"request"`
	Extractors  []ExtractorConfig `json:"extractors,omitempty" yaml:"extractors,omitempty"`
	Assertions  []AssertionConfig `json:"assertions,omitempty" yaml:"assertions,omitempty"`
	Transitions []Transition      `json:"transitions,omitempty" yaml:"transitions,omitempty"`
	OnFailure   FailurePolicy     `json:"on_failure,omitempty" yaml:"on_failure,omitempty"`

	// ThinkTime, when set, pauses the Virtual User for this long immediately before the step
	// executes — simulating a real user reading/deciding, or reproducing recorded inter-request
	// pacing during shadow-traffic replay. A step with ThinkTime set and no Request.Path performs
	// no HTTP call at all: it is a pure delay node.
	ThinkTime Duration `json:"think_time,omitempty" yaml:"think_time,omitempty"`
}

// Scenario represents the full finite state machine (Markov flow) executed by Virtual Users.
type Scenario struct {
	ID             string            `json:"id" yaml:"id"`
	Name           string            `json:"name" yaml:"name"`
	Description    string            `json:"description,omitempty" yaml:"description,omitempty"`
	BaseURL        string            `json:"base_url" yaml:"base_url"`
	InitialStepID  string            `json:"initial_step_id" yaml:"initial_step_id"`
	DefaultHeaders map[string]string `json:"default_headers,omitempty" yaml:"default_headers,omitempty"`
	Timeout        Duration          `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Steps          map[string]*Step  `json:"steps" yaml:"steps"`
}

// StepResult stores execution telemetry for a single step.
type StepResult struct {
	StepID        string         `json:"step_id"`
	StepName      string         `json:"step_name"`
	StartTime     time.Time      `json:"start_time"`
	Duration      time.Duration  `json:"duration_ms"`
	StatusCode    int            `json:"status_code"`
	Success       bool           `json:"success"`
	Error         string         `json:"error,omitempty"`
	ExtractedVars map[string]any `json:"extracted_vars,omitempty"`
	NextStepID    string         `json:"next_step_id,omitempty"`
}

// SessionReport summarizes the entire execution trace of a Virtual User session.
type SessionReport struct {
	VUID          string         `json:"vu_id"`
	ScenarioID    string         `json:"scenario_id"`
	StartTime     time.Time      `json:"start_time"`
	EndTime       time.Time      `json:"end_time"`
	TotalDuration time.Duration  `json:"total_duration"`
	StepResults   []*StepResult  `json:"step_results"`
	Success       bool           `json:"success"`
	FinalState    map[string]any `json:"final_state"`
	FailureReason string         `json:"failure_reason,omitempty"`
}
