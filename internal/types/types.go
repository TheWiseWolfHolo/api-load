package types

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	GetAuthConfig() AuthConfig
	GetCORSConfig() CORSConfig
	GetPerformanceConfig() PerformanceConfig
	GetLogConfig() LogConfig
	GetDatabaseConfig() DatabaseConfig
	GetEffectiveServerConfig() ServerConfig
	GetRedisDSN() string
	Validate() error
	DisplayConfig()
	ReloadConfig() error
}

// SystemSettings 定义所有系统配置项
type SystemSettings struct {
	// 基础参数
	AppUrl                  string `json:"app_url" default:"http://localhost:3000" name:"项目地址" category:"基础参数" desc:"项目的基础 URL，用于拼接分组终端节点地址。系统配置优先于环境变量 APP_URL。"`
	RequestLogRetentionDays int    `json:"request_log_retention_days" default:"7" name:"日志保留天数" category:"基础参数" desc:"请求日志在数据库中的保留天数" validate:"min=1"`

	// 服务超时
	ServerReadTimeout             int `json:"server_read_timeout" default:"120" name:"读取超时" category:"服务超时" desc:"HTTP 服务器读取超时时间（秒）" validate:"min=1"`
	ServerWriteTimeout            int `json:"server_write_timeout" default:"1800" name:"写入超时" category:"服务超时" desc:"HTTP 服务器写入超时时间（秒）" validate:"min=1"`
	ServerIdleTimeout             int `json:"server_idle_timeout" default:"120" name:"空闲超时" category:"服务超时" desc:"HTTP 服务器空闲超时时间（秒）" validate:"min=1"`
	ServerGracefulShutdownTimeout int `json:"server_graceful_shutdown_timeout" default:"60" name:"优雅关闭超时" category:"服务超时" desc:"服务优雅关闭的等待超时时间（秒）" validate:"min=1"`

	// 请求超时
	RequestTimeout        int  `json:"request_timeout" default:"600" name:"请求超时" category:"请求超时" desc:"转发请求的完整生命周期超时（秒），包括连接、重试等。" validate:"min=1"`
	ConnectTimeout        int  `json:"connect_timeout" default:"5" name:"连接超时" category:"请求超时" desc:"与上游服务建立新连接的超时时间（秒）。" validate:"min=1"`
	IdleConnTimeout       int  `json:"idle_conn_timeout" default:"120" name:"空闲连接超时" category:"请求超时" desc:"HTTP 客户端中空闲连接的超时时间（秒）。" validate:"min=1"`
	MaxIdleConns          int  `json:"max_idle_conns" default:"100" name:"最大空闲连接数" category:"请求超时" desc:"HTTP 客户端连接池中允许的最大空闲连接总数。" validate:"min=1"`
	MaxIdleConnsPerHost   int  `json:"max_idle_conns_per_host" default:"10" name:"每主机最大空闲连接数" category:"请求超时" desc:"HTTP 客户端连接池对每个上游主机允许的最大空闲连接数。" validate:"min=1"`
	ResponseHeaderTimeout int  `json:"response_header_timeout" default:"120" name:"响应头超时" category:"请求超时" desc:"等待上游服务响应头的最长时间（秒），用于流式请求。" validate:"min=1"`
	DisableCompression    bool `json:"disable_compression" default:"false" name:"禁用压缩" category:"请求超时" desc:"是否禁用对上游请求的传输压缩（Gzip）。对于流式请求建议开启以降低延迟。"`

	// 密钥配置
	MaxRetries                      int `json:"max_retries" default:"3" name:"最大重试次数" category:"密钥配置" desc:"单个请求使用不同 Key 的最大重试次数" validate:"min=0"`
	BlacklistThreshold              int `json:"blacklist_threshold" default:"1" name:"黑名单阈值" category:"密钥配置" desc:"一个 Key 连续失败多少次后进入黑名单" validate:"min=0"`
	KeyValidationIntervalMinutes    int `json:"key_validation_interval_minutes" default:"60" name:"定时验证周期" category:"密钥配置" desc:"后台定时验证密钥的默认周期（分钟）" validate:"min=5"`
	KeyValidationTaskTimeoutMinutes int `json:"key_validation_task_timeout_minutes" default:"60" name:"手动验证超时" category:"密钥配置" desc:"手动触发的全量验证任务的超时时间（分钟）" validate:"min=10"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port                    int    `json:"port"`
	Host                    string `json:"host"`
	ReadTimeout             int    `json:"read_timeout"`
	WriteTimeout            int    `json:"write_timeout"`
	IdleTimeout             int    `json:"idle_timeout"`
	GracefulShutdownTimeout int    `json:"graceful_shutdown_timeout"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Enabled          bool     `json:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
}

// PerformanceConfig represents performance configuration
type PerformanceConfig struct {
	MaxConcurrentRequests int  `json:"max_concurrent_requests"`
	KeyValidationPoolSize int  `json:"key_validation_pool_size"`
	EnableGzip            bool `json:"enable_gzip"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level         string `json:"level"`
	Format        string `json:"format"`
	EnableFile    bool   `json:"enable_file"`
	FilePath      string `json:"file_path"`
	EnableRequest bool   `json:"enable_request"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	DSN string `json:"dsn"`
}

type RetryError struct {
	StatusCode   int    `json:"status_code"`
	ErrorMessage string `json:"error_message"`
	KeyID        string `json:"key_id"`
	Attempt      int    `json:"attempt"`
}
