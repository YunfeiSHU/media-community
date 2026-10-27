// Package router 负责注册路由与全局中间件。
package router

import (
	"log/slog"

	"github.com/example/video-service-go/internal/config"
	"github.com/example/video-service-go/internal/handler"
	"github.com/example/video-service-go/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handlers 汇总全部 HTTP handler，避免路由构造参数过长。
type Handlers struct {
	Video       *handler.VideoHandler
	Upload      *handler.UploadHandler
	Category    *handler.CategoryHandler
	Media       *handler.MediaHandler
	Interaction *handler.InteractionHandler
	Health      *handler.HealthHandler
}

// New 构造 gin 引擎。
func New(cfg *config.Config, log *slog.Logger, h Handlers) *gin.Engine {
	if cfg.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	// 超过该阈值的内存缓冲会落到临时文件，避免大视频把内存打满
	engine.MaxMultipartMemory = 16 << 20 // 16MB

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

	// 分类：列表公开，新建仅 ADMIN
	v1.GET("/categories", h.Category.List)
	v1.POST("/categories", middleware.AdminOnly(), h.Category.Create)

	// 图片上传：封面 / 头像 / 商品图，必须登录（网关不把它放进白名单）
	media := v1.Group("/media", middleware.Auth())
	media.POST("/images", h.Media.UploadImage)

	// 视频：读接口公开（可见性在 service 层判定），写接口需要登录
	videos := v1.Group("/videos")
	videos.GET("", h.Video.List)
	// 静态路径要能和无害的 :id 共存（gin 1.12 已验证支持），
	// 顺序上先注册静态，语义更直观。
	videos.GET("/interactions", h.Interaction.Batch)
	videos.GET("/:id", h.Video.Get)
	videos.GET("/:id/play", h.Video.Play)
	videos.GET("/:id/interactions", h.Interaction.Get)

	authed := videos.Group("", middleware.Auth())
	authed.POST("", h.Video.Create)
	authed.PUT("/:id", h.Video.Update)
	authed.DELETE("/:id", h.Video.Delete)

	// 点赞 / 收藏：写操作必须登录，且由唯一键保证幂等
	authed.POST("/:id/like", h.Interaction.Like)
	authed.DELETE("/:id/like", h.Interaction.Unlike)
	authed.POST("/:id/favorite", h.Interaction.Favorite)
	authed.DELETE("/:id/favorite", h.Interaction.Unfavorite)

	// 上传：一次性（小文件）与分片（大文件，支持断点续传）两条入口
	authed.POST("/:id/upload", h.Video.Upload)
	authed.POST("/:id/upload/init", h.Upload.Init)
	authed.GET("/:id/upload/status", h.Upload.Status)
	authed.PUT("/:id/upload/parts/:partNumber", h.Upload.Part)
	authed.POST("/:id/upload/complete", h.Upload.Complete)
	authed.DELETE("/:id/upload", h.Upload.Abort)

	authed.POST("/:id/publish", h.Video.Publish)
	authed.POST("/:id/offline", h.Video.Offline)
	authed.GET("/:id/transcode", h.Video.TranscodeStatus)
	authed.POST("/:id/transcode/retry", h.Video.RetryTranscode)

	return engine
}
