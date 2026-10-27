package store

import (
	"context"
	"fmt"
	"time"

	"github.com/example/live-service-go/internal/config"
	"github.com/redis/go-redis/v9"
)

// NewRedis 建立 Redis 连接并 Ping 校验。
func NewRedis(cfg config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis %s 失败: %w", cfg.Addr, err)
	}
	return client, nil
}
