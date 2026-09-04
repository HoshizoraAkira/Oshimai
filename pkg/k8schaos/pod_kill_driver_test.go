package k8schaos

import (
	"context"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/chaos"
)

func TestPodKillDriverApply(t *testing.T) {
	srv, deleted := fakeK8sAPI(t)
	defer srv.Close()

	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "test-token", InsecureSkipVerify: true, Namespace: "default"})
	driver := NewPodKillDriver(client)

	err := driver.Apply(context.Background(), chaos.FaultSpec{
		ID: "pod-kill-test", Type: chaos.FaultComposite,
		Filter:      chaos.FilterConfig{Interface: "k8s", TargetDomains: []string{"app=checkout"}},
		LossPercent: 100, Duration: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if len(*deleted) != 2 {
		t.Errorf("expected both running pods to be killed at 100%%, got %d: %v", len(*deleted), *deleted)
	}
	if driver.Status().State != chaos.StateInjected {
		t.Errorf("expected state injected, got %s", driver.Status().State)
	}
}

func TestPodKillDriverPartialKill(t *testing.T) {
	srv, deleted := fakeK8sAPI(t)
	defer srv.Close()

	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "test-token", InsecureSkipVerify: true, Namespace: "default"})
	driver := NewPodKillDriver(client)

	err := driver.Apply(context.Background(), chaos.FaultSpec{
		ID: "pod-kill-partial", Type: chaos.FaultComposite,
		Filter:      chaos.FilterConfig{Interface: "k8s", TargetDomains: []string{"app=checkout"}},
		LossPercent: 50, Duration: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if len(*deleted) != 1 {
		t.Errorf("expected exactly 1 of 2 running pods killed at 50%%, got %d", len(*deleted))
	}
}

func TestPodKillDriverRequiresSelector(t *testing.T) {
	srv, _ := fakeK8sAPI(t)
	defer srv.Close()
	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "test-token", InsecureSkipVerify: true})
	driver := NewPodKillDriver(client)

	err := driver.Apply(context.Background(), chaos.FaultSpec{
		ID: "no-selector", Type: chaos.FaultComposite, Filter: chaos.FilterConfig{Interface: "k8s"}, Duration: time.Second,
	})
	if err == nil {
		t.Error("expected an error when no label selector is provided")
	}
}

func TestPodKillDriverRevert(t *testing.T) {
	srv, _ := fakeK8sAPI(t)
	defer srv.Close()
	client, _ := NewClient(Config{APIServer: srv.URL, BearerToken: "test-token", InsecureSkipVerify: true})
	driver := NewPodKillDriver(client)

	if err := driver.Revert(context.Background()); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}
	if driver.Status().State != chaos.StateReverted {
		t.Errorf("expected state reverted, got %s", driver.Status().State)
	}
}
