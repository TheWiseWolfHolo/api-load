package router

import (
	"embed"
	"gpt-load/internal/handler"
	"gpt-load/internal/middleware"
	"gpt-load/internal/proxy"
	"gpt-load/internal/types"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"

	"github.com/gin-gonic/gin"
)

type embedFileSystem struct {
	http.FileSystem
}

func (e embedFileSystem) Exists(prefix string, path string) bool {
	_, err := e.Open(path)
	return err == nil
}

func EmbedFolder(fsEmbed embed.FS, targetPath string) static.ServeFileSystem {
	efs, err := fs.Sub(fsEmbed, targetPath)
	if err != nil {
		panic(err)
	}
	return embedFileSystem{
		FileSystem: http.FS(efs),
	}
}

func New(
	serverHandler *handler.Server,
	proxyServer *proxy.ProxyServer,
	logCleanupHandler *handler.LogCleanupHandler,
	configManager types.ConfigManager,
	buildFS embed.FS,
	indexPage []byte,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// 注册全局中间件
	router.Use(middleware.Recovery())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.Logger(configManager.GetLogConfig()))
	router.Use(middleware.CORS(configManager.GetCORSConfig()))
	router.Use(middleware.RateLimiter(configManager.GetPerformanceConfig()))
	router.Use(func(c *gin.Context) {
		c.Set("serverStartTime", time.Now())
		c.Next()
	})

	// 注册路由
	registerSystemRoutes(router, serverHandler)
	registerAPIRoutes(router, serverHandler, logCleanupHandler, configManager)
	registerProxyRoutes(router, proxyServer, configManager)
	registerFrontendRoutes(router, buildFS, indexPage)

	return router
}

// registerSystemRoutes 注册系统级路由
func registerSystemRoutes(router *gin.Engine, serverHandler *handler.Server) {
	router.GET("/health", serverHandler.Health)
	router.GET("/stats", serverHandler.Stats)
}

// registerAPIRoutes 注册API路由
func registerAPIRoutes(router *gin.Engine, serverHandler *handler.Server, logCleanupHandler *handler.LogCleanupHandler, configManager types.ConfigManager) {
	api := router.Group("/api")
	authConfig := configManager.GetAuthConfig()

	// 公开
	registerPublicAPIRoutes(api, serverHandler)

	// 认证
	if authConfig.Enabled {
		protectedAPI := api.Group("")
		protectedAPI.Use(middleware.Auth(authConfig))
		registerProtectedAPIRoutes(protectedAPI, serverHandler, logCleanupHandler)
	} else {
		registerProtectedAPIRoutes(api, serverHandler, logCleanupHandler)
	}
}

// registerPublicAPIRoutes 公开API路由
func registerPublicAPIRoutes(api *gin.RouterGroup, serverHandler *handler.Server) {
	api.POST("/auth/login", serverHandler.Login)
}

// registerProtectedAPIRoutes 认证API路由
func registerProtectedAPIRoutes(api *gin.RouterGroup, serverHandler *handler.Server, logCleanupHandler *handler.LogCleanupHandler) {
	groups := api.Group("/groups")
	{
		groups.POST("", serverHandler.CreateGroup)
		groups.GET("", serverHandler.ListGroups)
		groups.GET("/config-options", serverHandler.GetGroupConfigOptions)
		groups.PUT("/:id", serverHandler.UpdateGroup)
		groups.DELETE("/:id", serverHandler.DeleteGroup)

		// Key-specific routes
		keys := groups.Group("/:id/keys")
		{
			keys.GET("", serverHandler.ListKeysInGroup)
			keys.POST("/add-multiple", serverHandler.AddMultipleKeys)
			keys.POST("/restore-all-invalid", serverHandler.RestoreAllInvalidKeys)
			keys.POST("/clear-all-invalid", serverHandler.ClearAllInvalidKeys)
			keys.GET("/export", serverHandler.ExportKeys)
			keys.DELETE("/:key_id", serverHandler.DeleteSingleKey)
			keys.POST("/:key_id/test", serverHandler.TestSingleKey)
		}

		// Group-level actions
		groups.POST("/:id/validate-keys", serverHandler.ValidateGroupKeys)
	}

	// Tasks
	tasks := api.Group("/tasks")
	{
		tasks.GET("/key-validation/status", serverHandler.GetTaskStatus)
		tasks.GET("/:task_id/result", serverHandler.GetTaskResult)
	}

	// 仪表板和日志
	dashboard := api.Group("/dashboard")
	{
		dashboard.GET("/stats", serverHandler.Stats)
	}

	// 日志
	logs := api.Group("/logs")
	{
		logs.GET("", handler.GetLogs)
		logs.POST("/cleanup", logCleanupHandler.CleanupLogsNow)
	}

	// 设置
	settings := api.Group("/settings")
	{
		settings.GET("", handler.GetSettings)
		settings.PUT("", handler.UpdateSettings)
	}
}

// registerProxyRoutes 注册代理路由
func registerProxyRoutes(router *gin.Engine, proxyServer *proxy.ProxyServer, configManager types.ConfigManager) {
	proxyGroup := router.Group("/proxy")
	authConfig := configManager.GetAuthConfig()

	if authConfig.Enabled {
		proxyGroup.Use(middleware.Auth(authConfig))
	}

	proxyGroup.Any("/:group_name/*path", proxyServer.HandleProxy)
}

// registerFrontendRoutes 注册前端路由
func registerFrontendRoutes(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
	})

	router.Use(static.Serve("/", EmbedFolder(buildFS, "dist")))
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/proxy") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})
}
