package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
	"gopkg.in/yaml.v3"
)

// Synthesize generates a fully validated vusession.Scenario from an OpenAPI spec and optional OTel traces.
func Synthesize(ctx context.Context, openapiData []byte, otelData []byte, cfg GeneratorConfig) (*vusession.Scenario, error) {
	if len(openapiData) == 0 {
		return nil, fmt.Errorf("openapi data cannot be empty")
	}

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("scenario_%d", time.Now().Unix())
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = "Autonomous Synthesized Scenario"
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	// 1. Parse OpenAPI Document
	endpoints, doc, err := ParseOpenAPISpec(ctx, openapiData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no valid operations found in OpenAPI spec")
	}

	// 2. Mine OTel Traces (if provided)
	var matrix *TransitionMatrix
	if len(otelData) > 0 {
		spans, err := ParseOTelSpansJSON(otelData)
		if err == nil && len(spans) > 0 {
			matrix, _ = MineBehavioralTransitions(spans)
		}
	}

	// Build endpoint lookup maps
	epByKey := make(map[string]*EndpointDef)
	epByID := make(map[string]*EndpointDef)
	for _, ep := range endpoints {
		key := formatEndpointKey(ep.Method, ep.Path)
		epByKey[key] = ep
		epByID[ep.ID] = ep
	}

	// 3. Build Steps
	stepsMap := make(map[string]*vusession.Step)
	var initialStepID string

	for i, ep := range endpoints {
		body := ep.DefaultPayload
		if cfg.Locale == "id" && body != "" {
			body = LocalizeIndonesian(body)
		}

		step := &vusession.Step{
			ID:   ep.ID,
			Name: ep.Summary,
			Request: vusession.RequestConfig{
				Method:  ep.Method,
				Path:    ep.NormalizedPath,
				Headers: make(map[string]string),
				Body:    body,
			},
			Extractors:  make([]vusession.ExtractorConfig, 0),
			Assertions:  make([]vusession.AssertionConfig, 0),
			Transitions: make([]vusession.Transition, 0),
			OnFailure: vusession.FailurePolicy{
				Action: vusession.FailureActionAbort,
			},
		}

		if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" {
			step.Request.Headers["Content-Type"] = "application/json"
		}

		// Configure Extractors for produced variables
		if ep.ProducesAuthToken {
			step.Extractors = append(step.Extractors, vusession.ExtractorConfig{
				Source:    vusession.ExtractorSourceBodyJSON,
				Path:      "token",
				TargetVar: "jwt_token",
				Default:   "mock-bearer-token-12345",
			})
			if initialStepID == "" {
				initialStepID = ep.ID
			}
		}

		for _, pVar := range ep.ProducedVars {
			if pVar == "jwt_token" {
				continue
			}
			step.Extractors = append(step.Extractors, vusession.ExtractorConfig{
				Source:    vusession.ExtractorSourceBodyJSON,
				Path:      "id",
				TargetVar: pVar,
				Default:   "101",
			})
		}

		// Build Transitions (From OTel if available, otherwise sequential graph)
		key := formatEndpointKey(ep.Method, ep.Path)
		var transitionsAdded bool

		if matrix != nil && len(matrix.Probabilities[key]) > 0 {
			for targetKey, prob := range matrix.Probabilities[key] {
				if targetKey == "END" {
					step.Transitions = append(step.Transitions, vusession.Transition{
						TargetStepID: "END",
						Probability:  prob,
					})
					transitionsAdded = true
					continue
				}

				if targetEP, exists := epByKey[targetKey]; exists {
					step.Transitions = append(step.Transitions, vusession.Transition{
						TargetStepID: targetEP.ID,
						Probability:  prob,
					})
					transitionsAdded = true
				}
			}
		}

		if !transitionsAdded {
			// Fallback: sequential progression to next step, or END if last
			if i+1 < len(endpoints) {
				step.Transitions = append(step.Transitions, vusession.Transition{
					TargetStepID: endpoints[i+1].ID,
					Probability:  1.0,
				})
			} else {
				step.Transitions = append(step.Transitions, vusession.Transition{
					TargetStepID: "END",
					Probability:  1.0,
				})
			}
		}

		stepsMap[ep.ID] = step
	}

	if initialStepID == "" {
		initialStepID = endpoints[0].ID
	}

	// 4. Construct Scenario
	scenario := &vusession.Scenario{
		ID:             cfg.ScenarioID,
		Name:           cfg.ScenarioName,
		BaseURL:        cfg.BaseURL,
		InitialStepID:  initialStepID,
		DefaultHeaders: cfg.DefaultHeaders,
		Timeout:        vusession.Duration(cfg.DefaultTimeout),
		Steps:          stepsMap,
	}

	// 5. Enrich Scenario (Heuristic or LLM)
	enricher := cfg.Enricher
	if enricher == nil {
		enricher = NewHeuristicEnricher()
	}

	enrichedSc, err := enricher.Enrich(ctx, scenario, doc)
	if err != nil {
		return nil, fmt.Errorf("scenario enrichment failed: %w", err)
	}

	// 6. Validate topology
	if err := vusession.ValidateScenario(enrichedSc); err != nil {
		return nil, fmt.Errorf("synthesized scenario validation failed: %w", err)
	}

	return enrichedSc, nil
}

// ExportYAML serializes a Scenario to YAML format.
func ExportYAML(sc *vusession.Scenario) ([]byte, error) {
	return yaml.Marshal(sc)
}

// ExportJSON serializes a Scenario to formatted JSON.
func ExportJSON(sc *vusession.Scenario) ([]byte, error) {
	return json.MarshalIndent(sc, "", "  ")
}
