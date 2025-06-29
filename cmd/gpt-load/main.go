// Package main provides the entry point for the GPT-Load proxy server
package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gpt-load/internal/config"
	"gpt-load/internal/db"
	"gpt-load/internal/handler"
	"gpt-load/internal/middleware"
	"gpt-load/internal/models"
	"gpt-load/internal/proxy"
	"gpt-load/internal/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	configManager, err := config.NewManager()
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logger
	setupLogger(configManager)

	// Initialize database
	database, err := db.InitDB()
	if err != nil {
		logrus.Fatalf("Failed to initialize database: %v", err)
	}

	// Display startup information
	displayStartupInfo(configManager)


	// --- Asynchronous Request Logging Setup ---
	requestLogChan := make(chan models.RequestLog, 1000)
	go startRequestLogger(database, requestLogChan)
	// ---

	// Create proxy server
	proxyServer, err := proxy.NewProxyServer(database, requestLogChan)
	if err != nil {
		logrus.Fatalf("Failed to create proxy server: %v", err)
	}
	defer proxyServer.Close()

	// Create handlers
	serverHandler := handler.NewServer(database, configManager)

	// Setup routes
	router := setupRoutes(serverHandler, proxyServer, configManager)

	// Create HTTP server with optimized timeout configuration
	serverConfig := configManager.GetServerConfig()
	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", serverConfig.Host, serverConfig.Port),
		Handler:        router,
		ReadTimeout:    time.Duration(serverConfig.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(serverConfig.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(serverConfig.IdleTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB header limit
	}

	// Start server
	go func() {
		logrus.Info("GPT-Load proxy server started successfully")
		logrus.Infof("Server address: http://%s:%d", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Statistics: http://%s:%d/stats", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Health check: http://%s:%d/health", serverConfig.Host, serverConfig.Port)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(serverConfig.GracefulShutdownTimeout)*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	} else {
		logrus.Info("Server exited gracefully")
	}
}

// setupRoutes configures the HTTP routes
func setupRoutes(serverHandler *handler.Server, proxyServer *proxy.ProxyServer, configManager types.ConfigManager) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add server start time middleware for uptime calculation
	startTime := time.Now()
	router.Use(func(c *gin.Context) {
		c.Set("serverStartTime", startTime)
		c.Next()
	})

	// Add middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.Logger(configManager.GetLogConfig()))
	router.Use(middleware.CORS(configManager.GetCORSConfig()))
	router.Use(middleware.RateLimiter(configManager.GetPerformanceConfig()))

	// Add authentication middleware if enabled
	if configManager.GetAuthConfig().Enabled {
		router.Use(middleware.Auth(configManager.GetAuthConfig()))
	}

	// Management endpoints
	router.GET("/health", serverHandler.Health)
	router.GET("/stats", serverHandler.Stats)
	router.GET("/config", serverHandler.GetConfig) // Debug endpoint

	// Register API routes for group and key management
	api := router.Group("/api")
	serverHandler.RegisterAPIRoutes(api)

	// Register the main proxy route
	proxy := router.Group("/proxy")
	proxyServer.RegisterProxyRoutes(proxy)

	// Handle 405 Method Not Allowed
	router.NoMethod(serverHandler.MethodNotAllowed)

	// Serve the frontend UI for all other requests
	router.NoRoute(ServeUI())

	return router
}

// ServeUI returns a gin.HandlerFunc to serve the embedded frontend UI.
func ServeUI() gin.HandlerFunc {
	subFS, err := fs.Sub(WebUI, "dist")
	if err != nil {
		// This should not happen at runtime if embed is correct.
		// Panic is acceptable here as it's a startup failure.
		panic(fmt.Sprintf("Failed to create sub filesystem for UI: %v", err))
	}
	fileServer := http.FileServer(http.FS(subFS))

	return func(c *gin.Context) {
		// Clean the path to prevent directory traversal attacks.
		upath := path.Clean(c.Request.URL.Path)
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
		}

		// Check if the file exists in the embedded filesystem.
		_, err := subFS.Open(strings.TrimPrefix(upath, "/"))
		if os.IsNotExist(err) {
			// The file does not exist, so we serve index.html for SPA routing.
			// This allows the Vue router to handle the path.
			c.Request.URL.Path = "/"
		}

		// Let the http.FileServer handle the request.
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
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
			ForceColors:     true,
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
	logrus.Infof("   Request timeout: %ds", openaiConfig.RequestTimeout)
	logrus.Infof("   Response timeout: %ds", openaiConfig.ResponseTimeout)
	logrus.Infof("   Idle connection timeout: %ds", openaiConfig.IdleConnTimeout)

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

// startRequestLogger runs a background goroutine to batch-insert request logs.
func startRequestLogger(db *gorm.DB, logChan <-chan models.RequestLog) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logBuffer := make([]models.RequestLog, 0, 100)

	for {
		select {
		case logEntry, ok := <-logChan:
			if !ok {
				// Channel closed, flush remaining logs and exit
				if len(logBuffer) > 0 {
					if err := db.Create(&logBuffer).Error; err != nil {
						logrus.Errorf("Failed to write remaining request logs: %v", err)
					}
				}
				logrus.Info("Request logger stopped.")
				return
			}
			logBuffer = append(logBuffer, logEntry)
			if len(logBuffer) >= 100 {
				if err := db.Create(&logBuffer).Error; err != nil {
					logrus.Errorf("Failed to write request logs: %v", err)
				}
				logBuffer = make([]models.RequestLog, 0, 100) // Reset buffer
			}
		case <-ticker.C:
			// Flush logs periodically
			if len(logBuffer) > 0 {
				if err := db.Create(&logBuffer).Error; err != nil {
					logrus.Errorf("Failed to write request logs on tick: %v", err)
				}
				logBuffer = make([]models.RequestLog, 0, 100) // Reset buffer
			}
		}
	}
}
