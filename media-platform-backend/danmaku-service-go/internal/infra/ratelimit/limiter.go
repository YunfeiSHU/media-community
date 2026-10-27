// Package ratelimit 实现弹幕发送的双层限流（用户级 + 房间级）。
//
// 为什么是固定窗口而不是滑动窗口（设计文档 §7.1 已论证）：
// 固定窗口只需 1 个 key + 1 次 INCR，在「1 秒」这种小窗口下边界误差可接受；
// 滑动窗口（ZSet）每条弹幕一个成员，在 5k msg/s 下 Redis 内存与 CPU 都要高一个量级。
// 这是刻意的取舍，不是实现偷懒。
package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Scope 标识被哪个层级的限流拒绝，便于日志与前端提示区分。
type Scope string

// 限流层级。
const (
	// ScopeUser 用户级：单用户每秒条数。
	ScopeUser Scope = "user"
	// ScopeRoom 房间级：单房间每秒条数（保护广播扇出）。
	ScopeRoom Scope = "room"
)

// Decision 是一次限流判定。
type Decision struct {
	Allowed      bool
	RetryAfterMs int64
	Scope        Scope
	// Degraded 表示本次判定走的是进程内降级计数（Redis 不可用）。
	Degraded bool
}

// fixedWindowScript 是固定窗口计数器。
//
// KEYS[1] 计数 key；ARGV[1] 上限；ARGV[2] 窗口毫秒数。
// 返回 {是否允许, 剩余等待毫秒}。
// 用 Lua 而不是「INCR 后再判断」是因为后者在并发下会超发。
const fixedWindowScript = `
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
if n > tonumber(ARGV[1]) then
  local ttl = redis.call('PTTL', KEYS[1])
  if ttl < 0 then ttl = 0 end
  return {0, ttl}
end
return {1, 0}
`

// keyPrefix 与设计文档 §3.3 的 Redis 结构保持一致。
const (
	userKeyPrefix = "danmaku:rate:u:"
	roomKeyPrefix = "danmaku:rate:room:"
)

// Limiter 是双层限流器。
type Limiter struct {
	rdb       *redis.Client
	userLimit int
	roomLimit int
	window    time.Duration
	log       *slog.Logger
	script    *redis.Script
	fallback  *localWindows
}

// New 构造限流器。rdb 为 nil 时直接走进程内降级实现。
func New(rdb *redis.Client, userLimit, roomLimit int, window time.Duration, log *slog.Logger) *Limiter {
	if log == nil {
		log = slog.Default()
	}
	if window <= 0 {
		window = time.Second
	}
	return &Limiter{
		rdb:       rdb,
		userLimit: userLimit,
		roomLimit: roomLimit,
		window:    window,
		log:       log,
		script:    redis.NewScript(fixedWindowScript),
		fallback:  newLocalWindows(),
	}
}

// Allow 依次执行用户级与房间级判定。
//
// 顺序：先用户级（便宜、命中率高），再房间级。
// 被房间级拒绝时**不回滚**用户级计数 —— 否则用户会觉得自己被无故限流
// （错误归属搞反，客服无法解释）。见设计文档 §7.2。
func (l *Limiter) Allow(ctx context.Context, uid, resource string) (Decision, error) {
	if l.userLimit > 0 && strings.TrimSpace(uid) != "" {
		ok, retry, degraded := l.hit(ctx, userKeyPrefix+uid, l.userLimit)
		if !ok {
			return Decision{Allowed: false, RetryAfterMs: retry, Scope: ScopeUser, Degraded: degraded}, nil
		}
	}
	if l.roomLimit > 0 && strings.TrimSpace(resource) != "" {
		ok, retry, degraded := l.hit(ctx, roomKeyPrefix+resource, l.roomLimit)
		if !ok {
			return Decision{Allowed: false, RetryAfterMs: retry, Scope: ScopeRoom, Degraded: degraded}, nil
		}
	}
	return Decision{Allowed: true}, nil
}

// hit 对单个 key 做一次固定窗口判定；Redis 失败时降级到进程内计数。
func (l *Limiter) hit(ctx context.Context, key string, limit int) (allowed bool, retryAfterMs int64, degraded bool) {
	if l.rdb != nil {
		values, err := l.script.Run(ctx, l.rdb, []string{key}, limit, l.window.Milliseconds()).Slice()
		if err == nil && len(values) == 2 {
			return toInt64(values[0]) == 1, toInt64(values[1]), false
		}
		if err != nil && !isContextDone(err) {
			// 降级必须打日志：静默降级会让「线上限流为什么放宽了」变成谜案。
			l.log.Warn("ratelimit_degraded_to_local",
				"key", key, "reason", err.Error(),
				"hint", "Redis 不可用，限流退化为单实例有效")
		}
	}
	ok, retry := l.fallback.allow(key, limit, l.window)
	return ok, retry, true
}

func isContextDone(err error) bool {
	if err == nil {
		return false
	}
	switch err {
	case context.Canceled, context.DeadlineExceeded:
		return true
	}
	return false
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case string:
		var parsed int64
		_, _ = fmt.Sscanf(n, "%d", &parsed)
		return parsed
	case float64:
		return int64(n)
	case nil:
		return 0
	default:
		return 0
	}
}

// ---------------------------------------------------------------- 降级实现

type localWindows struct {
	mu      sync.Mutex
	buckets map[string]*localBucket
}

type localBucket struct {
	start time.Time
	count int
}

func newLocalWindows() *localWindows {
	return &localWindows{buckets: make(map[string]*localBucket)}
}

// allow 是进程内固定窗口计数。单实例有效，多实例会放宽限流倍数（已知限制）。
func (w *localWindows) allow(key string, limit int, window time.Duration) (bool, int64) {
	now := time.Now()

	w.mu.Lock()
	defer w.mu.Unlock()

	// 顺手回收过期桶，避免 key 无限增长（弹幕 key 的基数 = 用户数 + 房间数）。
	if len(w.buckets) > 4096 {
		for k, b := range w.buckets {
			if now.Sub(b.start) > window {
				delete(w.buckets, k)
			}
		}
	}

	b, ok := w.buckets[key]
	if !ok || now.Sub(b.start) >= window {
		w.buckets[key] = &localBucket{start: now, count: 1}
		return true, 0
	}
	if b.count+1 > limit {
		remain := window - now.Sub(b.start)
		if remain < 0 {
			remain = 0
		}
		return false, remain.Milliseconds()
	}
	b.count++
	return true, 0
}
