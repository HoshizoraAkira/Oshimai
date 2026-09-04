import { apiClient } from './apiClient';
import { Run, RunDiagnostics } from '../types/models';
import { CreateRunPayload } from '../types/api';

export interface TrendReport {
  regressed: boolean;
  delta_p99_pct: number;
}

export interface ShareLinkResponse {
  share_token: string;
}

export const runsService = {
  createRun(payload: CreateRunPayload): Promise<{ run_id: string; status: string }> {
    return apiClient.post('/api/v1/runs', payload);
  },

  listRuns(params?: { limit?: number; offset?: number }): Promise<Run[]> {
    return apiClient.get<Run[]>('/api/v1/runs', { params });
  },

  getRun(runId: string): Promise<Run> {
    return apiClient.get<Run>(`/api/v1/runs/${runId}`);
  },

  getRunDiagnostics(runId: string): Promise<RunDiagnostics> {
    return apiClient.get<RunDiagnostics>(`/api/v1/runs/${runId}/diagnostics`);
  },

  abortRun(runId: string): Promise<{ message: string }> {
    return apiClient.post(`/api/v1/runs/${runId}/abort`);
  },

  approveRun(runId: string, approverName: string): Promise<{ approved: boolean }> {
    return apiClient.post(`/api/v1/runs/${runId}/approve`, { approver: approverName });
  },

  createShareLink(runId: string): Promise<ShareLinkResponse> {
    return apiClient.post<ShareLinkResponse>(`/api/v1/runs/${runId}/share`);
  },

  getRunTrend(runId: string): Promise<TrendReport> {
    return apiClient.get<TrendReport>(`/api/v1/runs/${runId}/trend`);
  },

  getPublicRun(token: string): Promise<Run> {
    return apiClient.get<Run>(`/api/v1/public/runs/${token}`);
  },
};
