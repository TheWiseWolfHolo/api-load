package router

import (
	"gpt-load/internal/handler"
	"gpt-load/internal/middleware"
	"gpt-load/internal/proxy"
	"gpt-load/internal/types"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// New 创建并配置一个完整的 gin.Engine 实例
func New(
	serverHandler *handler.Server,
	proxyServer *proxy.ProxyServer,
	configManager types.ConfigManager,
	webUI fs.FS,
) *gin.Engine {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// 注册全局中间件
	router.Use(middleware.Recovery())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.Logger(configManager.GetLogConfig()))
	router.Use(middleware.CORS(configManager.GetCORSConfig()))
	router.Use(middleware.RateLimiter(configManager.GetPerformanceConfig()))

	// 添加服务器启动时间中间件
	startTime := time.Now()
	router.Use(func(c *gin.Context) {
		c.Set("serverStartTime", startTime)
		c.Next()
	})

	// 注册 Web UI 和通用端点
	router.GET("/health", serverHandler.Health)
	router.GET("/stats", serverHandler.Stats)
	router.GET("/config", serverHandler.GetConfig) // Debug endpoint

	// 注册管理 API 路由
	api := router.Group("/api")
	authConfig := configManager.GetAuthConfig()
	if authConfig.Enabled {
		api.Use(middleware.Auth(authConfig))
	}
	serverHandler.RegisterAPIRoutes(api)

	// 注册代理路由
	proxyGroup := router.Group("/proxy")
	if authConfig.Enabled {
		proxyGroup.Use(middleware.Auth(authConfig))
	}
	proxyServer.RegisterProxyRoutes(proxyGroup)

	// 处理 405 Method Not Allowed
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
	})

	// 其他所有路由都交给前端 UI 处理
	router.NoRoute(ServeUI(webUI))

	return router
}

// ServeUI 返回一个 gin.HandlerFunc 来服务嵌入式前端 UI
func ServeUI(webUI fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(webUI))

	return func(c *gin.Context) {
		// 检查文件是否存在于嵌入的文件系统中
		if _, err := webUI.Open(strings.TrimPrefix(c.Request.URL.Path, "/")); err != nil {
			// 如果文件不存在，并且不是API或代理请求，则将请求重写为 /
			// 这将提供 index.html，以支持 SPA 的前端路由
			if !strings.HasPrefix(c.Request.URL.Path, "/api/") && !strings.HasPrefix(c.Request.URL.Path, "/proxy/") {
				c.Request.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
