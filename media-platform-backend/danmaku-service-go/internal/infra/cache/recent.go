// Package cache 封装弹幕的 Redis 热数据与各类短时状态。
//
// Key 设计（与设计文档 §3.3 一致）：
//
//	danmaku:room:{roomId}:recent   List  最新 N 条直播弹幕（LPUSH + LTRIM，2h）
//	danmaku:video:{videoId}:recent List  最新 N 条点播弹幕
//	danmaku:live:{roomId}          String 房间是否直播中（1/0），由 live.* 事件维护
//	danmaku:dedup:{clientMsgId}    String 客户端重试去重（10s）
//	danmaku:idem:{eventId}         String 消费侧幂等（TTL 见事件配置）
//	danmaku:pending:persist        List   MQ 不可用时的补发队列
//
// **Redis 与 Java 体系共用同一个实例、同一个 db 0**，
// 因此所有 key 必须带 `danmaku:` 前缀；任何全库操作（KEYS * / FLUSHDB）都是禁止的。
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	roomRecentPrefix  = "danmaku:room:"
	roomRecentSuffix  = ":recent"
	videoRecentPrefix = "danmaku:video:"
	livePrefix        = "danmaku:live:"
	dedupPrefix       = "danmaku:dedup:"
	idemPrefix        = "danmaku:idem:"
	pendingKey        = "danmaku:pending:persist"

	// pendingLimit 是补发队列长度上限：超出后丢弃最旧条目，避免 Redis 无界增长。
	pendingLimit = 100000
)

// DedupPendingMarker 是「同 clientMsgId 的首次请求正在处理中」的占位值。
//
// 它让并发的重复请求不会双双通过，又能让先到的请求在处理完后覆盖成真实快照。
// service 层用它区分「拿到的是结果」还是「首发还在处理中」。
const DedupPendingMarker = "__pending__"

// RecentCache 提供热数据、去重、幂等、直播状态与补发队列操作。
type RecentCache struct {
	rdb      *redis.Client
	hotSize  int64
	hotTTL   time.Duration
	dedupTTL time.Duration
	log      *slog.Logger
}

// NewRecentCache 构造缓存。rdb 为 nil 时所有方法返回「未命中」而不是 panic，
// 这样「本地没起 Redis」只会让热数据链路失效，而不会让进程崩溃。
func NewRecentCache(rdb *redis.Client, hotSize int64, hotTTL, dedupTTL time.Duration, log *slog.Logger) *RecentCache {
	if log == nil {
		log = slog.Default()
	}
	if hotSize <= 0 {
		hotSize = 500
	}
	if hotTTL <= 0 {
		hotTTL = 2 * time.Hour
	}
	if dedupTTL <= 0 {
		dedupTTL = 10 * time.Second
	}
	return &RecentCache{rdb: rdb, hotSize: hotSize, hotTTL: hotTTL, dedupTTL: dedupTTL, log: log}
}

// HotSize 返回热数据条数上限（service 判断「第一页是否能走热数据」）。
func (c *RecentCache) HotSize() int64 { return c.hotSize }

// HotTTL 返回热数据 TTL（/readyz 或文档展示用）。
func (c *RecentCache) HotTTL() time.Duration { return c.hotTTL }

// ---------------------------------------------------------------- key 构造

// RoomKey 返回直播房间热数据 key。
func RoomKey(roomID string) string { return roomRecentPrefix + roomID + roomRecentSuffix }

// VideoKey 返回点播视频热数据 key。
func VideoKey(videoID string) string { return videoRecentPrefix + videoID + roomRecentSuffix }

// ---------------------------------------------------------------- 热数据

// PushRoom 写入一条直播热数据（LPUSH + LTRIM + EXPIRE）。
func (c *RecentCache) PushRoom(ctx context.Context, roomID, raw string) error {
	return c.push(ctx, RoomKey(roomID), raw)
}

// PushVideo 写入一条点播热数据。
func (c *RecentCache) PushVideo(ctx context.Context, videoID, raw string) error {
	return c.push(ctx, VideoKey(videoID), raw)
}

func (c *RecentCache) push(ctx context.Context, key, raw string) error {
	if c.rdb == nil {
		return nil
	}
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, raw)
	// 有界列表：只保留最新 N 条，历史交给 MySQL。
	pipe.LTrim(ctx, key, 0, c.hotSize-1)
	pipe.Expire(ctx, key, c.hotTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("写入热数据失败: %w", err)
	}
	return nil
}

// ListRoom 从热数据读取一段（offset 用于分页；按最新在前）。
func (c *RecentCache) ListRoom(ctx context.Context, roomID string, offset, count int) ([]string, error) {
	return c.list(ctx, RoomKey(roomID), offset, count)
}

// ListVideo 从点播热数据读取（点播不走分页，取最新 count 条）。
func (c *RecentCache) ListVideo(ctx context.Context, videoID string, count int) ([]string, error) {
	return c.list(ctx, VideoKey(videoID), 0, count)
}

func (c *RecentCache) list(ctx context.Context, key string, offset, count int) ([]string, error) {
	if c.rdb == nil {
		return nil, nil
	}
	if count <= 0 {
		return nil, nil
	}
	items, err := c.rdb.LRange(ctx, key, int64(offset), int64(offset+count-1)).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("读取热数据失败: %w", err)
	}
	return items, nil
}

// RoomLen 返回房间热数据条数。
func (c *RecentCache) RoomLen(ctx context.Context, roomID string) (int64, error) {
	if c.rdb == nil {
		return 0, nil
	}
	n, err := c.rdb.LLen(ctx, RoomKey(roomID)).Result()
	if err != nil {
		return 0, fmt.Errorf("统计热数据失败: %w", err)
	}
	return n, nil
}

// DelRoom 清空房间热数据（开播时清理上一轮）。
func (c *RecentCache) DelRoom(ctx context.Context, roomID string) error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, RoomKey(roomID)).Err()
}

// RemoveRoomItem 从热数据中移除指定 clientMsgId 的条目（撤回时调用）。
//
// 为什么不是简单 LREM：List 元素是整条 JSON，LREM 需要精确值，
// 而撤回时手上只有 danmakuId / clientMsgId。因此读回后过滤再重建，
// 500 条量级下成本可接受；重建时从尾部 push 以保持「最新在前」的顺序。
func (c *RecentCache) RemoveRoomItem(ctx context.Context, roomID, clientMsgID string) error {
	if c.rdb == nil || strings.TrimSpace(clientMsgID) == "" {
		return nil
	}
	key := RoomKey(roomID)
	items, err := c.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("读取热数据失败: %w", err)
	}

	needle := `"clientMsgId":"` + clientMsgID + `"`
	kept := make([]string, 0, len(items))
	removed := false
	for _, item := range items {
		if !removed && strings.Contains(item, needle) {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	if !removed {
		return nil
	}

	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, key)
	for i := len(kept) - 1; i >= 0; i-- {
		pipe.RPush(ctx, key, kept[i])
	}
	if len(kept) > 0 {
		pipe.Expire(ctx, key, c.hotTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("重建热数据失败: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------- 去重 / 幂等

// ClaimDedup 抢占 clientMsgId（SETNX 占位）。
// 返回 true 表示本次是首发，可以继续处理。
func (c *RecentCache) ClaimDedup(ctx context.Context, clientMsgID string) (bool, error) {
	if c.rdb == nil || strings.TrimSpace(clientMsgID) == "" {
		return true, nil
	}
	ok, err := c.rdb.SetNX(ctx, dedupPrefix+clientMsgID, DedupPendingMarker, c.dedupTTL).Result()
	if err != nil {
		// 去重依赖不可用时选择「放行」而不是「拒绝」：
		// 拒绝会让 Redis 抖动直接变成全站无法发弹幕。
		c.log.Warn("dedup_unavailable", "clientMsgId", clientMsgID, "reason", err.Error())
		return true, nil
	}
	return ok, nil
}

// DedupSnapshot 读取首发的处理结果快照（不存在返回空串）。
func (c *RecentCache) DedupSnapshot(ctx context.Context, clientMsgID string) (string, error) {
	if c.rdb == nil || strings.TrimSpace(clientMsgID) == "" {
		return "", nil
	}
	raw, err := c.rdb.Get(ctx, dedupPrefix+clientMsgID).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取去重快照失败: %w", err)
	}
	return raw, nil
}

// PutDedupSnapshot 覆盖为真实结果快照，让后续重复请求能拿到与首发一致的响应。
func (c *RecentCache) PutDedupSnapshot(ctx context.Context, clientMsgID string, value any) error {
	if c.rdb == nil || strings.TrimSpace(clientMsgID) == "" {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化去重快照失败: %w", err)
	}
	return c.rdb.Set(ctx, dedupPrefix+clientMsgID, string(raw), c.dedupTTL).Err()
}

// MarkEvent 消费侧幂等（SETNX）。返回 true 表示首次消费。
func (c *RecentCache) MarkEvent(ctx context.Context, eventID string, ttl time.Duration) (bool, error) {
	if c.rdb == nil || strings.TrimSpace(eventID) == "" {
		return true, nil
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	ok, err := c.rdb.SetNX(ctx, idemPrefix+eventID, "1", ttl).Result()
	if err != nil {
		// 幂等键不可用时放行：重复落库由 (uid, clientMsgId) 的短时去重兜底。
		return true, fmt.Errorf("写入幂等键失败: %w", err)
	}
	return ok, nil
}

// ClearEvent 删除幂等键。
//
// 用途：消费侧先占位幂等键再落库，若落库失败必须**回滚**这个键，
// 否则 MQ 重投时会被幂等键挡住 —— 那是「标记已处理但实际没写库」的丢数据bug。
func (c *RecentCache) ClearEvent(ctx context.Context, eventID string) error {
	if c.rdb == nil || strings.TrimSpace(eventID) == "" {
		return nil
	}
	return c.rdb.Del(ctx, idemPrefix+eventID).Err()
}

// ---------------------------------------------------------------- 直播状态

// SetLiving 标记房间直播状态。键值恒为 "1" / "0"，**不删除键**。
//
// 为什么必须区分「0」与「不存在」：设计文档 §Redis key 规定本键是 `1`/`0`，
// 且 FR-6.3 要求「未开播的房间拒绝发送」。若关播时直接把键删掉，
// Living() 会返回 known=false（状态未知），在 require_living=false 下就变成
// **放行** —— 关播后还能继续刷弹幕，与需求相反。所以这里写 "0" 而不是 Del。
//
// TTL 为 0（不过期）：与 live 侧的房间生命周期一致，房间重新开播时
// live.started 会把它覆盖回 "1"。
func (c *RecentCache) SetLiving(ctx context.Context, roomID string, living bool) error {
	if c.rdb == nil {
		return nil
	}
	value := "0"
	if living {
		value = "1"
	}
	return c.rdb.Set(ctx, livePrefix+roomID, value, 0).Err()
}

// Living 返回房间直播状态；known=false 表示没有该房间的状态记录。
//
// 区分「明确未开播」与「状态未知」很重要：前者要拒绝发送（80003），
// 后者在 require_living=false 时应放行，否则本地独立调试永远发不出弹幕。
func (c *RecentCache) Living(ctx context.Context, roomID string) (living, known bool, err error) {
	if c.rdb == nil || strings.TrimSpace(roomID) == "" {
		return false, false, nil
	}
	raw, err := c.rdb.Get(ctx, livePrefix+roomID).Result()
	if err == redis.Nil {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("读取直播状态失败: %w", err)
	}
	return raw == "1", true, nil
}

// DelLiving 彻底删除直播状态记录。
//
// 只在「房间已被删除、状态记录成了垃圾」这类清理场景使用；
// **关播请用 SetLiving(ctx, roomID, false)**，删键会让状态退化成「未知」。
func (c *RecentCache) DelLiving(ctx context.Context, roomID string) error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, livePrefix+roomID).Err()
}

// ---------------------------------------------------------------- 补发队列

// PushPending 把投递失败的事件塞进补发队列（LPUSH + LTRIM 保留最新 10 万条）。
func (c *RecentCache) PushPending(ctx context.Context, raw string) error {
	if c.rdb == nil {
		return fmt.Errorf("Redis 不可用，补发队列写入失败")
	}
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, pendingKey, raw)
	pipe.LTrim(ctx, pendingKey, 0, pendingLimit-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("写入补发队列失败: %w", err)
	}
	return nil
}

// PopPending 取出最早的 n 条（LPUSH + RPOP = FIFO）。
func (c *RecentCache) PopPending(ctx context.Context, n int) ([]string, error) {
	if c.rdb == nil || n <= 0 {
		return nil, nil
	}
	items := make([]string, 0, n)
	for i := 0; i < n; i++ {
		raw, err := c.rdb.RPop(ctx, pendingKey).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return items, fmt.Errorf("读取补发队列失败: %w", err)
		}
		items = append(items, raw)
	}
	return items, nil
}

// RequeuePending 把重投失败的条目放回队列尾部（下次再试）。
func (c *RecentCache) RequeuePending(ctx context.Context, items []string) error {
	if c.rdb == nil || len(items) == 0 {
		return nil
	}
	args := make([]any, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		args = append(args, items[i])
	}
	return c.rdb.RPush(ctx, pendingKey, args...).Err()
}

// PendingLen 返回补发队列长度（监控 / 日志用）。
func (c *RecentCache) PendingLen(ctx context.Context) (int64, error) {
	if c.rdb == nil {
		return 0, nil
	}
	return c.rdb.LLen(ctx, pendingKey).Result()
}

// Ping 校验 Redis 可用性（/readyz）。
func (c *RecentCache) Ping(ctx context.Context) error {
	if c.rdb == nil {
		return fmt.Errorf("Redis 客户端未初始化")
	}
	return c.rdb.Ping(ctx).Err()
}
