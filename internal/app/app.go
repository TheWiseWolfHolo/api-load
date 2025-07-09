// Package app provides the main application logic and lifecycle management.
package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gpt-load/internal/config"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/proxy"
	"gpt-load/internal/services"
	"gpt-load/internal/store"
	"gpt-load/internal/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

// App holds all services and manages the application lifecycle.
type App struct {
	engine            *gin.Engine
	configManager     types.ConfigManager
	settingsManager   *config.SystemSettingsManager
	logCleanupService *services.LogCleanupService
	keyCronService    *services.KeyCronService
	keyValidationPool *services.KeyValidationPool
	keyPoolProvider   *keypool.KeyProvider
	leaderService     *services.LeaderService
	proxyServer       *proxy.ProxyServer
	storage           store.Store
	db                *gorm.DB
	httpServer        *http.Server
	requestLogChan    chan models.RequestLog
	wg                sync.WaitGroup
}

// AppParams defines the dependencies for the App.
type AppParams struct {
	dig.In
	Engine            *gin.Engine
	ConfigManager     types.ConfigManager
	SettingsManager   *config.SystemSettingsManager
	LogCleanupService *services.LogCleanupService
	KeyCronService    *services.KeyCronService
	KeyValidationPool *services.KeyValidationPool
	KeyPoolProvider   *keypool.KeyProvider
	LeaderService     *services.LeaderService
	ProxyServer       *proxy.ProxyServer
	Storage           store.Store
	DB                *gorm.DB
	RequestLogChan    chan models.RequestLog
}

// NewApp is the constructor for App, with dependencies injected by dig.
func NewApp(params AppParams) *App {
	return &App{
		engine:            params.Engine,
		configManager:     params.ConfigManager,
		settingsManager:   params.SettingsManager,
		logCleanupService: params.LogCleanupService,
		keyCronService:    params.KeyCronService,
		keyValidationPool: params.KeyValidationPool,
		keyPoolProvider:   params.KeyPoolProvider,
		leaderService:     params.LeaderService,
		proxyServer:       params.ProxyServer,
		storage:           params.Storage,
		db:                params.DB,
		requestLogChan:    params.RequestLogChan,
	}
}

// Start runs the application, it is a non-blocking call.
func (a *App) Start() error {
	// 1. 启动 Leader Service 并等待选举结果
	if err := a.leaderService.Start(); err != nil {
		return fmt.Errorf("leader service failed to start: %w", err)
	}

	// 2. Leader 节点执行不依赖配置的“写”操作
	if a.leaderService.IsLeader() {
		logrus.Info("Leader mode. Performing initial one-time tasks...")

		// 2.1. 数据库迁移
		if err := a.db.AutoMigrate(
			&models.RequestLog{},
			&models.APIKey{},
			&models.SystemSetting{},
			&models.Group{},
		); err != nil {
			return fmt.Errorf("database auto-migration failed: %w", err)
		}
		logrus.Info("Database auto-migration completed.")

		// 2.2. 初始化系统设置
		if err := a.settingsManager.EnsureSettingsInitialized(); err != nil {
			return fmt.Errorf("failed to initialize system settings: %w", err)
		}
		logrus.Info("System settings initialized in DB.")
	} else {
		logrus.Info("Follower Mode. Skipping initial one-time tasks.")
	}

	// 3. 所有节点从数据库加载配置到内存
	if err := a.settingsManager.LoadFromDatabase(); err != nil {
		return fmt.Errorf("failed to load system settings from database: %w", err)
	}
	logrus.Info("System settings loaded into memory.")

	// 4. Leader 节点执行依赖配置的“写”操作
	if a.leaderService.IsLeader() {
		// 4.1. 从数据库加载密钥到 Redis
		if err := a.keyPoolProvider.LoadKeysFromDB(); err != nil {
			return fmt.Errorf("failed to load keys into key pool: %w", err)
		}
		logrus.Info("API keys loaded into Redis cache by leader.")
	}

	// 5. 显示配置并启动所有后台服务
	a.settingsManager.DisplayCurrentSettings()
	a.configManager.DisplayConfig()

	a.startRequestLogger()
	a.logCleanupService.Start()
	a.keyValidationPool.Start()
	a.keyCronService.Start()

	// Create HTTP server
	serverConfig := a.configManager.GetEffectiveServerConfig()
	a.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", serverConfig.Host, serverConfig.Port),
		Handler:        a.engine,
		ReadTimeout:    time.Duration(serverConfig.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(serverConfig.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(serverConfig.IdleTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start HTTP server in a new goroutine
	go func() {
		logrus.Info("GPT-Load proxy server started successfully")
		logrus.Infof("Server address: http://%s:%d", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Statistics: http://%s:%d/stats", serverConfig.Host, serverConfig.Port)
		logrus.Infof("Health check: http://%s:%d/health", serverConfig.Host, serverConfig.Port)
		logrus.Info("")
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server startup failed: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the application.
func (a *App) Stop(ctx context.Context) {
	logrus.Info("Shutting down server...")

	// Shutdown http server
	if err := a.httpServer.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	}

	// Stop background services
	a.keyCronService.Stop()
	a.keyValidationPool.Stop()
	a.leaderService.Stop()
	a.logCleanupService.Stop()

	// Close resources
	a.proxyServer.Close()
	a.storage.Close()

	// Wait for the logger to finish writing all logs
	logrus.Info("Closing request log channel...")
	close(a.requestLogChan)
	a.wg.Wait()
	logrus.Info("All logs have been written.")

	logrus.Info("Server exited gracefully")
}

// startRequestLogger runs a background goroutine to batch-insert request logs.
func (a *App) startRequestLogger() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		logBuffer := make([]models.RequestLog, 0, 100)

		for {
			select {
			case logEntry, ok := <-a.requestLogChan:
				if !ok {
					// Channel closed, flush remaining logs and exit
					if len(logBuffer) > 0 {
						if err := a.db.Create(&logBuffer).Error; err != nil {
							logrus.Errorf("Failed to write remaining request logs: %v", err)
						}
					}
					logrus.Info("Request logger stopped.")
					return
				}
				logBuffer = append(logBuffer, logEntry)
				if len(logBuffer) >= 100 {
					if err := a.db.Create(&logBuffer).Error; err != nil {
						logrus.Errorf("Failed to write request logs: %v", err)
					}
					logBuffer = make([]models.RequestLog, 0, 100) // Reset buffer
				}
			case <-ticker.C:
				// Flush logs periodically
				if len(logBuffer) > 0 {
					if err := a.db.Create(&logBuffer).Error; err != nil {
						logrus.Errorf("Failed to write request logs on tick: %v", err)
					}
					logBuffer = make([]models.RequestLog, 0, 100) // Reset buffer
				}
			}
		}
	}()
}
