// Package config provides configuration management for the application
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
	config *Config
}

// Config represents the application configuration
type Config struct {
	Server      types.ServerConfig      `json:"server"`
	OpenAI      types.OpenAIConfig      `json:"openai"`
	Auth        types.AuthConfig        `json:"auth"`
	CORS        types.CORSConfig        `json:"cors"`
	Performance types.PerformanceConfig `json:"performance"`
	Log         types.LogConfig         `json:"log"`
	RedisDSN    string                  `json:"redis_dsn"`
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

	// Get business logic defaults from the single source of truth
	defaultSettings := DefaultSystemSettings()

	config := &Config{
		Server: types.ServerConfig{
			Port: parseInteger(os.Getenv("PORT"), 3000),
			Host: getEnvOrDefault("HOST", "0.0.0.0"),
			// Server timeout configs now come from system settings, not environment
			// Using defaults from SystemSettings struct as the initial value
			ReadTimeout:             defaultSettings.ServerReadTimeout,
			WriteTimeout:            defaultSettings.ServerWriteTimeout,
			IdleTimeout:             defaultSettings.ServerIdleTimeout,
			GracefulShutdownTimeout: defaultSettings.ServerGracefulShutdownTimeout,
		},
		OpenAI: types.OpenAIConfig{
			// BaseURLs will be configured per group
			BaseURLs: []string{},
			// Timeout configs now come from system settings
			RequestTimeout:  defaultSettings.RequestTimeout,
			ResponseTimeout: defaultSettings.ResponseTimeout,
			IdleConnTimeout: defaultSettings.IdleConnTimeout,
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
		RedisDSN: os.Getenv("REDIS_DSN"),
	}
	m.config = config

	// Validate configuration
	if err := m.Validate(); err != nil {
		return err
	}

	logrus.Info("Environment configuration reloaded successfully")

	return nil
}

// GetConfig returns the raw config struct
func (m *Manager) GetConfig() *Config {
	return m.config
}

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

// GetRedisDSN returns the Redis DSN string.
func (m *Manager) GetRedisDSN() string {
	return m.config.RedisDSN
}

// GetEffectiveServerConfig returns server configuration merged with system settings
func (m *Manager) GetEffectiveServerConfig() types.ServerConfig {
	config := m.config.Server

	// Merge with system settings from database
	settingsManager := GetSystemSettingsManager()
	systemSettings := settingsManager.GetSettings()

	config.ReadTimeout = systemSettings.ServerReadTimeout
	config.WriteTimeout = systemSettings.ServerWriteTimeout
	config.IdleTimeout = systemSettings.ServerIdleTimeout
	config.GracefulShutdownTimeout = systemSettings.ServerGracefulShutdownTimeout

	return config
}

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
		return errors.NewAPIError(errors.ErrValidation, strings.Join(validationErrors, "; "))
	}

	return nil
}

// DisplayConfig displays current configuration information
func (m *Manager) DisplayConfig() {
	serverConfig := m.GetEffectiveServerConfig()
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

// GetInt is a helper function for SystemSettingsManager to get an integer value with a default.
func (s *SystemSettingsManager) GetInt(key string, defaultValue int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if valStr, ok := s.settingsCache[key]; ok {
		if valInt, err := strconv.Atoi(valStr); err == nil {
			return valInt
		}
	}
	return defaultValue
}
