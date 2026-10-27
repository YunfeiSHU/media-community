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
	App    App    `yaml:"app"`
	MySQL  MySQL  `yaml:"mysql"`
	Redis  Redis  `yaml:"redis"`
	Events Events `yaml:"events"`
	Media  Media  `yaml:"media"`
	Room   Room   `yaml:"room"`
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
// 注意：与 Java 体系共用同一个实例与 db 0，靠 live: 前缀隔离。
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

// Media 是 SRS 适配器配置。
type Media struct {
	Driver     string `yaml:"driver"`
	RTMPHost   string `yaml:"rtmp_host"`
	RTMPPort   int    `yaml:"rtmp_port"`
	HTTPPort   int    `yaml:"http_port"`
	WebRTCHost string `yaml:"webrtc_host"`
	WebRTCPort int    `yaml:"webrtc_port"`
	APIBase    string `yaml:"api_base"`
	App        string `yaml:"app"`
	HookSecret string `yaml:"hook_secret"`
	// PublicBase 是回给浏览器的播放地址前缀（如 https://site/live）。
	//
	// 为空时退回「http://rtmp_host:http_port」——那是**直连 SRS** 的旧行为，
	// 只适合本地调试；一旦页面是 HTTPS，http 媒体会被浏览器按混合内容拦截，
	// 且 localhost 在别的机器上指向用户自己的电脑。
	PublicBase string `yaml:"public_base"`
}

// Room 是直播间相关的调参。
type Room struct {
	MaxPerOwner     int           `yaml:"max_per_owner"`
	StartingTimeout time.Duration `yaml:"starting_timeout"`
	OnlineTTL       time.Duration `yaml:"online_ttl"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	DetailCacheTTL  time.Duration `yaml:"detail_cache_ttl"`
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

// IsDebug 表示是否以开发模式运行（文本日志、gin debug、注册 dev 路由）。
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
		c.App.Port = 9002
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
		c.Events.Exchange = "go.media.live"
	}
	if c.Events.Prefetch == 0 {
		c.Events.Prefetch = 10
	}
	if c.Events.Retry == 0 {
		c.Events.Retry = 3
	}
	if c.Events.IdempotentTTL <= 0 {
		c.Events.IdempotentTTL = time.Hour
	}
	if c.Media.Driver == "" {
		c.Media.Driver = "mock"
	}
	if c.Media.RTMPHost == "" {
		c.Media.RTMPHost = "localhost"
	}
	if c.Media.RTMPPort == 0 {
		c.Media.RTMPPort = 1936
	}
	if c.Media.HTTPPort == 0 {
		c.Media.HTTPPort = 8088
	}
	if c.Media.WebRTCHost == "" {
		c.Media.WebRTCHost = "localhost"
	}
	if c.Media.WebRTCPort == 0 {
		c.Media.WebRTCPort = 8001
	}
	if c.Media.App == "" {
		c.Media.App = "live"
	}
	if c.Room.MaxPerOwner == 0 {
		c.Room.MaxPerOwner = 20
	}
	if c.Room.StartingTimeout <= 0 {
		c.Room.StartingTimeout = 60 * time.Second
	}
	if c.Room.OnlineTTL <= 0 {
		c.Room.OnlineTTL = 60 * time.Second
	}
	if c.Room.CleanupInterval <= 0 {
		c.Room.CleanupInterval = 30 * time.Second
	}
	if c.Room.DetailCacheTTL <= 0 {
		c.Room.DetailCacheTTL = 30 * time.Second
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
	switch strings.ToLower(c.Media.Driver) {
	case "mock", "srs":
	default:
		return fmt.Errorf("media.driver 仅支持 mock | srs，当前为 %q", c.Media.Driver)
	}
	return nil
}
