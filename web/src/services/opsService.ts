import { apiClient } from './apiClient';
import { Agent, GameDaySchedule } from '../types/models';

export const opsService = {
  listAgents(): Promise<Agent[]> {
    return apiClient.get<Agent[]>('/api/v1/agents');
  },

  runMultiRegion(payload: any): Promise<{ run_id: string; dispatched_to?: number; agents_used?: number; total_requests?: number; regional_results?: Record<string, any> }> {
    return apiClient.post('/api/v1/runs/multiregion', payload);
  },

  k8sPodKill(payload: any): Promise<{ state: string; applied_at?: string; expires_at?: string }> {
    return apiClient.post('/api/v1/k8s/pod-kill', payload);
  },

  k8sAutoscalerReport(payload: any): Promise<{ scaled_up: boolean; start_replicas: number; peak_replicas: number; verdict: string }> {
    return apiClient.post('/api/v1/k8s/autoscaler-report', payload);
  },

  listGameDaySchedules(): Promise<GameDaySchedule[]> {
    return apiClient.get<GameDaySchedule[]>('/api/v1/gameday/schedules');
  },

  createGameDaySchedule(schedule: Partial<GameDaySchedule>): Promise<GameDaySchedule> {
    return apiClient.post<GameDaySchedule>('/api/v1/gameday/schedules', schedule);
  },

  toggleGameDaySchedule(scheduleId: string, enabled: boolean): Promise<GameDaySchedule> {
    return apiClient.post<GameDaySchedule>(`/api/v1/gameday/schedules/${scheduleId}/toggle`, { enabled });
  },

  deleteGameDaySchedule(scheduleId: string): Promise<{ deleted: boolean }> {
    return apiClient.delete(`/api/v1/gameday/schedules/${scheduleId}`);
  },

  compareBenchmark(payload: any): Promise<{ sample_count: number; message?: string; health_score_percentile?: number; latency_percentile?: number }> {
    return apiClient.post('/api/v1/benchmark/compare', payload);
  },
};
