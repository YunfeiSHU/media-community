package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadDefaultConfig 验证 configs/config.yaml 能被完整解析：
// 这同时覆盖了「${ENV:-default} 占位展开」与「time.Duration 字符串解析」两个最容易踩坑的点。
func TestLoadDefaultConfig(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载默认配置失败: %v", err)
	}

	if cfg.App.Name != "video-service" {
		t.Errorf("app.name = %q, 期望 video-service", cfg.App.Name)
	}
	if cfg.App.Port != 9001 {
		t.Errorf("app.port = %d, 期望 9001", cfg.App.Port)
	}
	if cfg.App.ShutdownTimeout != 10*time.Second {
		t.Errorf("app.shutdown_timeout = %v, 期望 10s", cfg.App.ShutdownTimeout)
	}
	if cfg.Transcoder.Timeout != 30*time.Minute {
		t.Errorf("transcoder.timeout = %v, 期望 30m", cfg.Transcoder.Timeout)
	}
	if cfg.View.FlushInterval != 30*time.Second {
		t.Errorf("view.flush_interval = %v, 期望 30s", cfg.View.FlushInterval)
	}
	if cfg.Events.Exchange != "go.media.video" {
		t.Errorf("events.exchange = %q, 期望 go.media.video", cfg.Events.Exchange)
	}
	if len(cfg.Upload.AllowedExt) != 6 {
		t.Errorf("upload.allowed_ext 长度 = %d, 期望 6", len(cfg.Upload.AllowedExt))
	}
	if !cfg.IsDebug() {
		t.Errorf("默认 APP_ENV=local 应判定为 debug 模式")
	}
}

// TestLoadEnvOverride 验证环境变量优先级高于默认值。
func TestLoadEnvOverride(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.yaml")

	t.Setenv("VIDEO_MYSQL_DSN", "user:pwd@tcp(127.0.0.1:3306)/custom_db")
	t.Setenv("APP_ENV", "docker")
	t.Setenv("VIDEO_EVENTS_DRIVER", "local")

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
	os.Unsetenv("VIDEO_NOT_SET_VAR")

	got := expandEnv("a=${VIDEO_NOT_SET_VAR:-fallback} b=${VIDEO_SET_VAR:-fallback}")
	want := "a=fallback b=fallback"
	if got != want {
		t.Errorf("expandEnv = %q, 期望 %q", got, want)
	}

	t.Setenv("VIDEO_SET_VAR", "real")
	got = expandEnv("a=${VIDEO_NOT_SET_VAR:-fallback} b=${VIDEO_SET_VAR:-fallback}")
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
	cfg.MinIO.Endpoint = "127.0.0.1:9000"
	cfg.Events.Driver = "kafka"
	cfg.applyDefaults()
	if err := cfg.validate(); err == nil {
		t.Fatal("events.driver=kafka 应校验失败")
	}
}
