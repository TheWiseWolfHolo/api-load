// 数据模型定义
export interface APIKey {
  id: number;
  group_id: number;
  key_value: string;
  status: "active" | "inactive";
  request_count: number;
  failure_count: number;
  last_used_at?: string;
  created_at: string;
  updated_at: string;
}

// 类型别名，用于兼容
export type Key = APIKey;

export interface UpstreamInfo {
  url: string;
  weight: number;
}

export interface Group {
  id?: number;
  name: string;
  display_name: string;
  description: string;
  sort: number;
  test_model: string;
  channel_type: "openai" | "gemini";
  upstreams: UpstreamInfo[];
  config: Record<string, unknown>;
  api_keys?: APIKey[];
  param_overrides: any;
  created_at?: string;
  updated_at?: string;
}

export interface GroupConfigOption {
  key: string;
  name: string;
  description: string;
  default_value: number;
}

export interface GroupStats {
  total_keys: number;
  active_keys: number;
  requests_1h: number;
  requests_24h: number;
  requests_7d: number;
  failure_rate_24h: number;
}

export interface TaskInfo {
  is_running: boolean;
  task_name?: string;
  group_id?: number;
  group_name?: string;
  processed?: number;
  total?: number;
  started_at?: string;
  message?: string;
}

export interface RequestLog {
  id: string;
  timestamp: string;
  group_id: number;
  key_id: number;
  source_ip: string;
  status_code: number;
  request_path: string;
  request_body_snippet: string;
}

export interface LogsResponse {
  total: number;
  page: number;
  size: number;
  data: RequestLog[];
}

export interface LogFilter {
  page: number;
  size: number;
  group_id?: number;
  start_time?: string;
  end_time?: string;
  status_code?: number;
  source_ip?: string;
}

export interface DashboardStats {
  total_requests: number;
  success_requests: number;
  success_rate: number;
  group_stats: GroupRequestStat[];
}

export interface GroupRequestStat {
  display_name: string;
  request_count: number;
}
