// Command server 是 live-service-go 的进程入口。
//
// 装配顺序：配置 → 日志 → MySQL（含建表）→ Redis → 事件总线 → 媒体服务适配器
//
//	→ 仓储 → 服务 → handler → 路由 → 后台任务（开播超时扫描）→ 启动 → 优雅退出
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

	"github.com/example/live-service-go/internal/config"
	"github.com/example/live-service-go/internal/handler"
	"github.com/example/live-service-go/internal/infra/cache"
	"github.com/example/live-service-go/internal/infra/events"
	"github.com/example/live-service-go/internal/infra/media"
	"github.com/example/live-service-go/internal/infra/router"
	"github.com/example/live-service-go/internal/repository"
	"github.com/example/live-service-go/internal/service"
	"github.com/example/live-service-go/pkg/logx"
	"github.com/example/live-service-go/pkg/migrate"
	"github.com/example/live-service-go/pkg/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[live-service] 启动失败: %v\n", err)
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
		"mediaDriver", cfg.Media.Driver,
		"eventsDriver", cfg.Events.Driver,
		// 明确暴露「共用数据层」的事实，便于排障时一眼确认连的是不是 Java 那套
		"redisDB", cfg.Redis.DB,
	)

	// ---------------------------------------------------------- 基础设施
	db, err := store.NewMySQL(cfg.MySQL)
	if err != nil {
		return err
	}
	if err := migrate.AutoMigrate(db, log); err != nil {
		return err
	}

	rdb, err := store.NewRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	bus, err := buildBus(cfg, log)
	if err != nil {
		return err
	}
	defer func() { _ = bus.Close() }()

	mediaServer := media.New(cfg.Media)
	if mediaServer.Name() == "mock" {
		log.Warn("media_driver_mock_enabled",
			"hint", "当前使用 mock 媒体服务：地址格式真实但没有 SRS 在监听，请用 "+
				"POST /api/v1/live/dev/publish/{streamKey} 模拟推流回调")
	}

	// ---------------------------------------------------------- 装配
	roomCache := cache.NewRoomCache(rdb, cfg.Room.OnlineTTL, cfg.Room.DetailCacheTTL, log)
	roomRepo := repository.NewRoomRepository(db)
	recordRepo := repository.NewRecordRepository(db)

	roomService := service.NewRoomService(cfg, roomRepo, recordRepo, roomCache, mediaServer, log)
	liveService := service.NewLiveService(cfg, roomRepo, recordRepo, roomCache, mediaServer, bus, log)
	statsService := service.NewStatsService(cfg, roomRepo, recordRepo, roomCache, mediaServer, bus, roomCache, log)

	if err := statsService.Register(); err != nil {
		return fmt.Errorf("注册事件消费者失败: %w", err)
	}

	engine := router.New(cfg, log, router.Handlers{
		Room: handler.NewRoomHandler(roomService),
		Live: handler.NewLiveHandler(liveService, statsService),
		Hook: handler.NewHookHandler(liveService),
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
			{Name: "srs", Fn: func(ctx context.Context) (string, error) {
				// mock 实现恒返回 nil：本地没起 SRS 时 /readyz 不应永远是红的
				if err := mediaServer.Health(ctx); err != nil {
					return "", err
				}
				return "driver=" + mediaServer.Name(), nil
			}},
		}),
	})

	// ---------------------------------------------------------- 后台任务
	bgCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()

	go liveService.RunTimeoutSweeper(bgCtx)

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
			"mediaDriver", mediaServer.Name(),
			"srsApp", cfg.Media.App,
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
			"hint", "已降级为进程内事件总线；生产环境请确保 RabbitMQ 可用并设置 LIVE_EVENTS_DRIVER=rabbitmq",
		)
		return events.NewLocalBus(log), nil
	}
	return bus, nil
}
