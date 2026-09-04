// Package k8schaos implements Kubernetes-native chaos (pod-kill) and closed-loop autoscaler
// validation using nothing but net/http against the Kubernetes API server's plain REST+JSON
// interface — deliberately not client-go, whose transitive dependency graph is enormous for the
// handful of endpoints Oshimai actually needs (list/delete pods, read a Deployment's replica
// count, read an HPA's status). Anything a real client-go user could do that this package can't
// is out of scope by design, not by oversight.
package k8schaos

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Config addresses and authenticates against one Kubernetes API server.
type Config struct {
	APIServer          string // e.g. "https://10.0.0.1:6443"
	BearerToken        string
	CACertPEM          []byte // Optional; if nil and InsecureSkipVerify is false, the system trust store is used.
	InsecureSkipVerify bool
	Namespace          string
}

const (
	inClusterTokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	inClusterCACertPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// FromInCluster builds a Config from the standard service-account files Kubernetes mounts into
// every pod — the normal way an in-cluster agent authenticates, requiring zero configuration.
func FromInCluster() (Config, error) {
	token, err := os.ReadFile(inClusterTokenPath)
	if err != nil {
		return Config{}, fmt.Errorf("not running in-cluster (or service account not mounted): %w", err)
	}
	caCert, _ := os.ReadFile(inClusterCACertPath) // Best-effort; missing CA falls back to InsecureSkipVerify by the caller if needed.
	namespace, _ := os.ReadFile(inClusterNamespacePath)

	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return Config{}, fmt.Errorf("KUBERNETES_SERVICE_HOST/PORT not set — not running in-cluster")
	}

	ns := string(namespace)
	if ns == "" {
		ns = "default"
	}
	return Config{
		APIServer:   fmt.Sprintf("https://%s:%s", host, port),
		BearerToken: string(token),
		CACertPEM:   caCert,
		Namespace:   ns,
	}, nil
}

// Client is a minimal Kubernetes REST client for the specific resources Oshimai's chaos and
// autoscaler-validation features touch.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// NewClient builds a Client from cfg.
func NewClient(cfg Config) (*Client, error) {
	transport := &http.Transport{}
	if cfg.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else if len(cfg.CACertPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(cfg.CACertPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool}
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second, Transport: transport},
	}, nil
}

func (c *Client) do(ctx context.Context, method, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.APIServer+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.BearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kubernetes API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("kubernetes API returned %d: %s", resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// PodList mirrors the subset of a Kubernetes PodList the pod-kill driver needs.
type PodList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	} `json:"items"`
}

// ListPods returns pod names in namespace matching labelSelector (Kubernetes selector syntax,
// e.g. "app=checkout,tier=backend").
func (c *Client) ListPods(ctx context.Context, namespace, labelSelector string) ([]string, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods?labelSelector=%s", namespace, url.QueryEscape(labelSelector))
	var list PodList
	if err := c.do(ctx, http.MethodGet, path, &list); err != nil {
		return nil, err
	}
	var names []string
	for _, item := range list.Items {
		if item.Status.Phase == "Running" {
			names = append(names, item.Metadata.Name)
		}
	}
	return names, nil
}

// DeletePod deletes one pod by name — the actual chaos action: Kubernetes will reschedule it per
// the owning controller's policy, and how gracefully the application handles that is the point.
func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", namespace, name)
	return c.do(ctx, http.MethodDelete, path, nil)
}

// DeploymentStatus is the subset of a Deployment's status this package reads.
type DeploymentStatus struct {
	Replicas          int `json:"replicas"`
	ReadyReplicas     int `json:"readyReplicas"`
	AvailableReplicas int `json:"availableReplicas"`
}

// GetDeploymentReplicas reads a Deployment's current replica status.
func (c *Client) GetDeploymentReplicas(ctx context.Context, namespace, name string) (DeploymentStatus, error) {
	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", namespace, name)
	var doc struct {
		Status DeploymentStatus `json:"status"`
	}
	if err := c.do(ctx, http.MethodGet, path, &doc); err != nil {
		return DeploymentStatus{}, err
	}
	return doc.Status, nil
}

// HPAStatus is the subset of a HorizontalPodAutoscaler's status this package reads.
type HPAStatus struct {
	CurrentReplicas int `json:"currentReplicas"`
	DesiredReplicas int `json:"desiredReplicas"`
}

// GetHPAStatus reads a HorizontalPodAutoscaler's current/desired replica counts (autoscaling/v2).
func (c *Client) GetHPAStatus(ctx context.Context, namespace, name string) (HPAStatus, error) {
	path := fmt.Sprintf("/apis/autoscaling/v2/namespaces/%s/horizontalpodautoscalers/%s", namespace, name)
	var doc struct {
		Status HPAStatus `json:"status"`
	}
	if err := c.do(ctx, http.MethodGet, path, &doc); err != nil {
		return HPAStatus{}, err
	}
	return doc.Status, nil
}
