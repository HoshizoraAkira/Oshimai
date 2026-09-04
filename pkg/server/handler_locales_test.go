package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalesEndpoints(t *testing.T) {
	repo := NewMemoryRepository()
	eb := NewEventBus()
	defer eb.Close()
	coord := NewTestCoordinator(repo, eb, CoordinatorConfig{MaxConcurrentRuns: 1})
	h := NewAPIHandler(repo, coord, eb)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Test GET /api/v1/locales
	req := httptest.NewRequest(http.MethodGet, "/api/v1/locales", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var listRes struct {
		Languages []string `json:"languages"`
		Default   string   `json:"default"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listRes); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(listRes.Languages) != 3 {
		t.Fatalf("expected 3 languages, got %v", listRes.Languages)
	}

	// Test GET /api/v1/locales/id
	reqID := httptest.NewRequest(http.MethodGet, "/api/v1/locales/id", nil)
	recID := httptest.NewRecorder()
	mux.ServeHTTP(recID, reqID)

	if recID.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recID.Code)
	}
	var idRes struct {
		Language string            `json:"language"`
		Catalog  map[string]string `json:"catalog"`
	}
	if err := json.NewDecoder(recID.Body).Decode(&idRes); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if idRes.Language != "id" {
		t.Errorf("expected 'id', got %q", idRes.Language)
	}
	if idRes.Catalog["server.internal_error"] != "Terjadi kesalahan internal server" {
		t.Errorf("unexpected translation: %q", idRes.Catalog["server.internal_error"])
	}
}
