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

// postmanURL handles both Postman's structured URL object and the plain-string form some exports use.
type postmanURL struct {
	Raw string
}

func (u *postmanURL) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		u.Raw = asString
		return nil
	}
	var asObject struct {
		Raw string `json:"raw"`
	}
	if err := json.Unmarshal(data, &asObject); err != nil {
		return err
	}
	u.Raw = asObject.Raw
	return nil
}

type postmanItem struct {
	Name    string `json:"name"`
	Request *struct {
		Method string `json:"method"`
		Header []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"header"`
		Body *struct {
			Mode string `json:"mode"`
			Raw  string `json:"raw"`
		} `json:"body"`
		URL postmanURL `json:"url"`
	} `json:"request"`
	Item []postmanItem `json:"item"` // Nested folder.
}

type postmanCollection struct {
	Info struct {
		Name string `json:"name"`
	} `json:"info"`
	Item []postmanItem `json:"item"`
}

// ImportPostmanCollection converts a Postman Collection (v2.0/v2.1 schema) export into a
// Scenario. Folders are flattened depth-first into one sequential flow, in the same order they
// appear in the collection — the order most teams already use to represent a user journey
// (login folder, then browse, then checkout).
func ImportPostmanCollection(ctx context.Context, data []byte, cfg GeneratorConfig) (*vusession.Scenario, error) {
	var coll postmanCollection
	if err := json.Unmarshal(data, &coll); err != nil {
		return nil, fmt.Errorf("failed to parse Postman collection: %w", err)
	}
	if len(coll.Item) == 0 {
		return nil, fmt.Errorf("Postman collection contains no requests")
	}

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("scenario_postman_%d", time.Now().Unix())
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = coll.Info.Name
		if cfg.ScenarioName == "" {
			cfg.ScenarioName = "Imported from Postman Collection"
		}
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	var flat []postmanItem
	flattenPostmanItems(coll.Item, &flat)
	if len(flat) == 0 {
		return nil, fmt.Errorf("Postman collection has folders but no actual requests")
	}

	steps := make(map[string]*vusession.Step)
	var order []string
	baseURL := cfg.BaseURL

	for i, item := range flat {
		req := item.Request
		parsed, err := url.Parse(req.URL.Raw)
		if err != nil || parsed.Host == "" {
			continue
		}
		if baseURL == "" {
			baseURL = parsed.Scheme + "://" + parsed.Host
		}

		stepID := fmt.Sprintf("step_%d", i+1)
		headers := make(map[string]string)
		for _, h := range req.Header {
			if strings.EqualFold(h.Key, "host") || strings.EqualFold(h.Key, "content-length") {
				continue
			}
			headers[h.Key] = interpolatePostmanVars(h.Value)
		}

		body := ""
		if req.Body != nil && req.Body.Mode == "raw" {
			body = interpolatePostmanVars(req.Body.Raw)
			if cfg.Locale == "id" {
				body = LocalizeIndonesian(body)
			}
		}

		method := strings.ToUpper(req.Method)
		if method == "" {
			method = "GET"
		}

		steps[stepID] = &vusession.Step{
			ID:   stepID,
			Name: item.Name,
			Request: vusession.RequestConfig{
				Method:  method,
				Path:    interpolatePostmanVars(parsed.Path + queryOrEmpty(parsed)),
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
		return nil, fmt.Errorf("no valid HTTP requests found in Postman collection")
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
		Description:    fmt.Sprintf("Imported from a %d-request Postman collection.", len(order)),
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
		return nil, fmt.Errorf("Postman-derived scenario enrichment failed: %w", err)
	}

	if err := vusession.ValidateScenario(enriched); err != nil {
		return nil, fmt.Errorf("Postman-derived scenario failed validation: %w", err)
	}
	return enriched, nil
}

func flattenPostmanItems(items []postmanItem, out *[]postmanItem) {
	for _, item := range items {
		if item.Request != nil {
			*out = append(*out, item)
			continue
		}
		if len(item.Item) > 0 {
			flattenPostmanItems(item.Item, out)
		}
	}
}

// interpolatePostmanVars rewrites Postman's {{variable}} syntax into Oshimai's ${variable} syntax
// so extracted session variables (tokens, IDs) keep flowing correctly after import.
func interpolatePostmanVars(s string) string {
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			end := strings.Index(s[i+2:], "}}")
			if end >= 0 {
				key := strings.TrimSpace(s[i+2 : i+2+end])
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
