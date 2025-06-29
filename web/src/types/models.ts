// Based on internal/models/types.go

export interface Key {
    id: string;
    group_id: string;
    api_key: string;
    platform: 'OpenAI' | 'Gemini';
    model_types: string[];
    rate_limit: number;
    rate_limit_unit: 'minute' | 'hour' | 'day';
    usage: number;
    is_active: boolean;
    created_at: string;
    updated_at: string;
}

export interface Group {
    id: string;
    name: string;
    description: string;
    is_default: boolean;
    created_at: string;
    updated_at: string;
}

export interface GroupWithKeys extends Group {
    keys: Key[];
}

export interface GroupRequestStat {
  group_name: string;
  request_count: number;
}

export interface DashboardStats {
  total_requests: number;
  success_requests: number;
  success_rate: number;
  group_stats: GroupRequestStat[];
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
export interface Setting {
  key: string;
  value: string;
}
export interface Setting {
  key: string;
  value: string;
}