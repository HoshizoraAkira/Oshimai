package generator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ParseOTelSpansJSON parses a JSON array or newline-delimited JSON of OTelSpan objects.
func ParseOTelSpansJSON(data []byte) ([]OTelSpan, error) {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Try parsing as JSON array
	if strings.HasPrefix(trimmed, "[") {
		var spans []OTelSpan
		if err := json.Unmarshal(data, &spans); err != nil {
			return nil, fmt.Errorf("failed to parse OTel spans array: %w", err)
		}
		return spans, nil
	}

	// Try parsing newline-delimited JSON
	lines := strings.Split(trimmed, "\n")
	var spans []OTelSpan
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var span OTelSpan
		if err := json.Unmarshal([]byte(line), &span); err != nil {
			return nil, fmt.Errorf("failed to parse OTel span line %d: %w", i+1, err)
		}
		spans = append(spans, span)
	}

	return spans, nil
}

// MineBehavioralTransitions processes raw spans into a Markov transition probability matrix.
func MineBehavioralTransitions(spans []OTelSpan) (*TransitionMatrix, error) {
	if len(spans) == 0 {
		return &TransitionMatrix{
			Counts:           make(map[string]map[string]int),
			Probabilities:    make(map[string]map[string]float64),
			AvgLatencies:     make(map[string]time.Duration),
			TotalTransitions: make(map[string]int),
		}, nil
	}

	// 1. Group spans by TraceID
	traces := make(map[string][]OTelSpan)
	for _, span := range spans {
		if span.TraceID == "" {
			continue
		}
		// Resolve route/endpoint identifier
		route := resolveSpanRoute(span)
		if route == "" {
			continue
		}
		span.Route = route
		traces[span.TraceID] = append(traces[span.TraceID], span)
	}

	// 2. Sort each trace chronologically and build transition counts
	counts := make(map[string]map[string]int)
	totalTransitions := make(map[string]int)
	latencySums := make(map[string]time.Duration)
	latencyCounts := make(map[string]int)

	for _, traceSpans := range traces {
		if len(traceSpans) == 0 {
			continue
		}

		sort.Slice(traceSpans, func(i, j int) bool {
			return traceSpans[i].StartTime.Before(traceSpans[j].StartTime)
		})

		for i := 0; i < len(traceSpans)-1; i++ {
			from := formatEndpointKey(traceSpans[i].Method, traceSpans[i].Route)
			to := formatEndpointKey(traceSpans[i+1].Method, traceSpans[i+1].Route)

			if counts[from] == nil {
				counts[from] = make(map[string]int)
			}
			counts[from][to]++
			totalTransitions[from]++

			// Record inter-request think time / duration
			if traceSpans[i+1].StartTime.After(traceSpans[i].EndTime) {
				gap := traceSpans[i+1].StartTime.Sub(traceSpans[i].EndTime)
				latencySums[from] += gap
				latencyCounts[from]++
			}
		}

		// Terminal transition for the last span
		last := traceSpans[len(traceSpans)-1]
		lastKey := formatEndpointKey(last.Method, last.Route)
		if counts[lastKey] == nil {
			counts[lastKey] = make(map[string]int)
		}
		counts[lastKey]["END"]++
		totalTransitions[lastKey]++
	}

	// 3. Compute Probabilities (normalized CDF)
	probabilities := make(map[string]map[string]float64)
	for from, toMap := range counts {
		total := totalTransitions[from]
		if total <= 0 {
			continue
		}
		probabilities[from] = make(map[string]float64)
		for to, count := range toMap {
			probabilities[from][to] = float64(count) / float64(total)
		}
	}

	// 4. Compute Average Latencies
	avgLatencies := make(map[string]time.Duration)
	for k, count := range latencyCounts {
		if count > 0 {
			avgLatencies[k] = latencySums[k] / time.Duration(count)
		}
	}

	return &TransitionMatrix{
		Counts:           counts,
		Probabilities:    probabilities,
		AvgLatencies:     avgLatencies,
		TotalTransitions: totalTransitions,
	}, nil
}

func resolveSpanRoute(span OTelSpan) string {
	if span.Route != "" {
		return span.Route
	}
	if span.Attributes != nil {
		if r, ok := span.Attributes["http.route"].(string); ok && r != "" {
			return r
		}
		if t, ok := span.Attributes["http.target"].(string); ok && t != "" {
			return t
		}
		if u, ok := span.Attributes["url.path"].(string); ok && u != "" {
			return u
		}
	}
	if strings.HasPrefix(span.Name, "/") {
		return span.Name
	}
	// e.g. "GET /api/v1/users"
	parts := strings.Fields(span.Name)
	if len(parts) >= 2 && strings.HasPrefix(parts[1], "/") {
		return parts[1]
	}
	return ""
}

func formatEndpointKey(method, route string) string {
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		m = "GET"
	}
	r := strings.TrimSpace(route)
	return fmt.Sprintf("%s %s", m, r)
}
