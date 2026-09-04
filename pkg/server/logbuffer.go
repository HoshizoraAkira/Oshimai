package server

import (
	"strings"
	"sync"
	"time"
)

// LogEntry represents a structured log event emitted by the control plane backend.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level"` // "INFO", "WARN", "ERROR", "DEBUG"
}

// LogBuffer maintains an in-memory ring buffer of recent logs and provides pub-sub streaming.
type LogBuffer struct {
	mu          sync.RWMutex
	capacity    int
	entries     []LogEntry
	subscribers map[chan LogEntry]struct{}
}

var (
	defaultLogBuffer     *LogBuffer
	initLogBufferOnce    sync.Once
)

// GetDefaultLogBuffer returns the singleton LogBuffer used across the control plane.
func GetDefaultLogBuffer() *LogBuffer {
	initLogBufferOnce.Do(func() {
		defaultLogBuffer = NewLogBuffer(500)
	})
	return defaultLogBuffer
}

// NewLogBuffer initializes a LogBuffer with a maximum capacity.
func NewLogBuffer(capacity int) *LogBuffer {
	if capacity <= 0 {
		capacity = 500
	}
	return &LogBuffer{
		capacity:    capacity,
		entries:     make([]LogEntry, 0, capacity),
		subscribers: make(map[chan LogEntry]struct{}),
	}
}

// Write implements io.Writer so the buffer can receive output from standard Go log.Logger.
func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	text := string(p)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		level := "INFO"
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "fatal") {
			level = "ERROR"
		} else if strings.Contains(lower, "warn") || strings.Contains(lower, "abort") {
			level = "WARN"
		}

		entry := LogEntry{
			Timestamp: time.Now().Format("15:04:05.000"),
			Message:   trimmed,
			Level:     level,
		}
		lb.Add(entry)
	}
	return len(p), nil
}

// Add appends a new entry to the ring buffer and notifies active subscribers.
func (lb *LogBuffer) Add(entry LogEntry) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format("15:04:05.000")
	}
	if entry.Level == "" {
		entry.Level = "INFO"
	}

	if len(lb.entries) >= lb.capacity {
		lb.entries = lb.entries[1:]
	}
	lb.entries = append(lb.entries, entry)

	// Broadcast to subscribers without blocking
	for ch := range lb.subscribers {
		select {
		case ch <- entry:
		default:
			// Subscriber channel full, drop to prevent latency stalling
		}
	}
}

// GetRecent returns a copy of up to limit recent log entries.
func (lb *LogBuffer) GetRecent(limit int) []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	total := len(lb.entries)
	if limit <= 0 || limit > total {
		limit = total
	}

	start := total - limit
	result := make([]LogEntry, limit)
	copy(result, lb.entries[start:])
	return result
}

// Clear flushes all stored log entries.
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.entries = lb.entries[:0]
}

// Subscribe returns a channel of live LogEntry and an unsubscribe closure.
func (lb *LogBuffer) Subscribe(bufferSize int) (<-chan LogEntry, func()) {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	ch := make(chan LogEntry, bufferSize)

	lb.mu.Lock()
	lb.subscribers[ch] = struct{}{}
	lb.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			lb.mu.Lock()
			delete(lb.subscribers, ch)
			close(ch)
			lb.mu.Unlock()
		})
	}
	return ch, unsubscribe
}
