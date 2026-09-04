package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/vusession"
)

// AgentInfo describes one connected load-generation agent (see cmd/agent) — the building block for
// multi-region load generation: an agent is just a small binary that registers with the control
// plane, polls for work, executes it locally with the same loadengine/vusession packages the
// server itself uses, and reports results back.
type AgentInfo struct {
	ID           string    `json:"id"`
	Region       string    `json:"region"`
	RegisteredAt time.Time `json:"registered_at"`
	LastSeen     time.Time `json:"last_seen"`
}

// AgentAssignment is one unit of work dispatched to an agent.
type AgentAssignment struct {
	RunID    string                  `json:"run_id"`
	Scenario *vusession.Scenario     `json:"scenario"`
	Load     loadengine.EngineConfig `json:"load_config"`
}

// AgentResult is what an agent reports back after executing an AgentAssignment.
type AgentResult struct {
	RunID   string                       `json:"run_id"`
	AgentID string                       `json:"agent_id"`
	Summary *loadengine.ExecutionSummary `json:"summary"`
	Error   string                       `json:"error,omitempty"`
}

// agentHeartbeatTTL: an agent that hasn't polled or heartbeated within this window is considered
// disconnected and dropped from the registry (and from future work distribution).
const agentHeartbeatTTL = 45 * time.Second

// AgentRegistry tracks connected agents and routes assignments/results between the coordinator
// and each agent's long-poll connection.
type AgentRegistry struct {
	mu          sync.Mutex
	agents      map[string]*AgentInfo
	assignments map[string]chan AgentAssignment // agentID -> pending assignment channel
	results     map[string]chan AgentResult     // runID+agentID key -> result channel
}

// NewAgentRegistry creates an empty registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents:      make(map[string]*AgentInfo),
		assignments: make(map[string]chan AgentAssignment),
		results:     make(map[string]chan AgentResult),
	}
}

// Register adds or refreshes an agent.
func (r *AgentRegistry) Register(id, region string) *AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	info, exists := r.agents[id]
	if !exists {
		info = &AgentInfo{ID: id, Region: region, RegisteredAt: now}
		r.agents[id] = info
		r.assignments[id] = make(chan AgentAssignment, 1)
	}
	info.Region = region
	info.LastSeen = now
	return info
}

// Heartbeat refreshes an agent's LastSeen without changing its region.
func (r *AgentRegistry) Heartbeat(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.agents[id]
	if !ok {
		return false
	}
	info.LastSeen = time.Now()
	return true
}

// ListConnected returns every agent seen within agentHeartbeatTTL, optionally filtered by region
// (empty region returns all connected agents).
func (r *AgentRegistry) ListConnected(region string) []AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []AgentInfo
	cutoff := time.Now().Add(-agentHeartbeatTTL)
	for _, info := range r.agents {
		if info.LastSeen.Before(cutoff) {
			continue
		}
		if region != "" && info.Region != region {
			continue
		}
		out = append(out, *info)
	}
	return out
}

// Dispatch pushes an assignment to a specific agent's poll queue. Returns an error if the agent
// isn't registered or already has an outstanding assignment.
func (r *AgentRegistry) Dispatch(agentID string, assignment AgentAssignment) error {
	r.mu.Lock()
	ch, ok := r.assignments[agentID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("agent %q is not registered", agentID)
	}
	select {
	case ch <- assignment:
		return nil
	default:
		return fmt.Errorf("agent %q already has a pending assignment", agentID)
	}
}

// Poll blocks (up to timeout) waiting for an assignment for agentID, refreshing its heartbeat as
// a side effect (a poll IS a heartbeat — an agent that's actively long-polling is obviously alive).
func (r *AgentRegistry) Poll(agentID string, timeout time.Duration) (AgentAssignment, bool) {
	r.Heartbeat(agentID)
	r.mu.Lock()
	ch, ok := r.assignments[agentID]
	r.mu.Unlock()
	if !ok {
		return AgentAssignment{}, false
	}
	select {
	case a := <-ch:
		return a, true
	case <-time.After(timeout):
		return AgentAssignment{}, false
	}
}

func resultKey(runID, agentID string) string { return runID + "|" + agentID }

// AwaitResult registers interest in one agent's result for one run and blocks until it arrives or
// timeout elapses.
func (r *AgentRegistry) AwaitResult(runID, agentID string, timeout time.Duration) (AgentResult, bool) {
	key := resultKey(runID, agentID)
	r.mu.Lock()
	ch, ok := r.results[key]
	if !ok {
		ch = make(chan AgentResult, 1)
		r.results[key] = ch
	}
	r.mu.Unlock()

	select {
	case res := <-ch:
		r.mu.Lock()
		delete(r.results, key) // One-shot: a run ID is never reused, so this entry has no further readers.
		r.mu.Unlock()
		return res, true
	case <-time.After(timeout):
		return AgentResult{}, false
	}
}

// SubmitResult delivers an agent's result to whoever is awaiting it via AwaitResult.
func (r *AgentRegistry) SubmitResult(res AgentResult) {
	key := resultKey(res.RunID, res.AgentID)
	r.mu.Lock()
	ch, ok := r.results[key]
	if !ok {
		ch = make(chan AgentResult, 1)
		r.results[key] = ch
	}
	r.mu.Unlock()

	select {
	case ch <- res:
	default:
	}
}
