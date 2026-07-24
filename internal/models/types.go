package models

import (
	"api-load/internal/failover"
	"api-load/internal/types"
	"time"

	"gorm.io/datatypes"
)

// Key状态
const (
	KeyStatusActive   = "active"
	KeyStatusInvalid  = "invalid"
	KeyStatusDisabled = "disabled"

	ResourceStatusActive   = "active"
	ResourceStatusInvalid  = "invalid"
	ResourceStatusDisabled = "disabled"

	DefaultCredentialPriority = 10
	DefaultCredentialWeight   = 1
)

func Bool(value bool) *bool { return &value }

func CredentialEnabled(value *bool) bool {
	return value == nil || *value
}

// SystemSetting 对应 system_settings 表
type SystemSetting struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SettingKey   string    `gorm:"type:varchar(255);not null;unique" json:"setting_key"`
	SettingValue string    `gorm:"type:text;not null" json:"setting_value"`
	Description  string    `gorm:"type:varchar(512)" json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GroupConfig 存储特定于分组的配置
type GroupConfig struct {
	RequestTimeout               *int    `json:"request_timeout,omitempty"`
	IdleConnTimeout              *int    `json:"idle_conn_timeout,omitempty"`
	ConnectTimeout               *int    `json:"connect_timeout,omitempty"`
	MaxIdleConns                 *int    `json:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost          *int    `json:"max_idle_conns_per_host,omitempty"`
	ResponseHeaderTimeout        *int    `json:"response_header_timeout,omitempty"`
	ProxyURL                     *string `json:"proxy_url,omitempty"`
	MaxRetries                   *int    `json:"max_retries,omitempty"`
	BlacklistThreshold           *int    `json:"blacklist_threshold,omitempty"`
	FailoverStatusCodes          *string `json:"failover_status_codes,omitempty"`
	KeyValidationIntervalMinutes *int    `json:"key_validation_interval_minutes,omitempty"`
	KeyValidationConcurrency     *int    `json:"key_validation_concurrency,omitempty"`
	KeyValidationTimeoutSeconds  *int    `json:"key_validation_timeout_seconds,omitempty"`
	EnableRequestBodyLogging     *bool   `json:"enable_request_body_logging,omitempty"`
	KeySelectionStrategy         *string `json:"key_selection_strategy,omitempty"`
	KeyAffinityScope             *string `json:"key_affinity_scope,omitempty"`
	FillCooldownMinutes          *int    `json:"fill_cooldown_minutes,omitempty"`
	FillSwitchStatusCodes        *string `json:"fill_switch_status_codes,omitempty"`
	FillQuotaPatterns            *string `json:"fill_quota_patterns,omitempty"`
	FillMaxConsecutiveRequests   *int    `json:"fill_max_consecutive_requests,omitempty"`
	FillMaxConsecutiveTokens     *int    `json:"fill_max_consecutive_tokens,omitempty"`
	FillStickyTTLSeconds         *int    `json:"fill_sticky_ttl_seconds,omitempty"`
	AutoRestoreSchedule          *string `json:"auto_restore_schedule,omitempty"`
	AutoRestoreStatusCodes       *string `json:"auto_restore_status_codes,omitempty"`
}

// HeaderRule defines a single rule for header manipulation.
type HeaderRule struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Action string `json:"action"` // "set" or "remove"
}

// GroupSubGroup 聚合分组和子分组的关联表
type GroupSubGroup struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID    uint      `gorm:"not null;uniqueIndex:idx_group_sub" json:"group_id"`
	SubGroupID uint      `gorm:"not null;uniqueIndex:idx_group_sub" json:"sub_group_id"`
	Weight     int       `gorm:"default:0" json:"weight"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Lightweight association - only store necessary info for performance
	SubGroupName string `gorm:"-" json:"sub_group_name,omitempty"`
}

// SubGroupInfo 用于API响应的子分组信息
type SubGroupInfo struct {
	Group       Group `json:"group"`
	Weight      int   `json:"weight"`
	TotalKeys   int64 `json:"total_keys"`
	ActiveKeys  int64 `json:"active_keys"`
	InvalidKeys int64 `json:"invalid_keys"`
}

// ParentAggregateGroupInfo 用于API响应的父聚合分组信息
type ParentAggregateGroupInfo struct {
	GroupID     uint   `json:"group_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Weight      int    `json:"weight"`
}

// Group 对应 groups 表
type Group struct {
	ID                  uint                 `gorm:"primaryKey;autoIncrement" json:"id"`
	EffectiveConfig     types.SystemSettings `gorm:"-" json:"effective_config,omitempty"`
	Name                string               `gorm:"type:varchar(255);not null;unique" json:"name"`
	Endpoint            string               `gorm:"-" json:"endpoint"`
	DisplayName         string               `gorm:"type:varchar(255)" json:"display_name"`
	ProxyKeys           string               `gorm:"type:text" json:"proxy_keys"`
	Description         string               `gorm:"type:varchar(512)" json:"description"`
	GroupType           string               `gorm:"type:varchar(50);default:'standard'" json:"group_type"` // 'standard' or 'aggregate'
	ResourcePoolID      *uint                `gorm:"index" json:"resource_pool_id,omitempty"`
	ResourceEndpointID  *uint                `gorm:"index" json:"resource_endpoint_id,omitempty"`
	Upstreams           datatypes.JSON       `gorm:"type:json;not null" json:"upstreams"`
	ValidationEndpoint  string               `gorm:"type:varchar(255)" json:"validation_endpoint"`
	ChannelType         string               `gorm:"type:varchar(50);not null" json:"channel_type"`
	Sort                int                  `gorm:"default:0" json:"sort"`
	TestModel           string               `gorm:"type:varchar(255);not null" json:"test_model"`
	ParamOverrides      datatypes.JSONMap    `gorm:"type:json" json:"param_overrides"`
	Config              datatypes.JSONMap    `gorm:"type:json" json:"config"`
	HeaderRules         datatypes.JSON       `gorm:"type:json" json:"header_rules"`
	ModelRedirectRules  datatypes.JSONMap    `gorm:"type:json" json:"model_redirect_rules"`
	ModelRedirectStrict bool                 `gorm:"default:false" json:"model_redirect_strict"`
	Models              datatypes.JSON       `gorm:"type:json" json:"models"`
	ModelMappings       datatypes.JSON       `gorm:"type:json" json:"model_mappings"`
	APIKeys             []APIKey             `gorm:"foreignKey:GroupID" json:"api_keys"`
	SubGroups           []GroupSubGroup      `gorm:"-" json:"sub_groups,omitempty"`
	LastValidatedAt     *time.Time           `json:"last_validated_at"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`

	// For cache
	ProxyKeysMap              map[string]struct{}        `gorm:"-" json:"-"`
	HeaderRuleList            []HeaderRule               `gorm:"-" json:"-"`
	ModelRedirectMap          map[string]string          `gorm:"-" json:"-"`
	FailoverStatusCodeMatcher failover.StatusCodeMatcher `gorm:"-" json:"-"`
}

// ResourcePool owns credentials and their shared health/scheduling state.
// Protocol-specific upstream addresses are represented by ResourcePoolEndpoint.
type ResourcePool struct {
	ID                   uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                 string `gorm:"type:varchar(255);not null;unique" json:"name"`
	Description          string `gorm:"type:varchar(512)" json:"description"`
	Strategy             string `gorm:"type:varchar(50);not null;default:'round_robin'" json:"strategy"`
	AffinityTTLSeconds   int    `gorm:"not null;default:3600" json:"affinity_ttl_seconds"`
	BusyWaitMilliseconds int    `gorm:"not null;default:2000" json:"busy_wait_milliseconds"`
	// AutoRestoreSchedule 控制配额/账单类失败资源的自动恢复;空串(默认)表示
	// 不自动恢复,此类失败直接标记 invalid,语法与分组 auto_restore_schedule 一致。
	AutoRestoreSchedule string                 `gorm:"type:varchar(64);not null;default:''" json:"auto_restore_schedule"`
	Resources           []UpstreamResource     `gorm:"foreignKey:ResourcePoolID" json:"resources,omitempty"`
	Endpoints           []ResourcePoolEndpoint `gorm:"foreignKey:ResourcePoolID" json:"endpoints,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// ResourcePoolEndpoint is a protocol-specific address through which a group's
// requests use the credentials owned by the containing resource pool.
type ResourcePoolEndpoint struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ResourcePoolID uint      `gorm:"not null;index;uniqueIndex:idx_pool_endpoint_name;uniqueIndex:idx_pool_endpoint_identity" json:"resource_pool_id"`
	Name           string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_pool_endpoint_name" json:"name"`
	ChannelType    string    `gorm:"type:varchar(50);not null;index;uniqueIndex:idx_pool_endpoint_identity" json:"channel_type"`
	BaseURL        string    `gorm:"type:varchar(1000);not null;uniqueIndex:idx_pool_endpoint_identity" json:"base_url"`
	Enabled        *bool     `gorm:"not null;default:true;index" json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UpstreamResource is the scheduling unit for one credential. The deprecated
// UpstreamURL column remains only so existing databases and imports can migrate
// without data loss; request routing uses ResourcePoolEndpoint.BaseURL.
type UpstreamResource struct {
	ID                  uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	ResourcePoolID      uint       `gorm:"not null;index;uniqueIndex:idx_resource_pool_identity" json:"resource_pool_id"`
	Name                string     `gorm:"type:varchar(255)" json:"name"`
	UpstreamURL         string     `gorm:"type:varchar(1000);not null;default:''" json:"upstream_url,omitempty"`
	KeyValue            string     `gorm:"type:text;not null" json:"key_value"`
	KeyHash             string     `gorm:"type:varchar(128);not null;index" json:"key_hash"`
	IdentityHash        string     `gorm:"type:varchar(128);not null;uniqueIndex:idx_resource_pool_identity" json:"identity_hash"`
	Enabled             *bool      `gorm:"not null;default:true;index" json:"enabled"`
	Status              string     `gorm:"type:varchar(50);not null;default:'active';index" json:"status"`
	Priority            int        `gorm:"not null;default:10;index" json:"priority"`
	Weight              int        `gorm:"not null;default:1" json:"weight"`
	RequestCount        int64      `gorm:"not null;default:0" json:"request_count"`
	TotalFailureCount   int64      `gorm:"not null;default:0" json:"total_failure_count"`
	FailureCount        int64      `gorm:"not null;default:0" json:"failure_count"`
	GlobalCooldownUntil *time.Time `gorm:"index" json:"global_cooldown_until,omitempty"`
	DisabledReason      string     `gorm:"type:varchar(512)" json:"disabled_reason,omitempty"`
	LastUsedAt          *time.Time `gorm:"index" json:"last_used_at,omitempty"`
	LastSuccessAt       *time.Time `gorm:"index" json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `gorm:"index" json:"last_failure_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

const (
	UpstreamObjectTypeBatch = "batch"
	UpstreamObjectTypeFile  = "file"
)

// UpstreamObjectBinding keeps account-scoped OpenAI objects on the physical
// resource that created them. Batch and file identifiers are not portable
// across official upstream credentials.
type UpstreamObjectBinding struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID            uint      `gorm:"not null;index;uniqueIndex:idx_upstream_object_owner" json:"group_id"`
	ResourcePoolID     uint      `gorm:"not null;index" json:"resource_pool_id"`
	ResourceEndpointID uint      `gorm:"not null;default:0;index" json:"resource_endpoint_id"`
	ResourceID         uint      `gorm:"not null;index" json:"resource_id"`
	ObjectType         string    `gorm:"type:varchar(32);not null;uniqueIndex:idx_upstream_object_owner" json:"object_type"`
	ObjectID           string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_upstream_object_owner" json:"object_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// APIKey 对应 api_keys 表
type APIKey struct {
	ID                uint   `gorm:"primaryKey;autoIncrement;index:idx_api_keys_group_last_used_id,priority:3" json:"id"`
	KeyValue          string `gorm:"type:text;not null" json:"key_value"`
	KeyHash           string `gorm:"type:varchar(128);index" json:"key_hash"`
	GroupID           uint   `gorm:"not null;index;index:idx_api_keys_group_last_used_id,priority:1" json:"group_id"`
	Enabled           *bool  `gorm:"not null;default:true;index" json:"enabled"`
	Status            string `gorm:"type:varchar(50);not null;default:'active';index" json:"status"`
	Priority          int    `gorm:"not null;default:10;index" json:"priority"`
	Weight            int    `gorm:"not null;default:1" json:"weight"`
	Notes             string `gorm:"type:varchar(255);default:''" json:"notes"`
	RequestCount      int64  `gorm:"not null;default:0" json:"request_count"`
	TotalFailureCount int64  `gorm:"not null;default:0" json:"total_failure_count"`
	FailureCount      int64  `gorm:"not null;default:0" json:"failure_count"`
	// LastFailureStatusCode 记录最近一次失败的上游状态码,用于自动恢复的原因过滤
	LastFailureStatusCode int        `gorm:"not null;default:0" json:"last_failure_status_code"`
	CooldownUntil         *time.Time `gorm:"index" json:"cooldown_until,omitempty"`
	LastUsedAt            *time.Time `gorm:"index:idx_api_keys_group_last_used_id,priority:2" json:"last_used_at"`
	LastSuccessAt         *time.Time `gorm:"index" json:"last_success_at"`
	LastFailureAt         *time.Time `gorm:"index" json:"last_failure_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// RequestType 请求类型常量
const (
	RequestTypeRetry = "retry"
	RequestTypeFinal = "final"
)

// RequestLog 对应 request_logs 表
type RequestLog struct {
	ID                string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Timestamp         time.Time `gorm:"not null;index" json:"timestamp"`
	GroupID           uint      `gorm:"not null;index" json:"group_id"`
	ResourceID        uint      `gorm:"index" json:"resource_id,omitempty"`
	GroupName         string    `gorm:"type:varchar(255);index" json:"group_name"`
	ParentGroupID     uint      `gorm:"index" json:"parent_group_id"`
	ParentGroupName   string    `gorm:"type:varchar(255);index" json:"parent_group_name"`
	KeyValue          string    `gorm:"type:text" json:"key_value"`
	KeyHash           string    `gorm:"type:varchar(128);index" json:"key_hash"`
	Model             string    `gorm:"type:varchar(255);index" json:"model"`
	IsSuccess         bool      `gorm:"not null" json:"is_success"`
	SourceIP          string    `gorm:"type:varchar(64)" json:"source_ip"`
	StatusCode        int       `gorm:"not null" json:"status_code"`
	RequestPath       string    `gorm:"type:varchar(500)" json:"request_path"`
	Duration          int64     `gorm:"not null" json:"duration_ms"`
	ErrorMessage      string    `gorm:"type:text" json:"error_message"`
	UserAgent         string    `gorm:"type:varchar(512)" json:"user_agent"`
	RequestType       string    `gorm:"type:varchar(20);not null;default:'final';index" json:"request_type"`
	UpstreamAddr      string    `gorm:"type:varchar(500)" json:"upstream_addr"`
	IsStream          bool      `gorm:"not null" json:"is_stream"`
	RequestBody       string    `gorm:"type:text" json:"request_body"`
	IsSecurityWarning bool      `gorm:"not null;default:false" json:"is_security_warning"`
	InputTokens       int64     `gorm:"not null;default:0" json:"input_tokens"`
	OutputTokens      int64     `gorm:"not null;default:0" json:"output_tokens"`
	TotalTokens       int64     `gorm:"not null;default:0" json:"total_tokens"`
	CacheReadTokens   int64     `gorm:"not null;default:0" json:"cache_read_tokens"`
	CacheWriteTokens  int64     `gorm:"not null;default:0" json:"cache_write_tokens"`
	ThinkingTokens    int64     `gorm:"not null;default:0" json:"thinking_tokens"`
	TokenUsageSource  string    `gorm:"type:varchar(32);not null;default:'none';index" json:"token_usage_source"`
}

const (
	TokenUsageSourceNone      = "none"
	TokenUsageSourceUpstream  = "upstream"
	TokenUsageSourceEstimated = "estimated"
)

// StatCard 用于仪表盘的单个统计卡片数据
type StatCard struct {
	Value         float64 `json:"value"`
	SubValue      int64   `json:"sub_value,omitempty"`
	SubValueTip   string  `json:"sub_value_tip,omitempty"`
	Trend         float64 `json:"trend"`
	TrendIsGrowth bool    `json:"trend_is_growth"`
}

// SecurityWarning 用于安全警告信息
type SecurityWarning struct {
	Type       string `json:"type"`       // 警告类型：auth_key, encryption_key 等
	Message    string `json:"message"`    // 警告信息
	Severity   string `json:"severity"`   // 严重程度：low, medium, high
	Suggestion string `json:"suggestion"` // 建议解决方案
}

// DashboardStatsResponse 用于仪表盘基础统计的API响应
type DashboardStatsResponse struct {
	KeyCount         StatCard          `json:"key_count"`
	DisabledKeys     int64             `json:"disabled_keys"`
	RPM              StatCard          `json:"rpm"`
	RequestCount     StatCard          `json:"request_count"`
	ErrorRate        StatCard          `json:"error_rate"`
	SecurityWarnings []SecurityWarning `json:"security_warnings"`
}

// ChartDataset 用于图表的数据集
type ChartDataset struct {
	Label string  `json:"label"`
	Data  []int64 `json:"data"`
	Color string  `json:"color"`
}

// ChartData 用于图表的API响应
type ChartData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

// GroupHourlyStat 对应 group_hourly_stats 表，用于存储每个分组每小时的请求统计
type GroupHourlyStat struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Time         time.Time `gorm:"not null;uniqueIndex:idx_group_time" json:"time"` // 整点时间
	GroupID      uint      `gorm:"not null;uniqueIndex:idx_group_time" json:"group_id"`
	SuccessCount int64     `gorm:"not null;default:0" json:"success_count"`
	FailureCount int64     `gorm:"not null;default:0" json:"failure_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
