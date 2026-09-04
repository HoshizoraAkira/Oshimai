import { apiClient } from './apiClient';
import { TargetVerifyStatus } from '../types/models';

export interface ChallengeResponse {
  dns_record_name: string;
  dns_record_value: string;
  well_known_path: string;
  well_known_content: string;
}

export interface ConfirmResponse {
  verified: boolean;
  method: string;
}

export const verificationService = {
  getVerifyStatus(targetUrl: string): Promise<TargetVerifyStatus> {
    return apiClient.get<TargetVerifyStatus>('/api/v1/verify/status', {
      params: { target_url: targetUrl }
    });
  },

  requestChallenge(targetUrl: string): Promise<ChallengeResponse> {
    return apiClient.post<ChallengeResponse>('/api/v1/verify/challenge', { target_url: targetUrl });
  },

  confirmChallenge(targetUrl: string): Promise<ConfirmResponse> {
    return apiClient.post<ConfirmResponse>('/api/v1/verify/confirm', { target_url: targetUrl });
  },

  confirmCloud(payload: any): Promise<{ verified: boolean; method?: string; resource?: string; matched_ip?: string; error?: string }> {
    return apiClient.post('/api/v1/verify/cloud', payload);
  },

  verifyAwsIp(ipAddress: string): Promise<{ verified: boolean; range?: string; message?: string }> {
    return apiClient.post('/api/v1/verify/aws-ip', { ip_address: ipAddress });
  },
};
