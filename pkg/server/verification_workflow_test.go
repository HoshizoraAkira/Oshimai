package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/verify"
)

func newGuardedCoordinator(t *testing.T, verification *VerificationStore) *TestCoordinator {
	t.Helper()
	repo := NewMemoryRepository()
	eb := NewEventBus()
	if verification == nil {
		verification = NewVerificationStore()
	}
	return NewTestCoordinator(repo, eb, CoordinatorConfig{
		MaxConcurrentRuns:         5,
		ChaosDriver:               chaos.NewManagedChaosDriver(chaos.NewMockChaosDriver()),
		EnforceTargetVerification: true,
		Verification:              verification,
	})
}

func basicRunRequest(baseURL string) CreateRunRequest {
	return CreateRunRequest{
		ScenarioYAML: `
id: verify_test_scen
base_url: ` + baseURL + `
initial_step_id: step_one
steps:
  step_one:
    id: step_one
    request:
      path: /test
`,
		LoadConfig: loadengine.EngineConfig{
			Profile:  loadengine.ProfileFlatVU,
			VUs:      1,
			Duration: 200 * time.Millisecond,
		},
	}
}

func TestStartRunRejectsBlockedTarget(t *testing.T) {
	tc := newGuardedCoordinator(t, nil)
	_, err := tc.StartRun(context.Background(), basicRunRequest("https://google.com"))
	if err == nil {
		t.Fatal("expected StartRun to reject a blocklisted target")
	}
	if !strings.Contains(err.Error(), "well-known public domain") {
		t.Errorf("expected blocklist error message, got: %v", err)
	}
}

func TestStartRunRejectsUnverifiedPublicTarget(t *testing.T) {
	tc := newGuardedCoordinator(t, nil)
	_, err := tc.StartRun(context.Background(), basicRunRequest("https://my-unverified-shop.example"))
	if err == nil {
		t.Fatal("expected StartRun to reject an unverified public target")
	}
	if !strings.Contains(err.Error(), "ownership verification") {
		t.Errorf("expected ownership-verification error message, got: %v", err)
	}
}

func TestStartRunAllowsVerifiedPublicTarget(t *testing.T) {
	vs := NewVerificationStore()
	host := "my-verified-shop.example"
	// Directly mark as verified (same-package access) to bypass the real DNS/HTTP challenge,
	// which this unit test cannot satisfy against a domain it doesn't control.
	vs.mu.Lock()
	vs.records[host] = verify.Record{Host: host, Verified: true, Method: verify.MethodDNSTXT}
	vs.mu.Unlock()

	tc := newGuardedCoordinator(t, vs)
	run, err := tc.StartRun(context.Background(), basicRunRequest("https://"+host))
	if err != nil {
		t.Fatalf("expected StartRun to allow a verified target, got error: %v", err)
	}
	if run.Status != RunStatusQueued {
		t.Errorf("expected queued status, got %s", run.Status)
	}
}

func TestStartRunAllowsPrivateTargetWithoutVerification(t *testing.T) {
	tc := newGuardedCoordinator(t, nil)
	run, err := tc.StartRun(context.Background(), basicRunRequest("http://127.0.0.1:9999"))
	if err != nil {
		t.Fatalf("expected loopback target to bypass verification, got error: %v", err)
	}
	if run.Status != RunStatusQueued {
		t.Errorf("expected queued status, got %s", run.Status)
	}
}

func TestGuardedProductionRequiresApproval(t *testing.T) {
	repo := NewMemoryRepository()
	eb := NewEventBus()
	tc := NewTestCoordinator(repo, eb, CoordinatorConfig{
		MaxConcurrentRuns:            5,
		ChaosDriver:                  chaos.NewManagedChaosDriver(chaos.NewMockChaosDriver()),
		RequireApprovalForProduction: true,
	})

	req := basicRunRequest("http://127.0.0.1:9999")
	req.Environment = "production"

	run, err := tc.StartRun(context.Background(), req)
	if err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}
	if run.Status != RunStatusAwaitingApproval {
		t.Fatalf("expected awaiting_approval status, got %s", run.Status)
	}

	// A second call to ApproveRun releases it into the normal execution pipeline.
	approved, err := tc.ApproveRun(context.Background(), run.ID, "sre-oncall")
	if err != nil {
		t.Fatalf("ApproveRun failed: %v", err)
	}
	if approved.Status != RunStatusQueued {
		t.Errorf("expected queued status after approval, got %s", approved.Status)
	}
	if approved.ApprovedBy != "sre-oncall" {
		t.Errorf("expected approver to be recorded, got %q", approved.ApprovedBy)
	}
}

func TestApproveRunRejectsUnknownOrAlreadyApproved(t *testing.T) {
	repo := NewMemoryRepository()
	eb := NewEventBus()
	tc := NewTestCoordinator(repo, eb, CoordinatorConfig{
		MaxConcurrentRuns: 5,
		ChaosDriver:       chaos.NewManagedChaosDriver(chaos.NewMockChaosDriver()),
	})

	if _, err := tc.ApproveRun(context.Background(), "does-not-exist", "someone"); err == nil {
		t.Error("expected error approving a nonexistent run")
	}

	req := basicRunRequest("http://127.0.0.1:9999")
	run, _ := tc.StartRun(context.Background(), req) // Not production -> already queued, not awaiting approval.
	if _, err := tc.ApproveRun(context.Background(), run.ID, "someone"); err == nil {
		t.Error("expected error approving a run that was never held for approval")
	}
	if _, err := tc.ApproveRun(context.Background(), run.ID, ""); err == nil {
		t.Error("expected error when approver name is empty")
	}
}
