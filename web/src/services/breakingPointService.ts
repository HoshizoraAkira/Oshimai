import { apiClient } from './apiClient';
import { AutoPilotPayload, AutoFuzzPayload } from '../types/api';

export interface TrialResult {
  vus?: number;
  severity: number;
  health_score: number;
  pass: boolean;
}

export interface SearchResult {
  found: boolean;
  verdict: string;
  trials: TrialResult[];
}

export const breakingPointService = {
  runAutoPilot(payload: AutoPilotPayload): Promise<SearchResult> {
    return apiClient.post<SearchResult>('/api/v1/autopilot', payload);
  },

  runAutoFuzz(payload: AutoFuzzPayload): Promise<SearchResult> {
    return apiClient.post<SearchResult>('/api/v1/autofuzz', payload);
  },
};
