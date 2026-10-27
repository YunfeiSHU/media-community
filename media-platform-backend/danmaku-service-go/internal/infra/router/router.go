// Package router 负责注册路由与全局中间件。
package router

import (
	"log/slog"

	"github.com/example/danmaku-service-go/internal/config"
	"github.com/example/danmaku-service-go/internal/handler"
	"github.com/example/danmaku-service-go/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handlers 汇总全部 HTTP handler，避免路由构造参数过长。
type Handlers struct {
	Danmaku *handler.DanmakuHandler
	History *handler.HistoryHandler
	Admin   *handler.AdminHandler
	WS      *handler.WSHandler
	Health  *handler.HealthHandler
}

// New 构造 gin 引擎。
func New(cfg *config.Config, log *slog.Logger, h Handlers) *gin.Engine {
	if cfg.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(
		middleware.Recovery(log),
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.CORS(),
		middleware.ErrorHandler(log),
	)

	// ---------------------------------------------------------- 探针
	engine.GET("/healthz", h.Health.Healthz)
	engine.GET("/readyz", h.Health.Readyz)

	// ---------------------------------------------------------- WebSocket
	// 不走 /api/v1 前缀：它是长连接入口，与 REST 的资源语义不同。
	// 生产环境需要在网关上为它放行并做 JWT 校验（浏览器 WS 无法自定义请求头）。
	engine.GET("/ws/danmaku", h.WS.Serve)

	v1 := engine.Group("/api/v1")
	danmaku := v1.Group("/danmaku")

	// ---------------------------------------------------------- 读接口（公开）
	danmaku.GET("/rooms/:roomId/history", h.History.RoomHistory)
	danmaku.GET("/videos/:videoId/history", h.History.VideoHistory)
	danmaku.GET("/stats", h.History.Stats)

	// ---------------------------------------------------------- 写接口（必须登录）
	authed := danmaku.Group("", middleware.Auth())
	authed.POST("/send", h.Danmaku.Send)
	// 等价路径：roomId 放在路径里，避免与点播弹幕的请求体混淆。
	authed.POST("/rooms/:roomId/send", h.Danmaku.SendToRoom)
	authed.DELETE("/:danmakuId", h.Danmaku.Recall)

	// ---------------------------------------------------------- 管理接口（必须 ADMIN）
	admin := danmaku.Group("/admin", middleware.AdminOnly())
	admin.POST("/reload-words", h.Admin.ReloadWords)
	admin.GET("/words", h.Admin.ListWords)
	admin.POST("/words", h.Admin.CreateWord)

	return engine
}
