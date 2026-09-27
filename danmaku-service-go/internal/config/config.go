// Package config 负责加载服务配置。
//
// 设计：YAML 文件 + ${ENV:-default} 占位展开。
// 之所以不引入 viper：配置来源只有「文件 + 环境变量」两种，自己实现 20 行即可，
// 且占位语法可以让 config.yaml 自身携带可用的默认值，开箱即跑。
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// envPattern 匹配 ${VAR} 与 ${VAR:-default} 两种写法。
var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// Config 是服务的全量配置。
type Config struct {
	App       App       `yaml:"app"`
	MySQL     MySQL     `yaml:"mysql"`
	Redis     Redis     `yaml:"redis"`
	Events    Events    `yaml:"events"`
	Danmaku   Danmaku   `yaml:"danmaku"`
	WS        WS        `yaml:"ws"`
	RateLimit RateLimit `yaml:"ratelimit"`
	Filter    Filter    `yaml:"filter"`
}

// App 是进程级配置。
type App struct {
	Name            string        `yaml:"name"`
	Version         string        `yaml:"version"`
	Port            int           `yaml:"port"`
	Mode            string        `yaml:"mode"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// MySQL 是数据库配置。
//
// 注意：本服务与 Java 体系共用同一个 MySQL 实例与同一个 auth_service 库，
// 表名统一加 media_ 前缀（见 internal/model）。
type MySQL struct {
	DSN             string        `yaml:"dsn"`
	MaxOpen         int           `yaml:"max_open"`
	MaxIdle         int           `yaml:"max_idle"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	LogLevel        string        `yaml:"log_level"`
}

// Redis 是缓存配置。
//
// 注意：与 Java 体系共用同一个实例与 db 0，靠 danmaku: 前缀隔离。
type Redis struct {
	Addr        string        `yaml:"addr"`
	Password    string        `yaml:"password"`
	DB          int           `yaml:"db"`
	PoolSize    int           `yaml:"pool_size"`
	DialTimeout time.Duration `yaml:"dial_timeout"`
}

// Events 是事件总线配置。
type Events struct {
	Driver        string        `yaml:"driver"`
	URL           string        `yaml:"url"`
	Exchange      string        `yaml:"exchange"`
	Prefetch      int           `yaml:"prefetch"`
	Retry         int           `yaml:"retry"`
	IdempotentTTL time.Duration `yaml:"idempotent_ttl"`
}

// Danmaku 是弹幕业务参数。
type Danmaku struct {
	MaxContentLen   int           `yaml:"max_content_len"`
	HotSize         int64         `yaml:"hot_size"`
	HotTTL          time.Duration `yaml:"hot_ttl"`
	RecallWindow    time.Duration `yaml:"recall_window"`
	DedupTTL        time.Duration `yaml:"dedup_ttl"`
	RequireLiving   bool          `yaml:"require_living"`
	MaxPersistRetry int           `yaml:"max_persist_retry"`
}

// WS 是 WebSocket 相关参数。
type WS struct {
	ReadBufferSize  int           `yaml:"read_buffer_size"`
	WriteBufferSize int           `yaml:"write_buffer_size"`
	WriteWait       time.Duration `yaml:"write_wait"`
	PongWait        time.Duration `yaml:"pong_wait"`
	PingPeriod      time.Duration `yaml:"ping_period"`
	MaxConnsPerUser int           `yaml:"max_conns_per_user"`
	MaxFrameBytes   int64         `yaml:"max_frame_bytes"`
	RoomIdleGrace   time.Duration `yaml:"room_idle_grace"`
	JanitorInterval time.Duration `yaml:"janitor_interval"`
	AllowedOrigins  string        `yaml:"allowed_origins"`
}

// OriginAllowed 判断某个 Origin 是否允许（空配置或 * 表示不校验）。
func (w WS) OriginAllowed(origin string) bool {
	raw := strings.TrimSpace(w.AllowedOrigins)
	if raw == "" || raw == "*" {
		return true
	}
	for _, item := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(item), origin) {
			return true
		}
	}
	return false
}

// RateLimit 是限流参数。
type RateLimit struct {
	UserPerSecond int           `yaml:"user_per_second"`
	RoomPerSecond int           `yaml:"room_per_second"`
	Window        time.Duration `yaml:"window"`
}

// Filter 是敏感词过滤器参数。
type Filter struct {
	Enabled         bool          `yaml:"enabled"`
	DefaultStrategy string        `yaml:"default_strategy"`
	ReloadInterval  time.Duration `yaml:"reload_interval"`
}

// Load 读取配置文件并展开环境变量占位，最后补齐缺省值。
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	expanded := expandEnv(string(raw))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// expandEnv 展开 ${VAR} / ${VAR:-default}；环境变量存在且非空时优先取环境变量。
func expandEnv(in string) string {
	return envPattern.ReplaceAllStringFunc(in, func(match string) string {
		groups := envPattern.FindStringSubmatch(match)
		name, def := groups[1], groups[2]
		if v, ok := os.LookupEnv(name); ok && v != "" {
			return v
		}
		return def
	})
}

// IsDebug 表示是否以开发模式运行（文本日志、gin debug、详细错误）。
func (c *Config) IsDebug() bool {
	switch strings.ToLower(c.App.Mode) {
	case "local", "dev", "debug":
		return true
	default:
		return false
	}
}

func (c *Config) applyDefaults() {
	if c.App.Port == 0 {
		c.App.Port = 9003
	}
	if c.App.Mode == "" {
		c.App.Mode = "local"
	}
	if c.App.ShutdownTimeout <= 0 {
		c.App.ShutdownTimeout = 10 * time.Second
	}
	if c.MySQL.MaxOpen == 0 {
		c.MySQL.MaxOpen = 50
	}
	if c.MySQL.MaxIdle == 0 {
		c.MySQL.MaxIdle = 10
	}
	if c.MySQL.ConnMaxLifetime <= 0 {
		c.MySQL.ConnMaxLifetime = 30 * time.Minute
	}
	if c.MySQL.LogLevel == "" {
		c.MySQL.LogLevel = "warn"
	}
	if c.Redis.PoolSize == 0 {
		c.Redis.PoolSize = 20
	}
	if c.Redis.DialTimeout <= 0 {
		c.Redis.DialTimeout = 3 * time.Second
	}
	if c.Events.Driver == "" {
		c.Events.Driver = "rabbitmq"
	}
	if c.Events.Exchange == "" {
		c.Events.Exchange = "go.media.danmaku"
	}
	if c.Events.Prefetch == 0 {
		c.Events.Prefetch = 20
	}
	if c.Events.Retry == 0 {
		c.Events.Retry = 3
	}
	if c.Events.IdempotentTTL <= 0 {
		c.Events.IdempotentTTL = time.Hour
	}
	if c.Danmaku.MaxContentLen == 0 {
		c.Danmaku.MaxContentLen = 100
	}
	if c.Danmaku.HotSize == 0 {
		c.Danmaku.HotSize = 500
	}
	if c.Danmaku.HotTTL <= 0 {
		c.Danmaku.HotTTL = 2 * time.Hour
	}
	if c.Danmaku.RecallWindow < 0 {
		c.Danmaku.RecallWindow = 0
	}
	if c.Danmaku.DedupTTL <= 0 {
		c.Danmaku.DedupTTL = 10 * time.Second
	}
	if c.Danmaku.MaxPersistRetry <= 0 {
		c.Danmaku.MaxPersistRetry = 3
	}
	if c.WS.WriteBufferSize <= 0 {
		c.WS.WriteBufferSize = 4096
	}
	if c.WS.ReadBufferSize <= 0 {
		c.WS.ReadBufferSize = 1024
	}
	if c.WS.WriteWait <= 0 {
		c.WS.WriteWait = 10 * time.Second
	}
	if c.WS.PongWait <= 0 {
		c.WS.PongWait = 60 * time.Second
	}
	if c.WS.PingPeriod <= 0 {
		c.WS.PingPeriod = c.WS.PongWait * 9 / 10
	}
	if c.WS.PingPeriod >= c.WS.PongWait {
		c.WS.PingPeriod = c.WS.PongWait / 2
	}
	if c.WS.MaxConnsPerUser <= 0 {
		c.WS.MaxConnsPerUser = 3
	}
	if c.WS.MaxFrameBytes <= 0 {
		c.WS.MaxFrameBytes = 4096
	}
	if c.WS.RoomIdleGrace <= 0 {
		c.WS.RoomIdleGrace = 60 * time.Second
	}
	if c.WS.JanitorInterval <= 0 {
		c.WS.JanitorInterval = 30 * time.Second
	}
	if c.RateLimit.UserPerSecond <= 0 {
		c.RateLimit.UserPerSecond = 5
	}
	if c.RateLimit.RoomPerSecond <= 0 {
		c.RateLimit.RoomPerSecond = 200
	}
	if c.RateLimit.Window <= 0 {
		c.RateLimit.Window = time.Second
	}
	if c.Filter.DefaultStrategy == "" {
		c.Filter.DefaultStrategy = "REJECT"
	}
}

// validate 校验必需项与枚举取值，让配置错误在启动阶段就暴露。
func (c *Config) validate() error {
	if strings.TrimSpace(c.MySQL.DSN) == "" {
		return fmt.Errorf("mysql.dsn 不能为空")
	}
	if strings.TrimSpace(c.Redis.Addr) == "" {
		return fmt.Errorf("redis.addr 不能为空")
	}
	switch strings.ToLower(c.Events.Driver) {
	case "rabbitmq", "local":
	default:
		return fmt.Errorf("events.driver 仅支持 rabbitmq | local，当前为 %q", c.Events.Driver)
	}
	switch strings.ToUpper(c.Filter.DefaultStrategy) {
	case "REJECT", "REPLACE", "LOG":
	default:
		return fmt.Errorf("filter.default_strategy 仅支持 REJECT | REPLACE | LOG，当前为 %q", c.Filter.DefaultStrategy)
	}
	return nil
}
