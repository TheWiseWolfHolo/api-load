import request from './index';
import type { DashboardStats } from '@/types/models';

export const getDashboardStats = (): Promise<DashboardStats> => {
  return request.get('/api/dashboard/stats');
};