package chaos

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// MockChaosDriver provides an in-memory, zero-kernel-dependency driver for development,
// non-Linux OS environments (macOS/Windows), and CI pipelines.
type MockChaosDriver struct {
	mu               sync.RWMutex
	status           ChaosStatus
	appliedFaults    []FaultSpec
	revertCount      int
	shouldFailApply  bool
	shouldFailRevert bool
}

// NewMockChaosDriver creates an initialized mock driver.
func NewMockChaosDriver() *MockChaosDriver {
	return &MockChaosDriver{
		status: ChaosStatus{
			State: StateIdle,
		},
		appliedFaults: make([]FaultSpec, 0),
	}
}

// Apply records fault injection into memory.
func (m *MockChaosDriver) Apply(ctx context.Context, fault FaultSpec) error {
	if err := fault.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFailApply {
		m.status.State = StateFailed
		m.status.LastError = "simulated mock apply failure"
		return fmt.Errorf("%s", m.status.LastError)
	}

	now := time.Now()
	m.status = ChaosStatus{
		State:        StateInjected,
		CurrentFault: &fault,
		AppliedAt:    now,
		ExpiresAt:    now.Add(fault.Duration),
		LastError:    "",
	}
	m.appliedFaults = append(m.appliedFaults, fault)
	return nil
}

// Revert clears the active in-memory fault idempotenly.
func (m *MockChaosDriver) Revert(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFailRevert {
		m.status.State = StateFailed
		m.status.LastError = "simulated mock revert failure"
		return fmt.Errorf("%s", m.status.LastError)
	}

	m.revertCount++
	m.status.State = StateReverted
	m.status.CurrentFault = nil
	m.status.ExpiresAt = time.Time{}
	return nil
}

// Status returns a copy of live chaos status.
func (m *MockChaosDriver) Status() ChaosStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// History returns all applied faults.
func (m *MockChaosDriver) History() []FaultSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make([]FaultSpec, len(m.appliedFaults))
	copy(cp, m.appliedFaults)
	return cp
}

// RevertCount returns the total number of Revert calls.
func (m *MockChaosDriver) RevertCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.revertCount
}

// SetFailures configures simulation errors for testing.
func (m *MockChaosDriver) SetFailures(failApply, failRevert bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFailApply = failApply
	m.shouldFailRevert = failRevert
}

// MatchesActiveFault checks if a simulated outgoing network packet matches the currently active fault filter.
func (m *MockChaosDriver) MatchesActiveFault(dstIP net.IP, dstPort int, protocol string) (*FaultSpec, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.status.State != StateInjected || m.status.CurrentFault == nil {
		return nil, false
	}

	pf, err := ParseFilter(m.status.CurrentFault.Filter)
	if err != nil {
		return nil, false
	}

	if pf.Matches(dstIP, dstPort, protocol) {
		return m.status.CurrentFault, true
	}

	return nil, false
}
