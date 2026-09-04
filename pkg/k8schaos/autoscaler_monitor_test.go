package k8schaos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMonitorAutoscalerDetectsScaleUp(t *testing.T) {
	var replicas int32 = 2
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/autoscaling/v2/namespaces/default/horizontalpodautoscalers/checkout-hpa", func(w http.ResponseWriter, r *http.Request) {
		current := atomic.LoadInt32(&replicas)
		json.NewEncoder(w).Encode(map[string]any{"status": map[string]int{"currentReplicas": int(current), "desiredReplicas": int(current)}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "t", InsecureSkipVerify: true})

	// Bump the replica count shortly after monitoring starts, simulating a real HPA reaction.
	go func() {
		time.Sleep(60 * time.Millisecond)
		atomic.StoreInt32(&replicas, 5)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	report, err := MonitorAutoscaler(ctx, client, "default", "checkout-hpa", 20*time.Millisecond)
	if err != nil {
		t.Fatalf("MonitorAutoscaler failed: %v", err)
	}
	if !report.ScaledUp {
		t.Error("expected the monitor to detect a scale-up")
	}
	if report.PeakReplicas != 5 {
		t.Errorf("expected peak replicas of 5, got %d", report.PeakReplicas)
	}
	if report.Verdict == "" {
		t.Error("expected a non-empty verdict")
	}
}

func TestMonitorAutoscalerReportsNoScaleUp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/autoscaling/v2/namespaces/default/horizontalpodautoscalers/checkout-hpa", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": map[string]int{"currentReplicas": 3, "desiredReplicas": 3}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "t", InsecureSkipVerify: true})
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	report, err := MonitorAutoscaler(ctx, client, "default", "checkout-hpa", 20*time.Millisecond)
	if err != nil {
		t.Fatalf("MonitorAutoscaler failed: %v", err)
	}
	if report.ScaledUp {
		t.Error("did not expect a scale-up to be detected when replicas never change")
	}
}

func TestMonitorAutoscalerFailsWhenHPAUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "t", InsecureSkipVerify: true})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := MonitorAutoscaler(ctx, client, "default", "missing-hpa", 10*time.Millisecond); err == nil {
		t.Error("expected an error when the HPA can never be read")
	}
}
