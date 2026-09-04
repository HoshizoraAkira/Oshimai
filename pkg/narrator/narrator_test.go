package narrator

import (
	"context"
	"strings"
	"testing"

	"github.com/oshimai/twin/pkg/generator"
	"github.com/oshimai/twin/pkg/remediation"
	"github.com/oshimai/twin/pkg/scanner"
)

type fakeClient struct {
	response string
	err      error
}

func (f *fakeClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return f.response, f.err
}

func sampleReport() *remediation.DiagnosticReport {
	return &remediation.DiagnosticReport{
		HealthScore: 58,
		StatusLabel: "⚠️ APLIKASI MULAI MACET DI 50 USER",
		Summary:     "Server kamu kewalahan saat melayani 50 user bersamaan.",
		RootCause:   "Query database lambat atau downstream service timeout saat melayani lonjakan request.",
		DetectedIssues: []remediation.DetectedIssue{
			{ID: "ISSUE-504-TIMEOUT", Title: "Gateway Timeout (HTTP 504 / 408)", Category: "Database & Upstream", Severity: remediation.SeverityCritical},
		},
		ActionableFixes: []string{"Analisis query database lambat menggunakan `EXPLAIN ANALYZE`."},
	}
}

func TestNarrateNilReport(t *testing.T) {
	n := Narrate(context.Background(), nil, Input{})
	if n.Source != "heuristic" {
		t.Errorf("expected heuristic source for nil report, got %q", n.Source)
	}
	if n.Text == "" {
		t.Error("expected non-empty text even with no report")
	}
}

func TestNarrateFallsBackWithoutClient(t *testing.T) {
	n := Narrate(context.Background(), nil, Input{Report: sampleReport()})
	if n.Source != "heuristic" {
		t.Errorf("expected heuristic source, got %q", n.Source)
	}
	if !strings.Contains(n.Text, "kewalahan") {
		t.Errorf("expected heuristic text to include the report summary, got %q", n.Text)
	}
}

func TestNarrateUsesLLMWhenAvailable(t *testing.T) {
	client := &fakeClient{response: "  Narasi dari AI.  "}
	n := Narrate(context.Background(), client, Input{Report: sampleReport()})
	if n.Source != "llm" {
		t.Errorf("expected llm source, got %q", n.Source)
	}
	if n.Text != "Narasi dari AI." {
		t.Errorf("expected trimmed LLM text, got %q", n.Text)
	}
}

func TestNarrateFallsBackOnLLMError(t *testing.T) {
	client := &fakeClient{err: context.DeadlineExceeded}
	n := Narrate(context.Background(), client, Input{Report: sampleReport()})
	if n.Source != "heuristic" {
		t.Errorf("expected fallback to heuristic on LLM error, got %q", n.Source)
	}
}

func TestNarrateFallsBackOnEmptyLLMResponse(t *testing.T) {
	client := &fakeClient{response: "   "}
	n := Narrate(context.Background(), client, Input{Report: sampleReport()})
	if n.Source != "heuristic" {
		t.Errorf("expected fallback to heuristic on empty LLM response, got %q", n.Source)
	}
}

func TestCorrelateMatchesScannerFinding(t *testing.T) {
	in := Input{
		Report: sampleReport(),
		Findings: []scanner.Finding{
			{File: "internal/payment/client.go", Line: 47, Rule: "http-client-no-timeout", Severity: scanner.SeverityWarning, Message: "http.Client created without a Timeout"},
			{File: "internal/unrelated.go", Line: 1, Rule: "some-other-rule", Severity: scanner.SeverityWarning, Message: "irrelevant"},
		},
	}
	got := correlate(in)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 correlation, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "internal/payment/client.go:47") {
		t.Errorf("expected correlation to cite the matching finding's location, got %q", got[0])
	}
}

func TestCorrelateIgnoresUnrelatedFindings(t *testing.T) {
	in := Input{
		Report: sampleReport(),
		Findings: []scanner.Finding{
			{File: "internal/unrelated.go", Line: 1, Rule: "some-other-rule", Message: "irrelevant"},
		},
	}
	if got := correlate(in); len(got) != 0 {
		t.Errorf("expected no correlations for unrelated findings, got %v", got)
	}
}

func TestCorrelateIncludesMostCriticalGraphNode(t *testing.T) {
	in := Input{
		Report: sampleReport(),
		Graph: &generator.DependencyGraph{
			MostCritical: []string{"POST /api/v1/checkout", "GET /api/v1/catalog"},
		},
	}
	got := correlate(in)
	found := false
	for _, c := range got {
		if strings.Contains(c, "POST /api/v1/checkout") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a correlation citing the most critical endpoint, got %v", got)
	}
}

func TestCorrelateSkipsGraphWhenNoIssuesDetected(t *testing.T) {
	report := sampleReport()
	report.DetectedIssues = nil
	in := Input{
		Report: report,
		Graph:  &generator.DependencyGraph{MostCritical: []string{"GET /api/v1/catalog"}},
	}
	if got := correlate(in); len(got) != 0 {
		t.Errorf("expected no correlations when there are no detected issues to relate the graph to, got %v", got)
	}
}

func TestHeuristicNarrativeNeverEmpty(t *testing.T) {
	report := &remediation.DiagnosticReport{HealthScore: 100, StatusLabel: "OK"}
	text := heuristicNarrative(Input{Report: report}, nil)
	if text == "" {
		t.Error("expected a non-empty heuristic narrative even with a minimal report")
	}
}
