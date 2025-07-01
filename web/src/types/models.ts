// Based on internal/models/types.go

// Corresponds to the APIKey struct in Go - 修正版本
export interface APIKey {
  id: number; // uint -> number
  group_id: number; // uint -> number
  key_value: string; // 对应后端key_value字段
  status: "active" | "inactive" | "error"; // 对应后端status字段
  request_count: number; // int64 -> number
  failure_count: number; // int64 -> number
  last_used_at?: string; // *time.Time -> optional string
  created_at: string; // time.Time -> string
  updated_at: string; // time.Time -> string
}

// 为了兼容，保留Key别名
export type Key = APIKey;

// Corresponds to the Group struct in Go - 修正版本
export interface Group {
  id: number; // uint -> number
  name: string;
  description: string;
  channel_type: "openai" | "gemini"; // 明确的渠道类型
  is_default?: boolean; // 添加默认分组标识
  config: GroupConfig; // 解析后的配置对象
  api_keys?: APIKey[]; // 关联的API密钥，可选
  created_at: string;
  updated_at: string;
}

// 分组配置结构
export interface GroupConfig {
  upstream_url: string;
  timeout?: number;
  max_tokens?: number;
  [key: string]: any;
}

// 分组请求统计
export interface GroupRequestStat {
  group_name: string;
  request_count: number;
}

// 仪表盘统计数据 - 根据后端DashboardStats修正
export interface DashboardStats {
  total_requests: number; // 对应后端total_requests
  success_requests: number; // 对应后端success_requests
  success_rate: number; // 对应后端success_rate
  group_stats: GroupRequestStat[]; // 对应后端group_stats
  // 前端扩展字段
  total_keys?: number;
  active_keys?: number;
  inactive_keys?: number;
  error_keys?: number;
}

// 请求日志
export interface RequestLog {
  id: string;
  timestamp: string;
  group_id: number; // uint -> number
  key_id: number; // uint -> number
  source_ip: string;
  status_code: number;
  request_path: string;
  request_body_snippet: string;
}

// Corresponds to the SystemSetting struct in Go
export interface SystemSetting {
  id: number;
  setting_key: string;
  setting_value: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface AuthUser {
  key: string;
  isAuthenticated: boolean;
}

export interface User {
  id: string;
  username: string;
}

// Represents a simplified setting for frontend forms
export interface Setting {
  key: string;
  value: string;
}

// Corresponds to the structured system settings
export interface CorsSettings {
  allowed_origins: string;
}

export interface TimeoutSettings {
  read: number;
  write: number;
}

export interface SystemSettings {
  port: number;
  cors: CorsSettings;
  timeout: TimeoutSettings;
}

// A generic type for different setting categories
export type SettingCategory =
  | "system"
  | "auth"
  | "performance"
  | "logs"
  | "group";

// 数据转换适配器 - 兼容旧数据格式
export const adaptLegacyKey = (legacyKey: any): APIKey => ({
  id: Number(legacyKey.id),
  group_id: Number(legacyKey.group_id),
  key_value: legacyKey.api_key || legacyKey.key_value,
  status: legacyKey.is_active ? "active" : "inactive",
  request_count: legacyKey.usage || legacyKey.request_count || 0,
  failure_count: legacyKey.failure_count || 0,
  last_used_at: legacyKey.last_used_at,
  created_at: legacyKey.created_at,
  updated_at: legacyKey.updated_at,
});

export const adaptLegacyGroup = (legacyGroup: any): Group => ({
  id: Number(legacyGroup.id),
  name: legacyGroup.name,
  description: legacyGroup.description,
  channel_type: legacyGroup.channel_type || "openai",
  config:
    typeof legacyGroup.config === "string"
      ? JSON.parse(legacyGroup.config || "{}")
      : legacyGroup.config || {},
  api_keys: legacyGroup.api_keys?.map(adaptLegacyKey),
  created_at: legacyGroup.created_at,
  updated_at: legacyGroup.updated_at,
});

// 工具函数
export const maskKey = (key: string): string => {
  if (!key || key.length < 8) return "****";
  return key.substring(0, 4) + "****" + key.substring(key.length - 4);
};

export const getStatusColor = (status: string): string => {
  switch (status) {
    case "active":
      return "success";
    case "inactive":
      return "warning";
    case "error":
      return "danger";
    default:
      return "info";
  }
};

export const formatNumber = (num: number): string => {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + "M";
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + "K";
  }
  return num.toString();
};
