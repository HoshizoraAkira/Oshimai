import { apiClient } from './apiClient';
import { LogEntry } from '../types/models';

export const systemService = {
  getSystemLogs(): Promise<LogEntry[]> {
    return apiClient.get<LogEntry[]>('/api/v1/system/logs');
  },

  clearSystemLogs(): Promise<{ cleared: boolean }> {
    return apiClient.post('/api/v1/system/logs/clear');
  },
};
