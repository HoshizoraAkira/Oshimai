package server

import (
	"sync"
)

// EventBus provides lightweight in-memory pub-sub distribution of live telemetry events to subscribers.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan TelemetryEvent]struct{}
	closed      bool
}

// NewEventBus creates an initialized EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]map[chan TelemetryEvent]struct{}),
	}
}

// Publish distributes an event to all subscribers listening to event.RunID.
// Delivery is non-blocking to prevent slow clients from stalling load execution.
func (eb *EventBus) Publish(event TelemetryEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.closed {
		return
	}

	subs, exists := eb.subscribers[event.RunID]
	if !exists || len(subs) == 0 {
		return
	}

	for ch := range subs {
		select {
		case ch <- event:
		default:
			// Buffer full, drop event to prioritize low-latency execution
		}
	}
}

// Subscribe returns a channel receiving telemetry events for runID and an unsubscribe cleanup closure.
func (eb *EventBus) Subscribe(runID string, bufferSize int) (<-chan TelemetryEvent, func()) {
	if bufferSize <= 0 {
		bufferSize = 32
	}
	ch := make(chan TelemetryEvent, bufferSize)

	eb.mu.Lock()
	if eb.closed {
		eb.mu.Unlock()
		close(ch)
		return ch, func() {}
	}

	if _, ok := eb.subscribers[runID]; !ok {
		eb.subscribers[runID] = make(map[chan TelemetryEvent]struct{})
	}
	eb.subscribers[runID][ch] = struct{}{}
	eb.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			eb.mu.Lock()
			defer eb.mu.Unlock()

			if subs, ok := eb.subscribers[runID]; ok {
				delete(subs, ch)
				if len(subs) == 0 {
					delete(eb.subscribers, runID)
				}
			}
			if !eb.closed {
				close(ch)
			}
		})
	}

	return ch, unsubscribe
}

// Close terminates the event bus and shuts down all active subscriber channels.
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}
	eb.closed = true

	for _, subs := range eb.subscribers {
		for ch := range subs {
			close(ch)
		}
	}
	eb.subscribers = nil
}
