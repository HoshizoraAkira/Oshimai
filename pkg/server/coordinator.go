package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oshimai/twin/pkg/benchmark"
	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/generator"
	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/remediation"
	"github.com/oshimai/twin/pkg/verify"
	"github.com/oshimai/twin/pkg/vusession"
	"gopkg.in/yaml.v3"
)

// CoordinatorConfig options for the orchestrator.
type CoordinatorConfig struct {
	MaxConcurrentRuns int
	ChaosDriver       chaos.ChaosDriver

	// EnforceTargetVerification, when true, rejects StartRun for any public (non-private,
	// non-blocklisted) target host that has not completed ownership verification (see
	// pkg/verify and VerificationStore). Defaults to false so existing embedders/tests are
	// unaffected; the shipped CLI/server binary turns this on by default.
	EnforceTargetVerification bool
	Verification              *VerificationStore

	// RequireApprovalForProduction, when true, holds any run with Environment == "production"
	// in RunStatusAwaitingApproval until a second operator calls ApproveRun.
	RequireApprovalForProduction bool

	// HTTPClientFactory, when set, produces the HTTP client every Virtual User uses for a run
	// that doesn't specify its own. This is how the non-root HTTP chaos proxy driver (see
	// chaos.HTTPProxyDriver) gets traffic routed through it: main.go wires this to the driver's
	// HTTPClient method when -chaos-driver=http_proxy is selected.
	HTTPClientFactory func() (*http.Client, error)
}

// TestCoordinator manages coordinated execution of load tests and network chaos.
type TestCoordinator struct {
	repo         StorageRepository
	eventBus     *EventBus
	chaosDriver  chaos.ChaosDriver
	semaphore    chan struct{}
	verification *VerificationStore
	agents       *AgentRegistry
	benchmark    *benchmark.Aggregator
	cfg          CoordinatorConfig

	mu         sync.RWMutex
	activeRuns map[string]context.CancelFunc
	runCounter uint64
}

// NewTestCoordinator initializes the test orchestrator.
func NewTestCoordinator(repo StorageRepository, eventBus *EventBus, cfg CoordinatorConfig) *TestCoordinator {
	maxRuns := cfg.MaxConcurrentRuns
	if maxRuns <= 0 {
		maxRuns = 2 // Guard against exhausting host resources
	}

	cDriver := cfg.ChaosDriver
	if cDriver == nil {
		cDriver = chaos.NewManagedChaosDriver(chaos.NewMockChaosDriver())
	}

	verification := cfg.Verification
	if verification == nil {
		verification = NewVerificationStore()
	}

	return &TestCoordinator{
		repo:         repo,
		eventBus:     eventBus,
		chaosDriver:  cDriver,
		semaphore:    make(chan struct{}, maxRuns),
		verification: verification,
		agents:       NewAgentRegistry(),
		benchmark:    benchmark.NewAggregator(),
		cfg:          cfg,
		activeRuns:   make(map[string]context.CancelFunc),
	}
}

// Agents exposes the coordinator's AgentRegistry so API handlers can register/poll/dispatch
// without duplicating registry construction.
func (tc *TestCoordinator) Agents() *AgentRegistry {
	return tc.agents
}

// Benchmark exposes the coordinator's benchmark Aggregator for the compare/submit API endpoints.
func (tc *TestCoordinator) Benchmark() *benchmark.Aggregator {
	return tc.benchmark
}

// StartRun accepts a test run request, records it, and triggers asynchronous execution — unless
// it is rejected by the target-verification gate, or held for approval under guarded-production
// mode (see CoordinatorConfig).
func (tc *TestCoordinator) StartRun(ctx context.Context, req CreateRunRequest) (*TestRun, error) {
	if tc.cfg.EnforceTargetVerification {
		if err := tc.checkTargetAllowed(req); err != nil {
			return nil, err
		}
	}

	seq := atomic.AddUint64(&tc.runCounter, 1)
	runID := fmt.Sprintf("run-%d-%d", time.Now().Unix(), seq)

	needsApproval := tc.cfg.RequireApprovalForProduction && strings.EqualFold(req.Environment, "production")

	run := &TestRun{
		ID:     runID,
		Status: RunStatusQueued,
		Config: req,
	}
	if needsApproval {
		run.Status = RunStatusAwaitingApproval
	}

	if err := tc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to queue test run: %w", err)
	}

	if !needsApproval {
		go tc.executeRun(runID, req)
	}
	return run, nil
}

// Verification exposes the coordinator's VerificationStore so API handlers can drive the
// challenge/confirm flow without duplicating store construction.
func (tc *TestCoordinator) Verification() *VerificationStore {
	return tc.verification
}

// ApproveRun releases a run held in RunStatusAwaitingApproval (guarded-production mode) and
// begins execution. approver is recorded on the run for audit purposes.
func (tc *TestCoordinator) ApproveRun(ctx context.Context, id, approver string) (*TestRun, error) {
	run, err := tc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.Status != RunStatusAwaitingApproval {
		return nil, fmt.Errorf("run %q is not awaiting approval (current status: %s)", id, run.Status)
	}
	if strings.TrimSpace(approver) == "" {
		return nil, fmt.Errorf("approver name is required")
	}

	run.Status = RunStatusQueued
	run.ApprovedBy = approver
	run.ApprovedAt = time.Now()
	if err := tc.repo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to record approval: %w", err)
	}

	go tc.executeRun(id, run.Config)
	return run, nil
}

// checkTargetAllowed resolves the target host for req and enforces the blocklist / ownership
// verification gate. Requests where the target host cannot be determined (e.g. a bare OpenAPI
// spec with no TargetBaseURL and no embedded base_url) are allowed through unchanged — this gate
// is a safety net on top of user education, not the sole line of defense.
func (tc *TestCoordinator) checkTargetAllowed(req CreateRunRequest) error {
	host := resolveTargetHost(req)
	if host == "" {
		return nil
	}
	if verify.IsBlockedTarget(host) {
		return fmt.Errorf("target %q is a well-known public domain and cannot be used as a load/chaos test target", host)
	}
	if verify.IsPrivateOrLocalTarget(host) {
		return nil
	}
	if !tc.verification.IsVerified(host) {
		return fmt.Errorf("target %q has not completed ownership verification — call POST /api/v1/verify/challenge then /api/v1/verify/confirm before running a test against a public target", host)
	}
	return nil
}

// resolveTargetHost best-effort extracts the target hostname from whichever field carries it.
func resolveTargetHost(req CreateRunRequest) string {
	if req.TargetBaseURL != "" {
		if host, err := verify.ExtractHost(req.TargetBaseURL); err == nil {
			return host
		}
	}

	var embedded struct {
		BaseURL string `yaml:"base_url" json:"base_url"`
	}
	if req.ScenarioYAML != "" {
		if err := yaml.Unmarshal([]byte(req.ScenarioYAML), &embedded); err == nil && embedded.BaseURL != "" {
			if host, err := verify.ExtractHost(embedded.BaseURL); err == nil {
				return host
			}
		}
	}
	if req.ScenarioJSON != "" {
		var jsonEmbedded struct {
			BaseURL string `json:"base_url"`
		}
		if err := json.Unmarshal([]byte(req.ScenarioJSON), &jsonEmbedded); err == nil && jsonEmbedded.BaseURL != "" {
			if host, err := verify.ExtractHost(jsonEmbedded.BaseURL); err == nil {
				return host
			}
		}
	}
	return ""
}

// executeRun manages the full lifecycle: semaphore acquisition, scenario resolution,
// load engine execution, timed chaos injection, safety revert, and telemetry publishing.
func (tc *TestCoordinator) executeRun(runID string, req CreateRunRequest) {
	// 1. Acquire concurrency slot
	tc.semaphore <- struct{}{}
	defer func() { <-tc.semaphore }()

	runCtx, cancel := context.WithCancel(context.Background())
	tc.mu.Lock()
	tc.activeRuns[runID] = cancel
	tc.mu.Unlock()

	defer func() {
		tc.mu.Lock()
		delete(tc.activeRuns, runID)
		tc.mu.Unlock()
	}()

	// 2. Resolve Scenario (Direct YAML/JSON vs On-the-fly Synthesis)
	scenario, err := tc.resolveScenario(runCtx, req)
	if err != nil {
		tc.finalizeRun(runID, RunStatusFailed, nil, err.Error())
		return
	}
	tc.attachScenario(runID, scenario)

	// 3. Mark Run as Running
	startTime := time.Now()
	_ = tc.updateRunStatus(runID, RunStatusRunning, startTime, time.Time{}, nil, "")

	// 4. Initialize Load Engine
	if req.LoadConfig.Client == nil && tc.cfg.HTTPClientFactory != nil {
		if client, clientErr := tc.cfg.HTTPClientFactory(); clientErr == nil {
			req.LoadConfig.Client = client
		}
	}
	engine, err := loadengine.NewLoadEngine(req.LoadConfig)
	if err != nil {
		tc.finalizeRun(runID, RunStatusFailed, nil, fmt.Sprintf("load engine init error: %v", err))
		return
	}

	// 5. Schedule Chaos Injection (if enabled)
	var chaosTimer *time.Timer
	var chaosActive atomic.Bool
	var activeFaultID string

	if req.ChaosPlan.Enabled && req.ChaosPlan.Fault.ID != "" {
		activeFaultID = req.ChaosPlan.Fault.ID
		delay := req.ChaosPlan.ScheduleDelay
		if delay < 0 {
			delay = 0
		}

		chaosTimer = time.AfterFunc(delay, func() {
			select {
			case <-runCtx.Done():
				return
			default:
				if applyErr := tc.chaosDriver.Apply(runCtx, req.ChaosPlan.Fault); applyErr == nil {
					chaosActive.Store(true)
				}
			}
		})
		defer func() {
			if chaosTimer != nil {
				chaosTimer.Stop()
			}
			// Fail-safe: Always ensure Chaos is reverted when the run finishes!
			_ = tc.chaosDriver.Revert(context.Background())
			chaosActive.Store(false)
		}()
	}

	// 6. Telemetry streaming ticker
	ticker := time.NewTicker(500 * time.Millisecond)
	telemetryStop := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-telemetryStop:
				return
			case t := <-ticker.C:
				currentRPS := 0.0
				activeVUs := engine.ActiveVUs()
				if activeVUs == 0 && req.LoadConfig.Profile == loadengine.ProfileFlatVU {
					activeVUs = req.LoadConfig.VUs
				}
				var errorRate float64
				var errorCount int64
				var p50, p90, p99 time.Duration
				status := "Running"

				isChaos := chaosActive.Load()
				if isChaos {
					status = "Injected Chaos"
				}

				if stats, ok := engine.CurrentStats(); ok {
					errorRate = stats.ErrorRate
					errorCount = stats.Errors
					p50 = stats.P50Latency
					p90 = stats.P90Latency
					p99 = stats.P99Latency

					winSec := req.LoadConfig.CircuitBreaker.WindowDuration.Seconds()
					if winSec <= 0 {
						winSec = 5.0
					}
					if stats.Requests > 0 {
						currentRPS = float64(stats.Requests) / winSec
					}
				}
				if currentRPS == 0 && req.LoadConfig.TargetRPS > 0 && activeVUs > 0 {
					currentRPS = req.LoadConfig.TargetRPS
				}

				select {
				case <-runCtx.Done():
					status = "Draining"
				default:
				}

				tc.eventBus.Publish(TelemetryEvent{
					RunID:        runID,
					Timestamp:    t,
					CurrentRPS:   currentRPS,
					ActiveVUs:    activeVUs,
					ErrorRate:    errorRate,
					ErrorCount:   errorCount,
					LatencyP50:   p50,
					LatencyP90:   p90,
					LatencyP99:   p99,
					ChaosActive:  isChaos,
					ChaosFaultID: activeFaultID,
					Status:       status,
				})
			}
		}
	}()

	// 7. Run Load Engine
	summary, runErr := engine.Run(runCtx, scenario)
	close(telemetryStop)

	// 8. Determine final outcome
	var finalStatus RunStatus
	var errMsg string
	var statusLabel string

	if runErr != nil {
		finalStatus = RunStatusFailed
		errMsg = runErr.Error()
		statusLabel = "FAILED"
	} else if summary.TerminationStatus == loadengine.StatusAbortedByCircuitBreaker {
		finalStatus = RunStatusAborted
		errMsg = fmt.Sprintf("aborted by safety circuit breaker: %s", summary.AbortReason)
		statusLabel = "TRIPPED"
	} else if summary.TerminationStatus == loadengine.StatusContextCancelled {
		finalStatus = RunStatusAborted
		errMsg = "aborted by user cancellation"
		statusLabel = "ABORTED"
	} else {
		finalStatus = RunStatusCompleted
		statusLabel = "COMPLETED"
	}

	// Publish final terminal event
	if summary != nil {
		var errRate float64
		if summary.TotalRequests > 0 {
			errRate = float64(summary.TotalErrors) / float64(summary.TotalRequests)
		}
		tc.eventBus.Publish(TelemetryEvent{
			RunID:        runID,
			Timestamp:    time.Now(),
			CurrentRPS:   summary.ActualRPS,
			ActiveVUs:    0,
			ErrorRate:    errRate,
			ErrorCount:   summary.TotalErrors,
			LatencyP50:   summary.Latency.P50,
			LatencyP90:   summary.Latency.P90,
			LatencyP99:   summary.Latency.P99,
			ChaosActive:  false,
			ChaosFaultID: activeFaultID,
			Status:       statusLabel,
		})
	}

	tc.finalizeRun(runID, finalStatus, summary, errMsg)
}

func (tc *TestCoordinator) resolveScenario(ctx context.Context, req CreateRunRequest) (*vusession.Scenario, error) {
	if req.OpenAPISpec != "" {
		return generator.Synthesize(ctx, []byte(req.OpenAPISpec), []byte(req.OTelTraces), generator.GeneratorConfig{
			ScenarioID:   "auto_generated_scenario",
			ScenarioName: "Autonomous Scenario",
			BaseURL:      req.TargetBaseURL,
		})
	}
	if req.ScenarioYAML != "" {
		return vusession.ParseScenarioYAML([]byte(req.ScenarioYAML))
	}
	if req.ScenarioJSON != "" {
		return vusession.ParseScenarioJSON([]byte(req.ScenarioJSON))
	}
	return nil, fmt.Errorf("no scenario provided: require ScenarioYAML, ScenarioJSON, or OpenAPISpec")
}

func (tc *TestCoordinator) updateRunStatus(runID string, status RunStatus, start, end time.Time, summary *loadengine.ExecutionSummary, errStr string) error {
	run, err := tc.repo.Get(context.Background(), runID)
	if err != nil {
		return err
	}
	run.Status = status
	if !start.IsZero() {
		run.StartTime = start
	}
	if !end.IsZero() {
		run.EndTime = end
	}
	if summary != nil {
		run.Summary = summary
	}
	if errStr != "" {
		run.Error = errStr
	}
	if run.Summary != nil {
		targetVUs := run.Config.LoadConfig.VUs
		isChaos := run.Config.ChaosPlan.Enabled
		run.Diagnostics = remediation.Analyze(run.Summary, targetVUs, isChaos, run.Config.BusinessContext)
	}
	return tc.repo.Update(context.Background(), run)
}

// attachScenario persists the resolved step graph onto the run record so the dashboard can
// render the Flow Heatmap Report once results come in. Best-effort: a failure here shouldn't
// abort the run, since the scenario is only needed for that one diagnostic view.
func (tc *TestCoordinator) attachScenario(runID string, scenario *vusession.Scenario) {
	run, err := tc.repo.Get(context.Background(), runID)
	if err != nil {
		return
	}
	run.Scenario = scenario
	_ = tc.repo.Update(context.Background(), run)
}

func (tc *TestCoordinator) finalizeRun(runID string, status RunStatus, summary *loadengine.ExecutionSummary, errStr string) {
	endTime := time.Now()
	_ = tc.updateRunStatus(runID, status, time.Time{}, endTime, summary, errStr)

	if status == RunStatusCompleted {
		tc.maybeSubmitBenchmark(runID)
	}

	tc.eventBus.Publish(TelemetryEvent{
		RunID:       runID,
		Timestamp:   endTime,
		ChaosActive: false,
		Status:      strings.ToUpper(string(status)),
	})
}

// maybeSubmitBenchmark shares only health score / P99 / error rate / RPS — never the target URL,
// scenario, or payload — with the local benchmark pool, and only when the run opted in via
// Config.BenchmarkCategory.
func (tc *TestCoordinator) maybeSubmitBenchmark(runID string) {
	run, err := tc.repo.Get(context.Background(), runID)
	if err != nil || run.Config.BenchmarkCategory == "" || run.Summary == nil || run.Diagnostics == nil {
		return
	}
	var errRate float64
	if run.Summary.TotalRequests > 0 {
		errRate = float64(run.Summary.TotalErrors) / float64(run.Summary.TotalRequests) * 100
	}
	tc.benchmark.Submit(benchmark.Submission{
		Category:         run.Config.BenchmarkCategory,
		HealthScore:      run.Diagnostics.HealthScore,
		P99Ms:            float64(run.Summary.Latency.P99.Microseconds()) / 1000.0,
		ErrorRatePercent: errRate,
		ActualRPS:        run.Summary.ActualRPS,
	})
}

// AbortRun aborts an active or queued test run and immediately reverts any injected chaos.
func (tc *TestCoordinator) AbortRun(ctx context.Context, id string) error {
	tc.mu.Lock()
	cancel, active := tc.activeRuns[id]
	tc.mu.Unlock()

	// Immediately revert chaos network rules as priority fail-safe
	_ = tc.chaosDriver.Revert(context.Background())

	if active && cancel != nil {
		cancel()
	}

	run, err := tc.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if run.Status == RunStatusRunning || run.Status == RunStatusQueued {
		run.Status = RunStatusAborted
		run.EndTime = time.Now()
		run.Error = "manually aborted by user"
		if run.Summary != nil {
			run.Diagnostics = remediation.Analyze(run.Summary, run.Config.LoadConfig.VUs, run.Config.ChaosPlan.Enabled, run.Config.BusinessContext)
		}
		return tc.repo.Update(ctx, run)
	}

	return nil
}
