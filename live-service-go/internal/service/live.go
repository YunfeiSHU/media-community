package service

import (
	"context"
	"log/slog"
	"strconv"
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
	"github.com/example/live-service-go/pkg/identity"
	"github.com/example/live-service-go/pkg/trace"
)

// LiveService 负责「开播 / 关播 / 状态机推进」这条生命周期主线。
//
// 核心立场：**开播接口只把房间推到 STARTING**，LIVING 必须由 SRS 的 on_publish 回调置入。
// 否则主播推流失败（OBS 崩了 / 密钥打错 / 网络不通）时，
// 数据库显示「直播中」但观众看到黑屏，无法区分服务端问题与主播侧问题。
type LiveService struct {
	deps
	bus events.Bus
}

// NewLiveService 构造直播生命周期服务。
func NewLiveService(
	cfg *config.Config,
	rooms repository.RoomRepository,
	records repository.RecordRepository,
	roomCache *cache.RoomCache,
	mediaServer media.Server,
	bus events.Bus,
	log *slog.Logger,
) *LiveService {
	return &LiveService{
		deps: deps{cfg: cfg, rooms: rooms, records: records, cache: roomCache, media: mediaServer, log: log},
		bus:  bus,
	}
}

// Start 开播。
//
// 步骤：校验归属 → 「一人一房间」双重校验（Redis 快速路径 + MySQL 权威校验）
// → 生成新 streamKey / sessionId → 条件更新为 STARTING → 写场次记录
// → 重置并初始化 Redis 热数据 → 返回推流地址。
func (s *LiveService) Start(
	ctx context.Context, user identity.CurrentUser, roomID uint64,
) (*dto.StartResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !user.Owns(room.OwnerID) {
		return nil, errs.CodeForbidden.New("只能开播自己的直播间")
	}

	// 幂等：已经在开播流程中时直接返回当前状态，不换密钥。
	if room.Status.InProgress() {
		return s.startResult(ctx, room), nil
	}

	if err := s.ensureNoOtherLive(ctx, room); err != nil {
		return nil, err
	}

	streamKey, err := newStreamKey()
	if err != nil {
		return nil, err
	}
	sessionID, err := newSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now()

	// 条件更新：只有仍处于 IDLE/ENDED/FAILED 时才推进，避免并发开播互相覆盖。
	updated, err := s.rooms.Transition(ctx, room.ID, []model.RoomStatus{
		model.RoomStatusIdle, model.RoomStatusEnded, model.RoomStatusFailed,
	}, map[string]any{
		"status":          string(model.RoomStatusStarting),
		"stream_key":      streamKey,
		"current_session": sessionID,
		"last_started_at": now,
	})
	if err != nil {
		return nil, err
	}
	if !updated {
		// 并发场景：别人刚刚改了状态，按最新状态重新判定
		latest, err := s.getRoom(ctx, roomID)
		if err != nil {
			return nil, err
		}
		if latest.Status.InProgress() {
			return s.startResult(ctx, latest), nil
		}
		return nil, errs.CodeConflict.New("直播间状态已变化，请重试")
	}

	// 场次记录：uk_session 唯一让重复回调天然幂等
	if err := s.records.Create(ctx, &model.Record{
		RoomID:    room.ID,
		SessionID: sessionID,
		StartedAt: now,
	}); err != nil {
		// 建记录失败不应阻断开播（真正的开播事实是 on_publish）
		s.log.Warn("create_live_record_failed", "roomId", room.ID, "sessionId", sessionID, "error", err)
	}

	// 清掉上一场的热数据，避免在线人数/峰值/人次被继承
	s.cache.ResetCounters(ctx, room.ID)
	s.cache.BindStream(ctx, streamKey, room.ID, sessionID)
	s.cache.ClaimOwnerRoom(ctx, room.OwnerID, room.ID)
	s.cache.DelDetail(ctx, room.ID)

	fresh, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	s.cache.SetState(ctx, fresh)

	s.log.Info("live_start_requested",
		"roomId", room.ID, "ownerId", room.OwnerID, "sessionId", sessionID, "streamKey", streamKey)
	return s.startResult(ctx, fresh), nil
}

// Stop 关播（主播主动）。
func (s *LiveService) Stop(
	ctx context.Context, user identity.CurrentUser, roomID uint64,
) (*dto.StopResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !user.Owns(room.OwnerID) {
		return nil, errs.CodeForbidden.New("只能关播自己的直播间")
	}

	// 幂等：不在直播中时返回当前快照
	if !room.Status.CanStop() {
		return &dto.StopResult{
			RoomID:      room.ID,
			Status:      string(room.Status),
			SessionID:   room.CurrentSession,
			EndedAt:     room.LastEndedAt,
			PeakOnline:  room.PeakOnline,
			ViewerTotal: room.ViewerTotal,
		}, nil
	}

	return s.endLive(ctx, room, model.EndReasonManual, true)
}

// ForceClose 强制关闭（创建者或 ADMIN，用于内容治理）。
func (s *LiveService) ForceClose(
	ctx context.Context, user identity.CurrentUser, roomID uint64, reason string,
) (*dto.ForceCloseResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !user.Owns(room.OwnerID) && !user.IsAdmin() {
		return nil, errs.CodeForbidden.New("只有创建者或管理员可以强制关闭直播间")
	}
	if !room.Status.InProgress() {
		return nil, errs.CodeNotLiving.Newf("直播间当前不在直播中，状态: %s", room.Status)
	}

	if _, err := s.endLive(ctx, room, model.EndReasonForce, true); err != nil {
		return nil, err
	}
	s.log.Warn("live_force_closed",
		"roomId", room.ID, "operator", user.UID, "isAdmin", user.IsAdmin(), "reason", reason)

	return &dto.ForceCloseResult{
		RoomID:    room.ID,
		Status:    string(model.RoomStatusEnded),
		EndReason: model.EndReasonForce,
		Reason:    reason,
	}, nil
}

// Play 返回播放地址（仅 LIVING 可用）。
func (s *LiveService) Play(ctx context.Context, roomID uint64) (*dto.PlayResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.Status != model.RoomStatusLiving {
		return nil, errs.CodeNotLiving.Newf("直播间当前不在直播中，状态: %s", room.Status)
	}

	urls := s.media.PlayURLs(room.StreamKey)
	return &dto.PlayResult{
		RoomID:      room.ID,
		Title:       room.Title,
		Status:      string(room.Status),
		OnlineCount: s.onlineCount(ctx, room.ID),
		Format:      []string{"hls", "httpFlv", "webrtc", "rtmp"},
		PlayURLs: &dto.PlayURLs{
			HLS: urls.HLS, HTTPFLV: urls.HTTPFLV, WebRTC: urls.WebRTC, RTMP: urls.RTMP,
		},
	}, nil
}

// ---------------------------------------------------------------- SRS 回调

// OnPublish 处理 SRS on_publish：这是「真正开播」的唯一判定入口。
func (s *LiveService) OnPublish(ctx context.Context, req dto.HookRequest) *dto.HookResponse {
	room, sessionID, resp := s.resolveStream(ctx, req)
	if resp != nil {
		return resp
	}

	// 会话校验：推流地址里的 session 必须与当前会话一致，
	// 否则就是上一次直播的延迟回调（或有人拿到了旧密钥）。
	if sessionID != "" && room.CurrentSession != sessionID {
		s.log.Warn("on_publish_session_mismatch",
			"roomId", room.ID, "expect", room.CurrentSession, "got", sessionID)
		return reject("推流密钥无效或会话已过期")
	}

	switch room.Status {
	case model.RoomStatusLiving:
		// 幂等：不重复发事件
		return accept()
	case model.RoomStatusStarting, model.RoomStatusEnded:
		// ENDED 是「SRS 断线后同会话重推」，按新一轮直播处理
	default:
		// IDLE / FAILED 说明主播没走开播接口（或已超时失败），拒绝该流
		return reject("直播间当前不允许推流，请先在服务端开播")
	}

	now := time.Now()
	updated, err := s.rooms.Transition(ctx, room.ID, []model.RoomStatus{
		model.RoomStatusStarting, model.RoomStatusEnded,
	}, map[string]any{
		"status": string(model.RoomStatusLiving),
	})
	if err != nil {
		s.log.Error("on_publish_transition_failed", "roomId", room.ID, "error", err)
		return reject("服务内部错误，请重试")
	}
	if !updated {
		// 并发回调已经处理过了
		return accept()
	}

	// startedAt 以 media_live_room.last_started_at 为准
	startedAt := now
	if room.LastStartedAt != nil {
		startedAt = *room.LastStartedAt
	}
	if fresh, err := s.rooms.GetByID(ctx, room.ID); err == nil {
		room = fresh
		s.cache.SetState(ctx, fresh)
	}
	s.cache.DelDetail(ctx, room.ID)

	s.log.Info("live_started", "roomId", room.ID, "sessionId", room.CurrentSession, "startedAt", startedAt)
	s.publish(ctx, events.RoutingLiveStarted, dto.LiveStartedPayload{
		RoomID:    strconv.FormatUint(room.ID, 10),
		OwnerID:   room.OwnerID,
		SessionID: room.CurrentSession,
		Title:     room.Title,
		CoverURL:  room.CoverURL,
		StartedAt: startedAt,
	})
	return accept()
}

// OnUnpublish 处理 SRS on_unpublish：主播断流，语义等同关播。
func (s *LiveService) OnUnpublish(ctx context.Context, req dto.HookRequest) *dto.HookResponse {
	room, sessionID, resp := s.resolveStream(ctx, req)
	if resp != nil {
		// 密钥已经失效（比如我们主动关播过）时不算错误，直接放行让 SRS 收尾
		return accept()
	}
	if sessionID != "" && room.CurrentSession != sessionID {
		return accept()
	}
	if !room.Status.CanStop() {
		return accept()
	}

	if _, err := s.endLive(ctx, room, model.EndReasonUnpublish, false); err != nil {
		s.log.Error("on_unpublish_end_failed", "roomId", room.ID, "error", err)
	}
	return accept()
}

// OnPlay 观众开始播放：当前仅记日志（后续阶段的统计入口）。
func (s *LiveService) OnPlay(ctx context.Context, req dto.HookRequest) *dto.HookResponse {
	s.log.Info("srs_on_play", "stream", req.Stream, "ip", req.IP, "vhost", req.Vhost)
	return accept()
}

// OnStop 观众停止播放。
func (s *LiveService) OnStop(ctx context.Context, req dto.HookRequest) *dto.HookResponse {
	s.log.Info("srs_on_stop", "stream", req.Stream, "ip", req.IP, "vhost", req.Vhost)
	return accept()
}

// DevPublish / DevUnpublish 供本地无 SRS 时模拟回调（仅在 debug 模式注册路由）。
func (s *LiveService) DevPublish(ctx context.Context, streamKey, sessionID string) *dto.HookResponse {
	return s.OnPublish(ctx, dto.HookRequest{
		Action: "on_publish",
		App:    s.cfg.Media.App,
		Stream: streamKey,
		Param:  "?session=" + sessionID,
	})
}

// DevUnpublish 模拟断流。
func (s *LiveService) DevUnpublish(ctx context.Context, streamKey string) *dto.HookResponse {
	return s.OnUnpublish(ctx, dto.HookRequest{
		Action: "on_unpublish",
		App:    s.cfg.Media.App,
		Stream: streamKey,
	})
}

// ---------------------------------------------------------------- 超时兜底

// RunTimeoutSweeper 周期扫描「STARTING 超时未推流」的房间，把它们置为 FAILED。
//
// 为什么需要：主播拿到推流地址后直接关掉客户端，服务端永远不会收到 on_publish，
// 房间就会一直卡在 STARTING，同时占着「一人一房间」的名额。
func (s *LiveService) RunTimeoutSweeper(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Room.CleanupInterval)
	defer ticker.Stop()

	s.log.Info("starting_timeout_sweeper_started",
		"timeout", s.cfg.Room.StartingTimeout, "interval", s.cfg.Room.CleanupInterval)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("starting_timeout_sweeper_stopped")
			return
		case <-ticker.C:
			s.sweepStarting(ctx)
		}
	}
}

func (s *LiveService) sweepStarting(ctx context.Context) {
	deadline := time.Now().Add(-s.cfg.Room.StartingTimeout)
	rooms, err := s.rooms.ListStartingBefore(ctx, deadline)
	if err != nil {
		s.log.Warn("sweep_starting_query_failed", "error", err)
		return
	}

	for i := range rooms {
		room := &rooms[i]
		now := time.Now()
		updated, err := s.rooms.Transition(ctx, room.ID, []model.RoomStatus{model.RoomStatusStarting},
			map[string]any{
				"status":        string(model.RoomStatusFailed),
				"last_ended_at": now,
			})
		if err != nil {
			s.log.Warn("sweep_starting_transition_failed", "roomId", room.ID, "error", err)
			continue
		}
		if !updated {
			continue
		}
		if rec, err := s.records.GetBySessionID(ctx, room.CurrentSession); err == nil {
			_ = s.records.UpdateFields(ctx, rec.ID, map[string]any{
				"ended_at":   now,
				"end_reason": model.EndReasonTimeout,
			})
		}
		s.cache.ClearSession(ctx, room.ID, room.StreamKey)
		s.cache.ReleaseOwnerRoom(ctx, room.OwnerID)
		s.log.Warn("live_start_timeout",
			"roomId", room.ID, "ownerId", room.OwnerID,
			"startedAt", room.LastStartedAt, "timeout", s.cfg.Room.StartingTimeout)
	}
}

// ---------------------------------------------------------------- 内部

// endLive 是三条关播路径（关播接口 / on_unpublish / 强制关闭）的唯一汇聚点。
//
// 幂等性来自 rooms.Transition 的状态条件更新：只有当前仍处于
// STARTING/LIVING 的那一次调用会真正更新成功，其余返回当前快照。
func (s *LiveService) endLive(
	ctx context.Context, room *model.Room, reason string, kick bool,
) (*dto.StopResult, error) {
	now := time.Now()

	peak := s.cache.Peak(ctx, room.ID)
	if peak < int64(room.PeakOnline) {
		peak = int64(room.PeakOnline)
	}
	sessionViewers := s.cache.Viewers(ctx, room.ID)
	viewerTotal := room.ViewerTotal + sessionViewers

	duration := 0
	if room.LastStartedAt != nil {
		duration = int(now.Sub(*room.LastStartedAt).Seconds())
	}

	updated, err := s.rooms.Transition(ctx, room.ID, []model.RoomStatus{
		model.RoomStatusStarting, model.RoomStatusLiving,
	}, map[string]any{
		"status":        string(model.RoomStatusEnded),
		"last_ended_at": now,
		"peak_online":   int(peak),
		"viewer_total":  viewerTotal,
	})
	if err != nil {
		return nil, err
	}
	if !updated {
		latest, err := s.getRoom(ctx, room.ID)
		if err != nil {
			return nil, err
		}
		return &dto.StopResult{
			RoomID:      latest.ID,
			Status:      string(latest.Status),
			SessionID:   latest.CurrentSession,
			EndedAt:     latest.LastEndedAt,
			PeakOnline:  latest.PeakOnline,
			ViewerTotal: latest.ViewerTotal,
		}, nil
	}

	// 场次记录：用记录里的开始时间算时长更准（它不受后续改标题影响）
	if rec, err := s.records.GetBySessionID(ctx, room.CurrentSession); err == nil {
		if !rec.StartedAt.IsZero() {
			duration = int(now.Sub(rec.StartedAt).Seconds())
		}
		_ = s.records.UpdateFields(ctx, rec.ID, map[string]any{
			"ended_at":     now,
			"duration_sec": duration,
			"peak_online":  int(peak),
			"viewer_total": sessionViewers,
			"end_reason":   reason,
		})
	}

	// 清理热数据 + 释放「一人一房间」名额
	s.cache.ClearSession(ctx, room.ID, room.StreamKey)
	s.cache.ReleaseOwnerRoom(ctx, room.OwnerID)

	// 主动断流：失败不影响关播结果（主播重连时会因密钥失效被拒）
	kicked := false
	if kick {
		if err := s.media.KickOff(ctx, room.StreamKey); err != nil {
			s.log.Warn("kick_off_failed", "roomId", room.ID, "streamKey", room.StreamKey, "error", err)
		} else {
			kicked = true
		}
	}

	s.log.Info("live_stopped",
		"roomId", room.ID, "sessionId", room.CurrentSession, "reason", reason,
		"durationSec", duration, "peakOnline", peak, "viewerTotal", viewerTotal)

	s.publish(ctx, events.RoutingLiveStopped, dto.LiveStoppedPayload{
		RoomID:      strconv.FormatUint(room.ID, 10),
		SessionID:   room.CurrentSession,
		EndedAt:     now,
		DurationSec: duration,
		PeakOnline:  int(peak),
		ViewerTotal: viewerTotal,
		Reason:      reason,
	})

	return &dto.StopResult{
		RoomID:       room.ID,
		Status:       string(model.RoomStatusEnded),
		SessionID:    room.CurrentSession,
		EndedAt:      &now,
		DurationSec:  duration,
		PeakOnline:   int(peak),
		ViewerTotal:  viewerTotal,
		EndReason:    reason,
		StreamKicked: kicked,
	}, nil
}

// resolveStream 校验回调的合法性并反查房间。
// 返回的 resp 非 nil 时表示应当直接以该响应结束处理。
func (s *LiveService) resolveStream(
	ctx context.Context, req dto.HookRequest,
) (*model.Room, string, *dto.HookResponse) {
	// 1) app 必须与配置一致，否则视为伪造请求
	if req.App != "" && req.App != s.cfg.Media.App {
		s.log.Warn("hook_app_mismatch", "expect", s.cfg.Media.App, "got", req.App)
		return nil, "", reject("app 不匹配")
	}

	// 2) 可选的回调共享密钥
	if secret := strings.TrimSpace(s.cfg.Media.HookSecret); secret != "" {
		if media.ParamValue(req.Param, "secret") != secret {
			s.log.Warn("hook_secret_mismatch", "stream", req.Stream)
			return nil, "", reject("回调密钥校验失败")
		}
	}

	streamKey := strings.TrimSpace(req.Stream)
	if streamKey == "" {
		return nil, "", reject("缺少 stream")
	}

	sessionID := media.SessionFromParam(req.Param)

	// 3) 反查房间：Redis 快速路径，未命中回源 MySQL
	if roomID, sid, ok := s.cache.LookupStream(ctx, streamKey); ok {
		if room, err := s.rooms.GetByID(ctx, roomID); err == nil && room.StreamKey == streamKey {
			if sessionID == "" {
				sessionID = sid
			}
			return room, sessionID, nil
		}
	}

	room, err := s.rooms.GetByStreamKey(ctx, streamKey)
	if err != nil {
		s.log.Warn("hook_stream_key_not_found", "stream", streamKey)
		return nil, "", reject("推流密钥无效或会话已过期")
	}
	return room, sessionID, nil
}

// ensureNoOtherLive 校验「一人一房间」。
//
// Redis 是快速路径，MySQL 是权威校验：两者都过才允许开播。
// 只有 Redis 不够（它可能丢键），只有 MySQL 也不够（它挡不住并发突发）。
func (s *LiveService) ensureNoOtherLive(ctx context.Context, room *model.Room) error {
	if cur, ok := s.cache.OwnerRoom(ctx, room.OwnerID); ok && cur != room.ID {
		return errs.CodeAlreadyLiving.Newf("已有正在直播的直播间: %d", cur)
	}

	other, err := s.rooms.FindLiveByOwner(ctx, room.OwnerID, room.ID)
	if err != nil {
		return err
	}
	if other != nil {
		return errs.CodeAlreadyLiving.Newf("已有正在直播的直播间: %d", other.ID)
	}
	return nil
}

// startResult 构造开播响应（幂等路径复用）。
func (s *LiveService) startResult(ctx context.Context, room *model.Room) *dto.StartResult {
	urls := s.media.PlayURLs(room.StreamKey)
	startedAt := time.Now()
	if room.LastStartedAt != nil {
		startedAt = *room.LastStartedAt
	}
	return &dto.StartResult{
		RoomID:       room.ID,
		Status:       string(room.Status),
		SessionID:    room.CurrentSession,
		StreamKey:    room.StreamKey,
		PushURL:      s.media.PushURL(room.StreamKey, room.CurrentSession),
		PlayURLs:     &dto.PlayURLs{HLS: urls.HLS, HTTPFLV: urls.HTTPFLV, WebRTC: urls.WebRTC, RTMP: urls.RTMP},
		SRSAvailable: s.srsAvailable(ctx),
		StartedAt:    startedAt,
		Hint:         "请使用 pushUrl 推流；服务端收到 SRS on_publish 回调后状态才会变为 LIVING",
	}
}

// publish 发布事件；失败只告警（关播/开播的主流程不依赖 MQ 可用）。
func (s *LiveService) publish(ctx context.Context, routingKey string, payload any) {
	ev, err := events.New(routingKey, trace.From(ctx), payload)
	if err != nil {
		s.log.Warn("build_event_failed", "routingKey", routingKey, "error", err)
		return
	}
	if err := s.bus.Publish(ctx, routingKey, ev); err != nil {
		s.log.Warn("publish_event_failed", "routingKey", routingKey, "eventId", ev.EventID, "error", err)
	}
}

// accept / reject 是 SRS 回调的两种标准答复。
func accept() *dto.HookResponse {
	return &dto.HookResponse{Code: 0, Message: "success"}
}

func reject(message string) *dto.HookResponse {
	return &dto.HookResponse{Code: errs.CodeStreamKeyInvalid.Code, Message: message}
}
