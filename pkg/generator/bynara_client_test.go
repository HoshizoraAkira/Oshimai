package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBynamaClientComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify route
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		// Verify Authorization header
		if r.Header.Get("Authorization") != "Bearer test-bynara-key" {
			t.Errorf("expected Authorization header 'Bearer test-bynara-key', got %q", r.Header.Get("Authorization"))
		}
		// Verify request body
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model == "" {
			t.Errorf("expected a model to be set in request")
		}
		if len(req.Messages) == 0 {
			t.Errorf("expected at least one message in request")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIChatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "halo dari bynara"}}},
		})
	}))
	defer srv.Close()

	client := NewBynamaClient("test-bynara-key", "mistral-medium-3-5")
	client.baseURL = srv.URL

	out, err := client.Complete(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if out != "halo dari bynara" {
		t.Errorf("expected %q, got %q", "halo dari bynara", out)
	}
}

func TestBynamaClientCompleteSystemPromptOptional(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		// With no system prompt, only user message should be present
		if len(req.Messages) != 1 {
			t.Errorf("expected 1 message (no system), got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "user" {
			t.Errorf("expected user role, got %q", req.Messages[0].Role)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIChatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "ok"}}},
		})
	}))
	defer srv.Close()

	client := NewBynamaClient("key", "laguna-s-2.1")
	client.baseURL = srv.URL

	if _, err := client.Complete(context.Background(), "", "user only"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestBynamaClientCompletePropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(openAIChatResponse{
			Error: &struct {
				Message string `json:"message"`
				Code    string `json:"code,omitempty"`
			}{Message: "invalid api key"},
		})
	}))
	defer srv.Close()

	client := NewBynamaClient("bad-key", "")
	client.baseURL = srv.URL

	if _, err := client.Complete(context.Background(), "", "test"); err == nil {
		t.Error("expected an error for an unauthorized response")
	}
}

func TestNewBynamaClientFromEnv(t *testing.T) {
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }

	// No key → nil
	if c := NewBynamaClientFromEnv(getenv); c != nil {
		t.Error("expected nil client when BYNARA_API_KEY is unset")
	}

	// With key → non-nil, default model
	env["BYNARA_API_KEY"] = "sk-nry-test"
	c := NewBynamaClientFromEnv(getenv)
	if c == nil {
		t.Fatal("expected a client when BYNARA_API_KEY is set")
	}
	if c.model != BynamaDefaultModel {
		t.Errorf("expected default model %q, got %q", BynamaDefaultModel, c.model)
	}

	// With model override
	env["BYNARA_MODEL"] = "qwen3.8-27b"
	c2 := NewBynamaClientFromEnv(getenv)
	if c2.model != "qwen3.8-27b" {
		t.Errorf("expected model override %q, got %q", "qwen3.8-27b", c2.model)
	}
}

func TestNewBynamaClientDefaultModel(t *testing.T) {
	c := NewBynamaClient("key", "")
	if c.model != BynamaDefaultModel {
		t.Errorf("expected default model %q when empty, got %q", BynamaDefaultModel, c.model)
	}
}
