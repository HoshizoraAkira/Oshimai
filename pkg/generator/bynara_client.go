// Package generator — bynara_client.go
//
// BynamaClient implements LLMClient against the Bynara AI router, which exposes an
// OpenAI-compatible /v1/chat/completions endpoint. The base URL is
// https://router.bynara.id/v1 and the key is supplied as a Bearer token in the
// Authorization header.
//
// Free models available on the plan (as of 2026-09):
//   - agnes-2.0-flash  (Vision)
//   - agnes-2.5-flash  (Vision)
//   - laguna-s-2.1     (Text)
//   - minimax-m3-free  (Vision)
//   - mistral-large    (Text)
//   - mistral-medium-3-5 (Vision)
//   - qwen3.8-27b      (Text)
//   - stepfun-3.7-flash (Vision)
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

const (
	// BynamaBaseURL is the OpenAI-compatible router provided by Bynara.
	BynamaBaseURL = "https://router.bynara.id/v1"

	// BynamaDefaultModel is the default free text model used when BYNARA_MODEL is not set.
	// Chosen for its large 1M context window and zero cost.
	BynamaDefaultModel = "mistral-medium-3-5"
)

// BynamaClient implements LLMClient using the Bynara AI router (OpenAI-compatible).
// It is only ever constructed when a BYNARA_API_KEY is configured; everything else in this
// package degrades gracefully to heuristics when no LLMClient is available.
type BynamaClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
	baseURL    string // overrideable for tests
}

// NewBynamaClient creates a BynamaClient for the given API key and model.
// If model is empty, BynamaDefaultModel is used.
func NewBynamaClient(apiKey, model string) *BynamaClient {
	if model == "" {
		model = BynamaDefaultModel
	}
	return &BynamaClient{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    BynamaBaseURL,
	}
}

// NewBynamaClientFromEnv returns a client using the BYNARA_API_KEY environment variable,
// or nil if it is not set — the caller should treat nil as "AI feature disabled".
// The optional BYNARA_MODEL environment variable overrides the default model.
func NewBynamaClientFromEnv(getenv func(string) string) *BynamaClient {
	key := getenv("BYNARA_API_KEY")
	if key == "" {
		return nil
	}
	model := getenv("BYNARA_MODEL")
	return NewBynamaClient(key, model)
}

// openAIChatRequest is the standard OpenAI /v1/chat/completions request body.
type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// Complete implements LLMClient by sending a chat completion request to the Bynara router
// and returning the assistant's first content block.
func (c *BynamaClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []openAIChatMessage{}
	if systemPrompt != "" {
		messages = append(messages, openAIChatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, openAIChatMessage{Role: "user", Content: userPrompt})

	reqBody := openAIChatRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: 4096,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("bynara: failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("bynara: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("bynara: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("bynara: failed to read response: %w", err)
	}

	var parsed openAIChatResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("bynara: failed to parse response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("bynara API error: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bynara: unexpected status %d: %s", resp.StatusCode, string(data))
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("bynara: empty response from model %q", c.model)
	}
	return parsed.Choices[0].Message.Content, nil
}
