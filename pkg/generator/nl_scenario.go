package generator

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/oshimai/twin/pkg/vusession"
)

// NLScenarioRequest carries a plain-language description of an application's user journey — the
// "vibe coder" onboarding path where even OpenAPI/HAR/Postman is too technical: describe the app
// in a sentence or two and let an LLM draft the scenario.
type NLScenarioRequest struct {
	Description string
	BaseURL     string
	Locale      string
}

var yamlFenceRe = regexp.MustCompile("(?s)```(?:yaml|yml)?\\s*\\n(.*?)```")

const nlScenarioSystemPrompt = `You are an autonomous load-testing scenario architect. Given a short plain-language description of a web application, output ONLY a single YAML document (no prose, no explanation) matching this exact schema:

id: <snake_case_id>
name: <human readable name>
base_url: "<the base url, or leave empty if unknown>"
initial_step_id: <id of the first step>
steps:
  <step_id>:
    id: <step_id>
    name: <human readable step name>
    request:
      method: <GET|POST|PUT|DELETE|PATCH>
      path: <url path, e.g. /api/v1/login>
      headers: {}
      body: "<JSON string body if applicable, else omit>"
    assertions:
      - type: status_in_range
        min_code: 200
        max_code: 299
    transitions:
      - target_step_id: <next step id, or "END">
        probability: 1.0

Infer a realistic sequence of 3-6 steps representing the primary user journey implied by the description (e.g. an e-commerce app implies browse -> add_to_cart -> checkout -> payment). Use realistic REST path conventions. Output nothing but the YAML.`

// GenerateScenarioFromDescription turns a natural-language description into a validated Scenario
// using the supplied LLMClient. Returns a clear, actionable error (not a crash or an empty
// scenario) when no client is configured, so the API surface degrades predictably.
func GenerateScenarioFromDescription(ctx context.Context, client LLMClient, req NLScenarioRequest) (*vusession.Scenario, error) {
	if client == nil {
		return nil, fmt.Errorf("natural-language scenario generation requires an LLM client — set BYNARA_API_KEY on the Oshimai server to enable this feature")
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, fmt.Errorf("description cannot be empty")
	}

	userPrompt := req.Description
	if req.BaseURL != "" {
		userPrompt += fmt.Sprintf("\n\nThe base URL is: %s", req.BaseURL)
	}

	raw, err := client.Complete(ctx, nlScenarioSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	yamlDoc := extractYAML(raw)
	sc, err := vusession.ParseScenarioYAML([]byte(yamlDoc))
	if err != nil {
		return nil, fmt.Errorf("LLM produced a scenario that failed validation: %w", err)
	}

	if req.BaseURL != "" {
		sc.BaseURL = req.BaseURL
	}
	if req.Locale == "id" {
		for _, step := range sc.Steps {
			if step.Request.Body != "" {
				step.Request.Body = LocalizeIndonesian(step.Request.Body)
			}
		}
	}

	return sc, nil
}

// extractYAML pulls a fenced ```yaml ... ``` block out of raw if present, otherwise returns raw
// trimmed — LLMs frequently wrap output in markdown fences even when explicitly told not to.
func extractYAML(raw string) string {
	if m := yamlFenceRe.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	return strings.TrimSpace(raw)
}
