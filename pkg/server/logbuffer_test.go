package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLogBuffer_WriteAndGetRecent(t *testing.T) {
	lb := NewLogBuffer(5)

	_, err := fmt.Fprintf(lb, "line 1\nline 2 with error\nline 3 with warning\n")
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	recent := lb.GetRecent(10)
	if len(recent) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(recent))
	}

	if recent[0].Level != "INFO" {
		t.Errorf("expected line 1 to be INFO, got %s", recent[0].Level)
	}
	if recent[1].Level != "ERROR" {
		t.Errorf("expected line 2 to be ERROR, got %s", recent[1].Level)
	}
	if recent[2].Level != "WARN" {
		t.Errorf("expected line 3 to be WARN, got %s", recent[2].Level)
	}

	// Test ring buffer limit
	_, _ = fmt.Fprintf(lb, "line 4\nline 5\nline 6\n")
	recent = lb.GetRecent(10)
	if len(recent) != 5 {
		t.Fatalf("expected capacity limit 5, got %d", len(recent))
	}
	if recent[len(recent)-1].Message != "line 6" {
		t.Errorf("expected last message to be 'line 6', got %s", recent[len(recent)-1].Message)
	}

	// Test clear
	lb.Clear()
	if len(lb.GetRecent(10)) != 0 {
		t.Errorf("expected empty buffer after clear")
	}
}

func TestLogBuffer_Subscribe(t *testing.T) {
	lb := NewLogBuffer(10)
	ch, unsubscribe := lb.Subscribe(10)
	defer unsubscribe()

	lb.Add(LogEntry{Message: "streamed message", Level: "INFO"})

	select {
	case entry := <-ch:
		if entry.Message != "streamed message" {
			t.Errorf("unexpected message: %s", entry.Message)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for subscription event")
	}
}

func TestSystemLogs_Endpoints(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	customBuf := NewLogBuffer(10)
	customBuf.Add(LogEntry{Message: "test log 1", Level: "INFO"})
	customBuf.Add(LogEntry{Message: "test log 2", Level: "ERROR"})
	handler.SetLogBuffer(customBuf)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// GET /api/v1/system/logs
	resp, err := http.Get(ts.URL + "/api/v1/system/logs?limit=5")
	if err != nil {
		t.Fatalf("failed GET logs: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Logs []LogEntry `json:"logs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}
	if len(payload.Logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(payload.Logs))
	}

	// POST /api/v1/system/logs/clear
	clearResp, err := http.Post(ts.URL+"/api/v1/system/logs/clear", "application/json", nil)
	if err != nil {
		t.Fatalf("failed POST clear: %v", err)
	}
	defer clearResp.Body.Close()

	if len(customBuf.GetRecent(10)) != 0 {
		t.Errorf("expected buffer to be cleared")
	}
}
