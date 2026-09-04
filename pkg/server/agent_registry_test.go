package server

import (
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
)

func TestAgentRegistryRegisterAndList(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("agent-1", "jakarta")
	reg.Register("agent-2", "singapore")

	all := reg.ListConnected("")
	if len(all) != 2 {
		t.Fatalf("expected 2 connected agents, got %d", len(all))
	}

	jakarta := reg.ListConnected("jakarta")
	if len(jakarta) != 1 || jakarta[0].ID != "agent-1" {
		t.Errorf("expected only agent-1 in jakarta, got %+v", jakarta)
	}
}

func TestAgentRegistryDispatchAndPoll(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("agent-1", "jakarta")

	if err := reg.Dispatch("agent-1", AgentAssignment{RunID: "run-1"}); err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	a, ok := reg.Poll("agent-1", time.Second)
	if !ok || a.RunID != "run-1" {
		t.Fatalf("expected to receive the dispatched assignment, got ok=%v a=%+v", ok, a)
	}
}

func TestAgentRegistryDispatchRejectsUnknownAgent(t *testing.T) {
	reg := NewAgentRegistry()
	if err := reg.Dispatch("ghost", AgentAssignment{}); err == nil {
		t.Error("expected an error dispatching to an unregistered agent")
	}
}

func TestAgentRegistryPollTimesOutWithNoWork(t *testing.T) {
	reg := NewAgentRegistry()
	reg.Register("agent-1", "jakarta")

	start := time.Now()
	_, ok := reg.Poll("agent-1", 30*time.Millisecond)
	if ok {
		t.Error("expected no assignment within the timeout")
	}
	if time.Since(start) < 30*time.Millisecond {
		t.Error("expected Poll to actually wait for the timeout")
	}
}

func TestAgentRegistryResultRoundTrip(t *testing.T) {
	reg := NewAgentRegistry()

	done := make(chan AgentResult, 1)
	go func() {
		res, _ := reg.AwaitResult("run-1", "agent-1", time.Second)
		done <- res
	}()

	time.Sleep(20 * time.Millisecond) // Let AwaitResult register its channel first.
	reg.SubmitResult(AgentResult{RunID: "run-1", AgentID: "agent-1", Summary: &loadengine.ExecutionSummary{TotalRequests: 42}})

	res := <-done
	if res.Summary == nil || res.Summary.TotalRequests != 42 {
		t.Errorf("expected the submitted result to round-trip, got %+v", res)
	}
}

func TestMergeExecutionSummaries(t *testing.T) {
	a := &loadengine.ExecutionSummary{
		TotalRequests: 100, TotalErrors: 5, TotalDuration: 10 * time.Second,
		StatusCodes: map[int]int64{200: 95, 500: 5},
		Latency:     loadengine.LatencyStats{Min: 10 * time.Millisecond, Mean: 50 * time.Millisecond, P99: 200 * time.Millisecond, Max: 300 * time.Millisecond},
	}
	b := &loadengine.ExecutionSummary{
		TotalRequests: 300, TotalErrors: 0, TotalDuration: 12 * time.Second,
		StatusCodes: map[int]int64{200: 300},
		Latency:     loadengine.LatencyStats{Min: 5 * time.Millisecond, Mean: 30 * time.Millisecond, P99: 100 * time.Millisecond, Max: 150 * time.Millisecond},
	}

	merged := mergeExecutionSummaries([]*loadengine.ExecutionSummary{a, b})
	if merged.TotalRequests != 400 {
		t.Errorf("expected 400 total requests, got %d", merged.TotalRequests)
	}
	if merged.TotalErrors != 5 {
		t.Errorf("expected 5 total errors, got %d", merged.TotalErrors)
	}
	if merged.StatusCodes[200] != 395 || merged.StatusCodes[500] != 5 {
		t.Errorf("expected merged status codes, got %+v", merged.StatusCodes)
	}
	if merged.Latency.Min != 5*time.Millisecond {
		t.Errorf("expected merged Min to be the true minimum (5ms), got %v", merged.Latency.Min)
	}
	if merged.Latency.Max != 300*time.Millisecond {
		t.Errorf("expected merged Max to be the true maximum (300ms), got %v", merged.Latency.Max)
	}
	if merged.TotalDuration != 12*time.Second {
		t.Errorf("expected merged duration to be the longer of the two, got %v", merged.TotalDuration)
	}
}
