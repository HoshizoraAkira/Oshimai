package k8schaos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeK8sAPI(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var deletedPods []string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"metadata": map[string]string{"name": "checkout-abc"}, "status": map[string]string{"phase": "Running"}},
				{"metadata": map[string]string{"name": "checkout-def"}, "status": map[string]string{"phase": "Running"}},
				{"metadata": map[string]string{"name": "checkout-ghi"}, "status": map[string]string{"phase": "Pending"}},
			},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/pods/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/default/pods/")
		deletedPods = append(deletedPods, name)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/apis/apps/v1/namespaces/default/deployments/checkout", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": map[string]int{"replicas": 3, "readyReplicas": 3, "availableReplicas": 3},
		})
	})
	mux.HandleFunc("/apis/autoscaling/v2/namespaces/default/horizontalpodautoscalers/checkout-hpa", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": map[string]int{"currentReplicas": 3, "desiredReplicas": 5},
		})
	})

	return httptest.NewServer(mux), &deletedPods
}

func TestClientListPodsAndGetDeploymentReplicas(t *testing.T) {
	srv, _ := fakeK8sAPI(t)
	defer srv.Close()

	client, err := NewClient(Config{APIServer: srv.URL, BearerToken: "test-token", InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	pods, err := client.ListPods(context.Background(), "default", "app=checkout")
	if err != nil {
		t.Fatalf("ListPods failed: %v", err)
	}
	if len(pods) != 2 { // Only the 2 Running pods, not the Pending one.
		t.Fatalf("expected 2 running pods, got %d: %v", len(pods), pods)
	}

	status, err := client.GetDeploymentReplicas(context.Background(), "default", "checkout")
	if err != nil {
		t.Fatalf("GetDeploymentReplicas failed: %v", err)
	}
	if status.Replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", status.Replicas)
	}
}

func TestClientGetHPAStatus(t *testing.T) {
	srv, _ := fakeK8sAPI(t)
	defer srv.Close()

	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "test-token", InsecureSkipVerify: true})
	status, err := client.GetHPAStatus(context.Background(), "default", "checkout-hpa")
	if err != nil {
		t.Fatalf("GetHPAStatus failed: %v", err)
	}
	if status.CurrentReplicas != 3 || status.DesiredReplicas != 5 {
		t.Errorf("unexpected HPA status: %+v", status)
	}
}

func TestClientRejectsBadAuth(t *testing.T) {
	srv, _ := fakeK8sAPI(t)
	defer srv.Close()

	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "wrong-token", InsecureSkipVerify: true})
	if _, err := client.ListPods(context.Background(), "default", "app=checkout"); err == nil {
		t.Error("expected an error for an unauthorized request")
	}
}

func TestFromInClusterFailsOutsideCluster(t *testing.T) {
	if _, err := FromInCluster(); err == nil {
		t.Error("expected FromInCluster to fail when not actually running in a Kubernetes pod")
	}
}
