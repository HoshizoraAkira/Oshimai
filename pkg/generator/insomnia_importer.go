package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

// insomniaExport mirrors the subset of Insomnia's "Export Data" JSON format (schema
// "insomnia.desktop.app", resource type "request") needed to rebuild a request sequence.
type insomniaExport struct {
	Resources []struct {
		ID          string  `json:"_id"`
		Type        string  `json:"_type"`
		ParentID    string  `json:"parentId"`
		Name        string  `json:"name"`
		Method      string  `json:"method"`
		URL         string  `json:"url"`
		MetaSortKey float64 `json:"metaSortKey"`
		Headers     []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
		Body struct {
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"body"`
	} `json:"resources"`
}

// ImportInsomniaCollection converts an Insomnia "Export Data" JSON document into a Scenario.
// Requests are ordered by Insomnia's own metaSortKey (the order they appear in the sidebar),
// same simplification ImportPostmanCollection makes: folder hierarchy is flattened into one
// sequential flow.
func ImportInsomniaCollection(ctx context.Context, data []byte, cfg GeneratorConfig) (*vusession.Scenario, error) {
	var export insomniaExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("failed to parse Insomnia export: %w", err)
	}

	type req struct {
		name, method, rawURL, body string
		sortKey                    float64
		headers                    map[string]string
	}
	var requests []req
	for _, res := range export.Resources {
		if res.Type != "request" || res.URL == "" {
			continue
		}
		headers := make(map[string]string)
		for _, h := range res.Headers {
			if strings.EqualFold(h.Name, "host") || strings.EqualFold(h.Name, "content-length") {
				continue
			}
			headers[h.Name] = h.Value
		}
		requests = append(requests, req{
			name: res.Name, method: res.Method, rawURL: res.URL, body: res.Body.Text,
			sortKey: res.MetaSortKey, headers: headers,
		})
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("Insomnia export contains no requests")
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].sortKey < requests[j].sortKey })

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("scenario_insomnia_%d", time.Now().Unix())
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = "Imported from Insomnia"
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	steps := make(map[string]*vusession.Step)
	var order []string
	baseURL := cfg.BaseURL

	for i, rq := range requests {
		parsed, err := url.Parse(interpolateInsomniaVars(rq.rawURL))
		if err != nil || parsed.Host == "" {
			continue
		}
		if baseURL == "" {
			baseURL = parsed.Scheme + "://" + parsed.Host
		}

		stepID := fmt.Sprintf("step_%d", i+1)
		method := strings.ToUpper(rq.method)
		if method == "" {
			method = "GET"
		}
		body := interpolateInsomniaVars(rq.body)
		if cfg.Locale == "id" && body != "" {
			body = LocalizeIndonesian(body)
		}

		steps[stepID] = &vusession.Step{
			ID:      stepID,
			Name:    rq.name,
			Request: vusession.RequestConfig{Method: method, Path: parsed.Path + queryOrEmpty(parsed), Headers: rq.headers, Body: body, Timeout: vusession.Duration(cfg.DefaultTimeout)},
			Assertions: []vusession.AssertionConfig{
				{Type: vusession.AssertStatusCodeInRange, MinCode: 200, MaxCode: 399},
			},
			OnFailure: vusession.FailurePolicy{Action: vusession.FailureActionAbort},
		}
		order = append(order, stepID)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("no valid HTTP requests found in Insomnia export")
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
		Description: fmt.Sprintf("Imported from a %d-request Insomnia export.", len(order)),
		BaseURL:     baseURL, InitialStepID: order[0], DefaultHeaders: cfg.DefaultHeaders,
		Timeout: vusession.Duration(cfg.DefaultTimeout), Steps: steps,
	}

	enricher := cfg.Enricher
	if enricher == nil {
		enricher = NewHeuristicEnricher()
	}
	enriched, err := enricher.Enrich(ctx, sc, nil)
	if err != nil {
		return nil, fmt.Errorf("Insomnia-derived scenario enrichment failed: %w", err)
	}
	if err := vusession.ValidateScenario(enriched); err != nil {
		return nil, fmt.Errorf("Insomnia-derived scenario failed validation: %w", err)
	}
	return enriched, nil
}

// interpolateInsomniaVars rewrites Insomnia's {{ _.variable }} / {{variable}} syntax into
// Oshimai's ${variable} syntax.
func interpolateInsomniaVars(s string) string {
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			end := strings.Index(s[i+2:], "}}")
			if end >= 0 {
				key := strings.TrimSpace(s[i+2 : i+2+end])
				key = strings.TrimPrefix(key, "_.")
				sb.WriteString("${" + key + "}")
				i = i + 2 + end + 2
				continue
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}
