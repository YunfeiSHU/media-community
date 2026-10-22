package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/example/live-service-go/api/dto"
	"github.com/example/live-service-go/internal/config"
	"github.com/example/live-service-go/internal/infra/cache"
	"github.com/example/live-service-go/internal/infra/events"
	"github.com/example/live-service-go/internal/infra/media"
	"github.com/example/live-service-go/internal/model"
	"github.com/example/live-service-go/internal/repository"
	"github.com/example/live-service-go/pkg/errs"
)

// statsStartedKey / statsStoppedKey 是累计开播 / 关播次数计数器。
//
// 它由本服务订阅自己发出的 live.* 事件后累加——**这就是「既生产也消费」的意义**：
// 证明事件链路真的通了，同时给阶段二的统计/推荐服务留出接入点的样例。
const (
	statsStartedKey = "live:stats:started"
	statsStoppedKey = "live:stats:stopped"
	idemKeyPrefix   = "mq:idem:"
)

type statsCache interface {
	IncrStat(ctx context.Context, key string) error
	ClaimEvent(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
}

// StatsService 负责在线人数统计与事件订阅。
type StatsService struct {
	deps
	bus   events.Bus
	stats statsCache
}

// NewStatsService 构造统计服务。
func NewStatsService(
	cfg *config.Config,
	rooms repository.RoomRepository,
	records repository.RecordRepository,
	roomCache *cache.RoomCache,
	mediaServer media.Server,
	bus events.Bus,
	stats statsCache,
	log *slog.Logger,
) *StatsService {
	return &StatsService{
		deps:  deps{cfg: cfg, rooms: rooms, records: records, cache: roomCache, media: mediaServer, log: log},
		bus:   bus,
		stats: stats,
	}
}

// Heartbeat 记录一次观众心跳。
//
// 已登录用户用 uid 作为身份；游客用 (X-Session-Id 或请求体 sessionId)。
// 未开播的房间不接受心跳：否则可以对着一个空房间刷人气。
func (s *StatsService) Heartbeat(
	ctx context.Context, roomID uint64, uid, guestSession string,
) (*dto.HeartbeatResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.Status != model.RoomStatusLiving {
		return nil, errs.CodeNotLiving.Newf("直播间当前不在直播中，状态: %s", room.Status)
	}

	member := ""
	switch {
	case strings.TrimSpace(uid) != "":
		member = "u:" + strings.TrimSpace(uid)
	case strings.TrimSpace(guestSession) != "":
		member = "anon:" + strings.TrimSpace(guestSession)
	default:
		return nil, errs.CodeInvalidParam.New("游客心跳需要提供 sessionId 或 X-Session-Id")
	}

	count, firstEnter, err := s.cache.Heartbeat(ctx, room.ID, member, time.Now())
	if err != nil {
		return nil, errs.CodeDependencyDown.New("在线人数统计不可用")
	}

	s.cache.BumpPeak(ctx, room.ID, count)
	if firstEnter {
		// 只有「新进入」才计入累计人次，重复心跳不灌水
		s.cache.IncrViewers(ctx, room.ID)
	}

	return &dto.HeartbeatResult{
		RoomID:      room.ID,
		OnlineCount: count,
		MemberKey:   member,
		FirstEnter:  firstEnter,
		TTLSec:      int(s.cfg.Room.OnlineTTL.Seconds()),
	}, nil
}

// Online 返回在线人数（前端 5~10s 轮询的轻量接口）。
//
// 设计目标是「不查 MySQL」：开播时会把「开播前已累计的人次 / 历史峰值」写进状态 Hash，
// 因此在线人数 = Redis 增量，累计人次 = Hash 基数 + Redis 增量。
// 只有当热数据已经过期（房间早已关播）时才回源一次主键查询。
func (s *StatsService) Online(ctx context.Context, roomID uint64) (*dto.OnlineResult, error) {
	state, ok := s.cache.GetState(ctx, roomID)
	if !ok {
		room, err := s.getRoom(ctx, roomID)
		if err != nil {
			return nil, err
		}
		return &dto.OnlineResult{
			RoomID:      room.ID,
			OnlineCount: 0,
			PeakOnline:  room.PeakOnline,
			ViewerTotal: room.ViewerTotal,
		}, nil
	}

	online := s.cache.Online(ctx, roomID)
	peak := s.cache.Peak(ctx, roomID)
	if base := parseInt64(state["peakBase"]); peak < base {
		peak = base
	}
	viewerBase := parseInt64(state["viewerTotalBase"])

	return &dto.OnlineResult{
		RoomID:      roomID,
		OnlineCount: online,
		PeakOnline:  int(peak),
		ViewerTotal: viewerBase + s.cache.Viewers(ctx, roomID),
	}, nil
}

// Register 订阅自己发出的事件（占位消费者）。
//
// 真正的业务价值在阶段二：统计服务 / 推荐服务 / 通知服务只要订阅同一队列即可接入，
// 不需要改动 live-service 的代码。
func (s *StatsService) Register() error {
	if err := s.bus.Subscribe(events.RoutingLiveStarted, s.handleStarted); err != nil {
		return err
	}
	return s.bus.Subscribe(events.RoutingLiveStopped, s.handleStopped)
}

func (s *StatsService) handleStarted(ctx context.Context, ev events.Event) error {
	if s.duplicated(ctx, ev.EventID) {
		return nil
	}
	var payload dto.LiveStartedPayload
	if err := ev.Decode(&payload); err != nil {
		return err
	}
	s.log.Info("event_live_started",
		"roomId", payload.RoomID, "ownerId", payload.OwnerID, "sessionId", payload.SessionID)
	if err := s.stats.IncrStat(ctx, statsStartedKey); err != nil {
		s.log.Warn("stats_started_increment_failed", "error", err)
	}
	return nil
}

func (s *StatsService) handleStopped(ctx context.Context, ev events.Event) error {
	if s.duplicated(ctx, ev.EventID) {
		return nil
	}
	var payload dto.LiveStoppedPayload
	if err := ev.Decode(&payload); err != nil {
		return err
	}
	s.log.Info("event_live_stopped",
		"roomId", payload.RoomID, "sessionId", payload.SessionID,
		"durationSec", payload.DurationSec, "peakOnline", payload.PeakOnline, "reason", payload.Reason)
	if err := s.stats.IncrStat(ctx, statsStoppedKey); err != nil {
		s.log.Warn("stats_stopped_increment_failed", "error", err)
	}
	return nil
}

// duplicated 用 Redis SETNX 做短时幂等去重（MQ 是至少一次投递）。
//
// 只在成功后认领：失败时必须释放，否则重试会被自己挡掉。
func (s *StatsService) duplicated(ctx context.Context, eventID string) bool {
	if eventID == "" {
		return false
	}
	ok, err := s.stats.ClaimEvent(ctx, idemKeyPrefix+eventID, "1", s.cfg.Events.IdempotentTTL)
	if err != nil {
		s.log.Warn("idempotent_claim_failed", "eventId", eventID, "error", err)
		return false
	}
	return !ok
}

// parseInt64 解析 Hash 里的十进制字符串；非法时返回 0（缓存数据坏了也不该影响主流程）。
func parseInt64(raw string) int64 {
	if raw == "" {
		return 0
	}
	var out int64
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c < '0' || c > '9' {
			return out
		}
		out = out*10 + int64(c-'0')
	}
	return out
}
