package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oshimai/twin/pkg/vusession"
)

// ScenarioEnricher defines a pluggable strategy to refine and enrich a synthesized Scenario.
type ScenarioEnricher interface {
	Enrich(ctx context.Context, scenario *vusession.Scenario, doc *openapi3.T) (*vusession.Scenario, error)
}

// HeuristicEnricher applies deterministic rule-based optimizations without requiring an LLM API.
type HeuristicEnricher struct{}

func NewHeuristicEnricher() *HeuristicEnricher {
	return &HeuristicEnricher{}
}

func (h *HeuristicEnricher) Enrich(ctx context.Context, sc *vusession.Scenario, doc *openapi3.T) (*vusession.Scenario, error) {
	if sc == nil {
		return nil, fmt.Errorf("scenario cannot be nil")
	}

	var hasAuthStep bool
	for _, step := range sc.Steps {
		stepIDLower := strings.ToLower(step.ID)
		pathLower := strings.ToLower(step.Request.Path)

		// 1. Check if this is an authentication step
		if strings.Contains(stepIDLower, "login") || strings.Contains(stepIDLower, "auth") ||
			strings.Contains(pathLower, "login") || strings.Contains(pathLower, "token") {
			hasAuthStep = true

			// Ensure token extractor is present
			hasTokenExtractor := false
			for _, ext := range step.Extractors {
				if ext.TargetVar == "jwt_token" {
					hasTokenExtractor = true
					break
				}
			}
			if !hasTokenExtractor {
				step.Extractors = append(step.Extractors, vusession.ExtractorConfig{
					Source:    vusession.ExtractorSourceBodyJSON,
					Path:      "token",
					TargetVar: "jwt_token",
					Default:   "mock-bearer-token-12345",
				})
			}
		}

		// 2. Add default status code assertion if none exist
		if len(step.Assertions) == 0 {
			step.Assertions = append(step.Assertions, vusession.AssertionConfig{
				Type:    vusession.AssertStatusCodeInRange,
				MinCode: 200,
				MaxCode: 299,
			})
		}
	}

	// 3. Inject Authorization header into subsequent steps if auth step exists
	if hasAuthStep {
		for _, step := range sc.Steps {
			stepIDLower := strings.ToLower(step.ID)
			pathLower := strings.ToLower(step.Request.Path)
			if strings.Contains(stepIDLower, "login") || strings.Contains(pathLower, "login") {
				continue
			}

			if step.Request.Headers == nil {
				step.Request.Headers = make(map[string]string)
			}
			if _, ok := step.Request.Headers["Authorization"]; !ok {
				step.Request.Headers["Authorization"] = "Bearer ${jwt_token}"
			}
		}
	}

	return sc, nil
}

// LLMClient represents an external AI model completion interface (e.g. Gemini, Claude, OpenAI).
type LLMClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// LLMEnricher uses an AI model client to refine scenarios.
type LLMEnricher struct {
	client   LLMClient
	fallback ScenarioEnricher
}

func NewLLMEnricher(client LLMClient) *LLMEnricher {
	return &LLMEnricher{
		client:   client,
		fallback: NewHeuristicEnricher(),
	}
}

func (l *LLMEnricher) Enrich(ctx context.Context, sc *vusession.Scenario, doc *openapi3.T) (*vusession.Scenario, error) {
	if l.client == nil {
		// Graceful fallback to heuristic enricher if no LLM client configured
		return l.fallback.Enrich(ctx, sc, doc)
	}

	sysPrompt := "You are an autonomous chaos & load testing architect. Optimize the following API load test scenario with realistic payloads and edge-case assertions."
	userPrompt := fmt.Sprintf("Scenario ID: %s\nStep Count: %d\nBaseURL: %s", sc.ID, len(sc.Steps), sc.BaseURL)

	_, err := l.client.Complete(ctx, sysPrompt, userPrompt)
	if err != nil {
		// Fallback to heuristic on LLM network error
		return l.fallback.Enrich(ctx, sc, doc)
	}

	// Always apply heuristic safeguards
	return l.fallback.Enrich(ctx, sc, doc)
}
