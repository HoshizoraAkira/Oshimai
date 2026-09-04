package generator

import (
	"time"
)

// OTelSpan represents an OpenTelemetry HTTP span parsed from trace logs.
type OTelSpan struct {
	TraceID      string         `json:"trace_id"`
	SpanID       string         `json:"span_id"`
	ParentSpanID string         `json:"parent_span_id,omitempty"`
	Name         string         `json:"name"`
	Method       string         `json:"method"`
	Route        string         `json:"route"`
	StatusCode   int            `json:"status_code"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}

// TransitionMatrix captures empirical Markov transition frequencies, probabilities,
// and average latencies between HTTP endpoints mined from trace logs.
type TransitionMatrix struct {
	Counts           map[string]map[string]int     `json:"counts"`
	Probabilities    map[string]map[string]float64 `json:"probabilities"`
	AvgLatencies     map[string]time.Duration      `json:"avg_latencies"`
	TotalTransitions map[string]int                `json:"total_transitions"`
}

// GeneratorConfig holds configuration options for synthesizing a Scenario.
type GeneratorConfig struct {
	ScenarioID     string            `json:"scenario_id" yaml:"scenario_id"`
	ScenarioName   string            `json:"scenario_name" yaml:"scenario_name"`
	BaseURL        string            `json:"base_url" yaml:"base_url"`
	DefaultHeaders map[string]string `json:"default_headers,omitempty" yaml:"default_headers,omitempty"`
	DefaultTimeout time.Duration     `json:"default_timeout" yaml:"default_timeout"`
	Enricher       ScenarioEnricher  `json:"-" yaml:"-"`

	// Locale controls synthetic payload data generation. "id" produces Indonesian-locale values
	// (nama Nusantara, nomor HP +62, NIK, kota/alamat Indonesia) instead of generic en-US values.
	Locale string `json:"locale,omitempty" yaml:"locale,omitempty"`
}

// EndpointDef represents an abstracted API operation with dependency metadata.
type EndpointDef struct {
	ID                string   `json:"id"`
	Method            string   `json:"method"`
	Path              string   `json:"path"`            // e.g. "/api/v1/items/{id}"
	NormalizedPath    string   `json:"normalized_path"` // e.g. "/api/v1/items/${item_id}"
	OperationID       string   `json:"operation_id"`
	Summary           string   `json:"summary"`
	RequiresAuth      bool     `json:"requires_auth"`
	ProducesAuthToken bool     `json:"produces_auth_token"`
	ProducedVars      []string `json:"produced_vars"`
	RequiredVars      []string `json:"required_vars"`
	DefaultPayload    string   `json:"default_payload,omitempty"`
	ExpectedStatus    int      `json:"expected_status"`
}
