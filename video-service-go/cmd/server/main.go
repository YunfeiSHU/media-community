// Command server 是 video-service-go 的进程入口。
//
// 装配顺序：配置 → 日志 → MySQL（含建表）→ Redis → 对象存储 → 事件总线 → 转码器
//
//	→ 仓储 → 服务 → handler → 路由 → 后台任务 → 启动 → 优雅退出
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/video-service-go/internal/config"
	"github.com/example/video-service-go/internal/handler"
	"github.com/example/video-service-go/internal/infra/cache"
	"github.com/example/video-service-go/internal/infra/events"
	"github.com/example/video-service-go/internal/infra/media"
	"github.com/example/video-service-go/internal/infra/router"
	"github.com/example/video-service-go/internal/infra/storage"
	"github.com/example/video-service-go/internal/repository"
	"github.com/example/video-service-go/internal/service"
	"github.com/example/video-service-go/pkg/logx"
	"github.com/example/video-service-go/pkg/migrate"
	"github.com/example/video-service-go/pkg/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[video-service] 启动失败: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	log := logx.New(cfg.App.Mode, cfg.App.Name, cfg.App.Version)
	log.Info("booting",
		"config", *configPath,
		"mode", cfg.App.Mode,
		"port", cfg.App.Port,
		"transcoder", cfg.Transcoder.Driver,
		"eventsDriver", cfg.Events.Driver,
	)

	// ---------------------------------------------------------- 基础设施
	db, err := store.NewMySQL(cfg.MySQL)
	if err != nil {
		return err
	}
	if err := migrate.AutoMigrate(db, log); err != nil {
		return err
	}
	migrate.SeedDefaultCategories(db, log)

	rdb, err := store.NewRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	objectStore, err := storage.New(cfg.MinIO)
	if err != nil {
		return err
	}
	ensureCtx, ensureCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := objectStore.EnsureBucket(ensureCtx); err != nil {
		// 桶不存在不阻断启动：/readyz 会暴露问题，上传时也会给出明确错误
		log.Warn("ensure_bucket_failed", "bucket", objectStore.Bucket(), "error", err)
	}
	ensureCancel()

	bus, err := buildBus(cfg, log)
	if err != nil {
		return err
	}
	defer func() { _ = bus.Close() }()

	transcoder, err := media.New(cfg.Transcoder, objectStore, log)
	if err != nil {
		return err
	}
	if transcoder.Name() == "mock" {
		log.Warn("transcoder_mock_enabled",
			"hint", "当前使用 mock 转码器，产物为占位 HLS；需要真实可播放视频请设置 VIDEO_TRANSCODER_DRIVER=real")
	}

	// ---------------------------------------------------------- 装配
	videoCache := cache.NewVideoCache(rdb, cfg.View.DetailCacheTTL, log)
	videoRepo := repository.NewVideoRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	transcodeRepo := repository.NewTranscodeRepository(db)

	videoService := service.NewVideoService(
		cfg, videoRepo, categoryRepo, transcodeRepo, objectStore, bus, videoCache, log)
	categoryService := service.NewCategoryService(categoryRepo, videoCache, log)
	transcodeWorker := service.NewTranscodeWorker(
		cfg, transcodeRepo, bus, objectStore, transcoder, videoCache, log)

	if err := transcodeWorker.Register(); err != nil {
		return fmt.Errorf("注册转码消费者失败: %w", err)
	}

	engine := router.New(cfg, log, router.Handlers{
		Video:    handler.NewVideoHandler(videoService),
		Category: handler.NewCategoryHandler(categoryService),
		Health: handler.NewHealthHandler(cfg.App.Name, cfg.App.Version, 3*time.Second, []handler.HealthCheck{
			{Name: "mysql", Fn: func(ctx context.Context) (string, error) {
				sqlDB, err := db.DB()
				if err != nil {
					return "", err
				}
				if err := sqlDB.PingContext(ctx); err != nil {
					return "", err
				}
				return "ok", nil
			}},
			{Name: "redis", Fn: func(ctx context.Context) (string, error) {
				if err := rdb.Ping(ctx).Err(); err != nil {
					return "", err
				}
				return "ok", nil
			}},
			{Name: "events", Fn: func(ctx context.Context) (string, error) {
				if err := bus.Health(ctx); err != nil {
					return "", err
				}
				return "driver=" + bus.Driver(), nil
			}},
			{Name: "minio", Fn: func(ctx context.Context) (string, error) {
				if err := objectStore.Health(ctx); err != nil {
					return "", err
				}
				return "bucket=" + objectStore.Bucket(), nil
			}},
		}),
	})

	// ---------------------------------------------------------- 后台任务
	bgCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()

	viewFlusher := service.NewViewFlusher(videoCache, videoRepo, cfg.View.FlushInterval, log)
	go viewFlusher.Run(bgCtx)

	// ---------------------------------------------------------- HTTP 服务
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("server_started",
			"addr", addr,
			"eventsDriver", bus.Driver(),
			"transcoder", transcoder.Name(),
			"bucket", objectStore.Bucket(),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	case sig := <-quit:
		log.Info("shutdown_signal_received", "signal", sig.String())
	}

	// ---------------------------------------------------------- 优雅退出
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server_shutdown_error", "error", err)
	}

	stopBackground()

	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	log.Info("server_stopped")
	return nil
}

// buildBus 构造事件总线：RabbitMQ 不可用时降级为进程内实现，保证开发环境可启动。
func buildBus(cfg *config.Config, log *slog.Logger) (events.Bus, error) {
	if cfg.Events.Driver == "local" {
		log.Warn("events_driver_local", "hint", "进程内事件总线不持久化、不跨进程，仅用于开发")
		return events.NewLocalBus(log), nil
	}

	bus, err := events.NewRabbitBus(cfg.Events, log)
	if err != nil {
		log.Warn("rabbitmq_unavailable_fallback_local",
			"error", err,
			"hint", "已降级为进程内事件总线；生产环境请确保 RabbitMQ 可用并设置 VIDEO_EVENTS_DRIVER=rabbitmq",
		)
		return events.NewLocalBus(log), nil
	}
	return bus, nil
}
