package server

import (
	"context"
	"fmt"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

// MultiRegionRequest fans a load test out across connected agents (see cmd/agent) instead of
// running every Virtual User from the control plane's own host — the only way to measure real
// geographic latency differences, and to generate more load than one machine's NIC/CPU can push.
type MultiRegionRequest struct {
	Base    CreateRunRequest `json:"base"`
	Regions []string         `json:"regions,omitempty"` // Empty = use every connected agent, any region.
}

// MultiRegionResult reports the merged outcome plus each contributing agent's own summary, since
// "what did latency look like from Singapore vs. Jakarta vs. São Paulo" is the entire point.
type MultiRegionResult struct {
	RunID          string                                  `json:"run_id"`
	AgentsUsed     int                                     `json:"agents_used"`
	Merged         *loadengine.ExecutionSummary            `json:"merged"`
	PerAgent       map[string]*loadengine.ExecutionSummary `json:"per_agent"`
	PerAgentRegion map[string]string                       `json:"per_agent_region"`
}

// RunMultiRegion splits req.Base's target VU count evenly across every connected agent in
// req.Regions (or all connected agents if Regions is empty), waits for every agent's result (or
// until the test duration plus a grace period elapses), and merges the summaries.
func (tc *TestCoordinator) RunMultiRegion(ctx context.Context, req MultiRegionRequest) (*MultiRegionResult, error) {
	if tc.cfg.EnforceTargetVerification {
		if err := tc.checkTargetAllowed(req.Base); err != nil {
			return nil, err
		}
	}

	scenario, err := tc.resolveScenario(ctx, req.Base)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve scenario: %w", err)
	}

	var agents []AgentInfo
	if len(req.Regions) == 0 {
		agents = tc.agents.ListConnected("")
	} else {
		seen := make(map[string]bool)
		for _, region := range req.Regions {
			for _, a := range tc.agents.ListConnected(region) {
				if !seen[a.ID] {
					seen[a.ID] = true
					agents = append(agents, a)
				}
			}
		}
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no connected agents available (requested regions: %v) — start one or more `oshimai-agent` processes first", req.Regions)
	}

	totalVUs := req.Base.LoadConfig.VUs
	if totalVUs <= 0 {
		totalVUs = len(agents)
	}
	perAgentVUs := totalVUs / len(agents)
	if perAgentVUs < 1 {
		perAgentVUs = 1
	}

	runID := fmt.Sprintf("multiregion-%d", time.Now().UnixNano())

	for _, agent := range agents {
		agentLoad := req.Base.LoadConfig
		agentLoad.VUs = perAgentVUs
		if err := tc.agents.Dispatch(agent.ID, AgentAssignment{RunID: runID, Scenario: scenario, Load: agentLoad}); err != nil {
			return nil, fmt.Errorf("failed to dispatch to agent %q: %w", agent.ID, err)
		}
	}

	waitTimeout := req.Base.LoadConfig.Duration + 30*time.Second
	if waitTimeout <= 0 {
		waitTimeout = 60 * time.Second
	}

	perAgent := make(map[string]*loadengine.ExecutionSummary)
	perAgentRegion := make(map[string]string)
	var summaries []*loadengine.ExecutionSummary

	for _, agent := range agents {
		res, ok := tc.agents.AwaitResult(runID, agent.ID, waitTimeout)
		if !ok || res.Summary == nil {
			continue // A missing/slow agent shouldn't sink the whole distributed run.
		}
		perAgent[agent.ID] = res.Summary
		perAgentRegion[agent.ID] = agent.Region
		summaries = append(summaries, res.Summary)
	}
	if len(summaries) == 0 {
		return nil, fmt.Errorf("no agent reported a result within %v", waitTimeout)
	}

	return &MultiRegionResult{
		RunID:          runID,
		AgentsUsed:     len(summaries),
		Merged:         mergeExecutionSummaries(summaries),
		PerAgent:       perAgent,
		PerAgentRegion: perAgentRegion,
	}, nil
}

// mergeExecutionSummaries combines multiple agents' independently-measured summaries into one
// approximate whole-fleet view. Request/error/status-code counts merge exactly (they're sums);
// latency percentiles are request-count-weighted averages across agents, NOT a true merged
// histogram — an honest approximation given each agent only reports its own quantiles, not raw
// samples. Min/Max merge exactly since those are true bounds regardless of which agent saw them.
func mergeExecutionSummaries(summaries []*loadengine.ExecutionSummary) *loadengine.ExecutionSummary {
	merged := &loadengine.ExecutionSummary{StatusCodes: make(map[int]int64)}
	var totalReqForWeighting int64
	var weightedMean, weightedP50, weightedP90, weightedP95, weightedP99 float64

	for i, s := range summaries {
		merged.TotalRequests += s.TotalRequests
		merged.TotalErrors += s.TotalErrors
		for code, count := range s.StatusCodes {
			merged.StatusCodes[code] += count
		}
		if s.TotalDuration > merged.TotalDuration {
			merged.TotalDuration = s.TotalDuration
		}
		if i == 0 || s.Latency.Min < merged.Latency.Min {
			merged.Latency.Min = s.Latency.Min
		}
		if s.Latency.Max > merged.Latency.Max {
			merged.Latency.Max = s.Latency.Max
		}
		if s.TerminationStatus == loadengine.StatusAbortedByCircuitBreaker {
			merged.TerminationStatus = loadengine.StatusAbortedByCircuitBreaker
			merged.AbortReason = s.AbortReason
		}

		w := float64(s.TotalRequests)
		totalReqForWeighting += s.TotalRequests
		weightedMean += w * float64(s.Latency.Mean)
		weightedP50 += w * float64(s.Latency.P50)
		weightedP90 += w * float64(s.Latency.P90)
		weightedP95 += w * float64(s.Latency.P95)
		weightedP99 += w * float64(s.Latency.P99)
	}

	if totalReqForWeighting > 0 {
		merged.Latency.Mean = time.Duration(weightedMean / float64(totalReqForWeighting))
		merged.Latency.P50 = time.Duration(weightedP50 / float64(totalReqForWeighting))
		merged.Latency.P90 = time.Duration(weightedP90 / float64(totalReqForWeighting))
		merged.Latency.P95 = time.Duration(weightedP95 / float64(totalReqForWeighting))
		merged.Latency.P99 = time.Duration(weightedP99 / float64(totalReqForWeighting))
	}
	if merged.TerminationStatus == "" {
		merged.TerminationStatus = loadengine.StatusCompleted
	}
	if merged.TotalDuration > 0 {
		merged.ActualRPS = float64(merged.TotalRequests) / merged.TotalDuration.Seconds()
	}
	return merged
}
