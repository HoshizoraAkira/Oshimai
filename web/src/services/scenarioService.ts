import { apiClient } from './apiClient';
import { Scenario } from '../types/models';
import { SynthesizePayload, DependencyGraphData } from '../types/api';

export const scenarioService = {
  generateScenario(payload: SynthesizePayload): Promise<Scenario> {
    return apiClient.post<Scenario>('/api/v1/scenarios/generate', payload);
  },

  generateFromPrompt(promptText: string, baseUrl?: string): Promise<{ scenario_yaml: string; scenario: Scenario }> {
    return apiClient.post('/api/v1/scenarios/generate-nl', { prompt: promptText, base_url: baseUrl });
  },

  autoDiscover(targetBaseUrl: string): Promise<Scenario> {
    return apiClient.post<Scenario>('/api/v1/scenarios/auto-discover', {
      target_url: targetBaseUrl,
      target_base_url: targetBaseUrl,
    });
  },

  importFile(file: File, fileType: string): Promise<Scenario> {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('type', fileType);
    return apiClient.post<Scenario>('/api/v1/scenarios/import-file', formData);
  },

  getDependencyGraph(tracesJson: string): Promise<DependencyGraphData> {
    return apiClient.post<DependencyGraphData>('/api/v1/scenarios/dependency-graph', { otel_traces: tracesJson });
  },

  listTraceSessions(payload?: any): Promise<{ trace_ids?: string[]; sessions?: any[] }> {
    return apiClient.post('/api/v1/scenarios/traces', payload);
  },

  buildReplayScenario(payload: any): Promise<any> {
    return apiClient.post('/api/v1/scenarios/traces/replay', payload);
  },
};
