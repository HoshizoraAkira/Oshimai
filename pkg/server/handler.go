package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/oshimai/twin/pkg/benchmark"
	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/cloudverify"
	"github.com/oshimai/twin/pkg/generator"
	"github.com/oshimai/twin/pkg/i18n"
	"github.com/oshimai/twin/pkg/k8schaos"
	"github.com/oshimai/twin/pkg/narrator"
	"github.com/oshimai/twin/pkg/presets"
	"github.com/oshimai/twin/pkg/remediation"
	"github.com/oshimai/twin/pkg/scanner"
	"github.com/oshimai/twin/pkg/scheduler"
	"github.com/oshimai/twin/pkg/verify"
	"github.com/oshimai/twin/pkg/vusession"
	"github.com/oshimai/twin/web"
)

// APIHandler exposes RESTful endpoints and SSE real-time streaming for the control plane.
type APIHandler struct {
	repo        StorageRepository
	coordinator *TestCoordinator
	eventBus    *EventBus
	llmClient   generator.LLMClient // Optional — nil disables natural-language scenario generation.
	gameDays    *scheduler.Scheduler
	logBuffer   *LogBuffer
	presets     *presets.Store
}

// NewAPIHandler creates an initialized APIHandler, including a GameDay scheduler whose trigger
// launches a normal run via StartRun — a weekly chaos GameDay is just a StartRun call nobody has
// to remember to make. The scheduler's check loop is NOT started automatically: call
// StartGameDayScheduler once the server is ready to serve traffic (main.go does this; tests that
// only exercise the CRUD endpoints don't need a live background loop).
func NewAPIHandler(repo StorageRepository, coordinator *TestCoordinator, eventBus *EventBus) *APIHandler {
	h := &APIHandler{
		repo:        repo,
		coordinator: coordinator,
		eventBus:    eventBus,
		logBuffer:   GetDefaultLogBuffer(),
		presets:     presets.NewStore(),
	}
	h.gameDays = scheduler.New(
		func(ctx context.Context, sched scheduler.GameDaySchedule) error {
			var req CreateRunRequest
			if err := json.Unmarshal(sched.Payload, &req); err != nil {
				return fmt.Errorf("gameday %q has an invalid stored run payload: %w", sched.ID, err)
			}
			_, err := h.coordinator.StartRun(ctx, req)
			return err
		},
		func(scheduleID string, err error) {
			log.Printf("[Oshimai] GameDay schedule %q failed to fire: %v", scheduleID, err)
		},
	)
	return h
}

// StartGameDayScheduler begins the recurring check loop for GameDay schedules, running until ctx
// is cancelled.
func (h *APIHandler) StartGameDayScheduler(ctx context.Context) {
	h.gameDays.Start(ctx, 30*time.Second)
}

// SetLLMClient wires an LLMClient (e.g. generator.NewAnthropicClientFromEnv) to enable the
// natural-language scenario endpoint. Left unset, that endpoint returns a clear "not configured"
// error instead of silently falling back to something else.
func (h *APIHandler) SetLLMClient(client generator.LLMClient) {
	h.llmClient = client
}

// SetLogBuffer overrides the internal LogBuffer (useful in tests).
func (h *APIHandler) SetLogBuffer(lb *LogBuffer) {
	h.logBuffer = lb
}

// RegisterRoutes registers all API endpoints and embedded web UI into an http.ServeMux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// System control plane logs
	mux.HandleFunc("GET /api/v1/system/logs", h.handleGetSystemLogs)
	mux.HandleFunc("GET /api/v1/system/logs/stream", h.handleStreamSystemLogs)
	mux.HandleFunc("POST /api/v1/system/logs/clear", h.handleClearSystemLogs)

	// Core run lifecycle
	mux.HandleFunc("POST /api/v1/scenarios/generate", h.handleGenerateScenario)
	mux.HandleFunc("POST /api/v1/scenarios/generate-nl", h.handleGenerateScenarioNL)
	mux.HandleFunc("POST /api/v1/runs", h.handleCreateRun)
	mux.HandleFunc("GET /api/v1/runs", h.handleListRuns)
	mux.HandleFunc("GET /api/v1/runs/{id}", h.handleGetRun)
	mux.HandleFunc("GET /api/v1/runs/{id}/diagnostics", h.handleGetRunDiagnostics)
	mux.HandleFunc("POST /api/v1/runs/{id}/narrate", h.handleNarrateRun)
	mux.HandleFunc("POST /api/v1/runs/{id}/abort", h.handleAbortRun)
	mux.HandleFunc("POST /api/v1/runs/{id}/approve", h.handleApproveRun)
	mux.HandleFunc("GET /api/v1/runs/{id}/stream", h.handleStreamTelemetry)
	mux.HandleFunc("GET /api/v1/runs/{id}/trend", h.handleRunTrend)

	// Reporting & sharing
	mux.HandleFunc("GET /api/v1/runs/{id}/badge.svg", h.handleRunBadge)
	mux.HandleFunc("GET /api/v1/runs/{id}/report", h.handleExecutiveReport)
	mux.HandleFunc("GET /api/v1/runs/{id}/authorization-letter", h.handleAuthorizationLetter)
	mux.HandleFunc("POST /api/v1/runs/{id}/share", h.handleCreateShareLink)
	mux.HandleFunc("GET /api/v1/public/runs/{token}", h.handlePublicRun)
	mux.HandleFunc("GET /api/v1/public/runs/{token}/stream", h.handlePublicStreamTelemetry)

	// Target-ownership verification (trust & safety gate)
	mux.HandleFunc("POST /api/v1/verify/challenge", h.handleVerifyChallenge)
	mux.HandleFunc("POST /api/v1/verify/confirm", h.handleVerifyConfirm)
	mux.HandleFunc("GET /api/v1/verify/status", h.handleVerifyStatus)
	mux.HandleFunc("POST /api/v1/verify/confirm-cloud", h.handleVerifyConfirmCloud)

	// Scenario onboarding: zero-spec import paths for non-technical operators
	mux.HandleFunc("GET /api/v1/presets/cultural", h.handleListCulturalPresets)
	mux.HandleFunc("POST /api/v1/presets/cultural", h.handleCreateCulturalPreset)
	mux.HandleFunc("GET /api/v1/presets/cultural/{id}", h.handleGetCulturalPresetStages)
	mux.HandleFunc("PUT /api/v1/presets/cultural/{id}", h.handleUpdateCulturalPreset)
	mux.HandleFunc("DELETE /api/v1/presets/cultural/{id}", h.handleDeleteCulturalPreset)
	mux.HandleFunc("GET /api/v1/presets/dependencies", h.handleListDependencyPresets)
	mux.HandleFunc("GET /api/v1/presets/carriers", h.handleListCarrierPresets)
	mux.HandleFunc("POST /api/v1/scenarios/import/har", h.handleImportHAR)
	mux.HandleFunc("POST /api/v1/scenarios/import/postman", h.handleImportPostman)
	mux.HandleFunc("POST /api/v1/scenarios/import/insomnia", h.handleImportInsomnia)
	mux.HandleFunc("POST /api/v1/scenarios/import/jmeter", h.handleImportJMeter)
	mux.HandleFunc("POST /api/v1/scenarios/import/k6", h.handleImportK6)
	mux.HandleFunc("POST /api/v1/scenarios/autodiscover", h.handleAutoDiscover)
	mux.HandleFunc("POST /api/v1/scenarios/auto-discover", h.handleAutoDiscover)
	mux.HandleFunc("POST /api/v1/scenarios/fuzz", h.handleBuildFuzzScenario)
	mux.HandleFunc("POST /api/v1/traces/dependency-graph", h.handleDependencyGraph)
	mux.HandleFunc("POST /api/v1/scenarios/dependency-graph", h.handleDependencyGraph)
	mux.HandleFunc("POST /api/v1/traces/sessions", h.handleListTraceSessions)
	mux.HandleFunc("POST /api/v1/scenarios/traces", h.handleListTraceSessions)
	mux.HandleFunc("POST /api/v1/traces/replay", h.handleBuildReplayScenario)
	mux.HandleFunc("POST /api/v1/scenarios/traces/replay", h.handleBuildReplayScenario)

	// Breaking-point search: Auto Chaos Fuzzer and Auto-Pilot capacity search
	mux.HandleFunc("POST /api/v1/autofuzz", h.handleAutoFuzz)
	mux.HandleFunc("POST /api/v1/autopilot", h.handleAutoPilot)

	// Multi-region agents
	mux.HandleFunc("POST /api/v1/agents/register", h.handleAgentRegister)
	mux.HandleFunc("POST /api/v1/agents/{id}/heartbeat", h.handleAgentHeartbeat)
	mux.HandleFunc("GET /api/v1/agents", h.handleListAgents)
	mux.HandleFunc("GET /api/v1/agents/{id}/poll", h.handleAgentPoll)
	mux.HandleFunc("POST /api/v1/agents/{id}/results", h.handleAgentSubmitResult)
	mux.HandleFunc("POST /api/v1/runs/multiregion", h.handleRunMultiRegion)

	// Kubernetes-native chaos and closed-loop autoscaler validation
	mux.HandleFunc("POST /api/v1/k8s/pod-kill", h.handleK8sPodKill)
	mux.HandleFunc("POST /api/v1/k8s/autoscaler-report", h.handleK8sAutoscalerReport)

	// GameDay scheduling: recurring runs
	mux.HandleFunc("POST /api/v1/gameday/schedules", h.handleCreateGameDaySchedule)
	mux.HandleFunc("GET /api/v1/gameday/schedules", h.handleListGameDaySchedules)
	mux.HandleFunc("DELETE /api/v1/gameday/schedules/{id}", h.handleDeleteGameDaySchedule)
	mux.HandleFunc("POST /api/v1/gameday/schedules/{id}/toggle", h.handleToggleGameDaySchedule)

	// Anonymous cross-customer benchmark
	mux.HandleFunc("POST /api/v1/benchmark/submit", h.handleBenchmarkSubmit)
	mux.HandleFunc("POST /api/v1/benchmark/compare", h.handleBenchmarkCompare)

	// Internationalization / Locales (en, id, jp)
	mux.HandleFunc("GET /api/v1/locales", h.handleListLocales)
	mux.HandleFunc("GET /api/v1/locales/{lang}", h.handleGetLocale)

	// Web Dashboard Static Assets (Embedded via embed.FS)
	mux.Handle("GET /", web.Handler())
}

func (h *APIHandler) handleGenerateScenario(w http.ResponseWriter, r *http.Request) {
	var req GenerateScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}

	if req.OpenAPISpec == "" {
		writeJSONError(w, http.StatusBadRequest, "openapi_spec is required")
		return
	}

	sc, err := generator.Synthesize(r.Context(), []byte(req.OpenAPISpec), []byte(req.OTelTraces), req.Config)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, fmt.Sprintf("scenario synthesis failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, sc)
}

func (h *APIHandler) handleGenerateScenarioNL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// Accept both 'prompt' (sent by frontend) and 'description' for backwards compatibility.
		Prompt      string `json:"prompt"`
		Description string `json:"description"`
		BaseURL     string `json:"base_url,omitempty"`
		Locale      string `json:"locale,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}

	// Prefer 'prompt' (frontend convention), fall back to 'description' (API convention).
	description := req.Prompt
	if description == "" {
		description = req.Description
	}

	sc, err := generator.GenerateScenarioFromDescription(r.Context(), h.llmClient, generator.NLScenarioRequest{
		Description: description, BaseURL: req.BaseURL, Locale: req.Locale,
	})
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	yamlBytes, _ := generator.ExportYAML(sc)
	writeJSON(w, http.StatusOK, map[string]any{
		"scenario":      sc,
		"scenario_yaml": string(yamlBytes),
	})
}

func (h *APIHandler) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req CreateRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}

	run, err := h.coordinator.StartRun(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, run)
}

func (h *APIHandler) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	runs, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, runs)
}

func (h *APIHandler) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "run id is required")
		return
	}

	run, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("run %q not found", id))
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func (h *APIHandler) handleGetRunDiagnostics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "run id is required")
		return
	}

	run, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("run %q not found", id))
		return
	}

	if run.Diagnostics == nil {
		targetVUs := run.Config.LoadConfig.VUs
		isChaos := run.Config.ChaosPlan.Enabled
		run.Diagnostics = remediation.Analyze(run.Summary, targetVUs, isChaos, run.Config.BusinessContext)
	}

	writeJSON(w, http.StatusOK, run.Diagnostics)
}

// handleNarrateRun combines the run's diagnostic report with an optional static scan pass and an
// optional traffic dependency graph — both supplied by the caller, since scan and graph are
// mined independently of any one run — into one AI-narrated incident summary that cross-references
// all three, falling back to a deterministic narrative when no LLM client is configured.
func (h *APIHandler) handleNarrateRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "run id is required")
		return
	}

	run, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("run %q not found", id))
		return
	}

	if run.Diagnostics == nil {
		targetVUs := run.Config.LoadConfig.VUs
		isChaos := run.Config.ChaosPlan.Enabled
		run.Diagnostics = remediation.Analyze(run.Summary, targetVUs, isChaos, run.Config.BusinessContext)
	}

	var req struct {
		ScanFindings []scanner.Finding          `json:"scan_findings,omitempty"`
		Graph        *generator.DependencyGraph `json:"dependency_graph,omitempty"`
	}
	// Body is optional — a bare POST with no payload just narrates the run on its own.
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	result := narrator.Narrate(r.Context(), h.llmClient, narrator.Input{
		Report:   run.Diagnostics,
		Findings: req.ScanFindings,
		Graph:    req.Graph,
	})

	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleAbortRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "run id is required")
		return
	}

	if err := h.coordinator.AbortRun(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":     id,
		"status": "aborted",
	})
}

// handleStreamTelemetry serves real-time metrics via Server-Sent Events (SSE).
func (h *APIHandler) handleStreamTelemetry(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming unsupported by server")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "run id is required")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	events, unsubscribe := h.eventBus.Subscribe(id, 64)
	defer unsubscribe()

	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (h *APIHandler) handleApproveRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Approver string `json:"approver"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	run, err := h.coordinator.ApproveRun(r.Context(), id, req.Approver)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *APIHandler) handleRunTrend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	current, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("run %q not found", id))
		return
	}

	var baseline *TestRun
	if baselineID := r.URL.Query().Get("baseline"); baselineID != "" {
		baseline, err = h.repo.Get(r.Context(), baselineID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("baseline run %q not found", baselineID))
			return
		}
	} else {
		allRuns, err := h.repo.List(r.Context(), 0, 0)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		baseline = FindPreviousCompleted(allRuns, id)
	}

	if baseline == nil {
		writeJSONError(w, http.StatusUnprocessableEntity, "no baseline run available to compare against yet")
		return
	}

	report, err := CompareRuns(current, baseline)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *APIHandler) handleRunBadge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, _ := h.repo.Get(r.Context(), id) // A missing run just renders a "no data" badge.
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(GenerateBadge(run)))
}

func (h *APIHandler) handleExecutiveReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("run %q not found", id))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(GenerateExecutiveReport(run)))
}

func (h *APIHandler) handleAuthorizationLetter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("run %q not found", id))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(GenerateAuthorizationLetter(run)))
}

func (h *APIHandler) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("run %q not found", id))
		return
	}

	if run.ShareToken == "" {
		tokenBytes := make([]byte, 16)
		if _, err := rand.Read(tokenBytes); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to generate share token")
			return
		}
		run.ShareToken = hex.EncodeToString(tokenBytes)
		if err := h.repo.Update(r.Context(), run); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"share_token": run.ShareToken,
		"public_url":  fmt.Sprintf("/api/v1/public/runs/%s", run.ShareToken),
	})
}

func (h *APIHandler) handlePublicRun(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	run, err := h.repo.FindByShareToken(r.Context(), token)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "no public run found for this share link")
		return
	}
	// Strip anything a stakeholder viewing an unauthenticated public link shouldn't see.
	publicView := map[string]any{
		"id":          run.ID,
		"status":      run.Status,
		"start_time":  run.StartTime,
		"end_time":    run.EndTime,
		"summary":     run.Summary,
		"diagnostics": run.Diagnostics,
	}
	writeJSON(w, http.StatusOK, publicView)
}

func (h *APIHandler) handlePublicStreamTelemetry(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	run, err := h.repo.FindByShareToken(r.Context(), token)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "no public run found for this share link")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming unsupported by server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events, unsubscribe := h.eventBus.Subscribe(run.ID, 64)
	defer unsubscribe()
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (h *APIHandler) handleVerifyChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURL string `json:"target_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	ch, err := h.coordinator.Verification().Challenge(req.TargetURL)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

func (h *APIHandler) handleVerifyConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURL string `json:"target_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	rec, err := h.coordinator.Verification().Confirm(r.Context(), req.TargetURL)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleVerifyStatus(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target_url")
	if target == "" {
		writeJSONError(w, http.StatusBadRequest, "target_url query parameter is required")
		return
	}
	status, err := h.coordinator.Verification().Status(target)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *APIHandler) handleVerifyConfirmCloud(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURL    string `json:"target_url"`
		AWSAccessKey string `json:"aws_access_key"`
		AWSSecretKey string `json:"aws_secret_key"`
		AWSRegion    string `json:"aws_region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	if req.AWSAccessKey == "" || req.AWSSecretKey == "" {
		writeJSONError(w, http.StatusBadRequest, "aws_access_key and aws_secret_key are required")
		return
	}

	host, err := verify.ExtractHost(req.TargetURL)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if verify.IsBlockedTarget(host) {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("%q is a well-known public domain and cannot be verified as a target", host))
		return
	}

	ips, err := net.DefaultResolver.LookupHost(r.Context(), host)
	if err != nil || len(ips) == 0 {
		writeJSONError(w, http.StatusUnprocessableEntity, fmt.Sprintf("could not resolve %q: %v", host, err))
		return
	}

	checker := cloudverify.NewAWSChecker(req.AWSAccessKey, req.AWSSecretKey, req.AWSRegion)
	for _, ip := range ips {
		owns, resource, checkErr := checker.OwnsIP(r.Context(), ip)
		if checkErr != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, checkErr.Error())
			return
		}
		if owns {
			rec, err := h.coordinator.Verification().ConfirmVerified(req.TargetURL, verify.MethodCloudIPOwnership)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"verified": true, "method": rec.Method, "resource": resource, "matched_ip": ip})
			return
		}
	}
	writeJSONError(w, http.StatusUnprocessableEntity, fmt.Sprintf("none of %q's resolved IPs (%v) were found as Elastic IPs in this AWS account", host, ips))
}

func (h *APIHandler) handleListCulturalPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.presets.List())
}

// handleCreateCulturalPreset lets an operator add their own cultural load-shape preset — traffic
// events don't look the same everywhere, so the four Indonesian built-ins (Harbolnas, Gajian,
// Mudik, viral endorsement) are a starting point, not the whole list. An explicit Shape is
// optional; without one the preset falls back to a generic ramp/hold/taper curve.
func (h *APIHandler) handleCreateCulturalPreset(w http.ResponseWriter, r *http.Request) {
	var p presets.CulturalPreset
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	created, err := h.presets.Create(p)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleUpdateCulturalPreset edits a previously-created custom preset in place. Built-in presets
// are rejected here — see presets.Store.Update.
func (h *APIHandler) handleUpdateCulturalPreset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var p presets.CulturalPreset
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	updated, err := h.presets.Update(id, p)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleDeleteCulturalPreset removes a previously-created custom preset. Built-in presets are
// rejected here — see presets.Store.Delete.
func (h *APIHandler) handleDeleteCulturalPreset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.presets.Delete(id); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *APIHandler) handleListDependencyPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, presets.ListDependencies())
}

func (h *APIHandler) handleListCarrierPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, presets.ListCarrierProfiles())
}

func (h *APIHandler) handleGetCulturalPresetStages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	preset, ok := h.presets.Find(id)
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("cultural preset %q not found", id))
		return
	}

	baselineVUs := 10
	if v := r.URL.Query().Get("baseline_vus"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			baselineVUs = parsed
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"preset": preset,
		"stages": preset.BuildRampingStages(baselineVUs),
	})
}

func (h *APIHandler) handleImportHAR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HAR    string                    `json:"har"`
		Config generator.GeneratorConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	sc, err := generator.ImportHAR(r.Context(), []byte(req.HAR), req.Config)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *APIHandler) handleImportPostman(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Collection string                    `json:"collection"`
		Config     generator.GeneratorConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	sc, err := generator.ImportPostmanCollection(r.Context(), []byte(req.Collection), req.Config)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *APIHandler) handleImportInsomnia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Export string                    `json:"export"`
		Config generator.GeneratorConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	sc, err := generator.ImportInsomniaCollection(r.Context(), []byte(req.Export), req.Config)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *APIHandler) handleImportJMeter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Jmx    string                    `json:"jmx"`
		Config generator.GeneratorConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	sc, err := generator.ImportJMeterPlan(r.Context(), []byte(req.Jmx), req.Config)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *APIHandler) handleImportK6(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Script string                    `json:"script"`
		Config generator.GeneratorConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	sc, err := generator.ImportK6Script(r.Context(), req.Script, req.Config)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *APIHandler) handleAutoDiscover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURL     string                    `json:"target_url"`
		TargetBaseURL string                    `json:"target_base_url"`
		Config        generator.GeneratorConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	target := req.TargetURL
	if target == "" {
		target = req.TargetBaseURL
	}
	if target == "" {
		writeJSONError(w, http.StatusBadRequest, "target_url is required")
		return
	}
	sc, err := generator.AutoDiscover(r.Context(), target, req.Config)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *APIHandler) handleBuildFuzzScenario(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ScenarioYAML string `json:"scenario_yaml,omitempty"`
		ScenarioJSON string `json:"scenario_json,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}

	var base *vusession.Scenario
	var err error
	switch {
	case req.ScenarioYAML != "":
		base, err = vusession.ParseScenarioYAML([]byte(req.ScenarioYAML))
	case req.ScenarioJSON != "":
		base, err = vusession.ParseScenarioJSON([]byte(req.ScenarioJSON))
	default:
		writeJSONError(w, http.StatusBadRequest, "scenario_yaml or scenario_json is required")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	fuzzSc, err := generator.BuildFuzzScenario(base)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fuzzSc)
}

func (h *APIHandler) handleDependencyGraph(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OTelTraces string `json:"otel_traces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	if req.OTelTraces == "" {
		writeJSONError(w, http.StatusBadRequest, "otel_traces is required")
		return
	}

	spans, err := generator.ParseOTelSpansJSON([]byte(req.OTelTraces))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse otel_traces: %v", err))
		return
	}
	matrix, err := generator.MineBehavioralTransitions(spans)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	graph := generator.BuildDependencyGraph(matrix)
	writeJSON(w, http.StatusOK, graph)
}

func (h *APIHandler) handleListTraceSessions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OTelTraces string `json:"otel_traces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	spans, err := generator.ParseOTelSpansJSON([]byte(req.OTelTraces))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse otel_traces: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trace_ids": generator.ListReplayableTraces(spans)})
}

func (h *APIHandler) handleBuildReplayScenario(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OTelTraces      string  `json:"otel_traces"`
		TraceID         string  `json:"trace_id"`
		SpeedMultiplier float64 `json:"speed_multiplier,omitempty"`
		BaseURL         string  `json:"base_url,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	if req.TraceID == "" {
		writeJSONError(w, http.StatusBadRequest, "trace_id is required")
		return
	}
	spans, err := generator.ParseOTelSpansJSON([]byte(req.OTelTraces))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse otel_traces: %v", err))
		return
	}
	sc, err := generator.BuildReplayScenario(req.TraceID, spans, req.SpeedMultiplier, generator.GeneratorConfig{BaseURL: req.BaseURL})
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

type k8sConnectionRequest struct {
	APIServer          string `json:"api_server"`
	BearerToken        string `json:"bearer_token"`
	Namespace          string `json:"namespace"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

func (r k8sConnectionRequest) toClient() (*k8schaos.Client, error) {
	if r.APIServer == "" || r.BearerToken == "" {
		return nil, fmt.Errorf("api_server and bearer_token are required")
	}
	ns := r.Namespace
	if ns == "" {
		ns = "default"
	}
	return k8schaos.NewClient(k8schaos.Config{
		APIServer: r.APIServer, BearerToken: r.BearerToken, Namespace: ns, InsecureSkipVerify: r.InsecureSkipVerify,
	})
}

func (h *APIHandler) handleK8sPodKill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		k8sConnectionRequest
		LabelSelector   string  `json:"label_selector"`
		KillPercent     float64 `json:"kill_percent"`
		DurationSeconds int     `json:"duration_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	client, err := req.toClient()
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.LabelSelector == "" {
		writeJSONError(w, http.StatusBadRequest, "label_selector is required")
		return
	}
	duration := time.Duration(req.DurationSeconds) * time.Second
	if duration <= 0 {
		duration = 30 * time.Second
	}

	driver := k8schaos.NewPodKillDriver(client)
	fault := chaos.FaultSpec{
		ID:          "pod_kill_" + fmt.Sprint(time.Now().Unix()),
		Type:        chaos.FaultComposite,
		Filter:      chaos.FilterConfig{Interface: "k8s", TargetDomains: []string{req.LabelSelector}},
		LossPercent: req.KillPercent,
		Duration:    duration,
	}
	if err := driver.Apply(r.Context(), fault); err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, driver.Status())
}

func (h *APIHandler) handleK8sAutoscalerReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		k8sConnectionRequest
		HPAName              string `json:"hpa_name"`
		WatchDurationSeconds int    `json:"watch_duration_seconds"`
		PollIntervalSeconds  int    `json:"poll_interval_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	client, err := req.toClient()
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.HPAName == "" {
		writeJSONError(w, http.StatusBadRequest, "hpa_name is required")
		return
	}
	watchDuration := time.Duration(req.WatchDurationSeconds) * time.Second
	if watchDuration <= 0 {
		watchDuration = 60 * time.Second
	}
	pollInterval := time.Duration(req.PollIntervalSeconds) * time.Second

	// MonitorAutoscaler runs for exactly watchDuration; the request itself gets a little extra
	// slack on top so the HTTP round-trip isn't racing the monitor's own deadline.
	monitorCtx, cancelMonitor := context.WithTimeout(r.Context(), watchDuration)
	defer cancelMonitor()

	report, err := k8schaos.MonitorAutoscaler(monitorCtx, client, req.Namespace, req.HPAName, pollInterval)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *APIHandler) handleCreateGameDaySchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string           `json:"name"`
		Weekday   int              `json:"weekday"` // 0=Sunday .. 6=Saturday, matching time.Weekday
		HourUTC   int              `json:"hour_utc"`
		MinuteUTC int              `json:"minute_utc"`
		Run       CreateRunRequest `json:"run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	payload, err := json.Marshal(req.Run)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, err := h.gameDays.Add(req.Name, time.Weekday(req.Weekday%7), req.HourUTC, req.MinuteUTC, payload)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *APIHandler) handleListGameDaySchedules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.gameDays.List())
}

func (h *APIHandler) handleDeleteGameDaySchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.gameDays.Remove(id) {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("schedule %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *APIHandler) handleToggleGameDaySchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	if !h.gameDays.SetEnabled(id, req.Enabled) {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("schedule %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *APIHandler) handleBenchmarkSubmit(w http.ResponseWriter, r *http.Request) {
	var sub benchmark.Submission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, h.coordinator.Benchmark().Submit(sub))
}

func (h *APIHandler) handleBenchmarkCompare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Category    string  `json:"category"`
		HealthScore int     `json:"health_score"`
		P99Ms       float64 `json:"p99_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, h.coordinator.Benchmark().Compare(req.Category, req.HealthScore, req.P99Ms))
}

func (h *APIHandler) handleAgentRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Region string `json:"region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeJSONError(w, http.StatusBadRequest, "id is required")
		return
	}
	info := h.coordinator.Agents().Register(req.ID, req.Region)
	writeJSON(w, http.StatusOK, info)
}

func (h *APIHandler) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.coordinator.Agents().Heartbeat(id) {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("agent %q is not registered", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.coordinator.Agents().ListConnected(r.URL.Query().Get("region")))
}

func (h *APIHandler) handleAgentPoll(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	assignment, ok := h.coordinator.Agents().Poll(id, 25*time.Second)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, assignment)
}

func (h *APIHandler) handleAgentSubmitResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var res AgentResult
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	res.AgentID = id
	h.coordinator.Agents().SubmitResult(res)
	writeJSON(w, http.StatusOK, map[string]string{"status": "received"})
}

func (h *APIHandler) handleRunMultiRegion(w http.ResponseWriter, r *http.Request) {
	var req MultiRegionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}
	result, err := h.coordinator.RunMultiRegion(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleAutoFuzz(w http.ResponseWriter, r *http.Request) {
	var req AutoFuzzRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}

	result, err := h.coordinator.RunAutoFuzz(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleAutoPilot(w http.ResponseWriter, r *http.Request) {
	var req AutoPilotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json payload: %v", err))
		return
	}

	result, err := h.coordinator.RunAutoPilot(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleListLocales(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"languages": i18n.SupportedLanguages(),
		"default":   "en",
	})
}

func (h *APIHandler) handleGetLocale(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("lang")
	if lang == "" {
		lang = i18n.RequestLang(r)
	}
	catalog := i18n.GetCatalog(lang)
	writeJSON(w, http.StatusOK, map[string]any{
		"language": i18n.NormalizeLang(lang),
		"catalog":  catalog,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (h *APIHandler) handleGetSystemLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	logs := h.logBuffer.GetRecent(limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"logs": logs,
	})
}

func (h *APIHandler) handleStreamSystemLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming unsupported by server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Backlog of recent logs on initial connect
	recent := h.logBuffer.GetRecent(50)
	for _, entry := range recent {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	ch, unsubscribe := h.logBuffer.Subscribe(64)
	defer unsubscribe()

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (h *APIHandler) handleClearSystemLogs(w http.ResponseWriter, r *http.Request) {
	h.logBuffer.Clear()
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "cleared",
	})
}
