package generator

import (
	"testing"
	"time"
)

func sampleReplaySpans() []OTelSpan {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []OTelSpan{
		{TraceID: "t1", SpanID: "s1", Method: "GET", Route: "/login", StartTime: base, EndTime: base.Add(100 * time.Millisecond)},
		{TraceID: "t1", SpanID: "s2", Method: "GET", Route: "/browse", StartTime: base.Add(2 * time.Second), EndTime: base.Add(2200 * time.Millisecond)},
		{TraceID: "t1", SpanID: "s3", Method: "POST", Route: "/checkout", StartTime: base.Add(6 * time.Second), EndTime: base.Add(6300 * time.Millisecond)},
		{TraceID: "t2", SpanID: "s4", Method: "GET", Route: "/other-session", StartTime: base, EndTime: base.Add(50 * time.Millisecond)},
	}
}

func TestBuildReplayScenarioReproducesSequenceAndPacing(t *testing.T) {
	sc, err := BuildReplayScenario("t1", sampleReplaySpans(), 1.0, GeneratorConfig{BaseURL: "https://shop.example.co.id"})
	if err != nil {
		t.Fatalf("BuildReplayScenario failed: %v", err)
	}
	if len(sc.Steps) != 3 {
		t.Fatalf("expected 3 steps (only t1's spans), got %d", len(sc.Steps))
	}

	first := sc.Steps[sc.InitialStepID]
	if first.ThinkTime.AsDuration() != 0 {
		t.Errorf("expected the first step to have no think-time, got %v", first.ThinkTime.AsDuration())
	}

	// The second span started ~1.9s after the first ended — that gap must show up as think-time
	// on the corresponding step (found by following the first step's transition).
	secondID := first.Transitions[0].TargetStepID
	second := sc.Steps[secondID]
	if second.ThinkTime.AsDuration() < 1800*time.Millisecond || second.ThinkTime.AsDuration() > 2000*time.Millisecond {
		t.Errorf("expected ~1.9s think-time on the second step, got %v", second.ThinkTime.AsDuration())
	}
}

func TestBuildReplayScenarioSpeedMultiplier(t *testing.T) {
	normal, _ := BuildReplayScenario("t1", sampleReplaySpans(), 1.0, GeneratorConfig{BaseURL: "https://x.test"})
	fast, _ := BuildReplayScenario("t1", sampleReplaySpans(), 2.0, GeneratorConfig{BaseURL: "https://x.test"})

	secondNormal := normal.Steps[normal.Steps[normal.InitialStepID].Transitions[0].TargetStepID]
	secondFast := fast.Steps[fast.Steps[fast.InitialStepID].Transitions[0].TargetStepID]

	if secondFast.ThinkTime.AsDuration() >= secondNormal.ThinkTime.AsDuration() {
		t.Error("expected a 2x speed multiplier to roughly halve the think-time")
	}
}

func TestBuildReplayScenarioRejectsUnknownTraceID(t *testing.T) {
	if _, err := BuildReplayScenario("does-not-exist", sampleReplaySpans(), 1.0, GeneratorConfig{}); err == nil {
		t.Error("expected an error for a trace_id with no matching spans")
	}
}

func TestListReplayableTraces(t *testing.T) {
	ids := ListReplayableTraces(sampleReplaySpans())
	if len(ids) != 2 {
		t.Fatalf("expected 2 distinct trace IDs, got %d: %v", len(ids), ids)
	}
}
