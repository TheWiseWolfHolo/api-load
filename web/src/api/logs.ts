import request from './index';
import type { RequestLog } from '@/types/models';

export type { RequestLog };

export interface LogQuery {
  page?: number;
  size?: number;
  group_id?: number;
  start_time?: string;
  end_time?: string;
  status_code?: number;
}

export interface PaginatedLogs {
  total: number;
  page: number;
  size: number;
  data: RequestLog[];
}

export const getLogs = (query: LogQuery): Promise<PaginatedLogs> => {
  return request.get('/api/logs', { params: query });
};