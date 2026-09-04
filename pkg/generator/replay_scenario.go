package generator

import (
	"fmt"
	"sort"
	"time"

	"github.com/oshimai/twin/pkg/vusession"
)

// BuildReplayScenario derives a Scenario that reproduces one real recorded trace exactly — the
// literal sequence and inter-request think-time of a single production trace — instead of the
// probabilistic Markov flow MineBehavioralTransitions/Synthesize derive from many traces. This is
// "shadow mirror" replay: point it at a trace captured from real production traffic and it plays
// that exact user journey back, pacing included, at however many VUs you launch it with.
// speedMultiplier scales the inter-step think-time: 2.0 replays twice as fast, 0.5 replays at
// half speed; values <= 0 are treated as 1.0 (real-time pacing).
func BuildReplayScenario(traceID string, spans []OTelSpan, speedMultiplier float64, cfg GeneratorConfig) (*vusession.Scenario, error) {
	if speedMultiplier <= 0 {
		speedMultiplier = 1.0
	}

	var trace []OTelSpan
	for _, s := range spans {
		if s.TraceID == traceID {
			trace = append(trace, s)
		}
	}
	if len(trace) == 0 {
		return nil, fmt.Errorf("no spans found for trace_id %q", traceID)
	}
	sort.Slice(trace, func(i, j int) bool { return trace[i].StartTime.Before(trace[j].StartTime) })

	if cfg.ScenarioID == "" {
		cfg.ScenarioID = fmt.Sprintf("replay_%s", traceID)
	}
	if cfg.ScenarioName == "" {
		cfg.ScenarioName = fmt.Sprintf("Shadow Replay of Trace %s", traceID)
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Second
	}

	steps := make(map[string]*vusession.Step)
	var order []string
	baseURL := cfg.BaseURL

	for i, span := range trace {
		route := resolveSpanRoute(span)
		if route == "" {
			continue
		}
		if baseURL == "" {
			if scheme, ok := span.Attributes["url.scheme"].(string); ok {
				if host, ok2 := span.Attributes["server.address"].(string); ok2 {
					baseURL = scheme + "://" + host
				}
			}
		}

		// The recorded gap since the previous span ended becomes this step's think-time, scaled
		// by the replay speed — so step N waits, then fires, reproducing the original pacing.
		var thinkTime time.Duration
		if i > 0 {
			gap := span.StartTime.Sub(trace[i-1].EndTime)
			if gap > 0 {
				thinkTime = time.Duration(float64(gap) / speedMultiplier)
			}
		}

		method := span.Method
		if method == "" {
			method = "GET"
		}

		stepID := fmt.Sprintf("replay_step_%d", i+1)
		steps[stepID] = &vusession.Step{
			ID:        stepID,
			Name:      fmt.Sprintf("%s %s (replayed)", method, route),
			ThinkTime: vusession.Duration(thinkTime),
			Request:   vusession.RequestConfig{Method: method, Path: route, Timeout: vusession.Duration(cfg.DefaultTimeout)},
			Assertions: []vusession.AssertionConfig{
				{Type: vusession.AssertStatusCodeInRange, MinCode: 100, MaxCode: 599}, // Replay observes reality; it doesn't judge it.
			},
		}
		order = append(order, stepID)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("trace %q had no spans with a resolvable HTTP route", traceID)
	}

	for i, id := range order {
		if i+1 < len(order) {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: order[i+1], Probability: 1.0}}
		} else {
			steps[id].Transitions = []vusession.Transition{{TargetStepID: "END", Probability: 1.0}}
		}
	}

	sc := &vusession.Scenario{
		ID:            cfg.ScenarioID,
		Name:          cfg.ScenarioName,
		Description:   fmt.Sprintf("Exact replay of production trace %s (%d spans) at %.2fx speed.", traceID, len(order), speedMultiplier),
		BaseURL:       baseURL,
		InitialStepID: order[0],
		Timeout:       vusession.Duration(cfg.DefaultTimeout),
		Steps:         steps,
	}
	if err := vusession.ValidateScenario(sc); err != nil {
		return nil, fmt.Errorf("replay scenario failed validation: %w", err)
	}
	return sc, nil
}

// ListReplayableTraces returns every distinct trace_id present in spans, so a caller can present
// "which recorded session do you want to replay?" without the caller re-implementing the scan.
func ListReplayableTraces(spans []OTelSpan) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, s := range spans {
		if s.TraceID != "" && !seen[s.TraceID] {
			seen[s.TraceID] = true
			ids = append(ids, s.TraceID)
		}
	}
	return ids
}
