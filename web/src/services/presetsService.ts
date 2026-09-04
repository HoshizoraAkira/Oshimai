import { apiClient } from './apiClient';
import { CulturalPreset, CarrierPreset, DependencyPreset, RampingStageDTO } from '../types/models';

export const presetsService = {
  getCulturalPresets(): Promise<CulturalPreset[]> {
    return apiClient.get<CulturalPreset[]>('/api/v1/presets/cultural');
  },

  // baselineVUs scales the returned stages (peak = baselineVUs * peak_multiplier), matching
  // CulturalPreset.BuildRampingStages on the server — this is the one source of truth for the
  // ramp curve so the frontend never has to reimplement per-preset shape logic.
  getCulturalStages(presetId: string, baselineVUs: number): Promise<{ preset: CulturalPreset; stages: RampingStageDTO[] }> {
    return apiClient.get(`/api/v1/presets/cultural/${presetId}`, { params: { baseline_vus: baselineVUs } });
  },

  createCulturalPreset(preset: Partial<CulturalPreset>): Promise<CulturalPreset> {
    return apiClient.post<CulturalPreset>('/api/v1/presets/cultural', preset);
  },

  updateCulturalPreset(id: string, preset: Partial<CulturalPreset>): Promise<CulturalPreset> {
    return apiClient.put<CulturalPreset>(`/api/v1/presets/cultural/${id}`, preset);
  },

  deleteCulturalPreset(id: string): Promise<{ status: string }> {
    return apiClient.delete(`/api/v1/presets/cultural/${id}`);
  },

  getDependencyPresets(): Promise<DependencyPreset[]> {
    return apiClient.get<DependencyPreset[]>('/api/v1/presets/dependencies');
  },

  getCarrierPresets(): Promise<CarrierPreset[]> {
    return apiClient.get<CarrierPreset[]>('/api/v1/presets/carriers');
  },
};
