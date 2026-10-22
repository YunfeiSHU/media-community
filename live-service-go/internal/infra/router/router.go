// Package router 负责注册路由与全局中间件。
package router

import (
	"log/slog"

	"github.com/example/live-service-go/internal/config"
	"github.com/example/live-service-go/internal/handler"
	"github.com/example/live-service-go/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handlers 汇总全部 HTTP handler，避免路由构造参数过长。
type Handlers struct {
	Room   *handler.RoomHandler
	Live   *handler.LiveHandler
	Hook   *handler.HookHandler
	Health *handler.HealthHandler
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

	// 探针
	engine.GET("/healthz", h.Health.Healthz)
	engine.GET("/readyz", h.Health.Readyz)

	v1 := engine.Group("/api/v1")

	// ---------------------------------------------------------- SRS 回调
	// 不对客户端开放：在网关白名单放行，校验靠 streamKey + 可选 hook_secret
	hooks := v1.Group("/live/hooks")
	hooks.POST("/on-publish", h.Hook.OnPublish)
	hooks.POST("/on-unpublish", h.Hook.OnUnpublish)
	hooks.POST("/on-play", h.Hook.OnPlay)
	hooks.POST("/on-stop", h.Hook.OnStop)

	// ---------------------------------------------------------- 直播间
	rooms := v1.Group("/live/rooms")

	// 读接口：游客可见（streamKey / pushUrl 在 service 层按调用者裁剪）
	rooms.GET("", h.Room.List)
	rooms.GET("/:id", h.Room.Get)
	rooms.GET("/:id/online", h.Live.Online)
	rooms.GET("/:id/play", h.Live.Play)

	// 观众心跳：登录可选（游客用 X-Session-Id 或请求体 sessionId）
	rooms.POST("/:id/heartbeat", h.Live.Heartbeat)

	// 写接口：必须登录；归属校验在 service 层
	authed := rooms.Group("", middleware.Auth())
	authed.POST("", h.Room.Create)
	authed.PUT("/:id", h.Room.Update)
	authed.DELETE("/:id", h.Room.Delete)
	authed.GET("/:id/stream-key", h.Room.StreamKey)
	authed.GET("/:id/records", h.Room.Records)
	authed.POST("/:id/start", h.Live.Start)
	authed.POST("/:id/stop", h.Live.Stop)
	authed.POST("/:id/force-close", h.Live.ForceClose)

	// ---------------------------------------------------------- 本地联调
	// 本地没有 SRS 时用它模拟 on_publish / on_unpublish。
	// 这里做**编译期之外的注册期判断**（而不是靠中间件拦截），
	// 非 debug 环境下这两个路由干脆不存在，不会因为中间件写错而误开。
	if cfg.IsDebug() {
		dev := v1.Group("/live/dev")
		dev.POST("/publish/:streamKey", h.Live.DevPublish)
		dev.POST("/unpublish/:streamKey", h.Live.DevUnpublish)
		log.Warn("dev_routes_enabled", "paths", []string{
			"POST /api/v1/live/dev/publish/:streamKey",
			"POST /api/v1/live/dev/unpublish/:streamKey",
		})
	}

	return engine
}
