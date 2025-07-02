// Package main provides the entry point for the GPT-Load proxy server
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"gpt-load/internal/config"
	"gpt-load/internal/db"
	"gpt-load/internal/handler"
	"gpt-load/internal/models"
	"gpt-load/internal/proxy"
	"gpt-load/internal/router" // <-- 引入新的 router 包
	"gpt-load/internal/types"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

//go:embed dist
var buildFS embed.FS

//go:embed dist/index.html
var indexPage []byte

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
	var wg sync.WaitGroup
	wg.Add(1)
	go startRequestLogger(database, requestLogChan, &wg)
	// ---

	// Create proxy server
	proxyServer, err := proxy.NewProxyServer(database, requestLogChan)
	if err != nil {
		logrus.Fatalf("Failed to create proxy server: %v", err)
	}
	defer proxyServer.Close()

	// Create handlers
	serverHandler := handler.NewServer(database, configManager)

	// Setup routes using the new router package
	appRouter := router.New(serverHandler, proxyServer, configManager, buildFS, indexPage)

	// Create HTTP server with optimized timeout configuration
	serverConfig := configManager.GetServerConfig()
	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", serverConfig.Host, serverConfig.Port),
		Handler:        appRouter,
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
	}

	// Close the request log channel and wait for the logger to finish
	logrus.Info("Closing request log channel...")
	close(requestLogChan)
	wg.Wait()
	logrus.Info("All logs have been written.")

	logrus.Info("Server exited gracefully")
}

// setupLogger, displayStartupInfo, and startRequestLogger functions remain unchanged.
// The old setupRoutes and ServeUI functions are now removed from this file.

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
	openaiConfig := configManager.GetOpenAIConfig()
	authConfig := configManager.GetAuthConfig()
	corsConfig := configManager.GetCORSConfig()
	perfConfig := configManager.GetPerformanceConfig()
	logConfig := configManager.GetLogConfig()

	logrus.Info("Current Configuration:")
	logrus.Infof("   Server: %s:%d", serverConfig.Host, serverConfig.Port)
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
func startRequestLogger(db *gorm.DB, logChan <-chan models.RequestLog, wg *sync.WaitGroup) {
	defer wg.Done()
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
