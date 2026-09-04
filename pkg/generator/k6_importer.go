package generator

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

// k6CallRe extracts k6 http.<method>(url[, body]) calls from a script's default `export default
// function` VU body. This is a regex-based heuristic, not a JavaScript parser: it recognizes the
// overwhelmingly common case (a string-literal URL passed directly to http.get/post/put/del/patch)
// and does not evaluate template literals, variables, or k6 scenario/stages configuration blocks.
// Anything more dynamic than that needs to be recreated by hand — flagged clearly in the error
// when nothing matches, rather than silently producing an empty scenario.
var k6CallRe = regexp.MustCompile("(?s)http\\.(get|post|put|del|patch)\\(\\s*[`'\"]([^`'\"]+)[`'\"]\\s*(?:,\\s*(?:JSON\\.stringify\\()?([`'\"][^`']*?[`'\"]|\\{[^)]*?\\}))?")

// ImportK6Script converts a k6 JavaScript load-test script into a Scenario by extracting every
// recognizable http.<method>(url, ...) call, in the order they appear in the file.
func ImportK6Script(ctx context.Context, script string, cfg GeneratorConfig) (*vusession.Scenario, error) {
	matches := k6CallRe.FindAllStringSubmatch(script, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no recognizable http.get/post/put/del/patch(...) calls found — this importer only handles string-literal URLs, not k6 scripts built from variables or template config")
	}

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("scenario_k6_%d", time.Now().Unix())
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = "Imported from k6 Script"
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	steps := make(map[string]*vusession.Step)
	var order []string
	baseURL := cfg.BaseURL

	for i, m := range matches {
		method := strings.ToUpper(m[1])
		if method == "DEL" {
			method = "DELETE"
		}
		rawURL := m[2]
		body := strings.Trim(m[3], "`'\"")

		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" {
			continue // Not an absolute URL (likely built from a variable) — skip rather than guess.
		}
		if baseURL == "" {
			baseURL = parsed.Scheme + "://" + parsed.Host
		}

		if cfg.Locale == "id" && body != "" {
			body = LocalizeIndonesian(body)
		}

		stepID := fmt.Sprintf("step_%d", i+1)
		steps[stepID] = &vusession.Step{
			ID:      stepID,
			Name:    fmt.Sprintf("%s %s", method, parsed.Path),
			Request: vusession.RequestConfig{Method: method, Path: parsed.Path + queryOrEmpty(parsed), Body: body, Timeout: vusession.Duration(cfg.DefaultTimeout)},
			Assertions: []vusession.AssertionConfig{
				{Type: vusession.AssertStatusCodeInRange, MinCode: 200, MaxCode: 399},
			},
			OnFailure: vusession.FailurePolicy{Action: vusession.FailureActionAbort},
		}
		order = append(order, stepID)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("found http.*() calls but none used an absolute URL literal (e.g. `${BASE_URL}/path` variables aren't resolved)")
	}

	for i, id := range order {
		if i+1 < len(order) {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: order[i+1], Probability: 1.0}}
		} else {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: "END", Probability: 1.0}}
		}
	}

	sc := &vusession.Scenario{
		ID: cfg.ScenarioID, Name: cfg.ScenarioName,
		Description: fmt.Sprintf("Imported from a k6 script (%d recognized HTTP calls).", len(order)),
		BaseURL:     baseURL, InitialStepID: order[0], DefaultHeaders: cfg.DefaultHeaders,
		Timeout: vusession.Duration(cfg.DefaultTimeout), Steps: steps,
	}

	enricher := cfg.Enricher
	if enricher == nil {
		enricher = NewHeuristicEnricher()
	}
	enriched, err := enricher.Enrich(ctx, sc, nil)
	if err != nil {
		return nil, fmt.Errorf("k6-derived scenario enrichment failed: %w", err)
	}
	if err := vusession.ValidateScenario(enriched); err != nil {
		return nil, fmt.Errorf("k6-derived scenario failed validation: %w", err)
	}
	return enriched, nil
}
