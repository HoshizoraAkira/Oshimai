import { Scenario, BenchmarkResult, GameDaySchedule, Agent } from './models';

export interface CreateRunPayload {
  target_base_url?: string;
  load_config?: {
    profile?: string;
    vus?: number;
    target_rps?: number;
    duration?: number;
    stages?: Array<{ duration_seconds: number; target_vus: number }>;
  };
  circuit_breaker?: {
    max_error_rate_pct?: number;
    p99_latency_threshold_ns?: number;
  };
  chaos?: {
    fault_type?: string;
    probability?: number;
    netem?: {
      latency_ms?: number;
      jitter_ms?: number;
      packet_loss_percent?: number;
    };
  };
  scenario_json?: string;
  scenario_yaml?: string;
  approver?: string;
  financial_impact_risk?: boolean;
}

export interface MultiRegionPayload {
  agent_ids: string[];
  total_vus: number;
}

export interface PodKillPayload {
  namespace: string;
  label_selector: string;
  action: 'kill' | 'drain' | 'corrupt';
}

export interface AutoscalerPayload {
  min_replicas: number;
  max_replicas: number;
  target_cpu_percent: number;
}

export interface SynthesizePayload {
  openapi_spec?: string;
  otel_traces?: string;
  config?: {
    scenario_id?: string;
    base_url?: string;
  };
}

export interface AutoPilotPayload {
  base: any;
  min_vus?: number;
  max_vus?: number;
  trial_duration_seconds?: number;
  health_threshold?: number;
  max_iterations?: number;
}

export interface AutoFuzzPayload {
  base: any;
  trial_duration_seconds?: number;
  health_threshold?: number;
  max_iterations?: number;
}

export interface DependencyGraphNode {
  id: string;
  criticality: number;
}

export interface DependencyGraphData {
  nodes?: DependencyGraphNode[];
  edges?: Array<{ source: string; target: string }>;
}
