package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fakeK8sAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"metadata": map[string]string{"name": "checkout-1"}, "status": map[string]string{"phase": "Running"}},
			},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods/checkout-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/apis/autoscaling/v2/namespaces/default/horizontalpodautoscalers/checkout-hpa", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": map[string]int{"currentReplicas": 2, "desiredReplicas": 2}})
	})
	return httptest.NewServer(mux)
}

func TestHandlerK8sPodKill(t *testing.T) {
	fakeK8s := fakeK8sAPIServer(t)
	defer fakeK8s.Close()

	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{
		"api_server": fakeK8s.URL, "bearer_token": "t", "namespace": "default",
		"insecure_skip_verify": true, "label_selector": "app=checkout", "kill_percent": 100, "duration_seconds": 10,
	})
	resp, err := http.Post(ts.URL+"/api/v1/k8s/pod-kill", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandlerK8sPodKillRequiresSelector(t *testing.T) {
	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{"api_server": "https://x", "bearer_token": "t"})
	resp, err := http.Post(ts.URL+"/api/v1/k8s/pod-kill", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when label_selector is missing, got %d", resp.StatusCode)
	}
}

func TestHandlerK8sAutoscalerReport(t *testing.T) {
	fakeK8s := fakeK8sAPIServer(t)
	defer fakeK8s.Close()

	handler, _, _, _ := setupTestEnvironment()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// watch_duration_seconds/poll_interval_seconds of 0 fall back to 60s/5s server-side defaults,
	// which would make this test far too slow — use the smallest non-zero values instead (1s each).
	body, _ := json.Marshal(map[string]any{
		"api_server": fakeK8s.URL, "bearer_token": "t", "namespace": "default",
		"insecure_skip_verify": true, "hpa_name": "checkout-hpa",
		"watch_duration_seconds": 1, "poll_interval_seconds": 1,
	})

	resp, err := http.Post(ts.URL+"/api/v1/k8s/autoscaler-report", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
