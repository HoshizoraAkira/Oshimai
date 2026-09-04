package k8schaos

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/oshimai/twin/pkg/chaos"
)

// PodKillDriver implements chaos.ChaosDriver by deleting a percentage of running pods matching a
// label selector — the Kubernetes-native equivalent of the netem/HTTP-proxy drivers, using the
// same Apply/Revert/Status lifecycle (and gaining the same dead-man-switch safety for free when
// wrapped in chaos.NewManagedChaosDriver). "Revert" for a pod kill is necessarily a no-op: you
// cannot un-kill a pod, only rely on its controller (Deployment/StatefulSet) to reschedule it,
// which is the entire point of the experiment.
type PodKillDriver struct {
	client *Client

	mu     sync.Mutex
	status chaos.ChaosStatus
}

// NewPodKillDriver wraps an already-configured k8schaos.Client.
func NewPodKillDriver(client *Client) *PodKillDriver {
	return &PodKillDriver{client: client, status: chaos.ChaosStatus{State: chaos.StateIdle}}
}

// Apply deletes ceil(len(matching pods) * LossPercent/100) running pods matching
// fault.Filter.TargetDomains[0] (reused here as the label selector — chaos.FaultSpec has no
// Kubernetes-specific field, and TargetDomains is otherwise unused by this driver, so it doubles
// as "which pods" the same way it means "which hostnames" for the HTTP proxy driver). LossPercent
// reused as "percent of matching pods to kill" for the same reason: no new FaultSpec field needed
// for a chaos dimension this driver alone interprets.
func (d *PodKillDriver) Apply(ctx context.Context, fault chaos.FaultSpec) error {
	if err := fault.Validate(); err != nil {
		return err
	}
	if len(fault.Filter.TargetDomains) == 0 {
		return fmt.Errorf("pod-kill fault requires filter.target_domains[0] to be set to a Kubernetes label selector, e.g. \"app=checkout\"")
	}
	selector := fault.Filter.TargetDomains[0]
	namespace := d.client.cfg.Namespace
	if namespace == "" {
		namespace = "default"
	}

	pods, err := d.client.ListPods(ctx, namespace, selector)
	if err != nil {
		return fmt.Errorf("failed to list pods for selector %q: %w", selector, err)
	}
	if len(pods) == 0 {
		return fmt.Errorf("no running pods matched selector %q in namespace %q", selector, namespace)
	}

	percent := fault.LossPercent
	if percent <= 0 {
		percent = 100 // Default to "kill everything matched" if unspecified — an explicit selector already scopes blast radius.
	}
	killCount := int(float64(len(pods)) * percent / 100.0)
	if killCount < 1 {
		killCount = 1
	}
	if killCount > len(pods) {
		killCount = len(pods)
	}

	rand.Shuffle(len(pods), func(i, j int) { pods[i], pods[j] = pods[j], pods[i] })
	var killed []string
	for i := 0; i < killCount; i++ {
		if err := d.client.DeletePod(ctx, namespace, pods[i]); err != nil {
			continue // Best-effort: a pod that raced to termination on its own isn't a driver failure.
		}
		killed = append(killed, pods[i])
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	faultCopy := fault
	d.status = chaos.ChaosStatus{
		State: chaos.StateInjected, CurrentFault: &faultCopy, AppliedAt: now, ExpiresAt: now.Add(fault.Duration),
		LastError: fmt.Sprintf("killed pods: %v", killed),
	}
	return nil
}

// Revert cannot resurrect deleted pods; it only clears the driver's own status so a subsequent
// Apply is treated as a fresh experiment.
func (d *PodKillDriver) Revert(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.status = chaos.ChaosStatus{State: chaos.StateReverted}
	return nil
}

// Status returns a copy of the live chaos status.
func (d *PodKillDriver) Status() chaos.ChaosStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}
