// Package cache 封装 Redis 缓存与计数缓冲。
//
// 所有方法遵循同一原则：**缓存故障不阻断主流程**。
// 读失败退化为查库，写失败靠 TTL 兜底，只记 warning 日志。
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/example/video-service-go/api/dto"
	"github.com/redis/go-redis/v9"
)

// nilMarker 用于缓存穿透保护：把「不存在」也缓存起来。
const nilMarker = "__nil__"

// categoryListKey 是分类列表缓存键。
const categoryListKey = "video:cate:all"

// viewKeyPrefix 是播放量缓冲键前缀。
const viewKeyPrefix = "video:view:"

// flushViewScript 原子地「读取并删除」播放量计数，避免读-删间隙丢增量。
var flushViewScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v then
  redis.call('DEL', KEYS[1])
end
return v
`)

// VideoCache 提供视频详情、分类列表缓存与播放量缓冲。
type VideoCache struct {
	rdb *redis.Client
	ttl time.Duration
	log *slog.Logger
}

// NewVideoCache 构造缓存。
func NewVideoCache(rdb *redis.Client, ttl time.Duration, log *slog.Logger) *VideoCache {
	return &VideoCache{rdb: rdb, ttl: ttl, log: log}
}

func detailKey(id uint64) string { return fmt.Sprintf("video:detail:%d", id) }

func viewKey(id uint64) string { return viewKeyPrefix + strconv.FormatUint(id, 10) }

// GetDetail 读取视频详情缓存。
// 返回 (nil, true) 表示「命中空值缓存」，即该视频确实不存在。
func (c *VideoCache) GetDetail(ctx context.Context, id uint64) (*dto.VideoDetail, bool) {
	raw, err := c.rdb.Get(ctx, detailKey(id)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.log.Warn("cache_get_detail_failed", "videoId", id, "error", err)
		}
		return nil, false
	}
	if raw == nilMarker {
		return nil, true
	}

	var detail dto.VideoDetail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		c.log.Warn("cache_decode_detail_failed", "videoId", id, "error", err)
		return nil, false
	}
	return &detail, true
}

// SetDetail 写入视频详情缓存。
func (c *VideoCache) SetDetail(ctx context.Context, id uint64, detail *dto.VideoDetail) {
	data, err := json.Marshal(detail)
	if err != nil {
		c.log.Warn("cache_encode_detail_failed", "videoId", id, "error", err)
		return
	}
	c.set(ctx, detailKey(id), string(data), c.ttl)
}

// SetNil 写入空值缓存，防止缓存穿透。
func (c *VideoCache) SetNil(ctx context.Context, id uint64) {
	c.set(ctx, detailKey(id), nilMarker, time.Minute)
}

// DelDetail 删除详情缓存（写操作后调用）。
func (c *VideoCache) DelDetail(ctx context.Context, id uint64) {
	if err := c.rdb.Del(ctx, detailKey(id)).Err(); err != nil {
		c.log.Warn("cache_del_detail_failed", "videoId", id, "error", err)
	}
}

// ClaimEvent 使用 Redis SETNX 认领事件，返回 true 表示首次认领。
func (c *VideoCache) ClaimEvent(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

// ReleaseEvent 释放事件幂等键。
func (c *VideoCache) ReleaseEvent(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// IncrView 累加播放量（Redis 缓冲层）。
func (c *VideoCache) IncrView(ctx context.Context, id uint64) {
	if err := c.rdb.Incr(ctx, viewKey(id)).Err(); err != nil {
		c.log.Warn("cache_incr_view_failed", "videoId", id, "error", err)
	}
}

// FlushViews 取出并清零全部播放量缓冲，返回 videoId → 增量。
// 使用 Lua 保证「读取 + 删除」原子，避免并发 INCR 丢失。
func (c *VideoCache) FlushViews(ctx context.Context) (map[uint64]int64, error) {
	out := make(map[uint64]int64)

	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, viewKeyPrefix+"*", 200).Result()
		if err != nil {
			return out, fmt.Errorf("扫描播放量缓冲失败: %w", err)
		}

		for _, key := range keys {
			val, err := flushViewScript.Run(ctx, c.rdb, []string{key}).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue
				}
				return out, fmt.Errorf("读取播放量缓冲 %s 失败: %w", key, err)
			}
			s, ok := val.(string)
			if !ok {
				continue
			}
			delta, err := strconv.ParseInt(s, 10, 64)
			if err != nil || delta <= 0 {
				continue
			}
			id, err := strconv.ParseUint(strings.TrimPrefix(key, viewKeyPrefix), 10, 64)
			if err != nil {
				continue
			}
			out[id] += delta
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

// GetCategories 读取分类列表缓存。
func (c *VideoCache) GetCategories(ctx context.Context) ([]dto.CategoryDTO, bool) {
	raw, err := c.rdb.Get(ctx, categoryListKey).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.log.Warn("cache_get_categories_failed", "error", err)
		}
		return nil, false
	}

	var items []dto.CategoryDTO
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		c.log.Warn("cache_decode_categories_failed", "error", err)
		return nil, false
	}
	return items, true
}

// SetCategories 写入分类列表缓存。
func (c *VideoCache) SetCategories(ctx context.Context, items []dto.CategoryDTO) {
	data, err := json.Marshal(items)
	if err != nil {
		c.log.Warn("cache_encode_categories_failed", "error", err)
		return
	}
	c.set(ctx, categoryListKey, string(data), 10*time.Minute)
}

// DelCategories 删除分类列表缓存。
func (c *VideoCache) DelCategories(ctx context.Context) {
	if err := c.rdb.Del(ctx, categoryListKey).Err(); err != nil {
		c.log.Warn("cache_del_categories_failed", "error", err)
	}
}

func (c *VideoCache) set(ctx context.Context, key, value string, ttl time.Duration) {
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		c.log.Warn("cache_set_failed", "key", key, "error", err)
	}
}
