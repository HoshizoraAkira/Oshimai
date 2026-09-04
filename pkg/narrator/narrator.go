// Package narrator turns Oshimai's three independently-generated reports — a run's
// remediation.DiagnosticReport, an optional pkg/scanner static-analysis pass over the target's
// repo, and an optional generator.DependencyGraph mined from real traffic — into one coherent
// incident narrative that explicitly cross-references them, instead of leaving an operator to
// manually notice that "P99 spiked" (the run), "http.Client has no timeout" (the scanner), and
// "payment-svc carries 62% of traffic" (the graph) are the same story.
package narrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oshimai/twin/pkg/generator"
	"github.com/oshimai/twin/pkg/remediation"
	"github.com/oshimai/twin/pkg/scanner"
)

// LLMClient represents an external AI model completion interface. Declared locally rather than
// imported from pkg/generator so this package depends on only the one method it calls; every
// generator.LLMClient implementation (BynamaClient, AnthropicClient) already satisfies it
// structurally, no adapter required.
type LLMClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Input bundles everything the narrator can draw on. Findings and Graph are optional — the
// narrative degrades gracefully when they're not supplied, it just has less to cross-reference.
type Input struct {
	Report   *remediation.DiagnosticReport
	Findings []scanner.Finding
	Graph    *generator.DependencyGraph
}

// Narrative is the synthesized result.
type Narrative struct {
	Text         string   `json:"narrative"`              // Prose incident narrative, casual Indonesian.
	Correlations []string `json:"correlations,omitempty"` // Explicit "runtime issue <-> static/graph finding" links.
	Source       string   `json:"source"`                 // "llm" or "heuristic" — which path produced Text.
}

// issueScannerRules maps a remediation.DetectedIssue.ID to the scanner rule names that plausibly
// explain it, so a correlation is only drawn when there's a real mechanistic link, not just
// because two lists happen to both be non-empty.
var issueScannerRules = map[string][]string{
	"ISSUE-504-TIMEOUT":   {"http-client-no-timeout", "db-pool-unbounded", "context-background-in-request"},
	"ISSUE-502-CRASH":     {"unbounded-retry-loop", "db-pool-unbounded"},
	"ISSUE-LATENCY-SPIKE": {"db-pool-unbounded", "http-client-no-timeout"},
	"ISSUE-429-RATELIMIT": {},
	"ISSUE-CHAOS-3G":      {},
}

// Narrate produces a Narrative from in. When client is nil, or the completion call fails or
// returns an empty string, it falls back to a deterministic templated narrative built from the
// same Input — mirroring generator.LLMEnricher's fallback behavior — so the feature never returns
// nothing just because no LLM is configured or the router is briefly unreachable.
func Narrate(ctx context.Context, client LLMClient, in Input) *Narrative {
	if in.Report == nil {
		return &Narrative{Text: "Tidak ada laporan diagnostik untuk dianalisis.", Source: "heuristic"}
	}

	correlations := correlate(in)

	if client == nil {
		return &Narrative{Text: heuristicNarrative(in, correlations), Correlations: correlations, Source: "heuristic"}
	}

	text, err := client.Complete(ctx, systemPrompt(in), userPrompt(in, correlations))
	if err != nil || strings.TrimSpace(text) == "" {
		return &Narrative{Text: heuristicNarrative(in, correlations), Correlations: correlations, Source: "heuristic"}
	}

	return &Narrative{Text: strings.TrimSpace(text), Correlations: correlations, Source: "llm"}
}

// correlate finds mechanistic links between the run's detected issues, the static scan findings,
// and the dependency graph's most critical endpoint — the connections an operator would otherwise
// have to notice by reading three separate reports side by side.
func correlate(in Input) []string {
	var out []string

	for _, issue := range in.Report.DetectedIssues {
		rules := issueScannerRules[issue.ID]
		if len(rules) == 0 {
			continue
		}
		for _, f := range in.Findings {
			for _, rule := range rules {
				if f.Rule == rule {
					loc := f.File
					if f.Line > 0 {
						loc = fmt.Sprintf("%s:%d", f.File, f.Line)
					}
					out = append(out, fmt.Sprintf(
						"%s (dari load test) berkorelasi dengan temuan scanner di %s — %s",
						issue.Title, loc, f.Message,
					))
				}
			}
		}
	}

	if in.Graph != nil && len(in.Graph.MostCritical) > 0 && len(in.Report.DetectedIssues) > 0 {
		out = append(out, fmt.Sprintf(
			"Endpoint paling kritis di dependency graph adalah %s — prioritaskan ini saat menindaklanjuti temuan di atas.",
			in.Graph.MostCritical[0],
		))
	}

	return out
}

func systemPrompt(in Input) string {
	return fmt.Sprintf(
		"You are a senior SRE writing a short incident narrative in casual Bahasa Indonesia for a "+
			"non-technical stakeholder, matching the tone of this existing summary: %q. "+
			"You are given a load-test diagnostic report, an optional static code-scan pass, and an "+
			"optional traffic dependency graph, as JSON. Explicitly cross-reference them by name where "+
			"they plausibly relate to the same root cause — do not just restate each list separately. "+
			"If a section is empty, do not mention that it is empty. Keep it under 150 words, plain "+
			"prose, no markdown headers or bullet lists.",
		in.Report.Summary,
	)
}

func userPrompt(in Input, correlations []string) string {
	payload := map[string]any{
		"health_score":                 in.Report.HealthScore,
		"status_label":                 in.Report.StatusLabel,
		"root_cause":                   in.Report.RootCause,
		"detected_issues":              in.Report.DetectedIssues,
		"scan_findings":                capFindings(in.Findings, 10),
		"most_critical_endpoints":      criticalEndpoints(in.Graph, 5),
		"already_noticed_correlations": correlations,
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return string(b)
}

func capFindings(findings []scanner.Finding, max int) []scanner.Finding {
	if len(findings) <= max {
		return findings
	}
	return findings[:max]
}

func criticalEndpoints(g *generator.DependencyGraph, max int) []string {
	if g == nil {
		return nil
	}
	if len(g.MostCritical) <= max {
		return g.MostCritical
	}
	return g.MostCritical[:max]
}

// heuristicNarrative builds a deterministic, templated narrative when no LLM is available. It
// leads with the report's own summary and root cause, then appends any mechanistic correlations
// found, then the first actionable fix as a closing recommendation — never an empty result.
func heuristicNarrative(in Input, correlations []string) string {
	var b strings.Builder

	if in.Report.Summary != "" {
		b.WriteString(in.Report.Summary)
	} else {
		fmt.Fprintf(&b, "Skor kesehatan run ini %d/100 (%s).", in.Report.HealthScore, in.Report.StatusLabel)
	}

	if in.Report.RootCause != "" {
		fmt.Fprintf(&b, " Dugaan akar masalah: %s", in.Report.RootCause)
	}

	for _, c := range correlations {
		fmt.Fprintf(&b, " %s.", c)
	}

	if len(in.Report.ActionableFixes) > 0 {
		fmt.Fprintf(&b, " Langkah pertama yang disarankan: %s", in.Report.ActionableFixes[0])
	}

	return b.String()
}
