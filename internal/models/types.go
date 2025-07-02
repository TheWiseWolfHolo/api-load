package models

import (
	"time"
)

// SystemSetting 对应 system_settings 表
type SystemSetting struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SettingKey   string    `gorm:"type:varchar(255);not null;unique" json:"setting_key"`
	SettingValue string    `gorm:"type:text;not null" json:"setting_value"`
	Description  string    `gorm:"type:varchar(512)" json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Group 对应 groups 表
type Group struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null;unique" json:"name"`
	Description string    `gorm:"type:varchar(512)" json:"description"`
	ChannelType string    `gorm:"type:varchar(50);not null" json:"channel_type"`
	Config      string    `gorm:"type:json" json:"config"`
	APIKeys     []APIKey  `gorm:"foreignKey:GroupID" json:"api_keys"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// APIKey 对应 api_keys 表
type APIKey struct {
	ID           uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID      uint       `gorm:"not null" json:"group_id"`
	KeyValue     string     `gorm:"type:varchar(512);not null" json:"key_value"`
	Status       string     `gorm:"type:varchar(50);not null;default:'active'" json:"status"`
	RequestCount int64      `gorm:"not null;default:0" json:"request_count"`
	FailureCount int64      `gorm:"not null;default:0" json:"failure_count"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// RequestLog 对应 request_logs 表
type RequestLog struct {
	ID                 string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Timestamp          time.Time `gorm:"type:datetime(3);not null" json:"timestamp"`
	GroupID            uint      `gorm:"not null" json:"group_id"`
	KeyID              uint      `gorm:"not null" json:"key_id"`
	SourceIP           string    `gorm:"type:varchar(45)" json:"source_ip"`
	StatusCode         int       `gorm:"not null" json:"status_code"`
	RequestPath        string    `gorm:"type:varchar(1024)" json:"request_path"`
	RequestBodySnippet string    `gorm:"type:text" json:"request_body_snippet"`
}

// GroupRequestStat 用于表示每个分组的请求统计
type GroupRequestStat struct {
	GroupName    string `json:"group_name"`
	RequestCount int64  `json:"request_count"`
}

// DashboardStats 用于仪表盘的统计数据
type DashboardStats struct {
	TotalRequests   int64              `json:"total_requests"`
	SuccessRequests int64              `json:"success_requests"`
	SuccessRate     float64            `json:"success_rate"`
	GroupStats      []GroupRequestStat `json:"group_stats"`
}
