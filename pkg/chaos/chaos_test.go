package chaos

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStateMachineTransitions verifies correct lifecycle states:
// Idle -> Injected -> Reverted.
func TestStateMachineTransitions(t *testing.T) {
	mock := NewMockChaosDriver()
	driver := NewManagedChaosDriver(mock)
	defer driver.Close()

	if driver.Status().State != StateIdle {
		t.Fatalf("expected initial state %s, got %s", StateIdle, driver.Status().State)
	}

	fault := FaultSpec{
		ID:   "fault_latency_01",
		Type: FaultLatency,
		Filter: FilterConfig{
			Interface: "eth0",
		},
		Latency:  100 * time.Millisecond,
		Jitter:   10 * time.Millisecond,
		Duration: 5 * time.Second,
	}

	if err := driver.Apply(context.Background(), fault); err != nil {
		t.Fatalf("failed to apply fault: %v", err)
	}

	status := driver.Status()
	if status.State != StateInjected {
		t.Fatalf("expected state %s, got %s", StateInjected, status.State)
	}
	if status.CurrentFault == nil || status.CurrentFault.ID != "fault_latency_01" {
		t.Fatalf("expected CurrentFault to be fault_latency_01, got %+v", status.CurrentFault)
	}
	if status.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expected ExpiresAt to be in the future")
	}

	if err := driver.Revert(context.Background()); err != nil {
		t.Fatalf("failed to revert fault: %v", err)
	}

	if driver.Status().State != StateReverted {
		t.Fatalf("expected state %s, got %s", StateReverted, driver.Status().State)
	}
	if driver.Status().CurrentFault != nil {
		t.Fatalf("expected CurrentFault to be nil after revert")
	}
}

// TestDeadManSwitchAutoRevert verifies that the watchdog timer automatically cleans up
// the injected fault once the TTL expires.
func TestDeadManSwitchAutoRevert(t *testing.T) {
	mock := NewMockChaosDriver()
	driver := NewManagedChaosDriver(mock)
	defer driver.Close()

	ttl := 100 * time.Millisecond
	fault := FaultSpec{
		ID:   "fault_deadman_test",
		Type: FaultPacketLoss,
		Filter: FilterConfig{
			Interface: "eth0",
		},
		LossPercent: 15.0,
		Duration:    ttl,
	}

	if err := driver.Apply(context.Background(), fault); err != nil {
		t.Fatalf("failed to apply fault: %v", err)
	}

	if driver.Status().State != StateInjected {
		t.Fatalf("expected state %s immediately after apply", StateInjected)
	}

	// Wait for Dead-Man Switch to fire
	time.Sleep(ttl + 50*time.Millisecond)

	if driver.Status().State != StateReverted {
		t.Fatalf("dead-man switch failed to auto-revert: expected state %s, got %s",
			StateReverted, driver.Status().State)
	}

	if mock.RevertCount() < 1 {
		t.Fatalf("expected mock driver Revert to have been called by watchdog")
	}
}

// TestContextCancellationRevert verifies that canceling the apply context triggers immediate revert.
func TestContextCancellationRevert(t *testing.T) {
	mock := NewMockChaosDriver()
	driver := NewManagedChaosDriver(mock)
	defer driver.Close()

	ctx, cancel := context.WithCancel(context.Background())

	fault := FaultSpec{
		ID:   "fault_ctx_cancel",
		Type: FaultLatency,
		Filter: FilterConfig{
			Interface: "eth0",
		},
		Latency:  50 * time.Millisecond,
		Duration: 10 * time.Second, // Long duration; cancel should trigger first
	}

	if err := driver.Apply(ctx, fault); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	if driver.Status().State != StateInjected {
		t.Fatalf("expected state %s", StateInjected)
	}

	// Cancel context
	cancel()
	time.Sleep(50 * time.Millisecond)

	if driver.Status().State != StateReverted {
		t.Fatalf("context cancellation failed to revert: expected %s, got %s",
			StateReverted, driver.Status().State)
	}
}

// TestIdempotentRevert verifies that multiple calls to Revert do not cause errors or panics.
func TestIdempotentRevert(t *testing.T) {
	mock := NewMockChaosDriver()
	driver := NewManagedChaosDriver(mock)
	defer driver.Close()

	// Revert before apply
	if err := driver.Revert(context.Background()); err != nil {
		t.Fatalf("initial revert should be a no-op, got error: %v", err)
	}

	fault := FaultSpec{
		ID:       "idempotent_test",
		Type:     FaultLatency,
		Filter:   FilterConfig{Interface: "eth0"},
		Latency:  20 * time.Millisecond,
		Duration: 5 * time.Second,
	}

	_ = driver.Apply(context.Background(), fault)

	// Call Revert 5 times in a row
	for i := 0; i < 5; i++ {
		if err := driver.Revert(context.Background()); err != nil {
			t.Fatalf("revert call %d failed: %v", i, err)
		}
		if driver.Status().State != StateReverted {
			t.Fatalf("expected state %s after revert %d", StateReverted, i)
		}
	}
}

// TestFilterMatchingIsolation verifies that the traffic filter correctly distinguishes
// target packets from non-target packets to prevent blast radius leak.
func TestFilterMatchingIsolation(t *testing.T) {
	cfg := FilterConfig{
		Interface:  "eth0",
		TargetIP:   "10.200.0.0/16",
		TargetPort: 8080,
		Protocol:   "tcp",
	}

	pf, err := ParseFilter(cfg)
	if err != nil {
		t.Fatalf("failed to parse filter: %v", err)
	}

	// Case 1: Exact match
	targetIP := net.ParseIP("10.200.42.10")
	if !pf.Matches(targetIP, 8080, "tcp") {
		t.Errorf("expected packet to match filter")
	}

	// Case 2: Wrong port
	if pf.Matches(targetIP, 9090, "tcp") {
		t.Errorf("packet with port 9090 should NOT match filter for port 8080")
	}

	// Case 3: Wrong IP (outside CIDR)
	foreignIP := net.ParseIP("192.168.1.1")
	if pf.Matches(foreignIP, 8080, "tcp") {
		t.Errorf("foreign IP %s should NOT match filter for 10.200.0.0/16", foreignIP)
	}

	// Case 4: Wrong protocol
	if pf.Matches(targetIP, 8080, "udp") {
		t.Errorf("UDP packet should NOT match TCP filter")
	}

	// Case 5: Single IP without CIDR mask
	singleCfg := FilterConfig{
		Interface: "eth0",
		TargetIP:  "172.16.0.5",
	}
	singlePf, err := ParseFilter(singleCfg)
	if err != nil {
		t.Fatalf("failed to parse single IP: %v", err)
	}
	if !singlePf.Matches(net.ParseIP("172.16.0.5"), 80, "tcp") {
		t.Errorf("single IP 172.16.0.5 should match")
	}
	if singlePf.Matches(net.ParseIP("172.16.0.6"), 80, "tcp") {
		t.Errorf("single IP 172.16.0.6 should NOT match")
	}
}

// TestBuildU32FilterArgs verifies standard tc command generation.
func TestBuildU32FilterArgs(t *testing.T) {
	cfg := FilterConfig{
		Interface:  "eth0",
		TargetIP:   "192.168.1.100/32",
		TargetPort: 443,
	}

	args, err := BuildU32FilterArgs(cfg, "1:", "1:2")
	if err != nil {
		t.Fatalf("failed to build u32 args: %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "match ip dst 192.168.1.100/32") {
		t.Errorf("args missing ip match: %s", joined)
	}
	if !strings.Contains(joined, "match ip dport 443 0xffff") {
		t.Errorf("args missing port match: %s", joined)
	}
	if !strings.Contains(joined, "flowid 1:2") {
		t.Errorf("args missing flowid: %s", joined)
	}
}

// TestNetemBuilderArgs verifies tc netem parameter construction.
func TestNetemBuilderArgs(t *testing.T) {
	spec := FaultSpec{
		ID:   "full_spec",
		Type: FaultComposite,
		Filter: FilterConfig{
			Interface: "eth0",
		},
		Latency:           150 * time.Millisecond,
		Jitter:            20 * time.Millisecond,
		Correlation:       0.25,
		LossPercent:       5.5,
		CorruptionPercent: 2.0,
		RateLimitKbps:     1000,
		Duration:          10 * time.Second,
	}

	args, err := BuildNetemArgs(spec, "1:2", "20:")
	if err != nil {
		t.Fatalf("failed to build netem args: %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "delay 150ms 20ms 25.0%") {
		t.Errorf("expected delay and jitter with correlation, got: %s", joined)
	}
	if !strings.Contains(joined, "loss 5.50%") {
		t.Errorf("expected loss, got: %s", joined)
	}
	if !strings.Contains(joined, "corrupt 2.00%") {
		t.Errorf("expected corrupt, got: %s", joined)
	}
	if !strings.Contains(joined, "rate 1000kbit") {
		t.Errorf("expected rate, got: %s", joined)
	}
}

// TestFaultSpecValidation verifies validation error rules.
func TestFaultSpecValidation(t *testing.T) {
	// Missing ID
	if err := (&FaultSpec{Filter: FilterConfig{Interface: "eth0"}, Duration: time.Second}).Validate(); err == nil {
		t.Errorf("expected error on missing ID")
	}

	// Missing Interface
	if err := (&FaultSpec{ID: "f1", Duration: time.Second}).Validate(); err == nil {
		t.Errorf("expected error on missing Interface")
	}

	// Missing Duration (TTL)
	if err := (&FaultSpec{ID: "f1", Filter: FilterConfig{Interface: "eth0"}}).Validate(); err == nil {
		t.Errorf("expected error on missing Duration (Dead-Man Switch requirement)")
	}

	// Jitter > Latency
	if err := (&FaultSpec{
		ID:       "f1",
		Filter:   FilterConfig{Interface: "eth0"},
		Duration: time.Second,
		Latency:  10 * time.Millisecond,
		Jitter:   20 * time.Millisecond,
	}).Validate(); err == nil {
		t.Errorf("expected error when jitter > latency")
	}

	// Invalid Loss
	if err := (&FaultSpec{
		ID:          "f1",
		Filter:      FilterConfig{Interface: "eth0"},
		Duration:    time.Second,
		LossPercent: 150,
	}).Validate(); err == nil {
		t.Errorf("expected error when loss > 100")
	}
}

// TestConcurrencyAndRaceSafety verifies thread-safe calls to Apply, Revert, and Status under race detector.
func TestConcurrencyAndRaceSafety(t *testing.T) {
	mock := NewMockChaosDriver()
	driver := NewManagedChaosDriver(mock)
	defer driver.Close()

	var wg sync.WaitGroup
	const routines = 20

	for i := 0; i < routines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for iter := 0; iter < 10; iter++ {
				_ = driver.Apply(context.Background(), FaultSpec{
					ID:       "concurrent_fault",
					Type:     FaultLatency,
					Filter:   FilterConfig{Interface: "eth0"},
					Latency:  time.Duration(iter) * time.Millisecond,
					Duration: 50 * time.Millisecond,
				})
				_ = driver.Status()
				_ = driver.Revert(context.Background())
			}
		}(i)
	}

	wg.Wait()
}

// TestMockDriverHelpersAndFailures verifies mock driver simulation controls and inspection helpers.
func TestMockDriverHelpersAndFailures(t *testing.T) {
	mock := NewMockChaosDriver()
	mock.SetFailures(true, false)

	driver := NewManagedChaosDriver(mock)
	defer driver.Close()

	fault := FaultSpec{
		ID:       "fail_spec",
		Type:     FaultLatency,
		Filter:   FilterConfig{Interface: "eth0", TargetIP: "10.0.0.1"},
		Latency:  10 * time.Millisecond,
		Duration: 1 * time.Second,
	}

	err := driver.Apply(context.Background(), fault)
	if err == nil {
		t.Fatalf("expected apply to fail")
	}
	if driver.Status().State != StateFailed {
		t.Errorf("expected StateFailed, got %s", driver.Status().State)
	}

	// Reset failure and verify history and active matching
	mock.SetFailures(false, false)
	if err := driver.Apply(context.Background(), fault); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	hist := mock.History()
	if len(hist) != 1 || hist[0].ID != "fail_spec" {
		t.Errorf("unexpected history: %+v", hist)
	}

	matchedFault, matched := mock.MatchesActiveFault(net.ParseIP("10.0.0.1"), 0, "tcp")
	if !matched || matchedFault == nil {
		t.Errorf("expected 10.0.0.1 to match active fault")
	}

	_, matchedOther := mock.MatchesActiveFault(net.ParseIP("10.0.0.2"), 0, "tcp")
	if matchedOther {
		t.Errorf("10.0.0.2 should not match")
	}

	// Test mock revert failure
	mock.SetFailures(false, true)
	if err := driver.Revert(context.Background()); err == nil {
		t.Errorf("expected revert to fail when configured")
	}
}

// TestFilterAndNetemEdgeCases tests parsing edge cases and duration formatting.
func TestFilterAndNetemEdgeCases(t *testing.T) {
	// Empty interface
	if _, err := ParseFilter(FilterConfig{}); err == nil {
		t.Errorf("expected error on empty interface")
	}

	// Invalid IP
	if _, err := ParseFilter(FilterConfig{Interface: "eth0", TargetIP: "invalid-ip"}); err == nil {
		t.Errorf("expected error on invalid IP")
	}

	// Invalid Port
	if _, err := ParseFilter(FilterConfig{Interface: "eth0", TargetPort: 70000}); err == nil {
		t.Errorf("expected error on port > 65535")
	}

	// Second-based duration formatting in netem builder
	secSpec := FaultSpec{
		ID:       "sec_spec",
		Type:     FaultLatency,
		Filter:   FilterConfig{Interface: "eth0"},
		Latency:  2 * time.Second,
		Duration: 5 * time.Second,
	}
	args, err := BuildNetemArgs(secSpec, "", "")
	if err != nil {
		t.Fatalf("failed to build args: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "delay 2s") {
		t.Errorf("expected 'delay 2s', got %s", joined)
	}

	// Spec without any fault
	emptyFault := FaultSpec{
		ID:       "empty_fault",
		Filter:   FilterConfig{Interface: "eth0"},
		Duration: time.Second,
	}
	if _, err := BuildNetemArgs(emptyFault, "", ""); err == nil {
		t.Errorf("expected error when no fault attribute is configured")
	}
}
