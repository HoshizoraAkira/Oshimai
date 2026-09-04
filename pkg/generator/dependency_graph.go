package generator

import "sort"

// GraphNode represents one endpoint in the mined dependency graph, along with how central it is
// to overall traffic flow.
type GraphNode struct {
	ID           string  `json:"id"` // Endpoint key, e.g. "GET /api/v1/checkout".
	TotalTraffic int     `json:"total_traffic"`
	Criticality  float64 `json:"criticality"` // Normalized PageRank-style centrality, 0-1.
}

// GraphEdge represents one observed transition between two endpoints.
type GraphEdge struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Count       int     `json:"count"`
	Probability float64 `json:"probability"`
}

// DependencyGraph is the endpoint call-graph mined from real traffic, ranked by criticality so an
// operator knows which endpoint to chaos-test first: the one whose failure would ripple through
// the most user journeys, not just the one that happens to be slowest.
type DependencyGraph struct {
	Nodes        []GraphNode `json:"nodes"`
	Edges        []GraphEdge `json:"edges"`
	MostCritical []string    `json:"most_critical"` // Node IDs, most critical first.
}

const (
	pageRankDamping    = 0.85
	pageRankIterations = 25
)

// BuildDependencyGraph converts a TransitionMatrix (see otel_miner.go) into a ranked dependency
// graph using a small power-iteration PageRank: an endpoint is "critical" not because it's slow,
// but because a large share of user journeys flow through it, the same intuition PageRank applies
// to web pages via inbound links.
func BuildDependencyGraph(matrix *TransitionMatrix) *DependencyGraph {
	graph := &DependencyGraph{}
	if matrix == nil || len(matrix.Counts) == 0 {
		return graph
	}

	// 1. Collect every real node (endpoints only — "END" is a terminal sink, not an endpoint).
	nodeSet := make(map[string]bool)
	for from, targets := range matrix.Counts {
		nodeSet[from] = true
		for to := range targets {
			if to != "END" {
				nodeSet[to] = true
			}
		}
	}
	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	n := len(nodes)
	if n == 0 {
		return graph
	}
	indexOf := make(map[string]int, n)
	for i, id := range nodes {
		indexOf[id] = i
	}

	// 2. Build edges (excluding the END sink) and per-node total outbound traffic.
	var edges []GraphEdge
	totalTraffic := make(map[string]int)
	for from, targets := range matrix.Counts {
		for to, count := range targets {
			totalTraffic[from] += count
			if to == "END" {
				continue
			}
			edges = append(edges, GraphEdge{
				From:        from,
				To:          to,
				Count:       count,
				Probability: matrix.Probabilities[from][to],
			})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	// 3. Power-iteration PageRank over the transition-probability graph.
	rank := make([]float64, n)
	for i := range rank {
		rank[i] = 1.0 / float64(n)
	}
	for iter := 0; iter < pageRankIterations; iter++ {
		next := make([]float64, n)
		base := (1 - pageRankDamping) / float64(n)
		for i := range next {
			next[i] = base
		}
		for _, e := range edges {
			fromIdx, ok1 := indexOf[e.From]
			toIdx, ok2 := indexOf[e.To]
			if !ok1 || !ok2 || e.Probability <= 0 {
				continue
			}
			next[toIdx] += pageRankDamping * rank[fromIdx] * e.Probability
		}
		rank = next
	}

	// 4. Normalize so the most critical node scores 1.0, making the score readable at a glance.
	maxRank := 0.0
	for _, r := range rank {
		if r > maxRank {
			maxRank = r
		}
	}
	if maxRank == 0 {
		maxRank = 1
	}

	graph.Nodes = make([]GraphNode, n)
	for i, id := range nodes {
		graph.Nodes[i] = GraphNode{ID: id, TotalTraffic: totalTraffic[id], Criticality: rank[i] / maxRank}
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].Criticality > graph.Nodes[j].Criticality })

	graph.Edges = edges
	graph.MostCritical = make([]string, 0, n)
	for _, node := range graph.Nodes {
		graph.MostCritical = append(graph.MostCritical, node.ID)
	}

	return graph
}
