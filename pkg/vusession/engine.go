// Package vusession defines and executes Oshimai's scenario DSL: a probabilistic (Markov-style)
// state machine of HTTP steps, parsed from YAML/JSON (parser.go) into a Scenario of named Steps
// connected by weighted Transitions (model.go). A single VirtualUser (engine.go) walks that graph
// from Scenario.InitialStepID until it lands on a step with no outgoing transitions ("END"),
// extracting values out of each response into session-scoped variables (extractor.go) for later
// steps to interpolate into their own request bodies/headers/paths, and evaluating per-step
// assertions to decide success/failure — this is "one simulated user's real click-through session
// through your app," not just a flat list of requests. pkg/loadengine is the thing that runs many
// VirtualUsers concurrently against this same Scenario to actually generate load.
package vusession

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultStepTimeout = 10 * time.Second
	MaxStepTransitions = 1000 // Guard against infinite cyclic loops
)

// HTTPClient abstracts HTTP execution to support standard http.Client or mock/httptest clients.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// VirtualUser represents an autonomous, stateful Virtual User executing a scenario flow.
type VirtualUser struct {
	ID        string
	Scenario  *Scenario
	State     *SessionState
	Client    HTTPClient
	Evaluator *TransitionEvaluator
	mu        sync.RWMutex
	history   []*StepResult
	maxSteps  int
}

// Config provides configuration options for creating a VirtualUser.
type Config struct {
	ID       string
	Scenario *Scenario
	State    *SessionState
	Client   HTTPClient
	RandSrc  RandomSource
	MaxSteps int
}

// NewVirtualUser initializes a new VirtualUser.
func NewVirtualUser(cfg Config) (*VirtualUser, error) {
	if cfg.Scenario == nil {
		return nil, fmt.Errorf("scenario cannot be nil")
	}
	if cfg.Scenario.InitialStepID == "" {
		return nil, fmt.Errorf("scenario.InitialStepID must be specified")
	}
	if _, ok := cfg.Scenario.Steps[cfg.Scenario.InitialStepID]; !ok {
		return nil, fmt.Errorf("initial step %q not found in scenario steps", cfg.Scenario.InitialStepID)
	}

	vuID := cfg.ID
	if vuID == "" {
		vuID = fmt.Sprintf("vu-%d", time.Now().UnixNano())
	}

	state := cfg.State
	if state == nil {
		state = NewSessionState()
	}

	client := cfg.Client
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = MaxStepTransitions
	}

	evaluator := NewTransitionEvaluator(cfg.RandSrc)

	return &VirtualUser{
		ID:        vuID,
		Scenario:  cfg.Scenario,
		State:     state,
		Client:    client,
		Evaluator: evaluator,
		history:   make([]*StepResult, 0, 16),
		maxSteps:  maxSteps,
	}, nil
}

// Execute runs the complete scenario from InitialStepID until termination, abort, or context cancellation.
func (vu *VirtualUser) Execute(ctx context.Context) (*SessionReport, error) {
	startTime := time.Now()
	report := &SessionReport{
		VUID:       vu.ID,
		ScenarioID: vu.Scenario.ID,
		StartTime:  startTime,
		Success:    true,
	}

	currentStepID := vu.Scenario.InitialStepID
	stepCount := 0

	for {
		if err := ctx.Err(); err != nil {
			report.Success = false
			report.FailureReason = fmt.Sprintf("session context cancelled: %v", err)
			break
		}

		if stepCount >= vu.maxSteps {
			report.Success = false
			report.FailureReason = fmt.Sprintf("session exceeded maximum allowed steps (%d)", vu.maxSteps)
			break
		}

		step, exists := vu.Scenario.Steps[currentStepID]
		if !exists {
			report.Success = false
			report.FailureReason = fmt.Sprintf("step %q does not exist in scenario", currentStepID)
			break
		}

		// Execute step with potential retries
		result, err := vu.executeStepWithRetry(ctx, step)
		vu.recordStepResult(result)
		stepCount++

		if err != nil || !result.Success {
			// Handle failure
			if step.OnFailure.Action == FailureActionTransition && step.OnFailure.FallbackStepID != "" {
				currentStepID = step.OnFailure.FallbackStepID
				continue
			}

			// Default action is abort
			report.Success = false
			if result.Error != "" {
				report.FailureReason = fmt.Sprintf("step %q failed: %s", step.ID, result.Error)
			} else {
				report.FailureReason = fmt.Sprintf("step %q failed with status %d", step.ID, result.StatusCode)
			}
			break
		}

		// Transition to next step (Markov-style selection)
		transition, transErr := vu.Evaluator.SelectNextTransition(step, result, vu.State)
		if transErr != nil {
			report.Success = false
			report.FailureReason = fmt.Sprintf("transition evaluation failed: %v", transErr)
			break
		}

		if transition == nil || transition.TargetStepID == "" || strings.ToUpper(transition.TargetStepID) == "END" {
			// Clean termination of the session
			break
		}

		result.NextStepID = transition.TargetStepID
		currentStepID = transition.TargetStepID
	}

	endTime := time.Now()
	report.EndTime = endTime
	report.TotalDuration = endTime.Sub(startTime)
	report.StepResults = vu.GetHistory()
	report.FinalState = vu.State.Snapshot()

	return report, nil
}

func (vu *VirtualUser) executeStepWithRetry(ctx context.Context, step *Step) (*StepResult, error) {
	maxRetries := 0
	if step.OnFailure.Action == FailureActionRetry && step.OnFailure.MaxRetries > 0 {
		maxRetries = step.OnFailure.MaxRetries
	}

	var lastResult *StepResult
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check context before retry
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		lastResult, lastErr = vu.ExecuteSingleStep(ctx, step)
		if lastErr == nil && lastResult.Success {
			return lastResult, nil
		}
	}

	return lastResult, lastErr
}

// ExecuteSingleStep handles URL interpolation, header resolution, HTTP execution, assertions, and extraction.
func (vu *VirtualUser) ExecuteSingleStep(ctx context.Context, step *Step) (*StepResult, error) {
	result := &StepResult{
		StepID:        step.ID,
		StepName:      step.Name,
		StartTime:     time.Now(),
		ExtractedVars: make(map[string]any),
	}

	// A pure think-time/delay step: no HTTP call, just sleep for the configured duration
	// (interruptible by context cancellation) and report success.
	if step.Request.Path == "" && step.ThinkTime.AsDuration() > 0 {
		select {
		case <-time.After(step.ThinkTime.AsDuration()):
		case <-ctx.Done():
			result.Duration = time.Since(result.StartTime)
			result.Error = ctx.Err().Error()
			return result, ctx.Err()
		}
		result.Duration = time.Since(result.StartTime)
		result.Success = true
		result.StatusCode = 0
		return result, nil
	}
	if step.ThinkTime.AsDuration() > 0 {
		select {
		case <-time.After(step.ThinkTime.AsDuration()):
		case <-ctx.Done():
			result.Duration = time.Since(result.StartTime)
			result.Error = ctx.Err().Error()
			return result, ctx.Err()
		}
	}

	// 1. Build and interpolate Target URL
	rawURL := step.Request.Path
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		base := strings.TrimRight(vu.Scenario.BaseURL, "/")
		path := strings.TrimLeft(rawURL, "/")
		rawURL = base + "/" + path
	}

	resolvedURL, err := vu.State.Interpolate(rawURL)
	if err != nil {
		result.Duration = time.Since(result.StartTime)
		result.Error = fmt.Sprintf("URL interpolation failed: %v", err)
		return result, fmt.Errorf("%s", result.Error)
	}

	// 2. Interpolate Body
	resolvedBody, err := vu.State.Interpolate(step.Request.Body)
	if err != nil {
		result.Duration = time.Since(result.StartTime)
		result.Error = fmt.Sprintf("Body interpolation failed: %v", err)
		return result, fmt.Errorf("%s", result.Error)
	}

	// 3. Prepare HTTP Request with timeout
	stepTimeout := DefaultStepTimeout
	if step.Request.Timeout > 0 {
		stepTimeout = step.Request.Timeout.AsDuration()
	} else if vu.Scenario.Timeout > 0 {
		stepTimeout = vu.Scenario.Timeout.AsDuration()
	}

	reqCtx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()

	var bodyReader io.Reader
	if len(resolvedBody) > 0 {
		bodyReader = bytes.NewReader([]byte(resolvedBody))
	}

	method := strings.ToUpper(step.Request.Method)
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(reqCtx, method, resolvedURL, bodyReader)
	if err != nil {
		result.Duration = time.Since(result.StartTime)
		result.Error = fmt.Sprintf("failed to build http request: %v", err)
		return result, fmt.Errorf("%s", result.Error)
	}

	// 4. Set Headers (Scenario Defaults -> Step Headers) with interpolation
	mergedHeaders := make(map[string]string)
	for k, v := range vu.Scenario.DefaultHeaders {
		mergedHeaders[k] = v
	}
	for k, v := range step.Request.Headers {
		mergedHeaders[k] = v
	}

	for k, v := range mergedHeaders {
		interpolatedVal, err := vu.State.Interpolate(v)
		if err != nil {
			result.Duration = time.Since(result.StartTime)
			result.Error = fmt.Sprintf("header %q interpolation failed: %v", k, err)
			return result, fmt.Errorf("%s", result.Error)
		}
		req.Header.Set(k, interpolatedVal)
	}

	// 5. Execute HTTP Request
	callStart := time.Now()
	resp, err := vu.Client.Do(req)
	result.Duration = time.Since(callStart)

	if err != nil {
		result.Error = fmt.Sprintf("HTTP request error: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read response body: %v", err)
		return result, err
	}

	// 6. Evaluate Assertions
	if err := vu.evaluateAssertions(step, resp, bodyBytes); err != nil {
		result.Error = err.Error()
		result.Success = false
		return result, nil // Assertion failure is recorded in result.Success = false
	}

	// 7. Perform Extractions
	for _, extCfg := range step.Extractors {
		val, extErr := Extract(extCfg, resp, bodyBytes)
		if extErr != nil {
			result.Error = fmt.Sprintf("extractor for var %q failed: %v", extCfg.TargetVar, extErr)
			result.Success = false
			return result, nil
		}
		vu.State.Set(extCfg.TargetVar, val)
		result.ExtractedVars[extCfg.TargetVar] = val
	}

	result.Success = true
	return result, nil
}

func (vu *VirtualUser) evaluateAssertions(step *Step, resp *http.Response, body []byte) error {
	// If no assertions are explicitly defined, default assertion is HTTP 2xx (status < 400)
	if len(step.Assertions) == 0 {
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP status %d >= 400", resp.StatusCode)
		}
		return nil
	}

	for _, a := range step.Assertions {
		switch a.Type {
		case AssertStatusCodeInRange:
			minC := a.MinCode
			if minC == 0 {
				minC = 200
			}
			maxC := a.MaxCode
			if maxC == 0 {
				maxC = 299
			}
			if resp.StatusCode < minC || resp.StatusCode > maxC {
				return fmt.Errorf("status code %d outside expected range [%d, %d]", resp.StatusCode, minC, maxC)
			}

		case AssertBodyContains:
			expStr := fmt.Sprintf("%v", a.Expected)
			if !strings.Contains(string(body), expStr) {
				return fmt.Errorf("response body does not contain expected substring %q", expStr)
			}

		case AssertHeaderEquals:
			actual := resp.Header.Get(a.Target)
			expStr := fmt.Sprintf("%v", a.Expected)
			if actual != expStr {
				return fmt.Errorf("header %q expected %q, got %q", a.Target, expStr, actual)
			}
		}
	}

	return nil
}

func (vu *VirtualUser) recordStepResult(res *StepResult) {
	if res == nil {
		return
	}
	vu.mu.Lock()
	vu.history = append(vu.history, res)
	vu.mu.Unlock()
}

// GetHistory returns an immutable copy of step execution results.
func (vu *VirtualUser) GetHistory() []*StepResult {
	vu.mu.RLock()
	defer vu.mu.RUnlock()
	res := make([]*StepResult, len(vu.history))
	copy(res, vu.history)
	return res
}
