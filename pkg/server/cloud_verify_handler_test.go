package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerVerifyConfirmCloudRequiresCredentials(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"target_url": "https://my-shop.example"})
	resp, err := http.Post(ts.URL+"/api/v1/verify/confirm-cloud", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when AWS credentials are missing, got %d", resp.StatusCode)
	}
}

func TestHandlerVerifyConfirmCloudRejectsBlockedTarget(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"target_url": "https://google.com", "aws_access_key": "AKID", "aws_secret_key": "secret",
	})
	resp, err := http.Post(ts.URL+"/api/v1/verify/confirm-cloud", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for a blocklisted target, got %d", resp.StatusCode)
	}
}
