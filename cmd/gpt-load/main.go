// Package main provides the entry point for the GPT-Load proxy server
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gpt-load/internal/config"
	"gpt-load/internal/handler"
	"gpt-load/internal/keymanager"
	"gpt-load/internal/middleware"
	"gpt-load/internal/proxy"
	"gpt-load/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	configManager, err := config.NewManager()
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logger
	setupLogger(configManager)

	// Display startup information
	displayStartupInfo(configManager)

	// Create key manager
	keyManager, err := keymanager.NewManager(configManager.GetKeysConfig())
	if err != nil {
		logrus.Fatalf("Failed to create key manager: %v", err)
	}
	defer keyManager.Close()

	// Create proxy server
	proxyServer, err := proxy.NewProxyServer(keyManager, configManager)
	if err != nil {
		logrus.Fatalf("Failed to create proxy server: %v", err)
	}
	defer proxyServer.Close()

	// Create handlers
	handlers := handler.NewHandler(keyManager, configManager)

	// Setup routes
	router := setupRoutes(handlers, proxyServer, configManager)

	// Create HTTP server with optimized timeout configuration
	serverConfig := configManager.GetServerConfig()
	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", serverConfig.Host, serverConfig.Port),
		Handler:        router,
		ReadTimeout:    60 * time.Second,  // Increased read timeout for large file uploads
		WriteTimeout:   300 * time.Second, // Increased write timeout for streaming responses
		IdleTimeout:    120 * time.Second, // Increased idle timeout for connection reuse
		MaxHeaderBytes: 1 << 20,           // 1MB header limit
	}

	// Start server
	go func() {
		logrus.Info("GPT-Load proxy server started successfully")
		logrus.Infof("Server address: http://%s:%d", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Statistics: http://%s:%d/stats", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Health check: http://%s:%d/health", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Reset keys: http://%s:%d/reset-keys", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Blacklist query: http://%s:%d/blacklist", serverConfig.Host, serverConfig.Port)
		logrus.Info("")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrus.Info("Shutting down server...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	} else {
		logrus.Info("Server exited gracefully")
	}
}

// setupRoutes configures the HTTP routes
func setupRoutes(handlers *handler.Handler, proxyServer *proxy.ProxyServer, configManager types.ConfigManager) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger(configManager.GetLogConfig()))
	router.Use(middleware.CORS(configManager.GetCORSConfig()))
	router.Use(middleware.RateLimiter(configManager.GetPerformanceConfig()))

	// Add authentication middleware if enabled
	if configManager.GetAuthConfig().Enabled {
		router.Use(middleware.Auth(configManager.GetAuthConfig()))
	}

	// Management endpoints
	router.GET("/health", handlers.Health)
	router.GET("/stats", handlers.Stats)
	router.GET("/blacklist", handlers.Blacklist)
	router.GET("/reset-keys", handlers.ResetKeys)
	router.GET("/config", handlers.GetConfig) // Debug endpoint

	// Handle 405 Method Not Allowed
	router.NoMethod(handlers.MethodNotAllowed)

	// Proxy all other requests (this handles 404 as well)
	router.NoRoute(proxyServer.HandleProxy)

	return router
}

// setupLogger configures the logging system
func setupLogger(configManager types.ConfigManager) {
	logConfig := configManager.GetLogConfig()

	// Set log level
	level, err := logrus.ParseLevel(logConfig.Level)
	if err != nil {
		logrus.Warn("Invalid log level, using info")
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	// Set log format
	if logConfig.Format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// Setup file logging if enabled
	if logConfig.EnableFile {
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(logConfig.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logrus.Warnf("Failed to create log directory: %v", err)
		} else {
			// Open log file
			logFile, err := os.OpenFile(logConfig.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				logrus.Warnf("Failed to open log file: %v", err)
			} else {
				// Use both file and stdout
				logrus.SetOutput(io.MultiWriter(os.Stdout, logFile))
			}
		}
	}
}

// displayStartupInfo shows startup information
func displayStartupInfo(configManager types.ConfigManager) {
	serverConfig := configManager.GetServerConfig()
	keysConfig := configManager.GetKeysConfig()
	openaiConfig := configManager.GetOpenAIConfig()
	authConfig := configManager.GetAuthConfig()
	corsConfig := configManager.GetCORSConfig()
	perfConfig := configManager.GetPerformanceConfig()
	logConfig := configManager.GetLogConfig()

	logrus.Info("Current Configuration:")
	logrus.Infof("   Server: %s:%d", serverConfig.Host, serverConfig.Port)
	logrus.Infof("   Keys file: %s", keysConfig.FilePath)
	logrus.Infof("   Start index: %d", keysConfig.StartIndex)
	logrus.Infof("   Blacklist threshold: %d errors", keysConfig.BlacklistThreshold)
	logrus.Infof("   Max retries: %d", keysConfig.MaxRetries)
	logrus.Infof("   Upstream URL: %s", openaiConfig.BaseURL)
	logrus.Infof("   Request timeout: %dms", openaiConfig.Timeout)

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
