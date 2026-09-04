package chaos

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ChaosDriver abstracts network fault injection engines.
type ChaosDriver interface {
	Apply(ctx context.Context, fault FaultSpec) error
	Revert(ctx context.Context) error
	Status() ChaosStatus
}

// ManagedChaosDriver decorates any ChaosDriver with critical safety guarantees:
// 1. Dead-Man Switch (watchdog timer auto-revert)
// 2. OS Signal Trap (SIGINT / SIGTERM cleanup)
// 3. Idempotent concurrent execution locks
// 4. Strict State Machine transitions
type ManagedChaosDriver struct {
	underlying ChaosDriver

	mu            sync.Mutex
	watchdogTimer *time.Timer
	stopSignals   chan struct{}
	sigChan       chan os.Signal

	status ChaosStatus
}

// NewManagedChaosDriver creates a safety-wrapped chaos driver.
func NewManagedChaosDriver(driver ChaosDriver) *ManagedChaosDriver {
	m := &ManagedChaosDriver{
		underlying:  driver,
		stopSignals: make(chan struct{}),
		sigChan:     make(chan os.Signal, 1),
		status: ChaosStatus{
			State: StateIdle,
		},
	}

	// Register OS signal trap
	signal.Notify(m.sigChan, os.Interrupt, syscall.SIGTERM)
	go m.handleSignals()

	return m
}

func (m *ManagedChaosDriver) handleSignals() {
	select {
	case <-m.stopSignals:
		return
	case sig := <-m.sigChan:
		if sig != nil {
			_ = m.Revert(context.Background())
		}
	}
}

// Close unregisters signal traps and stops active timers.
func (m *ManagedChaosDriver) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	signal.Stop(m.sigChan)
	select {
	case <-m.stopSignals:
	default:
		close(m.stopSignals)
	}

	if m.watchdogTimer != nil {
		m.watchdogTimer.Stop()
		m.watchdogTimer = nil
	}

	return m.underlying.Revert(context.Background())
}

// Apply executes the fault injection and arms the Dead-Man Switch watchdog timer.
func (m *ManagedChaosDriver) Apply(ctx context.Context, fault FaultSpec) error {
	if err := fault.Validate(); err != nil {
		return fmt.Errorf("invalid fault spec: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop any preexisting watchdog
	if m.watchdogTimer != nil {
		m.watchdogTimer.Stop()
		m.watchdogTimer = nil
	}

	m.status.State = StateApplying
	m.status.CurrentFault = &fault

	// Delegate to underlying driver
	if err := m.underlying.Apply(ctx, fault); err != nil {
		m.status.State = StateFailed
		m.status.LastError = err.Error()
		return fmt.Errorf("chaos apply failed: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(fault.Duration)

	m.status.State = StateInjected
	m.status.AppliedAt = now
	m.status.ExpiresAt = expiresAt
	m.status.LastError = ""

	// Arm Dead-Man Switch watchdog timer
	m.watchdogTimer = time.AfterFunc(fault.Duration, func() {
		// Asynchronous auto-revert triggered by watchdog expiry
		_ = m.Revert(context.Background())
	})

	// Also monitor contextual cancellation
	go func(targetFaultID string) {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			isCurrent := m.status.CurrentFault != nil && m.status.CurrentFault.ID == targetFaultID
			m.mu.Unlock()
			if isCurrent {
				_ = m.Revert(context.Background())
			}
		case <-m.stopSignals:
			return
		}
	}(fault.ID)

	return nil
}

// Revert idempotenly removes active chaos rules and resets the Dead-Man Switch.
func (m *ManagedChaosDriver) Revert(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel active watchdog
	if m.watchdogTimer != nil {
		m.watchdogTimer.Stop()
		m.watchdogTimer = nil
	}

	if m.status.State == StateIdle || m.status.State == StateReverted {
		// Idempotent: nothing to revert
		return nil
	}

	m.status.State = StateReverting

	err := m.underlying.Revert(ctx)
	if err != nil {
		m.status.State = StateFailed
		m.status.LastError = err.Error()
		return fmt.Errorf("chaos revert failed: %w", err)
	}

	m.status.State = StateReverted
	m.status.CurrentFault = nil
	m.status.ExpiresAt = time.Time{}
	m.status.LastError = ""
	return nil
}

// Status returns a copy of live chaos status.
func (m *ManagedChaosDriver) Status() ChaosStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}
