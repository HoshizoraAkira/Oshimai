package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicClientComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header to be set")
		}
		var req anthropicRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model == "" {
			t.Errorf("expected a model to be set")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{Type: "text", Text: "hello from claude"}},
		})
	}))
	defer srv.Close()

	client := NewAnthropicClient("test-key", "claude-sonnet-5")
	client.baseURL = srv.URL

	out, err := client.Complete(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if out != "hello from claude" {
		t.Errorf("expected %q, got %q", "hello from claude", out)
	}
}

func TestAnthropicClientCompletePropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "invalid x-api-key"}})
	}))
	defer srv.Close()

	client := NewAnthropicClient("bad-key", "")
	client.baseURL = srv.URL

	if _, err := client.Complete(context.Background(), "s", "u"); err == nil {
		t.Error("expected an error for an unauthorized response")
	}
}

func TestNewAnthropicClientFromEnv(t *testing.T) {
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }

	if c := NewAnthropicClientFromEnv(getenv); c != nil {
		t.Error("expected nil client when ANTHROPIC_API_KEY is unset")
	}

	env["ANTHROPIC_API_KEY"] = "abc123"
	if c := NewAnthropicClientFromEnv(getenv); c == nil {
		t.Error("expected a client when ANTHROPIC_API_KEY is set")
	}
}
