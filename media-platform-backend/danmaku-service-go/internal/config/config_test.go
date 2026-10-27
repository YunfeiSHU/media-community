package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写临时配置失败: %v", err)
	}
	return path
}

const minimalConfig = `
app:
  name: danmaku-service
  version: v1.0.0
  port: 9003
mysql:
  dsn: "root:root123456@tcp(127.0.0.1:3306)/auth_service?charset=utf8mb4&parseTime=True&loc=Local"
redis:
  addr: "127.0.0.1:6379"
  db: 0
events:
  exchange: go.media.danmaku
`

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimalConfig))
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.App.Mode != "local" {
		t.Fatalf("mode 默认应为 local，实际 %q", cfg.App.Mode)
	}
	if cfg.MySQL.MaxOpen != 50 || cfg.MySQL.MaxIdle != 10 {
		t.Fatalf("MySQL 连接池默认值错误: %+v", cfg.MySQL)
	}
	if cfg.Redis.DB != 0 {
		t.Fatalf("Redis 必须沿用 Java 那一套的 db 0，实际 %d", cfg.Redis.DB)
	}
	if cfg.Events.Driver != "rabbitmq" {
		t.Fatalf("events.driver 默认应为 rabbitmq，实际 %q", cfg.Events.Driver)
	}
	if cfg.Danmaku.MaxContentLen != 100 || cfg.Danmaku.HotSize != 500 {
		t.Fatalf("弹幕默认参数错误: %+v", cfg.Danmaku)
	}
	if cfg.Danmaku.HotTTL != 2*time.Hour || cfg.Danmaku.DedupTTL != 10*time.Second {
		t.Fatalf("弹幕默认 TTL 错误: %+v", cfg.Danmaku)
	}
	if cfg.Danmaku.RequireLiving {
		t.Fatal("require_living 默认必须为 false，否则本地无 live-service 时全站发不出弹幕")
	}
	if cfg.RateLimit.UserPerSecond != 5 || cfg.RateLimit.RoomPerSecond != 200 {
		t.Fatalf("限流默认值错误: %+v", cfg.RateLimit)
	}
	if cfg.Filter.DefaultStrategy != "REJECT" {
		t.Fatalf("默认策略应为 REJECT，实际 %q", cfg.Filter.DefaultStrategy)
	}
	// PingPeriod 必须小于 PongWait，否则连接会被自己的心跳判死。
	if cfg.WS.PingPeriod >= cfg.WS.PongWait {
		t.Fatalf("ping_period(%s) 必须小于 pong_wait(%s)", cfg.WS.PingPeriod, cfg.WS.PongWait)
	}
	if !cfg.IsDebug() {
		t.Fatal("local 模式应被视为 debug")
	}
}

func TestLoadExpandsEnvPlaceholders(t *testing.T) {
	t.Setenv("DANMAKU_TEST_PORT", "19003")

	path := writeConfig(t, minimalConfig+`
danmaku:
  require_living: true
ws:
  write_buffer_size: ${DANMAKU_TEST_PORT}
  pong_wait: ${DANMAKU_MISSING:-90s}
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.WS.WriteBufferSize != 19003 {
		t.Fatalf("环境变量占位未展开: %d", cfg.WS.WriteBufferSize)
	}
	if cfg.WS.PongWait != 90*time.Second {
		t.Fatalf("缺省值占位未生效: %s", cfg.WS.PongWait)
	}
	if !cfg.Danmaku.RequireLiving {
		t.Fatal("显式配置的 require_living 应生效")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := map[string]string{
		"缺 dsn":  minimalConfig + "\nmysql:\n  dsn: \"\"\n",
		"非法事件驱动": minimalConfig + "\nevents:\n  driver: kafka\n",
		"非法默认策略": minimalConfig + "\nfilter:\n  default_strategy: BLOCK\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, content)); err == nil {
				t.Fatal("期望校验失败，实际通过了")
			}
		})
	}
}

func TestOriginAllowed(t *testing.T) {
	any := WS{AllowedOrigins: "*"}
	if !any.OriginAllowed("https://evil.example") {
		t.Fatal("* 表示不校验来源")
	}

	list := WS{AllowedOrigins: "https://a.example, https://b.example"}
	if !list.OriginAllowed("https://b.example") {
		t.Fatal("白名单内的来源应放行")
	}
	if list.OriginAllowed("https://c.example") {
		t.Fatal("白名单外的来源应拒绝")
	}

	empty := WS{}
	if !empty.OriginAllowed("https://any.example") {
		t.Fatal("未配置时不应校验来源（与 config.yaml 默认值一致）")
	}
}
