// Command server 是 danmaku-service-go 的进程入口。
//
// 装配顺序：配置 → 日志 → MySQL（含建表）→ Redis → 事件总线 → 敏感词过滤器
//
//	→ WebSocket Hub → 缓存/限流 → 仓储 → 服务 → handler → 路由
//	→ 后台任务（敏感词预热、补发重投、房间回收、连接数日志）→ 启动 → 优雅退出
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

	"github.com/example/danmaku-service-go/internal/config"
	"github.com/example/danmaku-service-go/internal/handler"
	"github.com/example/danmaku-service-go/internal/infra/cache"
	"github.com/example/danmaku-service-go/internal/infra/events"
	"github.com/example/danmaku-service-go/internal/infra/filter"
	"github.com/example/danmaku-service-go/internal/infra/hub"
	"github.com/example/danmaku-service-go/internal/infra/ratelimit"
	"github.com/example/danmaku-service-go/internal/infra/router"
	"github.com/example/danmaku-service-go/internal/repository"
	"github.com/example/danmaku-service-go/internal/service"
	"github.com/example/danmaku-service-go/pkg/logx"
	"github.com/example/danmaku-service-go/pkg/migrate"
	"github.com/example/danmaku-service-go/pkg/store"
)

// hubStatsInterval 是连接数日志的输出间隔（FR-7.2）。
const hubStatsInterval = 30 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[danmaku-service] 启动失败: %v\n", err)
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
		"eventsDriver", cfg.Events.Driver,
		"filterEnabled", cfg.Filter.Enabled,
		// 明确暴露「共用数据层」的事实，排障时一眼确认连的是不是 Java 那一套
		"redisDB", cfg.Redis.DB,
		"requireLiving", cfg.Danmaku.RequireLiving,
	)

	// ---------------------------------------------------------- 基础设施
	db, err := store.NewMySQL(cfg.MySQL)
	if err != nil {
		return err
	}
	if err := migrate.AutoMigrate(db, log); err != nil {
		return fmt.Errorf("建表失败: %w", err)
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

	// ---------------------------------------------------------- 装配
	filterEngine := filter.New(
		cfg.Filter.Enabled,
		filter.ParseStrategy(cfg.Filter.DefaultStrategy, filter.StrategyReject),
		log,
	)

	wsHub := hub.New(hub.Config{
		ReadBufferSize:  cfg.WS.ReadBufferSize,
		WriteBufferSize: cfg.WS.WriteBufferSize,
		WriteWait:       cfg.WS.WriteWait,
		PongWait:        cfg.WS.PongWait,
		PingPeriod:      cfg.WS.PingPeriod,
		MaxConnsPerUser: cfg.WS.MaxConnsPerUser,
		MaxFrameBytes:   cfg.WS.MaxFrameBytes,
		RoomIdleGrace:   cfg.WS.RoomIdleGrace,
	}, log)

	recent := cache.NewRecentCache(rdb, cfg.Danmaku.HotSize, cfg.Danmaku.HotTTL, cfg.Danmaku.DedupTTL, log)
	limiter := ratelimit.New(rdb, cfg.RateLimit.UserPerSecond, cfg.RateLimit.RoomPerSecond, cfg.RateLimit.Window, log)

	deps := service.Deps{
		Config:  cfg,
		Danmaku: repository.NewDanmakuRepository(db),
		Words:   repository.NewWordRepository(db),
		Recent:  recent,
		Limiter: limiter,
		Filter:  filterEngine,
		Hub:     wsHub,
		Bus:     bus,
		Log:     log,
	}

	danmakuService := service.NewDanmakuService(deps)
	persistWorker := service.NewPersistWorker(deps)
	liveBridge := service.NewLiveBridge(deps, persistWorker)

	if err := persistWorker.Register(); err != nil {
		return fmt.Errorf("订阅落库事件失败: %w", err)
	}
	if err := liveBridge.Register(); err != nil {
		return fmt.Errorf("订阅直播事件失败: %w", err)
	}

	// 敏感词预热：失败只告警。词库拉不到时过滤器会放行全部内容，
	// 而不是让整个服务起不来（可用性优先于内容安全，且 /readyz 会暴露 filter 状态）。
	if cfg.Filter.Enabled {
		if _, err := danmakuService.LoadWords(context.Background()); err != nil {
			log.Warn("sensitive_word_preload_failed", "err", err.Error(),
				"hint", "过滤器将放行全部内容；请检查 media_sensitive_word 表是否可读")
		}
	} else {
		log.Warn("filter_disabled", "hint", "敏感词过滤已关闭，仅应在本地调试时使用")
	}

	// ---------------------------------------------------------- 路由
	engine := router.New(cfg, log, router.Handlers{
		Danmaku: handler.NewDanmakuHandler(danmakuService),
		History: handler.NewHistoryHandler(danmakuService),
		Admin:   handler.NewAdminHandler(danmakuService),
		WS:      handler.NewWSHandler(cfg, wsHub, danmakuService, log),
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
				if err := recent.Ping(ctx); err != nil {
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
			{Name: "filter", Fn: func(_ context.Context) (string, error) {
				// 暴露已加载词数：排查「敏感词为什么没生效」最直接的手段。
				return fmt.Sprintf("enabled=%t words=%d strategy=%s",
					filterEngine.Enabled(), filterEngine.Count(), filterEngine.DefaultStrategy()), nil
			}},
		}),
	})

	// ---------------------------------------------------------- 后台任务
	bgCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()

	go wsHub.RunJanitor(bgCtx)
	go persistWorker.RunRepublisher(bgCtx)
	go logHubStats(bgCtx, wsHub, log)

	// ---------------------------------------------------------- HTTP 服务
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		// 注意：不要设置 WriteTimeout / IdleTimeout 为较小值
		// —— WebSocket 是长连接，会被这两项直接掐断。
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("server_started",
			"addr", addr,
			"eventsDriver", bus.Driver(),
			"hubBackend", wsHub.Backend().Name(),
			"words", filterEngine.Count(),
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
	// 先通知所有 WS 客户端「服务端要关了」，客户端据此做退避重连而不是立刻重试。
	if n := wsHub.ClientCount(); n > 0 {
		log.Info("closing_ws_connections", "count", n)
		wsHub.CloseAll("服务端正在重启，请稍后重连", hub.CloseGoingAway)
	}

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

// logHubStats 周期性输出各房间连接数（FR-7.2）。
//
// 连接数是这个服务最关键的运行时指标：出问题时第一个要看的就是
// 「房间数 / 连接数 / 丢弃帧数」这三个数。
func logHubStats(ctx context.Context, wsHub *hub.Hub, log *slog.Logger) {
	ticker := time.NewTicker(hubStatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Info("hub_stats",
				"rooms", wsHub.RoomCount(),
				"clients", wsHub.ClientCount(),
				"droppedTotal", wsHub.Dropped(),
				"perRoom", wsHub.Stats(),
			)
		}
	}
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
			"hint", "已降级为进程内事件总线；弹幕将无法异步落库，生产环境请确保 RabbitMQ 可用并设置 DANMAKU_EVENTS_DRIVER=rabbitmq",
		)
		return events.NewLocalBus(log), nil
	}
	return bus, nil
}
