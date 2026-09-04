package generator

import "testing"

func sampleTransitionMatrix() *TransitionMatrix {
	counts := map[string]map[string]int{
		"GET /login":     {"GET /browse": 100},
		"GET /browse":    {"GET /browse": 50, "POST /checkout": 50},
		"POST /checkout": {"POST /payment": 100},
		"POST /payment":  {"END": 100},
	}
	probabilities := make(map[string]map[string]float64)
	for from, targets := range counts {
		total := 0
		for _, c := range targets {
			total += c
		}
		probabilities[from] = make(map[string]float64)
		for to, c := range targets {
			probabilities[from][to] = float64(c) / float64(total)
		}
	}
	return &TransitionMatrix{Counts: counts, Probabilities: probabilities}
}

func TestBuildDependencyGraphExcludesEndSink(t *testing.T) {
	graph := BuildDependencyGraph(sampleTransitionMatrix())

	for _, node := range graph.Nodes {
		if node.ID == "END" {
			t.Error("expected END sink to be excluded from graph nodes")
		}
	}
	for _, edge := range graph.Edges {
		if edge.To == "END" {
			t.Error("expected edges targeting END to be excluded")
		}
	}
	if len(graph.Nodes) != 4 {
		t.Errorf("expected 4 real endpoint nodes, got %d", len(graph.Nodes))
	}
}

func TestBuildDependencyGraphRanksDownstreamHigherThanEntryPoint(t *testing.T) {
	graph := BuildDependencyGraph(sampleTransitionMatrix())

	rankOf := make(map[string]float64)
	for _, n := range graph.Nodes {
		rankOf[n.ID] = n.Criticality
	}

	// /login has no incoming edges (it's the entry point) so it should rank at the bottom, while
	// /checkout and /payment funnel 100% of traffic and should outrank it.
	if rankOf["GET /login"] >= rankOf["POST /checkout"] {
		t.Errorf("expected /checkout (%f) to outrank entry point /login (%f)", rankOf["POST /checkout"], rankOf["GET /login"])
	}
	if len(graph.MostCritical) == 0 || graph.MostCritical[0] == "GET /login" {
		t.Errorf("expected the most critical node to not be the traffic-less entry point, got %v", graph.MostCritical)
	}
}

func TestBuildDependencyGraphHandlesEmptyMatrix(t *testing.T) {
	graph := BuildDependencyGraph(&TransitionMatrix{})
	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 {
		t.Error("expected an empty graph for an empty transition matrix")
	}

	nilGraph := BuildDependencyGraph(nil)
	if len(nilGraph.Nodes) != 0 {
		t.Error("expected an empty graph for a nil transition matrix")
	}
}
