// Package middleware provides HTTP middleware for the application
package middleware

import (
	"fmt"
	"strings"
	"time"

	"gpt-load/internal/response"
	"gpt-load/internal/types"
	"gpt-load/internal/channel"
	"gpt-load/internal/services"
	app_errors "gpt-load/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Logger creates a high-performance logging middleware
func Logger(config types.LogConfig) gin.HandlerFunc {
	return func(c *gin.Context) {

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
func Auth(
	authConfig types.AuthConfig,
	groupManager *services.GroupManager,
	channelFactory *channel.Factory,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip authentication for health/stats endpoints
		if isMonitoringEndpoint(path) {
			c.Next()
			return
		}

		var key string
		var err error

		if strings.HasPrefix(path, "/api") {
			// Handle backend API authentication
			key = extractBearerKey(c)
			if key == "" || key != authConfig.Key {
				response.Error(c, app_errors.ErrUnauthorized)
				c.Abort()
				return
			}
		} else if strings.HasPrefix(path, "/proxy/") {
			// Handle proxy authentication
			key, err = extractProxyKey(c, groupManager, channelFactory)
			if err != nil {
				// The error from extractProxyKey is already an APIError
				if apiErr, ok := err.(*app_errors.APIError); ok {
					response.Error(c, apiErr)
				} else {
					response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error()))
				}
				c.Abort()
				return
			}
		} else {
			// For any other paths, deny access by default
			response.Error(c, app_errors.ErrResourceNotFound)
			c.Abort()
			return
		}

		if key == "" {
			response.Error(c, app_errors.ErrUnauthorized)
			c.Abort()
			return
		}

		// Key is extracted, but validation is handled by the proxy logic itself.
		// For the backend API, we've already validated it.
		c.Next()
	}
}

// Recovery creates a recovery middleware with custom error handling
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logrus.Errorf("Panic recovered: %v", recovered)
		response.Error(c, app_errors.ErrInternalServer)
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
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, "Too many concurrent requests"))
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
			if apiErr, ok := err.(*app_errors.APIError); ok {
				response.Error(c, apiErr)
				return
			}

			// Handle other errors
			logrus.Errorf("Unhandled error: %v", err)
			response.Error(c, app_errors.ErrInternalServer)
		}
	}
}

// isMonitoringEndpoint checks if the path is a monitoring endpoint
func isMonitoringEndpoint(path string) bool {
	monitoringPaths := []string{"/health", "/stats"}
	for _, monitoringPath := range monitoringPaths {
		if path == monitoringPath {
			return true
		}
	}
	return false
}

// extractBearerKey extracts a key from the "Authorization: Bearer <key>" header.
func extractBearerKey(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(authHeader, bearerPrefix) {
			return authHeader[len(bearerPrefix):]
		}
	}
	return ""
}

// extractProxyKey handles key extraction for proxy routes.
func extractProxyKey(
	c *gin.Context,
	groupManager *services.GroupManager,
	channelFactory *channel.Factory,
) (string, error) {
	groupName := c.Param("group_name")
	if groupName == "" {
		return "", app_errors.NewAPIError(app_errors.ErrBadRequest, "Group name is missing in the URL path")
	}

	group, err := groupManager.GetGroupByName(groupName)
	if err != nil {
		return "", app_errors.NewAPIError(app_errors.ErrResourceNotFound, fmt.Sprintf("Group '%s' not found", groupName))
	}

	channel, err := channelFactory.GetChannel(group)
	if err != nil {
		return "", app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get channel for group '%s'", groupName))
	}

	key := channel.ExtractKey(c)
	if key == "" {
		return "", app_errors.ErrUnauthorized
	}

	return key, nil
}
