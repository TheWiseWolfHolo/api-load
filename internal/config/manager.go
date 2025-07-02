// Package config provides configuration management for the application
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"gpt-load/internal/errors"
	"gpt-load/internal/types"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// Constants represents configuration constants
type Constants struct {
	MinPort               int
	MaxPort               int
	MinTimeout            int
	DefaultTimeout        int
	DefaultMaxSockets     int
	DefaultMaxFreeSockets int
}

// DefaultConstants holds default configuration values
var DefaultConstants = Constants{
	MinPort:               1,
	MaxPort:               65535,
	MinTimeout:            1,
	DefaultTimeout:        30,
	DefaultMaxSockets:     50,
	DefaultMaxFreeSockets: 10,
}

// Manager implements the ConfigManager interface
type Manager struct {
	config            *Config
	roundRobinCounter uint64
}

// Config represents the application configuration
type Config struct {
	Server      types.ServerConfig      `json:"server"`
	OpenAI      types.OpenAIConfig      `json:"openai"`
	Auth        types.AuthConfig        `json:"auth"`
	CORS        types.CORSConfig        `json:"cors"`
	Performance types.PerformanceConfig `json:"performance"`
	Log         types.LogConfig         `json:"log"`
}

// NewManager creates a new configuration manager
func NewManager() (types.ConfigManager, error) {
	manager := &Manager{}
	if err := manager.ReloadConfig(); err != nil {
		return nil, err
	}
	return manager, nil
}

// ReloadConfig reloads the configuration from environment variables
func (m *Manager) ReloadConfig() error {
	// Try to load .env file
	if err := godotenv.Load(); err != nil {
		logrus.Info("Info: Create .env file to support environment variable configuration")
	}

	config := &Config{
		Server: types.ServerConfig{
			Port: parseInteger(os.Getenv("PORT"), 3000),
			Host: getEnvOrDefault("HOST", "0.0.0.0"),
			// Server timeout configs now come from system settings, not environment
			// Using defaults here, will be overridden by system settings
			ReadTimeout:             120,
			WriteTimeout:            1800,
			IdleTimeout:             120,
			GracefulShutdownTimeout: 60,
		},
		OpenAI: types.OpenAIConfig{
			// OPENAI_BASE_URL is removed from environment config
			// Base URLs will be configured per group
			BaseURLs: []string{}, // Will be set per group
			// Timeout configs now come from system settings
			RequestTimeout:  30,
			ResponseTimeout: 30,
			IdleConnTimeout: 120,
		},
		Auth: types.AuthConfig{
			Key:     os.Getenv("AUTH_KEY"),
			Enabled: os.Getenv("AUTH_KEY") != "",
		},
		CORS: types.CORSConfig{
			Enabled:          parseBoolean(os.Getenv("ENABLE_CORS"), true),
			AllowedOrigins:   parseArray(os.Getenv("ALLOWED_ORIGINS"), []string{"*"}),
			AllowedMethods:   parseArray(os.Getenv("ALLOWED_METHODS"), []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			AllowedHeaders:   parseArray(os.Getenv("ALLOWED_HEADERS"), []string{"*"}),
			AllowCredentials: parseBoolean(os.Getenv("ALLOW_CREDENTIALS"), false),
		},
		Performance: types.PerformanceConfig{
			MaxConcurrentRequests: parseInteger(os.Getenv("MAX_CONCURRENT_REQUESTS"), 100),
			EnableGzip:            parseBoolean(os.Getenv("ENABLE_GZIP"), true),
		},
		Log: types.LogConfig{
			Level:         getEnvOrDefault("LOG_LEVEL", "info"),
			Format:        getEnvOrDefault("LOG_FORMAT", "text"),
			EnableFile:    parseBoolean(os.Getenv("LOG_ENABLE_FILE"), false),
			FilePath:      getEnvOrDefault("LOG_FILE_PATH", "logs/app.log"),
			EnableRequest: parseBoolean(os.Getenv("LOG_ENABLE_REQUEST"), true),
		},
	}
	m.config = config

	// Validate configuration
	if err := m.Validate(); err != nil {
		return err
	}

	logrus.Info("Environment configuration reloaded successfully")

	return nil
}

// GetServerConfig returns server configuration
// func (m *Manager) GetServerConfig() types.ServerConfig {
// 	return m.config.Server
// }

// GetOpenAIConfig returns OpenAI configuration
// func (m *Manager) GetOpenAIConfig() types.OpenAIConfig {
// 	config := m.config.OpenAI
// 	if len(config.BaseURLs) > 1 {
// 		// Use atomic counter for thread-safe round-robin
// 		index := atomic.AddUint64(&m.roundRobinCounter, 1) - 1
// 		config.BaseURL = config.BaseURLs[index%uint64(len(config.BaseURLs))]
// 	} else if len(config.BaseURLs) == 1 {
// 		config.BaseURL = config.BaseURLs[0]
// 	}
// 	return config
// }

// GetAuthConfig returns authentication configuration
func (m *Manager) GetAuthConfig() types.AuthConfig {
	return m.config.Auth
}

// GetCORSConfig returns CORS configuration
func (m *Manager) GetCORSConfig() types.CORSConfig {
	return m.config.CORS
}

// GetPerformanceConfig returns performance configuration
func (m *Manager) GetPerformanceConfig() types.PerformanceConfig {
	return m.config.Performance
}

// GetLogConfig returns logging configuration
func (m *Manager) GetLogConfig() types.LogConfig {
	return m.config.Log
}

// GetEffectiveServerConfig returns server configuration merged with system settings
func (m *Manager) GetEffectiveServerConfig() types.ServerConfig {
	config := m.config.Server

	// Merge with system settings
	settingsManager := GetSystemSettingsManager()
	systemSettings := settingsManager.GetSettings()

	config.ReadTimeout = systemSettings.ServerReadTimeout
	config.WriteTimeout = systemSettings.ServerWriteTimeout
	config.IdleTimeout = systemSettings.ServerIdleTimeout
	config.GracefulShutdownTimeout = systemSettings.ServerGracefulShutdownTimeout

	return config
}

// GetEffectiveOpenAIConfig returns OpenAI configuration merged with system settings and group config
func (m *Manager) GetEffectiveOpenAIConfig(groupConfig map[string]any) types.OpenAIConfig {
	config := m.config.OpenAI

	// Merge with system settings
	settingsManager := GetSystemSettingsManager()
	effectiveSettings := settingsManager.GetEffectiveConfig(groupConfig)

	config.RequestTimeout = effectiveSettings.RequestTimeout
	config.ResponseTimeout = effectiveSettings.ResponseTimeout
	config.IdleConnTimeout = effectiveSettings.IdleConnTimeout

	// Apply round-robin for multiple URLs if configured
	if len(config.BaseURLs) > 1 {
		index := atomic.AddUint64(&m.roundRobinCounter, 1) - 1
		config.BaseURL = config.BaseURLs[index%uint64(len(config.BaseURLs))]
	} else if len(config.BaseURLs) == 1 {
		config.BaseURL = config.BaseURLs[0]
	}

	return config
}

// GetEffectiveLogConfig returns log configuration (now uses environment config only)
// func (m *Manager) GetEffectiveLogConfig() types.LogConfig {
// 	// Log configuration is now managed via environment variables only
// 	return m.config.Log
// }

// Validate validates the configuration
func (m *Manager) Validate() error {
	var validationErrors []string

	// Validate port
	if m.config.Server.Port < DefaultConstants.MinPort || m.config.Server.Port > DefaultConstants.MaxPort {
		validationErrors = append(validationErrors, fmt.Sprintf("port must be between %d-%d", DefaultConstants.MinPort, DefaultConstants.MaxPort))
	}

	if m.config.Performance.MaxConcurrentRequests < 1 {
		validationErrors = append(validationErrors, "max concurrent requests cannot be less than 1")
	}

	if len(validationErrors) > 0 {
		logrus.Error("Configuration validation failed:")
		for _, err := range validationErrors {
			logrus.Errorf("   - %s", err)
		}
		return errors.NewAppErrorWithDetails(errors.ErrConfigValidation, "Configuration validation failed", strings.Join(validationErrors, "; "))
	}

	return nil
}

// DisplayConfig displays current configuration information
func (m *Manager) DisplayConfig() {
	serverConfig := m.GetEffectiveServerConfig()
	// openaiConfig := m.GetOpenAIConfig()
	authConfig := m.GetAuthConfig()
	corsConfig := m.GetCORSConfig()
	perfConfig := m.GetPerformanceConfig()
	logConfig := m.GetLogConfig()

	logrus.Info("Current Configuration:")
	logrus.Infof("   Server: %s:%d", serverConfig.Host, serverConfig.Port)

	authStatus := "disabled"
	if authConfig.Enabled {
		authStatus = "enabled"
	}
	logrus.Infof("   Authentication: %s", authStatus)

	corsStatus := "disabled"
	if corsConfig.Enabled {
		corsStatus = "enabled"
	}
	logrus.Infof("   CORS: %s", corsStatus)
	logrus.Infof("   Max concurrent requests: %d", perfConfig.MaxConcurrentRequests)

	gzipStatus := "disabled"
	if perfConfig.EnableGzip {
		gzipStatus = "enabled"
	}
	logrus.Infof("   Gzip compression: %s", gzipStatus)

	requestLogStatus := "enabled"
	if !logConfig.EnableRequest {
		requestLogStatus = "disabled"
	}
	logrus.Infof("   Request logging: %s", requestLogStatus)
}

// Helper functions

// parseInteger parses integer environment variable
func parseInteger(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return defaultValue
}

// parseBoolean parses boolean environment variable
func parseBoolean(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}

	lowerValue := strings.ToLower(value)
	switch lowerValue {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultValue
	}
}

// parseArray parses array environment variable (comma-separated)
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

// getEnvOrDefault gets environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
