package server

import (
	"context"
	"fmt"
	"time"

	"github.com/oshimai/twin/pkg/autofuzz"
	"github.com/oshimai/twin/pkg/autopilot"
	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/remediation"
)

// AutoFuzzRequest configures an Auto Chaos Fuzzer search: a base scenario/load configuration plus
// optional overrides for the bisection itself. Trials reuse the same target-verification gate as
// a normal StartRun, since this still sends real (chaos-degraded) traffic to the target.
type AutoFuzzRequest struct {
	Base                 CreateRunRequest `json:"base"`
	TrialDurationSeconds int              `json:"trial_duration_seconds,omitempty"`
	HealthThreshold      int              `json:"health_threshold,omitempty"`
	MaxIterations        int              `json:"max_iterations,omitempty"`
}

// RunAutoFuzz bisects chaos severity to find the minimal fault that breaks the target scenario.
func (tc *TestCoordinator) RunAutoFuzz(ctx context.Context, req AutoFuzzRequest) (*autofuzz.Result, error) {
	if tc.cfg.EnforceTargetVerification {
		if err := tc.checkTargetAllowed(req.Base); err != nil {
			return nil, err
		}
	}

	scenario, err := tc.resolveScenario(ctx, req.Base)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve scenario: %w", err)
	}

	cfg := autofuzz.DefaultConfig()
	if req.TrialDurationSeconds > 0 {
		cfg.TrialDuration = time.Duration(req.TrialDurationSeconds) * time.Second
	}
	if req.HealthThreshold > 0 {
		cfg.HealthThreshold = req.HealthThreshold
	}
	if req.MaxIterations > 0 {
		cfg.MaxIterations = req.MaxIterations
	}

	baseLoad := req.Base.LoadConfig
	targetVUs := baseLoad.VUs
	if targetVUs <= 0 {
		targetVUs = 10
	}

	trialRunner := func(ctx context.Context, severity float64, fault chaos.FaultSpec) (int, error) {
		engineCfg := baseLoad
		engineCfg.Profile = loadengine.ProfileFlatVU
		engineCfg.VUs = targetVUs
		engineCfg.Duration = cfg.TrialDuration
		engineCfg.CircuitBreaker.Enabled = false // A tripped breaker mid-trial would just look like "broken" prematurely.
		if engineCfg.Client == nil && tc.cfg.HTTPClientFactory != nil {
			if client, clientErr := tc.cfg.HTTPClientFactory(); clientErr == nil {
				engineCfg.Client = client
			}
		}

		engine, err := loadengine.NewLoadEngine(engineCfg)
		if err != nil {
			return 0, fmt.Errorf("trial engine init failed: %w", err)
		}

		if err := tc.chaosDriver.Apply(ctx, fault); err != nil {
			return 0, fmt.Errorf("failed to apply trial fault: %w", err)
		}
		defer func() { _ = tc.chaosDriver.Revert(context.Background()) }()

		summary, runErr := engine.Run(ctx, scenario)
		if runErr != nil {
			return 0, fmt.Errorf("trial execution failed: %w", runErr)
		}

		diag := remediation.Analyze(summary, targetVUs, true, nil)
		return diag.HealthScore, nil
	}

	return autofuzz.Run(ctx, cfg, trialRunner)
}

// AutoPilotRequest configures an Auto-Pilot capacity search.
type AutoPilotRequest struct {
	Base                 CreateRunRequest `json:"base"`
	MinVUs               int              `json:"min_vus,omitempty"`
	MaxVUs               int              `json:"max_vus,omitempty"`
	TrialDurationSeconds int              `json:"trial_duration_seconds,omitempty"`
	HealthThreshold      int              `json:"health_threshold,omitempty"`
	MaxIterations        int              `json:"max_iterations,omitempty"`
}

// RunAutoPilot bisects virtual-user concurrency to find the exact safe capacity ceiling.
func (tc *TestCoordinator) RunAutoPilot(ctx context.Context, req AutoPilotRequest) (*autopilot.Result, error) {
	if tc.cfg.EnforceTargetVerification {
		if err := tc.checkTargetAllowed(req.Base); err != nil {
			return nil, err
		}
	}

	scenario, err := tc.resolveScenario(ctx, req.Base)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve scenario: %w", err)
	}

	cfg := autopilot.DefaultConfig()
	if req.MinVUs > 0 {
		cfg.MinVUs = req.MinVUs
	}
	if req.MaxVUs > 0 {
		cfg.MaxVUs = req.MaxVUs
	}
	if req.HealthThreshold > 0 {
		cfg.HealthThreshold = req.HealthThreshold
	}
	if req.MaxIterations > 0 {
		cfg.MaxIterations = req.MaxIterations
	}

	trialDuration := 5 * time.Second
	if req.TrialDurationSeconds > 0 {
		trialDuration = time.Duration(req.TrialDurationSeconds) * time.Second
	}

	baseLoad := req.Base.LoadConfig

	trialRunner := func(ctx context.Context, vus int) (int, error) {
		engineCfg := baseLoad
		engineCfg.Profile = loadengine.ProfileFlatVU
		engineCfg.VUs = vus
		engineCfg.Duration = trialDuration
		engineCfg.CircuitBreaker.Enabled = false
		if engineCfg.Client == nil && tc.cfg.HTTPClientFactory != nil {
			if client, clientErr := tc.cfg.HTTPClientFactory(); clientErr == nil {
				engineCfg.Client = client
			}
		}

		engine, err := loadengine.NewLoadEngine(engineCfg)
		if err != nil {
			return 0, fmt.Errorf("trial engine init failed: %w", err)
		}

		summary, runErr := engine.Run(ctx, scenario)
		if runErr != nil {
			return 0, fmt.Errorf("trial execution failed: %w", runErr)
		}

		diag := remediation.Analyze(summary, vus, false, nil)
		return diag.HealthScore, nil
	}

	return autopilot.Run(ctx, cfg, trialRunner)
}
