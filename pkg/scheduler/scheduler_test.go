package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

func TestSchedulerAddListRemove(t *testing.T) {
	s := New(func(ctx context.Context, sched GameDaySchedule) error { return nil }, nil)

	id, err := s.Add("Weekly Checkout GameDay", time.Monday, 9, 0, json.RawMessage(`{"vus":50}`))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if len(s.List()) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(s.List()))
	}

	if !s.Remove(id) {
		t.Error("expected Remove to succeed for an existing schedule")
	}
	if len(s.List()) != 0 {
		t.Error("expected 0 schedules after removal")
	}
	if s.Remove("does-not-exist") {
		t.Error("expected Remove to report false for an unknown ID")
	}
}

func TestSchedulerAddRejectsInvalidTime(t *testing.T) {
	s := New(func(ctx context.Context, sched GameDaySchedule) error { return nil }, nil)
	if _, err := s.Add("bad", time.Monday, 25, 0, nil); err == nil {
		t.Error("expected an error for an invalid hour")
	}
	if _, err := s.Add("bad", time.Monday, 9, 61, nil); err == nil {
		t.Error("expected an error for an invalid minute")
	}
}

func TestGameDayScheduleMatches(t *testing.T) {
	sched := &GameDaySchedule{Weekday: time.Monday, HourUTC: 9, MinuteUTC: 0, Enabled: true}
	monday9am := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC) // A Monday.
	if !sched.matches(monday9am) {
		t.Error("expected a schedule to match its exact configured time")
	}

	tuesday9am := monday9am.AddDate(0, 0, 1)
	if sched.matches(tuesday9am) {
		t.Error("did not expect a match on the wrong weekday")
	}

	sched.Enabled = false
	if sched.matches(monday9am) {
		t.Error("did not expect a disabled schedule to match")
	}
}

func TestGameDayScheduleDoesNotDoubleFire(t *testing.T) {
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	sched := &GameDaySchedule{Weekday: time.Monday, HourUTC: 9, MinuteUTC: 0, Enabled: true, LastFiredAt: now}
	if sched.matches(now.Add(10 * time.Second)) {
		t.Error("expected a schedule that just fired to not immediately re-fire")
	}
}

func TestSchedulerCheckAndFireInvokesTrigger(t *testing.T) {
	var fired int32
	var capturedPayload json.RawMessage
	s := New(func(ctx context.Context, sched GameDaySchedule) error {
		atomic.AddInt32(&fired, 1)
		capturedPayload = sched.Payload
		return nil
	}, nil)

	payload := json.RawMessage(`{"scenario":"checkout"}`)
	s.Add("Weekly GameDay", time.Monday, 9, 0, payload)

	monday9am := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	s.checkAndFire(context.Background(), monday9am)

	if atomic.LoadInt32(&fired) != 1 {
		t.Fatalf("expected the trigger to fire exactly once, got %d", fired)
	}
	if string(capturedPayload) != string(payload) {
		t.Errorf("expected the trigger to receive the schedule's payload, got %s", capturedPayload)
	}

	// A second check at the same minute must not re-fire.
	s.checkAndFire(context.Background(), monday9am.Add(5*time.Second))
	if atomic.LoadInt32(&fired) != 1 {
		t.Error("expected no double-fire within the same window")
	}
}

func TestSchedulerReportsTriggerErrors(t *testing.T) {
	var reportedID string
	s := New(
		func(ctx context.Context, sched GameDaySchedule) error { return errBoom },
		func(id string, err error) { reportedID = id },
	)
	id, _ := s.Add("Failing GameDay", time.Monday, 9, 0, nil)
	monday9am := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	s.checkAndFire(context.Background(), monday9am)

	if reportedID != id {
		t.Errorf("expected onError to be called with schedule ID %q, got %q", id, reportedID)
	}
}
