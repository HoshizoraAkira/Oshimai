package generator

import (
	"context"
	"testing"
)

type fakeLLMClient struct {
	response string
	err      error
}

func (f *fakeLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return f.response, f.err
}

const sampleLLMYAML = "```yaml\n" + `id: shop_flow
name: Shop Flow
base_url: ""
initial_step_id: browse
steps:
  browse:
    id: browse
    name: Browse Items
    request:
      method: GET
      path: /api/v1/items
    assertions:
      - type: status_in_range
        min_code: 200
        max_code: 299
    transitions:
      - target_step_id: checkout
        probability: 1.0
  checkout:
    id: checkout
    name: Checkout
    request:
      method: POST
      path: /api/v1/checkout
      body: "{\"item_id\": 1}"
    assertions:
      - type: status_in_range
        min_code: 200
        max_code: 299
    transitions:
      - target_step_id: "END"
        probability: 1.0
` + "```\n"

func TestGenerateScenarioFromDescription(t *testing.T) {
	client := &fakeLLMClient{response: sampleLLMYAML}
	sc, err := GenerateScenarioFromDescription(context.Background(), client, NLScenarioRequest{
		Description: "Aplikasi toko online sederhana dengan browse dan checkout",
		BaseURL:     "https://toko.example.co.id",
	})
	if err != nil {
		t.Fatalf("GenerateScenarioFromDescription failed: %v", err)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(sc.Steps))
	}
	if sc.BaseURL != "https://toko.example.co.id" {
		t.Errorf("expected explicit base URL to override the LLM's, got %s", sc.BaseURL)
	}
}

func TestGenerateScenarioFromDescriptionRequiresClient(t *testing.T) {
	_, err := GenerateScenarioFromDescription(context.Background(), nil, NLScenarioRequest{Description: "x"})
	if err == nil {
		t.Error("expected an error when no LLM client is configured")
	}
}

func TestGenerateScenarioFromDescriptionRequiresDescription(t *testing.T) {
	client := &fakeLLMClient{response: sampleLLMYAML}
	if _, err := GenerateScenarioFromDescription(context.Background(), client, NLScenarioRequest{}); err == nil {
		t.Error("expected an error for an empty description")
	}
}

func TestGenerateScenarioFromDescriptionRejectsInvalidYAML(t *testing.T) {
	client := &fakeLLMClient{response: "this is not yaml at all: [unterminated"}
	if _, err := GenerateScenarioFromDescription(context.Background(), client, NLScenarioRequest{Description: "x"}); err == nil {
		t.Error("expected an error when the LLM output fails scenario validation")
	}
}

func TestExtractYAMLStripsFence(t *testing.T) {
	got := extractYAML("```yaml\nid: x\n```")
	if got != "id: x\n" {
		t.Errorf("expected fence to be stripped, got %q", got)
	}

	got = extractYAML("id: x")
	if got != "id: x" {
		t.Errorf("expected unfenced input to pass through, got %q", got)
	}
}
