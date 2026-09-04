package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

// jmeterSampler is one flattened <HTTPSamplerProxy> extracted from a .jmx test plan.
type jmeterSampler struct {
	name, domain, port, protocol, path, method, body string
}

// ImportJMeterPlan converts a JMeter .jmx test plan's HTTP samplers into a Scenario. JMeter test
// plans nest samplers inside a recursive <hashTree> alongside controllers (loops, if-controllers,
// thread groups); rather than modeling that whole schema, this streams the XML token-by-token and
// flattens every <HTTPSamplerProxy> into one sequential flow in document order — the same
// simplification the Postman/Insomnia importers make for folders. Loop/conditional semantics are
// not reproduced; each sampler runs exactly once per Virtual User pass.
func ImportJMeterPlan(ctx context.Context, data []byte, cfg GeneratorConfig) (*vusession.Scenario, error) {
	samplers, err := extractJMeterSamplers(data)
	if err != nil {
		return nil, err
	}
	if len(samplers) == 0 {
		return nil, fmt.Errorf("no HTTPSamplerProxy elements found in JMeter test plan")
	}

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("scenario_jmeter_%d", time.Now().Unix())
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = "Imported from JMeter"
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	steps := make(map[string]*vusession.Step)
	var order []string
	baseURL := cfg.BaseURL

	for i, s := range samplers {
		protocol := s.protocol
		if protocol == "" {
			protocol = "http"
		}
		host := s.domain
		if s.port != "" && s.port != "0" {
			host = host + ":" + s.port
		}
		if host == "" {
			continue
		}
		if baseURL == "" {
			baseURL = protocol + "://" + host
		}

		method := strings.ToUpper(s.method)
		if method == "" {
			method = "GET"
		}
		body := s.body
		if cfg.Locale == "id" && body != "" {
			body = LocalizeIndonesian(body)
		}

		stepID := fmt.Sprintf("step_%d", i+1)
		name := s.name
		if name == "" {
			name = fmt.Sprintf("%s %s", method, s.path)
		}
		steps[stepID] = &vusession.Step{
			ID:      stepID,
			Name:    name,
			Request: vusession.RequestConfig{Method: method, Path: s.path, Body: body, Timeout: vusession.Duration(cfg.DefaultTimeout)},
			Assertions: []vusession.AssertionConfig{
				{Type: vusession.AssertStatusCodeInRange, MinCode: 200, MaxCode: 399},
			},
			OnFailure: vusession.FailurePolicy{Action: vusession.FailureActionAbort},
		}
		order = append(order, stepID)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("JMeter test plan had HTTP samplers, but none carried a usable domain/path")
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
		Description: fmt.Sprintf("Imported from a %d-sampler JMeter test plan.", len(order)),
		BaseURL:     baseURL, InitialStepID: order[0], DefaultHeaders: cfg.DefaultHeaders,
		Timeout: vusession.Duration(cfg.DefaultTimeout), Steps: steps,
	}

	enricher := cfg.Enricher
	if enricher == nil {
		enricher = NewHeuristicEnricher()
	}
	enriched, err := enricher.Enrich(ctx, sc, nil)
	if err != nil {
		return nil, fmt.Errorf("JMeter-derived scenario enrichment failed: %w", err)
	}
	if err := vusession.ValidateScenario(enriched); err != nil {
		return nil, fmt.Errorf("JMeter-derived scenario failed validation: %w", err)
	}
	return enriched, nil
}

func extractJMeterSamplers(data []byte) ([]jmeterSampler, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var samplers []jmeterSampler

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse JMeter XML: %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "HTTPSamplerProxy" {
			continue
		}

		sampler := jmeterSampler{}
		for _, attr := range start.Attr {
			if attr.Name.Local == "testname" {
				sampler.name = attr.Value
			}
		}

		// Consume everything until the matching </HTTPSamplerProxy>, reading each <stringProp
		// name="...">value</stringProp> along the way.
		depth := 1
		for depth > 0 {
			t, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to parse JMeter XML: %w", err)
			}
			switch el := t.(type) {
			case xml.StartElement:
				if el.Name.Local == "HTTPSamplerProxy" {
					depth++
					break
				}
				if el.Name.Local == "stringProp" {
					var propName string
					for _, a := range el.Attr {
						if a.Name.Local == "name" {
							propName = a.Value
						}
					}
					var charData string
					if err := decoder.DecodeElement(&charData, &el); err == nil {
						switch propName {
						case "HTTPSampler.domain":
							sampler.domain = charData
						case "HTTPSampler.port":
							sampler.port = charData
						case "HTTPSampler.protocol":
							sampler.protocol = charData
						case "HTTPSampler.path":
							sampler.path = charData
						case "HTTPSampler.method":
							sampler.method = charData
						case "Argument.value":
							if sampler.body == "" {
								sampler.body = charData
							}
						}
					}
				}
			case xml.EndElement:
				if el.Name.Local == "HTTPSamplerProxy" {
					depth--
				}
			}
		}

		samplers = append(samplers, sampler)
	}

	return samplers, nil
}
