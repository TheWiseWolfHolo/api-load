// Package handler provides HTTP handlers for the application
package handler

import (
	"net/http"
	"time"

	"gpt-load/internal/models"
	"gpt-load/internal/services"
	"gpt-load/internal/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server contains dependencies for HTTP handlers
type Server struct {
	DB                         *gorm.DB
	config                     types.ConfigManager
	KeyValidatorService        *services.KeyValidatorService
	KeyManualValidationService *services.KeyManualValidationService
	TaskService                *services.TaskService
	KeyService                 *services.KeyService
}

// NewServer creates a new handler instance
func NewServer(
	db *gorm.DB,
	config types.ConfigManager,
	keyValidatorService *services.KeyValidatorService,
	keyManualValidationService *services.KeyManualValidationService,
	taskService *services.TaskService,
	keyService *services.KeyService,
) *Server {
	return &Server{
		DB:                         db,
		config:                     config,
		KeyValidatorService:        keyValidatorService,
		KeyManualValidationService: keyManualValidationService,
		TaskService:                taskService,
		KeyService:                 keyService,
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
