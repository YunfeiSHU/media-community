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
	App        App        `yaml:"app"`
	MySQL      MySQL      `yaml:"mysql"`
	Redis      Redis      `yaml:"redis"`
	Events     Events     `yaml:"events"`
	MinIO      MinIO      `yaml:"minio"`
	Transcoder Transcoder `yaml:"transcoder"`
	Upload     Upload     `yaml:"upload"`
	View       View       `yaml:"view"`
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
type MySQL struct {
	DSN             string        `yaml:"dsn"`
	MaxOpen         int           `yaml:"max_open"`
	MaxIdle         int           `yaml:"max_idle"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	LogLevel        string        `yaml:"log_level"`
}

// Redis 是缓存配置。
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

// MinIO 是对象存储配置。
type MinIO struct {
	Endpoint       string `yaml:"endpoint"`
	PublicEndpoint string `yaml:"public_endpoint"`
	AccessKey      string `yaml:"access_key"`
	SecretKey      string `yaml:"secret_key"`
	Bucket         string `yaml:"bucket"`
	UseSSL         bool   `yaml:"use_ssl"`
	Region         string `yaml:"region"`
}

// Transcoder 是转码器配置。
type Transcoder struct {
	Driver      string        `yaml:"driver"`
	FFmpegPath  string        `yaml:"ffmpeg_path"`
	WorkDir     string        `yaml:"work_dir"`
	Timeout     time.Duration `yaml:"timeout"`
	Concurrency int           `yaml:"concurrency"`
}

// Upload 是上传限制与分片参数配置。
type Upload struct {
	MaxSizeMB  int      `yaml:"max_size_mb"`
	AllowedExt []string `yaml:"allowed_ext"`
	// PartSizeMB 是服务端建议的分片大小；客户端可以不传 partSize，由服务端按文件大小推导。
	PartSizeMB int `yaml:"part_size_mb"`
	// MinPartSizeMB 是 S3 协议下限（非末片不得小于 5MiB），配置只是显式化，不可改小。
	MinPartSizeMB int `yaml:"min_part_size_mb"`
	// MaxParts 是单次上传允许的最大分片数（S3 协议上限 10000）。
	MaxParts int `yaml:"max_parts"`
}

// View 是播放量统计配置。
type View struct {
	FlushInterval  time.Duration `yaml:"flush_interval"`
	DetailCacheTTL time.Duration `yaml:"detail_cache_ttl"`
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

// IsDebug 表示是否以开发模式运行（文本日志、gin debug）。
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
		c.App.Port = 9001
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
		c.Events.Exchange = "go.media.video"
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
	if c.MinIO.Bucket == "" {
		c.MinIO.Bucket = "media"
	}
	if c.MinIO.PublicEndpoint == "" {
		scheme := "http"
		if c.MinIO.UseSSL {
			scheme = "https"
		}
		c.MinIO.PublicEndpoint = fmt.Sprintf("%s://%s", scheme, c.MinIO.Endpoint)
	}
	if c.Transcoder.Driver == "" {
		c.Transcoder.Driver = "mock"
	}
	if c.Transcoder.FFmpegPath == "" {
		c.Transcoder.FFmpegPath = "ffmpeg"
	}
	if c.Transcoder.WorkDir == "" {
		c.Transcoder.WorkDir = "./.work"
	}
	if c.Transcoder.Timeout <= 0 {
		c.Transcoder.Timeout = 30 * time.Minute
	}
	if c.Transcoder.Concurrency <= 0 {
		c.Transcoder.Concurrency = 2
	}
	if c.Upload.MaxSizeMB == 0 {
		c.Upload.MaxSizeMB = 2048
	}
	if len(c.Upload.AllowedExt) == 0 {
		c.Upload.AllowedExt = []string{"mp4", "mov", "mkv", "avi", "flv", "webm"}
	}
	if c.Upload.PartSizeMB <= 0 {
		c.Upload.PartSizeMB = 8
	}
	if c.Upload.MinPartSizeMB <= 0 {
		c.Upload.MinPartSizeMB = 5
	}
	// S3 协议硬上限：分片数不能超过 10000，配置写大了也没意义。
	if c.Upload.MaxParts <= 0 || c.Upload.MaxParts > 10000 {
		c.Upload.MaxParts = 10000
	}
	if c.View.FlushInterval <= 0 {
		c.View.FlushInterval = 30 * time.Second
	}
	if c.View.DetailCacheTTL <= 0 {
		c.View.DetailCacheTTL = 5 * time.Minute
	}
}

func (c *Config) validate() error {
	if c.MySQL.DSN == "" {
		return fmt.Errorf("mysql.dsn 不能为空")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis.addr 不能为空")
	}
	if c.MinIO.Endpoint == "" {
		return fmt.Errorf("minio.endpoint 不能为空")
	}
	switch c.Events.Driver {
	case "rabbitmq", "local":
	default:
		return fmt.Errorf("events.driver 只支持 rabbitmq|local，当前为 %q", c.Events.Driver)
	}
	switch c.Transcoder.Driver {
	case "mock", "real":
	default:
		return fmt.Errorf("transcoder.driver 只支持 mock|real，当前为 %q", c.Transcoder.Driver)
	}
	return nil
}
