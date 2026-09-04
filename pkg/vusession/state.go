package vusession

import (
	"fmt"
	"strings"
	"sync"
)

// SessionState provides a thread-safe, concurrency-resilient context store for a single Virtual User.
// It stores dynamic variables (such as tokens, session cookies, extracted IDs) and provides
// high-speed, low-allocation template interpolation.
type SessionState struct {
	mu   sync.RWMutex
	vars map[string]any
}

// NewSessionState initializes an empty thread-safe SessionState.
func NewSessionState() *SessionState {
	return &SessionState{
		vars: make(map[string]any),
	}
}

// NewSessionStateWith initializes SessionState populated with initial key-values.
func NewSessionStateWith(initial map[string]any) *SessionState {
	s := NewSessionState()
	if initial != nil {
		for k, v := range initial {
			s.vars[k] = v
		}
	}
	return s
}

// Set stores a key-value pair in the session state.
func (s *SessionState) Set(key string, val any) {
	s.mu.Lock()
	s.vars[key] = val
	s.mu.Unlock()
}

// SetMultiple stores multiple key-value pairs atomically under a single write lock.
func (s *SessionState) SetMultiple(items map[string]any) {
	if len(items) == 0 {
		return
	}
	s.mu.Lock()
	for k, v := range items {
		s.vars[k] = v
	}
	s.mu.Unlock()
}

// Get retrieves a value by key.
func (s *SessionState) Get(key string) (any, bool) {
	s.mu.RLock()
	val, ok := s.vars[key]
	s.mu.RUnlock()
	return val, ok
}

// GetString retrieves a value as string if found.
func (s *SessionState) GetString(key string) (string, bool) {
	s.mu.RLock()
	val, ok := s.vars[key]
	s.mu.RUnlock()
	if !ok || val == nil {
		return "", false
	}
	if str, ok := val.(string); ok {
		return str, true
	}
	return fmt.Sprintf("%v", val), true
}

// Delete removes a key from state.
func (s *SessionState) Delete(key string) {
	s.mu.Lock()
	delete(s.vars, key)
	s.mu.Unlock()
}

// Snapshot returns a shallow copy of the state's variables.
func (s *SessionState) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := make(map[string]any, len(s.vars))
	for k, v := range s.vars {
		cp[k] = v
	}
	return cp
}

// Interpolate substitutes placeholders (${var} or {{.var}} or {{var}}) with values from SessionState.
// Performs a single-pass scan with strings.Builder for minimal memory allocation.
func (s *SessionState) Interpolate(input string) (string, error) {
	if !strings.Contains(input, "${") && !strings.Contains(input, "{{") {
		return input, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	sb.Grow(len(input) + 32)

	n := len(input)
	i := 0
	for i < n {
		// Case 1: check for ${var}
		if i+1 < n && input[i] == '$' && input[i+1] == '{' {
			end := strings.IndexByte(input[i+2:], '}')
			if end >= 0 {
				varKey := strings.TrimSpace(input[i+2 : i+2+end])
				val, exists := s.vars[varKey]
				if !exists {
					return "", fmt.Errorf("variable ${%s} not found in session state", varKey)
				}
				s.appendVal(&sb, val)
				i = i + 2 + end + 1
				continue
			}
		}

		// Case 2: check for {{.var}} or {{var}}
		if i+1 < n && input[i] == '{' && input[i+1] == '{' {
			end := strings.Index(input[i+2:], "}}")
			if end >= 0 {
				rawKey := strings.TrimSpace(input[i+2 : i+2+end])
				varKey := strings.TrimPrefix(rawKey, ".")
				varKey = strings.TrimSpace(varKey)
				val, exists := s.vars[varKey]
				if !exists {
					return "", fmt.Errorf("variable {{.%s}} not found in session state", varKey)
				}
				s.appendVal(&sb, val)
				i = i + 2 + end + 2
				continue
			}
		}

		sb.WriteByte(input[i])
		i++
	}

	return sb.String(), nil
}

func (s *SessionState) appendVal(sb *strings.Builder, val any) {
	if val == nil {
		return
	}
	switch v := val.(type) {
	case string:
		sb.WriteString(v)
	case []byte:
		sb.Write(v)
	default:
		sb.WriteString(fmt.Sprintf("%v", v))
	}
}
