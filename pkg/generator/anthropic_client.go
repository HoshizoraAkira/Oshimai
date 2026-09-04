package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicClient implements LLMClient against the real Claude Messages API — the concrete
// backing for natural-language scenario generation. It is only ever constructed when an API key
// is configured (see NewAnthropicClientFromEnv); everything else in this package degrades
// gracefully to heuristics when no LLMClient is available, so this stays fully optional.
type AnthropicClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
	baseURL    string
}

// NewAnthropicClient creates a client for the given API key and model.
func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	if model == "" {
		model = "claude-sonnet-5"
	}
	return &AnthropicClient{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    "https://api.anthropic.com/v1/messages",
	}
}

// NewAnthropicClientFromEnv returns a client using the ANTHROPIC_API_KEY environment variable,
// or nil if it isn't set — the caller (e.g. cmd/server) should treat nil as "feature disabled".
func NewAnthropicClientFromEnv(getenv func(string) string) *AnthropicClient {
	key := getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil
	}
	model := getenv("ANTHROPIC_MODEL")
	return NewAnthropicClient(key, model)
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete implements LLMClient by calling the Anthropic Messages API and returning the first
// text block of the response.
func (c *AnthropicClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: userPrompt}},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode Anthropic request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build Anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Anthropic API request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read Anthropic response: %w", err)
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse Anthropic response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API returned status %d", resp.StatusCode)
	}
	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("Anthropic API returned no content")
	}
	return parsed.Content[0].Text, nil
}
