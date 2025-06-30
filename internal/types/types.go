// Package types defines common interfaces and types used across the application
package types

import (
	"github.com/gin-gonic/gin"
)

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	GetServerConfig() ServerConfig
	GetOpenAIConfig() OpenAIConfig
	GetAuthConfig() AuthConfig
	GetCORSConfig() CORSConfig
	GetPerformanceConfig() PerformanceConfig
	GetLogConfig() LogConfig
	Validate() error
	DisplayConfig()
	ReloadConfig() error
}

// ProxyServer defines the interface for proxy server
type ProxyServer interface {
	HandleProxy(c *gin.Context)
	Close()
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port                    int    `json:"port"`
	Host                    string `json:"host"`
	ReadTimeout             int    `json:"readTimeout"`
	WriteTimeout            int    `json:"writeTimeout"`
	IdleTimeout             int    `json:"idleTimeout"`
	GracefulShutdownTimeout int    `json:"gracefulShutdownTimeout"`
}

// OpenAIConfig represents OpenAI API configuration
type OpenAIConfig struct {
	BaseURL         string   `json:"baseUrl"`
	BaseURLs        []string `json:"baseUrls"`
	RequestTimeout  int      `json:"requestTimeout"`
	ResponseTimeout int      `json:"responseTimeout"`
	IdleConnTimeout int      `json:"idleConnTimeout"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Enabled          bool     `json:"enabled"`
	AllowedOrigins   []string `json:"allowedOrigins"`
	AllowedMethods   []string `json:"allowedMethods"`
	AllowedHeaders   []string `json:"allowedHeaders"`
	AllowCredentials bool     `json:"allowCredentials"`
}

// PerformanceConfig represents performance configuration
type PerformanceConfig struct {
	MaxConcurrentRequests int  `json:"maxConcurrentRequests"`
	EnableGzip            bool `json:"enableGzip"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level         string `json:"level"`
	Format        string `json:"format"`
	EnableFile    bool   `json:"enableFile"`
	FilePath      string `json:"filePath"`
	EnableRequest bool   `json:"enableRequest"`
}
