// Package middleware provides HTTP middleware for the application
package middleware

import (
	"fmt"
	"strings"
	"time"

	"gpt-load/internal/errors"
	"gpt-load/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Logger creates a high-performance logging middleware
func Logger(config types.LogConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request logging is enabled
		if !config.EnableRequest {
			// Don't log requests, only process them
			c.Next()
			// Only log errors
			if c.Writer.Status() >= 400 {
				logrus.Errorf("Error %d: %s %s", c.Writer.Status(), c.Request.Method, c.Request.URL.Path)
			}
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate response time
		latency := time.Since(start)

		// Get basic information
		method := c.Request.Method
		statusCode := c.Writer.Status()

		// Build full path (avoid string concatenation)
		fullPath := path
		if raw != "" {
			fullPath = path + "?" + raw
		}

		// Get key information (if exists)
		keyInfo := ""
		if keyIndex, exists := c.Get("keyIndex"); exists {
			if keyPreview, exists := c.Get("keyPreview"); exists {
				keyInfo = fmt.Sprintf(" - Key[%v] %v", keyIndex, keyPreview)
			}
		}

		// Get retry information (if exists)
		retryInfo := ""
		if retryCount, exists := c.Get("retryCount"); exists {
			retryInfo = fmt.Sprintf(" - Retry[%d]", retryCount)
		}

		// Filter health check and other monitoring endpoint logs to reduce noise
		if isMonitoringEndpoint(path) {
			// Only log errors for monitoring endpoints
			if statusCode >= 400 {
				logrus.Warnf("%s %s - %d - %v", method, fullPath, statusCode, latency)
			}
			return
		}

		// Choose log level based on status code
		if statusCode >= 500 {
			logrus.Errorf("%s %s - %d - %v%s%s", method, fullPath, statusCode, latency, keyInfo, retryInfo)
		} else if statusCode >= 400 {
			logrus.Warnf("%s %s - %d - %v%s%s", method, fullPath, statusCode, latency, keyInfo, retryInfo)
		} else {
			logrus.Infof("%s %s - %d - %v%s%s", method, fullPath, statusCode, latency, keyInfo, retryInfo)
		}
	}
}

// CORS creates a CORS middleware
func CORS(config types.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range config.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		// Set other CORS headers
		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))

		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Auth creates an authentication middleware
func Auth(config types.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// Skip authentication for management endpoints
		path := c.Request.URL.Path
		if path == "/health" || path == "/stats" || path == "/blacklist" || path == "/reset-keys" {
			c.Next()
			return
		}

		// Extract key from multiple sources
		key := extractKey(c)
		if key == "" {
			c.JSON(401, gin.H{
				"error": "Authorization required",
				"code":  errors.ErrAuthMissing,
			})
			c.Abort()
			return
		}

		// Validate key
		if key != config.Key {
			c.JSON(401, gin.H{
				"error": "Invalid authentication token",
				"code":  errors.ErrAuthInvalid,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Recovery creates a recovery middleware with custom error handling
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		if err, ok := recovered.(string); ok {
			logrus.Errorf("Panic recovered: %s", err)
			c.JSON(500, gin.H{
				"error": "Internal server error",
				"code":  errors.ErrServerInternal,
			})
		} else {
			logrus.Errorf("Panic recovered: %v", recovered)
			c.JSON(500, gin.H{
				"error": "Internal server error",
				"code":  errors.ErrServerInternal,
			})
		}
		c.Abort()
	})
}

// RateLimiter creates a simple rate limiting middleware
func RateLimiter(config types.PerformanceConfig) gin.HandlerFunc {
	// Simple semaphore-based rate limiting
	semaphore := make(chan struct{}, config.MaxConcurrentRequests)

	return func(c *gin.Context) {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			c.Next()
		default:
			c.JSON(429, gin.H{
				"error": "Too many concurrent requests",
				"code":  errors.ErrServerUnavailable,
			})
			c.Abort()
		}
	}
}

// ErrorHandler creates an error handling middleware
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Handle any errors that occurred during request processing
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// Check if it's our custom error type
			if appErr, ok := err.(*errors.AppError); ok {
				c.JSON(appErr.HTTPStatus, gin.H{
					"error": appErr.Message,
					"code":  appErr.Code,
				})
				return
			}

			// Handle other errors
			logrus.Errorf("Unhandled error: %v", err)
			c.JSON(500, gin.H{
				"error": "Internal server error",
				"code":  errors.ErrServerInternal,
			})
		}
	}
}

// isMonitoringEndpoint checks if the path is a monitoring endpoint
func isMonitoringEndpoint(path string) bool {
	monitoringPaths := []string{"/health", "/stats", "/blacklist", "/reset-keys"}
	for _, monitoringPath := range monitoringPaths {
		if path == monitoringPath {
			return true
		}
	}
	return false
}

// extractKey extracts the API key from the request, checking the Authorization header,
// the X-Goog-Api-Key header, and the "key" query parameter.
func extractKey(c *gin.Context) string {
	// 1. Check Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(authHeader, bearerPrefix) {
			return authHeader[len(bearerPrefix):]
		}
	}

	// 2. Check X-Goog-Api-Key header
	if key := c.GetHeader("X-Goog-Api-Key"); key != "" {
		return key
	}

	// 3. Check "key" query parameter
	if key := c.Query("key"); key != "" {
		return key
	}

	return ""
}
