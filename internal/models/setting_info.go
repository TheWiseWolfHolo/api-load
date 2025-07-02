package models

// SystemSettingInfo 表示系统配置的详细信息（用于API返回）
type SystemSettingInfo struct {
	Key          string      `json:"key"`
	Value        interface{} `json:"value"`
	Type         string      `json:"type"` // "int", "bool", "string"
	DefaultValue interface{} `json:"default_value"`
	Description  string      `json:"description"`
	Category     string      `json:"category"` // "timeout", "performance", "logging", etc.
	Required     bool        `json:"required"`
	MinValue     *int        `json:"min_value,omitempty"`
	MaxValue     *int        `json:"max_value,omitempty"`
	ValidOptions []string    `json:"valid_options,omitempty"`
}
