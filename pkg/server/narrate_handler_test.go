package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oshimai/twin/pkg/generator"
	"github.com/oshimai/twin/pkg/narrator"
	"github.com/oshimai/twin/pkg/remediation"
	"github.com/oshimai/twin/pkg/scanner"
)

type fakeNarratorLLMClient struct{ response string }

func (f *fakeNarratorLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return f.response, nil
}

func TestHandlerNarrateRunNotFound(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/runs/does-not-exist/narrate", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown run, got %d", resp.StatusCode)
	}
}

func TestHandlerNarrateRunHeuristicFallbackWithCorrelations(t *testing.T) {
	handler, repo, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	run := &TestRun{
		ID:     "run-narrate-1",
		Status: RunStatusCompleted,
		Diagnostics: &remediation.DiagnosticReport{
			HealthScore: 58,
			StatusLabel: "⚠️ APLIKASI MULAI MACET DI 50 USER",
			Summary:     "Server kewalahan.",
			RootCause:   "Query lambat.",
			DetectedIssues: []remediation.DetectedIssue{
				{ID: "ISSUE-504-TIMEOUT", Title: "Gateway Timeout"},
			},
		},
	}
	if err := repo.Save(context.Background(), run); err != nil {
		t.Fatalf("failed to seed run: %v", err)
	}

	reqBody, _ := json.Marshal(map[string]any{
		"scan_findings": []scanner.Finding{
			{File: "internal/x.go", Line: 10, Rule: "http-client-no-timeout", Message: "no timeout set"},
		},
		"dependency_graph": generator.DependencyGraph{
			MostCritical: []string{"POST /api/v1/checkout"},
		},
	})
	resp, err := http.Post(ts.URL+"/api/v1/runs/"+run.ID+"/narrate", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out narrator.Narrative
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Source != "heuristic" {
		t.Errorf("expected heuristic source without a configured LLM client, got %q", out.Source)
	}
	if out.Text == "" {
		t.Error("expected non-empty narrative text")
	}
	if len(out.Correlations) != 2 {
		t.Errorf("expected 2 correlations (scan finding + critical endpoint), got %d: %v", len(out.Correlations), out.Correlations)
	}
}

func TestHandlerNarrateRunUsesLLM(t *testing.T) {
	handler, repo, _, _ := setupTestEnvironment()
	handler.SetLLMClient(&fakeNarratorLLMClient{response: "Narasi gabungan dari AI."})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	run := &TestRun{
		ID:     "run-narrate-2",
		Status: RunStatusCompleted,
		Diagnostics: &remediation.DiagnosticReport{
			HealthScore: 90,
			StatusLabel: "OK",
			Summary:     "Sehat.",
		},
	}
	if err := repo.Save(context.Background(), run); err != nil {
		t.Fatalf("failed to seed run: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/v1/runs/"+run.ID+"/narrate", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var out narrator.Narrative
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Source != "llm" {
		t.Errorf("expected llm source, got %q", out.Source)
	}
	if out.Text != "Narasi gabungan dari AI." {
		t.Errorf("unexpected narrative text: %q", out.Text)
	}
}

func TestHandlerNarrateRunComputesDiagnosticsWhenMissing(t *testing.T) {
	handler, repo, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	run := &TestRun{ID: "run-narrate-3", Status: RunStatusCompleted}
	if err := repo.Save(context.Background(), run); err != nil {
		t.Fatalf("failed to seed run: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/v1/runs/"+run.ID+"/narrate", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 even when diagnostics were not precomputed, got %d", resp.StatusCode)
	}
	var out narrator.Narrative
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Text == "" {
		t.Error("expected a narrative to be generated from lazily-computed diagnostics")
	}
}
