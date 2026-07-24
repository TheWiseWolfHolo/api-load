// 通用 API 响应结构
export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

// 密钥状态
export type KeyStatus = "active" | "invalid" | "disabled" | undefined;

// 分组类型
export type GroupType = "standard" | "aggregate";

export type KeySelectionStrategy = "round_robin" | "random" | "sticky" | "fill_first";

export type KeyAffinityScope = "group" | "model" | "model+proxy_key";

// 渠道类型
export type ChannelType =
  | "openai"
  | "openai-response"
  | "gemini"
  | "anthropic"
  | "openrouter"
  | "deepseek"
  | "qwen"
  | "xai"
  | "azure-openai";

// 数据模型定义
export interface APIKey {
  id: number;
  group_id: number;
  key_value: string;
  notes?: string;
  status: KeyStatus;
  enabled: boolean;
  priority: number;
  weight: number;
  request_count: number;
  total_failure_count: number;
  failure_count: number;
  last_used_at?: string;
  last_success_at?: string;
  last_failure_at?: string;
  created_at: string;
  updated_at: string;
}

export interface UpstreamInfo {
  url: string;
  weight: number;
}

export interface HeaderRule {
  key: string;
  value: string;
  action: "set" | "remove";
}

export interface GroupConfig {
  request_timeout?: number;
  idle_conn_timeout?: number;
  connect_timeout?: number;
  max_idle_conns?: number;
  max_idle_conns_per_host?: number;
  response_header_timeout?: number;
  proxy_url?: string;
  max_retries?: number;
  blacklist_threshold?: number;
  failover_status_codes?: string;
  key_validation_interval_minutes?: number;
  key_validation_concurrency?: number;
  key_validation_timeout_seconds?: number;
  enable_request_body_logging?: boolean;
  key_selection_strategy?: KeySelectionStrategy;
  key_affinity_scope?: KeyAffinityScope;
  fill_cooldown_minutes?: number;
  fill_switch_status_codes?: string;
  fill_quota_patterns?: string;
  fill_max_consecutive_requests?: number;
  fill_max_consecutive_tokens?: number;
  fill_sticky_ttl_seconds?: number;
  auto_restore_schedule?: string;
  auto_restore_status_codes?: string;
}

export interface ModelMappingTarget {
  sub_group_id: number;
  model: string;
  weight: number;
}

export interface ModelMappingRule {
  alias: string;
  targets: ModelMappingTarget[];
}

// 子分组配置（创建/更新时使用）
export interface SubGroupConfig {
  group_id: number;
  weight: number;
}

// 子分组信息（展示时使用）
export interface SubGroupInfo {
  group: Group;
  weight: number;
  total_keys: number;
  active_keys: number;
  invalid_keys: number;
}

// 父聚合分组信息（展示时使用）
export interface ParentAggregateGroup {
  group_id: number;
  name: string;
  display_name: string;
  weight: number;
}

export interface Group {
  id?: number;
  name: string;
  display_name: string;
  description: string;
  sort: number;
  test_model: string;
  channel_type: ChannelType;
  upstreams: UpstreamInfo[];
  validation_endpoint: string;
  config: GroupConfig;
  api_keys?: APIKey[];
  endpoint?: string;
  param_overrides: Record<string, unknown>;
  model_redirect_rules: Record<string, string>;
  model_redirect_strict: boolean;
  models?: string[];
  model_mappings?: ModelMappingRule[];
  header_rules?: HeaderRule[];
  proxy_keys: string;
  group_type?: GroupType;
  resource_pool_id?: number | null;
  sub_groups?: SubGroupInfo[]; // 子分组列表（仅聚合分组）
  sub_group_ids?: number[]; // 子分组ID列表
  created_at?: string;
  updated_at?: string;
}

export interface GroupConfigOption {
  key: string;
  name: string;
  description: string;
  default_value: string | number | boolean;
}

// GroupStatsResponse defines the complete statistics for a group.
export interface GroupStatsResponse {
  key_stats: KeyStats;
  stats_24_hour: RequestStats;
  stats_7_day: RequestStats;
  stats_30_day: RequestStats;
}

// KeyStats defines the statistics for API keys in a group.
export interface KeyStats {
  total_keys: number;
  active_keys: number;
  invalid_keys: number;
  disabled_keys: number;
}

// RequestStats defines the statistics for requests over a period.
export interface RequestStats {
  total_requests: number;
  failed_requests: number;
  failure_rate: number;
}

export type TaskType = "KEY_VALIDATION" | "KEY_IMPORT" | "KEY_DELETE";

export interface KeyValidationResult {
  invalid_keys: number;
  total_keys: number;
  valid_keys: number;
}

export interface KeyImportResult {
  added_count: number;
  ignored_count: number;
  duplicate_count: number;
  updated_count: number;
}

export type ResourceStatus = "active" | "invalid" | "disabled";

export interface UpstreamResource {
  id: number;
  resource_pool_id: number;
  name: string;
  upstream_url: string;
  masked_key: string;
  status: ResourceStatus;
  enabled: boolean;
  priority: number;
  weight: number;
  request_count: number;
  total_failure_count: number;
  failure_count: number;
  global_cooldown_until?: string;
  disabled_reason?: string;
  last_used_at?: string;
  last_success_at?: string;
  last_failure_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ResourcePool {
  id: number;
  name: string;
  description: string;
  strategy: "round_robin";
  affinity_ttl_seconds: number;
  busy_wait_milliseconds: number;
  auto_restore_schedule: string;
  resource_count: number;
  created_at: string;
  updated_at: string;
}

export interface ResourcePoolInput {
  name: string;
  description?: string;
  strategy?: "round_robin";
  affinity_ttl_seconds?: number;
  busy_wait_milliseconds?: number;
  auto_restore_schedule?: string;
}

export interface UpstreamResourceInput {
  name?: string;
  upstream_url: string;
  key: string;
  enabled?: boolean;
  priority?: number;
  weight?: number;
}

export interface UpstreamResourceUpdateInput {
  name: string;
  upstream_url: string;
  key?: string;
  enabled?: boolean;
  status?: Extract<ResourceStatus, "active" | "invalid">;
  priority?: number;
  weight?: number;
}

export interface ResourceListParams {
  page?: number;
  page_size?: number;
  search?: string;
  status?: ResourceStatus | "";
  enabled?: boolean;
}

export interface CredentialBatchUpdateInput {
  enabled?: boolean;
  status?: "active" | "invalid";
  priority?: number;
  weight?: number;
}

export interface ResourceListResponse {
  items: UpstreamResource[];
  pagination: Pagination;
}

export interface ResourceValidationGroup {
  id: number;
  name: string;
  display_name: string;
  channel_type: ChannelType;
  test_model: string;
  validation_endpoint: string;
}

export interface ResourceValidationResult {
  resource_id: number;
  group_id: number;
  group_name: string;
  channel_type: ChannelType;
  is_valid: boolean;
  error?: string;
  duration_ms: number;
}

export interface BulkResourceStatusResult {
  requested_count: number;
  matched_count: number;
  updated_count: number;
}

export interface BulkResourceDeleteResult {
  requested_id_count: number;
  requested_key_count: number;
  matched_count: number;
  deleted_count: number;
  blocked_count: number;
  missing_key_count: number;
}

export interface KeyDeleteResult {
  deleted_count: number;
  ignored_count: number;
}

export interface TaskInfo {
  task_type: TaskType;
  is_running: boolean;
  group_name?: string;
  processed?: number;
  total?: number;
  started_at?: string;
  finished_at?: string;
  result?: KeyValidationResult | KeyImportResult | KeyDeleteResult;
  error?: string;
}

// Based on backend response
export interface RequestLog {
  id: string;
  timestamp: string;
  group_id: number;
  key_id: number;
  is_success: boolean;
  source_ip: string;
  status_code: number;
  request_path: string;
  duration_ms: number;
  error_message: string;
  user_agent: string;
  request_type: "retry" | "final";
  group_name?: string;
  parent_group_name?: string;
  key_value?: string;
  model: string;
  upstream_addr: string;
  is_stream: boolean;
  request_body?: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  thinking_tokens: number;
  token_usage_source: "none" | "upstream" | "estimated";
}

export interface Pagination {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface LogsResponse {
  items: RequestLog[];
  pagination: Pagination;
}

export interface LogFilter {
  page?: number;
  page_size?: number;
  group_name?: string;
  parent_group_name?: string;
  key_value?: string;
  model?: string;
  is_success?: boolean | null;
  status_code?: number | null;
  source_ip?: string;
  error_contains?: string;
  start_time?: string | null;
  end_time?: string | null;
  request_type?: "retry" | "final";
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

// 仪表盘统计卡片数据
export interface StatCard {
  value: number;
  sub_value?: number;
  sub_value_tip?: string;
  trend: number;
  trend_is_growth: boolean;
}

// 安全警告信息
export interface SecurityWarning {
  type: string; // 警告类型：auth_key, encryption_key 等
  message: string; // 警告信息
  severity: string; // 严重程度：low, medium, high
  suggestion: string; // 建议解决方案
}

// 仪表盘基础统计响应
export interface DashboardStatsResponse {
  key_count: StatCard;
  disabled_keys: number;
  rpm: StatCard;
  request_count: StatCard;
  error_rate: StatCard;
  security_warnings: SecurityWarning[];
}

export interface TokenStatsItem {
  dimension: string;
  total_tokens: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  thinking_tokens: number;
}

export interface TokenStatsResponse {
  items: TokenStatsItem[];
}

// 图表数据集
export interface ChartDataset {
  label: string;
  data: number[];
  color: string;
}

// 图表数据
export interface ChartData {
  labels: string[];
  datasets: ChartDataset[];
}
