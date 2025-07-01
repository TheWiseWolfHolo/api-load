import request from './index';
import type { DashboardStats } from '@/types/models';

export const getDashboardData = (timeRange: string, groupId: number | null): Promise<DashboardStats> => {
  const params = new URLSearchParams();
  params.append('time_range', timeRange);
  if (groupId) {
    params.append('group_id', groupId.toString());
  }
  return request.get(`/dashboard/data?${params.toString()}`);
};