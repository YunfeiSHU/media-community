package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoadDefaultConfig 验证 configs/config.yaml 能被完整解析：
// 这同时覆盖了「${ENV:-default} 占位展开」与「time.Duration 字符串解析」两个最容易踩坑的点。
//
// 另外还顺带守住了 media.* 的数值端口：它们必须写成**不带引号**的 ${X:-default}，
// 一旦被写成 "${X:-1936}"，展开后是带引号的字符串，yaml.v3 无法解到 int 字段，
// 会在启动时直接报 `cannot unmarshal !!str '1936' into int`（曾真实发生）。
func TestLoadDefaultConfig(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载默认配置失败: %v", err)
	}

	if cfg.App.Name != "live-service" {
		t.Errorf("app.name = %q, 期望 live-service", cfg.App.Name)
	}
	if cfg.App.Port != 9002 {
		t.Errorf("app.port = %d, 期望 9002", cfg.App.Port)
	}
	if cfg.App.ShutdownTimeout != 10*time.Second {
		t.Errorf("app.shutdown_timeout = %v, 期望 10s", cfg.App.ShutdownTimeout)
	}
	if cfg.Events.Exchange != "go.media.live" {
		t.Errorf("events.exchange = %q, 期望 go.media.live", cfg.Events.Exchange)
	}
	if cfg.Events.Retry != 3 {
		t.Errorf("events.retry = %d, 期望 3", cfg.Events.Retry)
	}
	if cfg.Events.IdempotentTTL != time.Hour {
		t.Errorf("events.idempotent_ttl = %v, 期望 1h", cfg.Events.IdempotentTTL)
	}

	// 数值端口：展开成一个真正能解到 int 的值
	if cfg.Media.RTMPPort != 1936 {
		t.Errorf("media.rtmp_port = %d, 期望 1936", cfg.Media.RTMPPort)
	}
	if cfg.Media.HTTPPort != 8088 {
		t.Errorf("media.http_port = %d, 期望 8088", cfg.Media.HTTPPort)
	}
	if cfg.Media.WebRTCPort != 8001 {
		t.Errorf("media.webrtc_port = %d, 期望 8001", cfg.Media.WebRTCPort)
	}
	if cfg.Media.App != "live" {
		t.Errorf("media.app = %q, 期望 live（必须与 deploy/media/srs.conf 一致）", cfg.Media.App)
	}
	if cfg.Media.Driver != "mock" {
		t.Errorf("media.driver = %q, 期望 mock", cfg.Media.Driver)
	}

	if cfg.Room.MaxPerOwner != 20 {
		t.Errorf("room.max_per_owner = %d, 期望 20", cfg.Room.MaxPerOwner)
	}
	if cfg.Room.StartingTimeout != 60*time.Second {
		t.Errorf("room.starting_timeout = %v, 期望 60s", cfg.Room.StartingTimeout)
	}
	if cfg.Room.OnlineTTL != 60*time.Second {
		t.Errorf("room.online_ttl = %v, 期望 60s", cfg.Room.OnlineTTL)
	}
	if cfg.Room.CleanupInterval != 30*time.Second {
		t.Errorf("room.cleanup_interval = %v, 期望 30s", cfg.Room.CleanupInterval)
	}

	// 共库共 Redis 的硬约束：库名必须是 Java 那套 auth_service，Redis 必须落在 db 0
	if cfg.Redis.DB != 0 {
		t.Errorf("redis.db = %d, 期望 0（与 Java 共用同一实例）", cfg.Redis.DB)
	}
	if cfg.MySQL.DSN == "" || !strings.Contains(cfg.MySQL.DSN, "auth_service") {
		t.Errorf("mysql.dsn = %q, 期望指向 auth_service 库", cfg.MySQL.DSN)
	}

	if !cfg.IsDebug() {
		t.Errorf("默认 APP_ENV=local 应判定为 debug 模式")
	}
}

// TestLoadEnvOverride 验证环境变量优先级高于默认值。
func TestLoadEnvOverride(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.yaml")

	t.Setenv("LIVE_MYSQL_DSN", "user:pwd@tcp(127.0.0.1:3306)/custom_db")
	t.Setenv("APP_ENV", "docker")
	t.Setenv("LIVE_EVENTS_DRIVER", "local")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.MySQL.DSN != "user:pwd@tcp(127.0.0.1:3306)/custom_db" {
		t.Errorf("mysql.dsn 未被环境变量覆盖: %q", cfg.MySQL.DSN)
	}
	if cfg.IsDebug() {
		t.Errorf("APP_ENV=docker 不应判定为 debug")
	}
	if cfg.Events.Driver != "local" {
		t.Errorf("events.driver = %q, 期望 local", cfg.Events.Driver)
	}
}

// TestExpandEnvFallback 验证默认值语法（环境变量为空时回退到默认值）。
func TestExpandEnvFallback(t *testing.T) {
	os.Unsetenv("LIVE_NOT_SET_VAR")

	got := expandEnv("a=${LIVE_NOT_SET_VAR:-fallback} b=${LIVE_SET_VAR:-fallback}")
	want := "a=fallback b=fallback"
	if got != want {
		t.Errorf("expandEnv = %q, 期望 %q", got, want)
	}

	t.Setenv("LIVE_SET_VAR", "real")
	got = expandEnv("a=${LIVE_NOT_SET_VAR:-fallback} b=${LIVE_SET_VAR:-fallback}")
	want = "a=fallback b=real"
	if got != want {
		t.Errorf("expandEnv = %q, 期望 %q", got, want)
	}
}

// TestValidateRejectsUnknownDriver 验证非法驱动被拒绝。
func TestValidateRejectsUnknownDriver(t *testing.T) {
	cfg := &Config{}
	cfg.MySQL.DSN = "dsn"
	cfg.Redis.Addr = "127.0.0.1:6379"
	cfg.Events.Driver = "kafka"
	cfg.applyDefaults()
	if err := cfg.validate(); err == nil {
		t.Fatal("events.driver=kafka 应校验失败")
	}

	cfg2 := &Config{}
	cfg2.MySQL.DSN = "dsn"
	cfg2.Redis.Addr = "127.0.0.1:6379"
	cfg2.Media.Driver = "ffmpeg"
	cfg2.applyDefaults()
	if err := cfg2.validate(); err == nil {
		t.Fatal("media.driver=ffmpeg 应校验失败")
	}
}

// TestRequireMySQLAndRedis 验证 DSN / Redis 地址为空时启动即失败（配置错误前置暴露）。
func TestRequireMySQLAndRedis(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	if err := cfg.validate(); err == nil {
		t.Fatal("mysql.dsn 为空应校验失败")
	}

	cfg.MySQL.DSN = "dsn"
	if err := cfg.validate(); err == nil {
		t.Fatal("redis.addr 为空应校验失败")
	}
}
