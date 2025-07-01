import apiClient from './index';

export interface LoginRequest {
  auth_key: string;
}

export interface LoginResponse {
  success: boolean;
  message: string;
}

export const login = async (authKey: string): Promise<LoginResponse> => {
  const response = await apiClient.post<LoginResponse>('/auth/login', {
    auth_key: authKey
  });
  return response.data;
};