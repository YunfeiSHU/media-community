// Package cache 封装直播间的 Redis 热数据。
//
// 所有方法遵循同一原则：**缓存故障不阻断主流程**。
// 读失败退化为查库，写失败只记 warning —— MySQL 始终是权威事实来源。
//
// key 前缀统一用 live:，因为本服务与 Java 体系（gateway/auth）共用同一个 Redis 实例与 db 0，
// 前缀是唯一的隔离手段（Java 侧占用 auth:blacklist:* / gateway:rate:*）。
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/example/live-service-go/api/dto"
	"github.com/example/live-service-go/internal/model"
	"github.com/redis/go-redis/v9"
)

// nilMarker 用于缓存穿透保护：把「不存在」也缓存起来。
const nilMarker = "__nil__"

// bumpPeakScript 原子地把在线峰值抬到不低于当前在线人数。
// 必须原子：多个并发心跳各自「读峰值 → 比较 → 写回」会互相覆盖。
var bumpPeakScript = redis.NewScript(`
local cur = tonumber(ARGV[1])
local peak = tonumber(redis.call('GET', KEYS[1]) or '0')
if cur > peak then
  redis.call('SET', KEYS[1], cur)
  return cur
end
return peak
`)

// RoomCache 提供直播间状态、在线人数、人气值、密钥反查与详情缓存。
type RoomCache struct {
	rdb       *redis.Client
	onlineTTL time.Duration
	detailTTL time.Duration
	log       *slog.Logger
}

// NewRoomCache 构造缓存。
func NewRoomCache(rdb *redis.Client, onlineTTL, detailTTL time.Duration, log *slog.Logger) *RoomCache {
	return &RoomCache{rdb: rdb, onlineTTL: onlineTTL, detailTTL: detailTTL, log: log}
}

// ---------------------------------------------------------------- key 构造

func stateKey(roomID uint64) string        { return "live:room:" + u64(roomID) }
func onlineKey(roomID uint64) string       { return "live:online:" + u64(roomID) }
func peakKey(roomID uint64) string         { return "live:peak:" + u64(roomID) }
func viewerKey(roomID uint64) string       { return "live:viewers:" + u64(roomID) }
func detailKey(roomID uint64) string       { return "live:detail:" + u64(roomID) }
func streamKeyKey(streamKey string) string { return "live:key:" + streamKey }
func ownerKey(ownerID string) string       { return "live:mine:" + ownerID }

func u64(v uint64) string { return strconv.FormatUint(v, 10) }

// ---------------------------------------------------------------- 详情缓存

// GetDetail 读取详情缓存；返回 (nil, true) 表示命中空值缓存（直播间确实不存在）。
func (c *RoomCache) GetDetail(ctx context.Context, roomID uint64) (*dto.RoomDetail, bool) {
	raw, err := c.rdb.Get(ctx, detailKey(roomID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.log.Warn("cache_get_detail_failed", "roomId", roomID, "error", err)
		}
		return nil, false
	}
	if raw == nilMarker {
		return nil, true
	}

	var detail dto.RoomDetail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		c.log.Warn("cache_decode_detail_failed", "roomId", roomID, "error", err)
		return nil, false
	}
	return &detail, true
}

// SetDetail 写入详情缓存。
func (c *RoomCache) SetDetail(ctx context.Context, roomID uint64, detail *dto.RoomDetail) {
	data, err := json.Marshal(detail)
	if err != nil {
		c.log.Warn("cache_encode_detail_failed", "roomId", roomID, "error", err)
		return
	}
	c.set(ctx, detailKey(roomID), string(data), c.detailTTL)
}

// SetNil 写入空值缓存，防止缓存穿透。
func (c *RoomCache) SetNil(ctx context.Context, roomID uint64) {
	c.set(ctx, detailKey(roomID), nilMarker, time.Minute)
}

// DelDetail 删除详情缓存（状态变更后调用）。
func (c *RoomCache) DelDetail(ctx context.Context, roomID uint64) {
	if err := c.rdb.Del(ctx, detailKey(roomID)).Err(); err != nil {
		c.log.Warn("cache_del_detail_failed", "roomId", roomID, "error", err)
	}
}

// ---------------------------------------------------------------- 状态 Hash

// SetState 写入直播间实时状态 Hash（TTL = detail_cache_ttl）。
//
// 同时把「开播前的历史基数」写进去（viewerTotalBase / peakBase）：
// 这样 /rooms/:id/online 可以直接用 Hash 里的基数 + Redis 增量算出
// 「累计人次」与「历史峰值」，无需回源 MySQL。
func (c *RoomCache) SetState(ctx context.Context, room *model.Room) {
	startedAt := ""
	if room.LastStartedAt != nil {
		startedAt = room.LastStartedAt.Format(time.RFC3339)
	}
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, stateKey(room.ID), map[string]any{
		"status":          string(room.Status),
		"sessionId":       room.CurrentSession,
		"startedAt":       startedAt,
		"ownerId":         room.OwnerID,
		"title":           room.Title,
		"coverUrl":        room.CoverURL,
		"viewerTotalBase": room.ViewerTotal,
		"peakBase":        room.PeakOnline,
	})
	pipe.Expire(ctx, stateKey(room.ID), c.detailTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		c.log.Warn("cache_set_state_failed", "roomId", room.ID, "error", err)
	}
}

// GetState 读取状态 Hash；miss 时返回 (nil, false)。
func (c *RoomCache) GetState(ctx context.Context, roomID uint64) (map[string]string, bool) {
	state, err := c.rdb.HGetAll(ctx, stateKey(roomID)).Result()
	if err != nil {
		c.log.Warn("cache_get_state_failed", "roomId", roomID, "error", err)
		return nil, false
	}
	if len(state) == 0 {
		return nil, false
	}
	return state, true
}

// DelState 删除状态 Hash。
func (c *RoomCache) DelState(ctx context.Context, roomID uint64) {
	if err := c.rdb.Del(ctx, stateKey(roomID)).Err(); err != nil {
		c.log.Warn("cache_del_state_failed", "roomId", roomID, "error", err)
	}
}

// ---------------------------------------------------------------- 在线人数

// Heartbeat 记录一次观众心跳。
//
// 实现：ZSet 的 score 是「最后心跳时间戳」，每次心跳
//
//	ZADD（score = now） →  ZREMRANGEBYSCORE 清理过期成员  →  ZCARD
//
// 用 ZSet 而不是简单的计数器：不需要额外的清理任务就能让「异常退出的观众」
// 在 online_ttl 后自动从在线人数里消失（无需客户端调用 leave 接口）。
func (c *RoomCache) Heartbeat(
	ctx context.Context, roomID uint64, member string, now time.Time,
) (count int64, firstEnter bool, err error) {
	key := onlineKey(roomID)

	added, err := c.rdb.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.Unix()),
		Member: member,
	}).Result()
	if err != nil {
		return 0, false, fmt.Errorf("写入在线心跳失败: %w", err)
	}
	firstEnter = added > 0

	// 清理超过有效期的成员（先加后清，保证刚写进去的成员不会被误删）
	if err := c.rdb.ZRemRangeByScore(ctx, key,
		"-inf", strconv.FormatInt(now.Add(-c.onlineTTL).Unix(), 10)).Err(); err != nil {
		c.log.Warn("cache_prune_online_failed", "roomId", roomID, "error", err)
	}

	count, err = c.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return 0, false, fmt.Errorf("统计在线人数失败: %w", err)
	}

	// key 的 TTL 比成员有效期略长，避免「最后一个观众离开后 key 立刻消失」
	// 导致前端轮询看到在线人数为 0 但峰值还在的观感割裂
	c.rdb.Expire(ctx, key, c.onlineTTL*2)
	return count, firstEnter, nil
}

// Online 返回当前在线人数（内部会顺带清理过期成员）。
func (c *RoomCache) Online(ctx context.Context, roomID uint64) int64 {
	key := onlineKey(roomID)
	now := time.Now()
	if err := c.rdb.ZRemRangeByScore(ctx, key,
		"-inf", strconv.FormatInt(now.Add(-c.onlineTTL).Unix(), 10)).Err(); err != nil {
		c.log.Warn("cache_prune_online_failed", "roomId", roomID, "error", err)
		return 0
	}
	count, err := c.rdb.ZCard(ctx, key).Result()
	if err != nil {
		c.log.Warn("cache_online_count_failed", "roomId", roomID, "error", err)
		return 0
	}
	return count
}

// BumpPeak 把在线峰值抬到不低于 cur，返回最新峰值。
func (c *RoomCache) BumpPeak(ctx context.Context, roomID uint64, cur int64) int64 {
	val, err := bumpPeakScript.Run(ctx, c.rdb, []string{peakKey(roomID)}, cur).Result()
	if err != nil {
		c.log.Warn("cache_bump_peak_failed", "roomId", roomID, "error", err)
		return cur
	}
	return toInt64(val, cur)
}

// Peak 读取在线峰值（Redis 侧，关播时会写回 MySQL）。
func (c *RoomCache) Peak(ctx context.Context, roomID uint64) int64 {
	raw, err := c.rdb.Get(ctx, peakKey(roomID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.log.Warn("cache_get_peak_failed", "roomId", roomID, "error", err)
		}
		return 0
	}
	return toInt64(raw, 0)
}

// IncrViewers 累加本次直播的累计观看人次。
func (c *RoomCache) IncrViewers(ctx context.Context, roomID uint64) {
	if err := c.rdb.Incr(ctx, viewerKey(roomID)).Err(); err != nil {
		c.log.Warn("cache_incr_viewers_failed", "roomId", roomID, "error", err)
	}
}

// Viewers 读取本次直播累计观看人次。
func (c *RoomCache) Viewers(ctx context.Context, roomID uint64) int64 {
	raw, err := c.rdb.Get(ctx, viewerKey(roomID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.log.Warn("cache_get_viewers_failed", "roomId", roomID, "error", err)
		}
		return 0
	}
	return toInt64(raw, 0)
}

// IncrStat 累加服务级统计计数。
func (c *RoomCache) IncrStat(ctx context.Context, key string) error {
	return c.rdb.Incr(ctx, key).Err()
}

// ClaimEvent 使用 Redis SETNX 认领事件，返回 true 表示首次认领。
func (c *RoomCache) ClaimEvent(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

// ---------------------------------------------------------------- 密钥 / 一人一房间

// BindStream 建立 streamKey → {roomId, sessionId} 反查（SRS 回调只带 streamKey）。
func (c *RoomCache) BindStream(ctx context.Context, streamKey string, roomID uint64, sessionID string) {
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, streamKeyKey(streamKey), map[string]any{
		"roomId":    u64(roomID),
		"sessionId": sessionID,
	})
	// 兜底 TTL：即使关播时删除失败，残留 key 也不会永远存在
	pipe.Expire(ctx, streamKeyKey(streamKey), 24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		c.log.Warn("cache_bind_stream_failed", "streamKey", streamKey, "error", err)
	}
}

// LookupStream 反查 streamKey 所属房间；miss 时 ok=false（此时回源 MySQL）。
func (c *RoomCache) LookupStream(ctx context.Context, streamKey string) (roomID uint64, sessionID string, ok bool) {
	values, err := c.rdb.HGetAll(ctx, streamKeyKey(streamKey)).Result()
	if err != nil {
		c.log.Warn("cache_lookup_stream_failed", "streamKey", streamKey, "error", err)
		return 0, "", false
	}
	if len(values) == 0 {
		return 0, "", false
	}
	id, err := strconv.ParseUint(values["roomId"], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return id, values["sessionId"], true
}

// DelStream 删除密钥反查。
func (c *RoomCache) DelStream(ctx context.Context, streamKey string) {
	if err := c.rdb.Del(ctx, streamKeyKey(streamKey)).Err(); err != nil {
		c.log.Warn("cache_del_stream_failed", "streamKey", streamKey, "error", err)
	}
}

// ClaimOwnerRoom 抢占「该主播当前正在直播的房间」标记（SETNX）。
// 返回 false 表示该主播已经有正在直播的房间。
func (c *RoomCache) ClaimOwnerRoom(ctx context.Context, ownerID string, roomID uint64) bool {
	ok, err := c.rdb.SetNX(ctx, ownerKey(ownerID), u64(roomID), 0).Result()
	if err != nil {
		c.log.Warn("cache_claim_owner_failed", "ownerId", ownerID, "error", err)
		// Redis 不可用时不要阻断开播：MySQL 的兜底查询仍会拦住重复开播
		return true
	}
	return ok
}

// ReleaseOwnerRoom 释放「一人一房间」标记。
func (c *RoomCache) ReleaseOwnerRoom(ctx context.Context, ownerID string) {
	if err := c.rdb.Del(ctx, ownerKey(ownerID)).Err(); err != nil {
		c.log.Warn("cache_release_owner_failed", "ownerId", ownerID, "error", err)
	}
}

// OwnerRoom 返回该主播当前正在直播的房间 ID。
func (c *RoomCache) OwnerRoom(ctx context.Context, ownerID string) (uint64, bool) {
	raw, err := c.rdb.Get(ctx, ownerKey(ownerID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.log.Warn("cache_get_owner_room_failed", "ownerId", ownerID, "error", err)
		}
		return 0, false
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// ---------------------------------------------------------------- 清理

// ClearSession 清除一场直播的全部热数据（关播时调用）。
//
// 注意：不加 online/peak/viewers 的清空会让下一次开播「继承」上一次的人数，
// 因此每次开播都会先 ClearSession 再重新初始化。
func (c *RoomCache) ClearSession(ctx context.Context, roomID uint64, streamKey string) {
	keys := []string{
		onlineKey(roomID),
		peakKey(roomID),
		viewerKey(roomID),
		stateKey(roomID),
		detailKey(roomID),
	}
	if streamKey != "" {
		keys = append(keys, streamKeyKey(streamKey))
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		c.log.Warn("cache_clear_session_failed", "roomId", roomID, "error", err)
	}
}

// ResetCounters 只重置人数相关计数（开播时调用，保留状态 Hash 随后由 SetState 覆盖）。
func (c *RoomCache) ResetCounters(ctx context.Context, roomID uint64) {
	if err := c.rdb.Del(ctx, onlineKey(roomID), peakKey(roomID), viewerKey(roomID)).Err(); err != nil {
		c.log.Warn("cache_reset_counters_failed", "roomId", roomID, "error", err)
	}
}

// ---------------------------------------------------------------- 内部工具

func (c *RoomCache) set(ctx context.Context, key, value string, ttl time.Duration) {
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		c.log.Warn("cache_set_failed", "key", key, "error", err)
	}
}

func toInt64(v any, fallback int64) int64 {
	switch value := v.(type) {
	case int64:
		return value
	case string:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fallback
		}
		return n
	default:
		return fallback
	}
}
