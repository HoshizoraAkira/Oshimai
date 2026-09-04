package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

// harFile mirrors the subset of the HTTP Archive (HAR 1.2) format Chrome/Firefox DevTools export
// via "Copy all as HAR" — enough for a vibe coder to record a click-through session in their
// browser and drop the file straight into Oshimai with zero OpenAPI/OTel spec required.
type harFile struct {
	Log struct {
		Entries []struct {
			Request struct {
				Method  string `json:"method"`
				URL     string `json:"url"`
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
				PostData *struct {
					MimeType string `json:"mimeType"`
					Text     string `json:"text"`
				} `json:"postData"`
			} `json:"request"`
		} `json:"entries"`
	} `json:"log"`
}

// ImportHAR converts a browser-exported HAR file into a Scenario: every recorded request becomes
// a sequential Step (in original chronological order), reproducing exactly what the user clicked
// through — no OpenAPI spec or OTel trace needed.
func ImportHAR(ctx context.Context, data []byte, cfg GeneratorConfig) (*vusession.Scenario, error) {
	var har harFile
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, fmt.Errorf("failed to parse HAR file: %w", err)
	}
	if len(har.Log.Entries) == 0 {
		return nil, fmt.Errorf("HAR file contains no recorded requests")
	}

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("scenario_har_%d", time.Now().Unix())
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = "Imported from HAR Recording"
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	steps := make(map[string]*vusession.Step)
	var order []string
	baseURL := cfg.BaseURL

	for i, entry := range har.Log.Entries {
		parsed, err := url.Parse(entry.Request.URL)
		if err != nil || parsed.Host == "" {
			continue // Skip unparseable entries (e.g. data: URIs, browser telemetry beacons).
		}
		if baseURL == "" {
			baseURL = parsed.Scheme + "://" + parsed.Host
		}
		// Only keep requests to the same origin as the first valid entry — HAR recordings are
		// full of third-party analytics/ad calls we don't want to replay against the target.
		if origin := parsed.Scheme + "://" + parsed.Host; origin != baseURL {
			continue
		}

		stepID := fmt.Sprintf("step_%d", i+1)
		headers := make(map[string]string)
		for _, h := range entry.Request.Headers {
			lname := strings.ToLower(h.Name)
			// Drop hop-by-hop / browser-managed headers that a load-test client shouldn't replay verbatim.
			if lname == "cookie" || lname == "host" || lname == "content-length" || strings.HasPrefix(lname, ":") {
				continue
			}
			headers[h.Name] = h.Value
		}

		body := ""
		if entry.Request.PostData != nil {
			body = entry.Request.PostData.Text
			if cfg.Locale == "id" && strings.Contains(entry.Request.PostData.MimeType, "json") {
				body = LocalizeIndonesian(body)
			}
		}

		steps[stepID] = &vusession.Step{
			ID:   stepID,
			Name: fmt.Sprintf("%s %s", entry.Request.Method, parsed.Path),
			Request: vusession.RequestConfig{
				Method:  strings.ToUpper(entry.Request.Method),
				Path:    parsed.Path + queryOrEmpty(parsed),
				Headers: headers,
				Body:    body,
				Timeout: vusession.Duration(cfg.DefaultTimeout),
			},
			Assertions: []vusession.AssertionConfig{
				{Type: vusession.AssertStatusCodeInRange, MinCode: 200, MaxCode: 399},
			},
			OnFailure: vusession.FailurePolicy{Action: vusession.FailureActionAbort},
		}
		order = append(order, stepID)
	}

	if len(order) == 0 {
		return nil, fmt.Errorf("no same-origin HTTP requests found in HAR recording")
	}

	for i, id := range order {
		if i+1 < len(order) {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: order[i+1], Probability: 1.0}}
		} else {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: "END", Probability: 1.0}}
		}
	}

	sc := &vusession.Scenario{
		ID:             cfg.ScenarioID,
		Name:           cfg.ScenarioName,
		Description:    fmt.Sprintf("Imported from a %d-request browser HAR recording.", len(order)),
		BaseURL:        baseURL,
		InitialStepID:  order[0],
		DefaultHeaders: cfg.DefaultHeaders,
		Timeout:        vusession.Duration(cfg.DefaultTimeout),
		Steps:          steps,
	}

	enricher := cfg.Enricher
	if enricher == nil {
		enricher = NewHeuristicEnricher()
	}
	enriched, err := enricher.Enrich(ctx, sc, nil)
	if err != nil {
		return nil, fmt.Errorf("HAR-derived scenario enrichment failed: %w", err)
	}

	if err := vusession.ValidateScenario(enriched); err != nil {
		return nil, fmt.Errorf("HAR-derived scenario failed validation: %w", err)
	}
	return enriched, nil
}

func queryOrEmpty(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	return "?" + u.RawQuery
}
