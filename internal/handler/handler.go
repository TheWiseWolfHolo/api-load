// Package handler provides HTTP handlers for the application
package handler

import (
	"net/http"
	"runtime"
	"time"

	"gpt-load/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Handler contains dependencies for HTTP handlers
type Handler struct {
	keyManager types.KeyManager
	config     types.ConfigManager
}

// NewHandler creates a new handler instance
func NewHandler(keyManager types.KeyManager, config types.ConfigManager) *Handler {
	return &Handler{
		keyManager: keyManager,
		config:     config,
	}
}

// Health handles health check requests
func (h *Handler) Health(c *gin.Context) {
	stats := h.keyManager.GetStats()
	
	status := "healthy"
	httpStatus := http.StatusOK
	
	// Check if there are any healthy keys
	if stats.HealthyKeys == 0 {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":       status,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"healthy_keys": stats.HealthyKeys,
		"total_keys":   stats.TotalKeys,
		"uptime":       time.Since(time.Now()).String(), // This would need to be tracked properly
	})
}

// Stats handles statistics requests
func (h *Handler) Stats(c *gin.Context) {
	stats := h.keyManager.GetStats()
	
	// Add additional system information
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	response := gin.H{
		"keys": gin.H{
			"total":       stats.TotalKeys,
			"healthy":     stats.HealthyKeys,
			"blacklisted": stats.BlacklistedKeys,
			"current_index": stats.CurrentIndex,
		},
		"requests": gin.H{
			"success_count": stats.SuccessCount,
			"failure_count": stats.FailureCount,
			"total_count":   stats.SuccessCount + stats.FailureCount,
		},
		"memory": gin.H{
			"alloc_mb":       bToMb(m.Alloc),
			"total_alloc_mb": bToMb(m.TotalAlloc),
			"sys_mb":         bToMb(m.Sys),
			"num_gc":         m.NumGC,
			"last_gc":        time.Unix(0, int64(m.LastGC)).Format("2006-01-02 15:04:05"),
			"next_gc_mb":     bToMb(m.NextGC),
		},
		"system": gin.H{
			"goroutines":   runtime.NumGoroutine(),
			"cpu_count":    runtime.NumCPU(),
			"go_version":   runtime.Version(),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, response)
}

// Blacklist handles blacklist requests
func (h *Handler) Blacklist(c *gin.Context) {
	blacklist := h.keyManager.GetBlacklist()
	
	response := gin.H{
		"blacklisted_keys": blacklist,
		"count":           len(blacklist),
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, response)
}

// ResetKeys handles key reset requests
func (h *Handler) ResetKeys(c *gin.Context) {
	// Reset blacklist
	h.keyManager.ResetBlacklist()
	
	// Reload keys from file
	if err := h.keyManager.LoadKeys(); err != nil {
		logrus.Errorf("Failed to reload keys: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to reload keys",
			"message": err.Error(),
		})
		return
	}

	stats := h.keyManager.GetStats()
	
	c.JSON(http.StatusOK, gin.H{
		"message":     "Keys reset and reloaded successfully",
		"total_keys":  stats.TotalKeys,
		"healthy_keys": stats.HealthyKeys,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
	
	logrus.Info("Keys reset and reloaded successfully")
}

// NotFound handles 404 requests
func (h *Handler) NotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":     "Endpoint not found",
		"path":      c.Request.URL.Path,
		"method":    c.Request.Method,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// MethodNotAllowed handles 405 requests
func (h *Handler) MethodNotAllowed(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"error":     "Method not allowed",
		"path":      c.Request.URL.Path,
		"method":    c.Request.Method,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetConfig returns configuration information (for debugging)
func (h *Handler) GetConfig(c *gin.Context) {
	// Only allow in development mode or with special header
	if c.GetHeader("X-Debug-Config") != "true" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	serverConfig := h.config.GetServerConfig()
	keysConfig := h.config.GetKeysConfig()
	openaiConfig := h.config.GetOpenAIConfig()
	authConfig := h.config.GetAuthConfig()
	corsConfig := h.config.GetCORSConfig()
	perfConfig := h.config.GetPerformanceConfig()
	logConfig := h.config.GetLogConfig()

	// Sanitize sensitive information
	sanitizedConfig := gin.H{
		"server": gin.H{
			"host": serverConfig.Host,
			"port": serverConfig.Port,
		},
		"keys": gin.H{
			"file_path":           keysConfig.FilePath,
			"start_index":         keysConfig.StartIndex,
			"blacklist_threshold": keysConfig.BlacklistThreshold,
			"max_retries":         keysConfig.MaxRetries,
		},
		"openai": gin.H{
			"base_url": openaiConfig.BaseURL,
			"timeout":  openaiConfig.Timeout,
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
			"request_timeout":         perfConfig.RequestTimeout,
			"enable_gzip":             perfConfig.EnableGzip,
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

// Helper function to convert bytes to megabytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
