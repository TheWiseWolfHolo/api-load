// Package config 配置管理模块
// @author OpenAI Proxy Team
// @version 2.0.0
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// Constants 配置常量
type Constants struct {
	MinPort               int
	MaxPort               int
	MinTimeout            int
	DefaultTimeout        int
	DefaultMaxSockets     int
	DefaultMaxFreeSockets int
}

// DefaultConstants 默认常量
var DefaultConstants = Constants{
	MinPort:               1,
	MaxPort:               65535,
	MinTimeout:            1000,
	DefaultTimeout:        30000,
	DefaultMaxSockets:     50,
	DefaultMaxFreeSockets: 10,
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

// KeysConfig 密钥管理配置
type KeysConfig struct {
	FilePath           string `json:"filePath"`
	StartIndex         int    `json:"startIndex"`
	BlacklistThreshold int    `json:"blacklistThreshold"`
	MaxRetries         int    `json:"maxRetries"` // 最大重试次数
}

// OpenAIConfig OpenAI API 配置
type OpenAIConfig struct {
	BaseURL string `json:"baseURL"`
	Timeout int    `json:"timeout"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled        bool     `json:"enabled"`
	AllowedOrigins []string `json:"allowedOrigins"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxSockets          int  `json:"maxSockets"`
	MaxFreeSockets      int  `json:"maxFreeSockets"`
	EnableKeepAlive     bool `json:"enableKeepAlive"`
	DisableCompression  bool `json:"disableCompression"`
	BufferSize          int  `json:"bufferSize"`
	StreamBufferSize    int  `json:"streamBufferSize"`    // 流式传输缓冲区大小
	StreamHeaderTimeout int  `json:"streamHeaderTimeout"` // 流式请求响应头超时（毫秒）
}

// LogConfig 日志配置
type LogConfig struct {
	Level         string `json:"level"`         // debug, info, warn, error
	Format        string `json:"format"`        // text, json
	EnableFile    bool   `json:"enableFile"`    // 是否启用文件日志
	FilePath      string `json:"filePath"`      // 日志文件路径
	EnableRequest bool   `json:"enableRequest"` // 是否启用请求日志
}

// Config 应用配置
type Config struct {
	Server      ServerConfig      `json:"server"`
	Keys        KeysConfig        `json:"keys"`
	OpenAI      OpenAIConfig      `json:"openai"`
	Auth        AuthConfig        `json:"auth"`
	CORS        CORSConfig        `json:"cors"`
	Performance PerformanceConfig `json:"performance"`
	Log         LogConfig         `json:"log"`
}

// Global config instance
var AppConfig *Config

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	// 尝试加载 .env 文件
	if err := godotenv.Load(); err != nil {
		logrus.Info("💡 提示: 创建 .env 文件以支持环境变量配置")
	}

	config := &Config{
		Server: ServerConfig{
			Port: parseInteger(os.Getenv("PORT"), 3000),
			Host: getEnvOrDefault("HOST", "0.0.0.0"),
		},
		Keys: KeysConfig{
			FilePath:           getEnvOrDefault("KEYS_FILE", "keys.txt"),
			StartIndex:         parseInteger(os.Getenv("START_INDEX"), 0),
			BlacklistThreshold: parseInteger(os.Getenv("BLACKLIST_THRESHOLD"), 1),
			MaxRetries:         parseInteger(os.Getenv("MAX_RETRIES"), 3),
		},
		OpenAI: OpenAIConfig{
			BaseURL: getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com"),
			Timeout: parseInteger(os.Getenv("REQUEST_TIMEOUT"), DefaultConstants.DefaultTimeout),
		},
		Auth: AuthConfig{
			Key:     os.Getenv("AUTH_KEY"),
			Enabled: os.Getenv("AUTH_KEY") != "",
		},
		CORS: CORSConfig{
			Enabled:        parseBoolean(os.Getenv("ENABLE_CORS"), true),
			AllowedOrigins: parseArray(os.Getenv("ALLOWED_ORIGINS"), []string{"*"}),
		},
		Performance: PerformanceConfig{
			MaxSockets:          parseInteger(os.Getenv("MAX_SOCKETS"), DefaultConstants.DefaultMaxSockets),
			MaxFreeSockets:      parseInteger(os.Getenv("MAX_FREE_SOCKETS"), DefaultConstants.DefaultMaxFreeSockets),
			EnableKeepAlive:     parseBoolean(os.Getenv("ENABLE_KEEP_ALIVE"), true),
			DisableCompression:  parseBoolean(os.Getenv("DISABLE_COMPRESSION"), true),
			BufferSize:          parseInteger(os.Getenv("BUFFER_SIZE"), 32*1024),
			StreamBufferSize:    parseInteger(os.Getenv("STREAM_BUFFER_SIZE"), 64*1024),      // 默认64KB
			StreamHeaderTimeout: parseInteger(os.Getenv("STREAM_HEADER_TIMEOUT"), 10000),     // 默认10秒
		},
		Log: LogConfig{
			Level:         getEnvOrDefault("LOG_LEVEL", "info"),
			Format:        getEnvOrDefault("LOG_FORMAT", "text"),
			EnableFile:    parseBoolean(os.Getenv("LOG_ENABLE_FILE"), false),
			FilePath:      getEnvOrDefault("LOG_FILE_PATH", "logs/app.log"),
			EnableRequest: parseBoolean(os.Getenv("LOG_ENABLE_REQUEST"), true),
		},
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	AppConfig = config
	return config, nil
}

// validateConfig 验证配置有效性
func validateConfig(config *Config) error {
	var errors []string

	// 验证端口
	if config.Server.Port < DefaultConstants.MinPort || config.Server.Port > DefaultConstants.MaxPort {
		errors = append(errors, fmt.Sprintf("端口号必须在 %d-%d 之间", DefaultConstants.MinPort, DefaultConstants.MaxPort))
	}

	// 验证起始索引
	if config.Keys.StartIndex < 0 {
		errors = append(errors, "起始索引不能小于 0")
	}

	// 验证黑名单阈值
	if config.Keys.BlacklistThreshold < 1 {
		errors = append(errors, "黑名单阈值不能小于 1")
	}

	// 验证超时时间
	if config.OpenAI.Timeout < DefaultConstants.MinTimeout {
		errors = append(errors, fmt.Sprintf("请求超时时间不能小于 %dms", DefaultConstants.MinTimeout))
	}

	// 验证上游URL格式
	if _, err := url.Parse(config.OpenAI.BaseURL); err != nil {
		errors = append(errors, "上游API地址格式无效")
	}

	// 验证性能配置
	if config.Performance.MaxSockets < 1 {
		errors = append(errors, "最大连接数不能小于 1")
	}

	if config.Performance.MaxFreeSockets < 0 {
		errors = append(errors, "最大空闲连接数不能小于 0")
	}

	if config.Performance.StreamBufferSize < 1024 {
		errors = append(errors, "流式缓冲区大小不能小于 1KB")
	}

	if config.Performance.StreamHeaderTimeout < 1000 {
		errors = append(errors, "流式响应头超时不能小于 1秒")
	}

	if len(errors) > 0 {
		logrus.Error("❌ 配置验证失败:")
		for _, err := range errors {
			logrus.Errorf("   - %s", err)
		}
		return fmt.Errorf("配置验证失败")
	}

	return nil
}

// DisplayConfig 显示当前配置信息
func DisplayConfig(config *Config) {
	logrus.Info("⚙️ 当前配置:")
	logrus.Infof("   服务器: %s:%d", config.Server.Host, config.Server.Port)
	logrus.Infof("   密钥文件: %s", config.Keys.FilePath)
	logrus.Infof("   起始索引: %d", config.Keys.StartIndex)
	logrus.Infof("   黑名单阈值: %d 次错误", config.Keys.BlacklistThreshold)
	logrus.Infof("   最大重试次数: %d", config.Keys.MaxRetries)
	logrus.Infof("   上游地址: %s", config.OpenAI.BaseURL)
	logrus.Infof("   请求超时: %dms", config.OpenAI.Timeout)

	authStatus := "未启用"
	if config.Auth.Enabled {
		authStatus = "已启用"
	}
	logrus.Infof("   认证: %s", authStatus)

	corsStatus := "已禁用"
	if config.CORS.Enabled {
		corsStatus = "已启用"
	}
	logrus.Infof("   CORS: %s", corsStatus)
	logrus.Infof("   连接池: %d/%d", config.Performance.MaxSockets, config.Performance.MaxFreeSockets)

	keepAliveStatus := "已启用"
	if !config.Performance.EnableKeepAlive {
		keepAliveStatus = "已禁用"
	}
	logrus.Infof("   Keep-Alive: %s", keepAliveStatus)

	compressionStatus := "已启用"
	if config.Performance.DisableCompression {
		compressionStatus = "已禁用"
	}
	logrus.Infof("   压缩: %s", compressionStatus)
	logrus.Infof("   缓冲区大小: %d bytes", config.Performance.BufferSize)
	logrus.Infof("   流式缓冲区: %d bytes", config.Performance.StreamBufferSize)
	logrus.Infof("   流式响应头超时: %dms", config.Performance.StreamHeaderTimeout)

	// 显示日志配置
	requestLogStatus := "已启用"
	if !config.Log.EnableRequest {
		requestLogStatus = "已禁用"
	}
	logrus.Infof("   请求日志: %s", requestLogStatus)
}

// 辅助函数

// parseInteger 解析整数环境变量
func parseInteger(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return defaultValue
}

// parseBoolean 解析布尔值环境变量
func parseBoolean(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}
	return strings.ToLower(value) == "true"
}

// parseArray 解析数组环境变量（逗号分隔）
func parseArray(value string, defaultValue []string) []string {
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}
	return result
}

// getEnvOrDefault 获取环境变量或默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
