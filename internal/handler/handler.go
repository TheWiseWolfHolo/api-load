// Package handler provides HTTP handlers for the application
package handler

import (
	"net/http"
	"time"

	"gpt-load/internal/models"
	"gpt-load/internal/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server contains dependencies for HTTP handlers
type Server struct {
	DB     *gorm.DB
	config types.ConfigManager
}

// NewServer creates a new handler instance
func NewServer(db *gorm.DB, config types.ConfigManager) *Server {
	return &Server{
		DB:     db,
		config: config,
	}
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	AuthKey string `json:"auth_key" binding:"required"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Login handles authentication verification
func (s *Server) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format",
		})
		return
	}

	authConfig := s.config.GetAuthConfig()
	
	if !authConfig.Enabled {
		c.JSON(http.StatusOK, LoginResponse{
			Success: true,
			Message: "Authentication disabled",
		})
		return
	}

	if req.AuthKey == authConfig.Key {
		c.JSON(http.StatusOK, LoginResponse{
			Success: true,
			Message: "Authentication successful",
		})
	} else {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Invalid authentication key",
		})
	}
}

// Health handles health check requests
func (s *Server) Health(c *gin.Context) {
	var totalKeys, healthyKeys int64
	s.DB.Model(&models.APIKey{}).Count(&totalKeys)
	s.DB.Model(&models.APIKey{}).Where("status = ?", "active").Count(&healthyKeys)

	status := "healthy"
	httpStatus := http.StatusOK

	// Check if there are any healthy keys
	if healthyKeys == 0 && totalKeys > 0 {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	// Calculate uptime (this should be tracked from server start time)
	uptime := "unknown"
	if startTime, exists := c.Get("serverStartTime"); exists {
		if st, ok := startTime.(time.Time); ok {
			uptime = time.Since(st).String()
		}
	}

	c.JSON(httpStatus, gin.H{
		"status":       status,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"healthy_keys": healthyKeys,
		"total_keys":   totalKeys,
		"uptime":       uptime,
	})
}


// GetConfig returns configuration information (for debugging)
func (s *Server) GetConfig(c *gin.Context) {
	// Only allow in development mode or with special header
	if c.GetHeader("X-Debug-Config") != "true" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	serverConfig := s.config.GetServerConfig()
	openaiConfig := s.config.GetOpenAIConfig()
	authConfig := s.config.GetAuthConfig()
	corsConfig := s.config.GetCORSConfig()
	perfConfig := s.config.GetPerformanceConfig()
	logConfig := s.config.GetLogConfig()

	// Sanitize sensitive information
	sanitizedConfig := gin.H{
		"server": gin.H{
			"host": serverConfig.Host,
			"port": serverConfig.Port,
		},
		"openai": gin.H{
			"base_url":          openaiConfig.BaseURL,
			"request_timeout":   openaiConfig.RequestTimeout,
			"response_timeout":  openaiConfig.ResponseTimeout,
			"idle_conn_timeout": openaiConfig.IdleConnTimeout,
		},
		"auth": gin.H{
			"enabled": authConfig.Enabled,
			// Don't expose the actual key
		},
		"cors": gin.H{
			"enabled":           corsConfig.Enabled,
			"allowed_origins":   corsConfig.AllowedOrigins,
			"allowed_methods":   corsConfig.AllowedMethods,
			"allowed_headers":   corsConfig.AllowedHeaders,
			"allow_credentials": corsConfig.AllowCredentials,
		},
		"performance": gin.H{
			"max_concurrent_requests": perfConfig.MaxConcurrentRequests,
			"enable_gzip":             perfConfig.EnableGzip,
		},
		"timeout_config": gin.H{
			"request_timeout_s":           openaiConfig.RequestTimeout,
			"response_timeout_s":          openaiConfig.ResponseTimeout,
			"idle_conn_timeout_s":         openaiConfig.IdleConnTimeout,
			"server_read_timeout_s":       serverConfig.ReadTimeout,
			"server_write_timeout_s":      serverConfig.WriteTimeout,
			"server_idle_timeout_s":       serverConfig.IdleTimeout,
			"graceful_shutdown_timeout_s": serverConfig.GracefulShutdownTimeout,
		},
		"log": gin.H{
			"level":          logConfig.Level,
			"format":         logConfig.Format,
			"enable_file":    logConfig.EnableFile,
			"file_path":      logConfig.FilePath,
			"enable_request": logConfig.EnableRequest,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, sanitizedConfig)
}
