// Package scheduler implements recurring "GameDay" triggers: a saved run configuration that fires
// automatically on a weekly cadence instead of requiring someone to remember to click "run" every
// week. It knows nothing about scenarios, chaos, or the control plane — it just calls a Trigger
// function on schedule — so it has no import dependency on pkg/server and can't create one.
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// GameDaySchedule is one recurring trigger: fire every Weekday at HourUTC:MinuteUTC, calling the
// Scheduler's Trigger with Payload (typically a JSON-encoded run request the caller knows how to
// interpret — the scheduler itself never looks inside it).
type GameDaySchedule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Weekday     time.Weekday    `json:"weekday"`
	HourUTC     int             `json:"hour_utc"`   // 0-23
	MinuteUTC   int             `json:"minute_utc"` // 0-59
	Enabled     bool            `json:"enabled"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"created_at"`
	LastFiredAt time.Time       `json:"last_fired_at,omitempty"`
}

// matches reports whether now falls within the firing window (same weekday/hour/minute) and this
// schedule hasn't already fired for that same window.
func (s *GameDaySchedule) matches(now time.Time) bool {
	if !s.Enabled {
		return false
	}
	if now.Weekday() != s.Weekday || now.UTC().Hour() != s.HourUTC || now.UTC().Minute() != s.MinuteUTC {
		return false
	}
	// Already fired within the last 90 seconds — don't double-fire if the check loop ticks twice
	// inside the same minute window.
	return now.Sub(s.LastFiredAt) > 90*time.Second
}

// Trigger is called once per firing schedule. Errors are logged by the caller of Start's loop but
// never stop the scheduler — one bad GameDay shouldn't cancel every future one.
type Trigger func(ctx context.Context, schedule GameDaySchedule) error

// Scheduler owns a set of GameDaySchedules and fires Trigger for each as its window arrives.
type Scheduler struct {
	mu        sync.Mutex
	schedules map[string]*GameDaySchedule
	trigger   Trigger
	onError   func(scheduleID string, err error)
	cancel    context.CancelFunc
	seq       int
}

// New creates a Scheduler that calls trigger for each firing schedule. onError (optional, may be
// nil) is called if a trigger invocation returns an error.
func New(trigger Trigger, onError func(scheduleID string, err error)) *Scheduler {
	return &Scheduler{
		schedules: make(map[string]*GameDaySchedule),
		trigger:   trigger,
		onError:   onError,
	}
}

// Add registers a new schedule and returns its assigned ID.
func (s *Scheduler) Add(name string, weekday time.Weekday, hourUTC, minuteUTC int, payload json.RawMessage) (string, error) {
	if hourUTC < 0 || hourUTC > 23 || minuteUTC < 0 || minuteUTC > 59 {
		return "", fmt.Errorf("invalid time of day %02d:%02d UTC", hourUTC, minuteUTC)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := fmt.Sprintf("gameday-%d", s.seq)
	s.schedules[id] = &GameDaySchedule{
		ID: id, Name: name, Weekday: weekday, HourUTC: hourUTC, MinuteUTC: minuteUTC,
		Enabled: true, Payload: payload, CreatedAt: time.Now(),
	}
	return id, nil
}

// Remove deletes a schedule. Returns false if it didn't exist.
func (s *Scheduler) Remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.schedules[id]; !ok {
		return false
	}
	delete(s.schedules, id)
	return true
}

// SetEnabled toggles a schedule without deleting it.
func (s *Scheduler) SetEnabled(id string, enabled bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sched, ok := s.schedules[id]
	if !ok {
		return false
	}
	sched.Enabled = enabled
	return true
}

// List returns every schedule, a snapshot safe for the caller to serialize.
func (s *Scheduler) List() []GameDaySchedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]GameDaySchedule, 0, len(s.schedules))
	for _, sched := range s.schedules {
		out = append(out, *sched)
	}
	return out
}

// Start begins the check loop (default interval 30s if checkInterval <= 0) and returns
// immediately; call the returned stop function (or Stop) to end it.
func (s *Scheduler) Start(ctx context.Context, checkInterval time.Duration) {
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}
	loopCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-loopCtx.Done():
				return
			case now := <-ticker.C:
				s.checkAndFire(loopCtx, now)
			}
		}
	}()
}

// Stop ends the check loop started by Start. Safe to call even if Start was never called.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scheduler) checkAndFire(ctx context.Context, now time.Time) {
	s.mu.Lock()
	var due []*GameDaySchedule
	for _, sched := range s.schedules {
		if sched.matches(now) {
			sched.LastFiredAt = now
			due = append(due, sched)
		}
	}
	s.mu.Unlock()

	for _, sched := range due {
		if err := s.trigger(ctx, *sched); err != nil && s.onError != nil {
			s.onError(sched.ID, err)
		}
	}
}
