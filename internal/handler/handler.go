// Package handler provides HTTP handlers for the application
package handler

import (
	"net/http"
	"time"

	"gpt-load/internal/config"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/services"
	"gpt-load/internal/types"

	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

// Server contains dependencies for HTTP handlers
type Server struct {
	DB                         *gorm.DB
	config                     types.ConfigManager
	SettingsManager            *config.SystemSettingsManager
	KeyValidator               *keypool.KeyValidator
	KeyManualValidationService *services.KeyManualValidationService
	TaskService                *services.TaskService
	KeyService                 *services.KeyService
	CommonHandler              *CommonHandler
}

// NewServerParams defines the dependencies for the NewServer constructor.
type NewServerParams struct {
	dig.In
	DB                         *gorm.DB
	Config                     types.ConfigManager
	SettingsManager            *config.SystemSettingsManager
	KeyValidator               *keypool.KeyValidator
	KeyManualValidationService *services.KeyManualValidationService
	TaskService                *services.TaskService
	KeyService                 *services.KeyService
	CommonHandler              *CommonHandler
}

// NewServer creates a new handler instance with dependencies injected by dig.
func NewServer(params NewServerParams) *Server {
	return &Server{
		DB:                         params.DB,
		config:                     params.Config,
		SettingsManager:            params.SettingsManager,
		KeyValidator:               params.KeyValidator,
		KeyManualValidationService: params.KeyManualValidationService,
		TaskService:                params.TaskService,
		KeyService:                 params.KeyService,
		CommonHandler:              params.CommonHandler,
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
	s.DB.Model(&models.APIKey{}).Where("status = ?", models.KeyStatusActive).Count(&healthyKeys)

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
