export type RunStatus = 
  | 'pending' 
  | 'running' 
  | 'completed' 
  | 'aborted' 
  | 'failed' 
  | 'TRIPPED' 
  | 'DRAINING' 
  | 'INJECTED CHAOS' 
  | 'IDLE';

export interface LatencyPercentiles {
  min: number;
  max: number;
  mean: number;
  median: number;
  p50: number;
  p90: number;
  p95: number;
  p99: number;
  p999: number;
}

export interface StepMetric {
  step_id: string;
  count: number;
  errors: number;
  p50: number;
  p99: number;
  latency?: LatencyPercentiles;
  total_requests?: number;
  total_errors?: number;
  error_rate?: number;
  status_codes?: Record<string, number>;
}

export interface RunSummary {
  total_requests: number;
  total_responses: number;
  total_errors: number;
  actual_rps: number;
  duration?: number;
  total_duration?: number;
  latency: LatencyPercentiles;
  termination_status?: string;
  abort_reason?: string;
  status_codes?: Record<string, number>;
  step_metrics?: Record<string, StepMetric>;
}

export interface Remediation {
  type?: string;
  description?: string;
  diff_snippet?: string;
  severity?: string;
  title?: string;
}

export interface RunDiagnostics {
  run_id?: string;
  health_score: number;
  status_label: string;
  summary: string;
  summary_short?: string;
  remediations?: Remediation[];
  root_cause?: string;
  root_causes?: string[];
  executive_verdict?: string;
  compliance_summary?: string;
  revenue_loss_note?: string;
  suggested_config_patch?: string;
  estimated_revenue_loss_idr?: number;
  safe_capacity_estimate?: string;
  detected_issues?: Array<{ title: string; severity: string; description: string }>;
  actionable_fixes?: string[];
}

export interface RunConfig {
  target_base_url?: string;
  base_url?: string;
  load_config?: {
    profile?: string;
    vus?: number;
    target_rps?: number;
    duration?: number;
  };
  circuit_breaker?: {
    max_error_rate_pct?: number;
    p99_latency_threshold_ns?: number;
  };
  chaos?: {
    fault_type?: string;
    probability?: number;
  };
}

export interface Run {
  id: string;
  status: string;
  target_base_url?: string;
  start_time?: string;
  end_time?: string;
  config?: RunConfig;
  summary?: RunSummary;
  diagnostics?: RunDiagnostics;
  scenario?: Scenario;
  error?: string;
}

export interface TelemetryData {
  status?: string;
  current_rps: number;
  active_vus: number;
  latency_p50: number;
  latency_p90: number;
  latency_p99: number;
  error_rate: number;
  error_count: number;
  chaos_active: boolean;
  chaos_fault_id?: string;
}

export interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
}

// Field names must match pkg/vusession/model.go's AssertionConfig exactly — the engine
// silently ignores an assertion whose `type` it doesn't recognize (see evaluateAssertions
// in pkg/vusession/engine.go), so a mismatched shape here validates nothing at runtime.
export interface ScenarioAssertion {
  type: 'status_in_range' | 'body_contains' | 'header_equals';
  min_code?: number;
  max_code?: number;
  expected?: string;
  target?: string;
}

// Field names must match pkg/vusession/model.go's ExtractorConfig exactly — same silent-
// no-op risk as ScenarioAssertion above if `source` doesn't match a known ExtractorSource.
export interface ScenarioExtractor {
  source: 'body_json' | 'header' | 'status_code' | 'regex';
  path?: string;
  regex?: string;
  target_var: string;
  default?: string;
}

export interface ScenarioTransition {
  target_step_id: string;
  probability?: number;
  condition?: string;
}

export interface ScenarioStep {
  id: string;
  name: string;
  type?: string;
  request?: {
    method: string;
    path: string;
    headers?: Record<string, string>;
    body?: string;
  };
  think_time?: string;
  condition?: string;
  transitions?: ScenarioTransition[];
  assertions?: ScenarioAssertion[];
  extractors?: ScenarioExtractor[];
}

export interface Scenario {
  id: string;
  name?: string;
  version?: string;
  base_url?: string;
  initial_step_id?: string;
  steps: Record<string, ScenarioStep>;
  variables?: Record<string, any>;
}

export interface FlowNode {
  id: string;
  name: string;
  type: 'http' | 'delay' | 'decision' | 'note';
  x: number;
  y: number;
  method?: string;
  path?: string;
  isInitial?: boolean;
  headers?: Record<string, string>;
  authBearer?: string;
  body?: string;
  assertions?: ScenarioAssertion[];
  extractors?: ScenarioExtractor[];
  transitions?: ScenarioTransition[];
  delayMs?: number;
  noteText?: string;
  condition?: string;
}

// User-authored scenario templates, persisted client-side (browser localStorage)
// so operators can build a personal library beyond the two built-in presets.
export interface SavedScenario {
  id: string;
  name: string;
  savedAt: string;
  scenarioId: string;
  baseUrl: string;
  nodes: FlowNode[];
}

export interface Agent {
  id: string;
  region: string;
  status: string;
  last_heartbeat?: string;
  target_rps?: number;
  weight?: number;
}

export interface GameDaySchedule {
  id: string;
  name: string;
  cron?: string;
  profile?: string;
  enabled: boolean;
  next_run?: string;
  fault_type?: string;
  weekday: number;
  hour_utc: number;
  minute_utc: number;
  last_fired_at?: string;
}

export interface BenchmarkResult {
  industry?: string;
  p99_ms?: number;
  error_pct?: number;
  score?: number;
  comparison_delta?: number;
  sample_count: number;
  message?: string;
  health_score_percentile?: number;
  latency_percentile?: number;
}

export interface TargetVerifyStatus {
  host: string;
  is_private: boolean;
  is_verified: boolean;
  is_blocked: boolean;
  verification_token?: string;
}

export interface PresetShapeStage {
  duration_percent: number;
  vu_multiplier: number;
}

export interface CulturalPreset {
  id: string;
  name: string;
  description_id?: string;
  description_en?: string;
  peak_multiplier: number;
  suggested_total_duration_sec: number;
  shape?: PresetShapeStage[];
  is_custom?: boolean;
}

export interface RampingStageDTO {
  duration: number; // nanoseconds, as returned by the Go backend
  target_vus: number;
}

export interface CarrierPreset {
  id?: string;
  name?: string;
  operator?: string;
  generation?: string;
  latency_ms: number;
  jitter_ms: number;
  packet_loss_percent?: number;
}

export interface DependencyPreset {
  id?: string;
  name: string;
  fault?: string;
  rate_limit_rps?: number;
  error_code?: number;
  recommended_delay_seconds?: number;
  latency_ms?: number;
  packet_loss_percent?: number;
  target_domain?: string;
}

export type ToastType = 'info' | 'success' | 'warning' | 'error';

export interface ToastMessage {
  id: number;
  message: string;
  type: ToastType;
}
