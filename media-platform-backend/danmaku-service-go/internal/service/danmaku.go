package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/infra/cache"
	"github.com/example/danmaku-service-go/internal/infra/events"
	"github.com/example/danmaku-service-go/internal/infra/filter"
	"github.com/example/danmaku-service-go/internal/infra/hub"
	"github.com/example/danmaku-service-go/internal/infra/ratelimit"
	"github.com/example/danmaku-service-go/internal/model"
	"github.com/example/danmaku-service-go/internal/repository"
	"github.com/example/danmaku-service-go/pkg/errs"
	"github.com/example/danmaku-service-go/pkg/identity"
	"github.com/example/danmaku-service-go/pkg/trace"
)

// DanmakuService 实现发送、撤回、历史与统计。
type DanmakuService struct {
	Deps
}

// NewDanmakuService 构造弹幕服务。
func NewDanmakuService(d Deps) *DanmakuService { return &DanmakuService{Deps: d} }

// RoomHotCount 返回房间当前的热数据条数。
func (s *DanmakuService) RoomHotCount(ctx context.Context, roomID string) (int64, error) {
	return s.Recent.RoomLen(ctx, roomID)
}

// Send 是 HTTP 与 WebSocket **共用的**发送入口。
//
// 执行顺序与理由（设计文档 §5）：
//
//		校验 → 去重 → 直播状态 → 敏感词 → 限流 → 写热数据 → 广播 → 投递落库事件
//
//	  - 校验类操作必须最先；
//	  - 敏感词（纯内存，微秒级）排在限流（要打 Redis）之前，先做便宜的；
//	  - 先写 Redis 再广播：保证「客户端收到弹幕时，历史接口一定查得到」；
//	  - 广播不等 MQ：持久化不该出现在实时链路的延迟路径上。
func (s *DanmakuService) Send(ctx context.Context, in dto.SendInput) (*dto.SendResult, error) {
	uid := in.UID
	if uid == "" {
		// 游客不能发送（FR-2 规则）。WebSocket 侧由 handler 先判 ReadOnly，
		// 这里是「无论如何都挡住」的最后一道。
		return nil, errs.CodeUnauthorized.Plain()
	}

	roomID, videoID, content, color, position, err := s.validateSendInput(in)
	if err != nil {
		return nil, err
	}

	clientMsgID := in.ClientMsgID
	if result, err := s.replayIfDuplicated(ctx, clientMsgID); err != nil || result != nil {
		return result, err
	}

	if err := s.checkLiving(ctx, roomID); err != nil {
		return nil, err
	}

	content, err = s.applyFilter(content, uid, roomID, videoID)
	if err != nil {
		return nil, err
	}

	if err := s.applyRateLimit(ctx, uid, roomID, videoID); err != nil {
		return nil, err
	}

	item := dto.DanmakuDTO{
		DanmakuID:   0, // 落库由 MQ 异步完成，此处必然还没有自增 ID
		ClientMsgID: clientMsgID,
		RoomID:      roomID,
		VideoID:     videoID,
		UID:         uid,
		Nickname:    displayName(in.Nickname, uid),
		Content:     content,
		Color:       color,
		Position:    position,
		TimeMs:      in.TimeMs,
		SentAt:      time.Now(),
		Status:      model.StatusNormal,
	}

	s.persistHot(ctx, item)
	s.broadcast(item)

	result := &dto.SendResult{DanmakuDTO: item}
	result.Persisted = s.publishCreated(ctx, item)

	// 覆盖去重占位符：后续重复请求拿到与本响应一致的快照。
	if clientMsgID != "" {
		if err := s.Recent.PutDedupSnapshot(ctx, clientMsgID, result); err != nil {
			s.logger().Warn("dedup_snapshot_failed", "clientMsgId", clientMsgID, "err", err.Error())
		}
	}
	return result, nil
}

// validateSendInput 做全部字段级校验，并返回归一化后的值。
func (s *DanmakuService) validateSendInput(in dto.SendInput) (roomID, videoID, content, color, position string, err error) {
	roomID = trim(in.RoomID)
	videoID = trim(in.VideoID)
	if (roomID == "") == (videoID == "") {
		return "", "", "", "", "", errs.CodeInvalidParam.New("roomId 与 videoId 必须二选一")
	}

	content = trim(in.Content)
	if content == "" {
		return "", "", "", "", "", errs.CodeInvalidParam.New("content 不能为空")
	}
	if n := utf8.RuneCountInString(content); n > s.Config.Danmaku.MaxContentLen {
		return "", "", "", "", "", errs.CodeInvalidParam.Newf(
			"content 长度不能超过 %d 个字符", s.Config.Danmaku.MaxContentLen)
	}

	if color, err = normalizeColor(in.Color); err != nil {
		return "", "", "", "", "", err
	}
	if position, err = normalizePosition(in.Position); err != nil {
		return "", "", "", "", "", err
	}
	if in.TimeMs < 0 {
		return "", "", "", "", "", errs.CodeInvalidParam.New("timeMs 不能为负数")
	}
	return roomID, videoID, content, color, position, nil
}

// replayIfDuplicated 处理 clientMsgId 去重。
//
// 返回 (non-nil, nil) 表示应直接把首发结果回放给调用方；
// 返回 (nil, err) 表示应当中止；
// 返回 (nil, nil) 表示可以继续处理（首次请求）。
func (s *DanmakuService) replayIfDuplicated(ctx context.Context, clientMsgID string) (*dto.SendResult, error) {
	if clientMsgID == "" {
		return nil, nil
	}
	first, err := s.Recent.ClaimDedup(ctx, clientMsgID)
	if err != nil {
		s.logger().Warn("dedup_claim_failed", "clientMsgId", clientMsgID, "err", err.Error())
	}
	if first {
		return nil, nil
	}

	snapshot, err := s.Recent.DedupSnapshot(ctx, clientMsgID)
	if err != nil {
		s.logger().Warn("dedup_read_failed", "clientMsgId", clientMsgID, "err", err.Error())
	}
	if snapshot == "" || snapshot == cache.DedupPendingMarker {
		// 首发的处理还没结束：明确告诉调用方，而不是让它以为「发成功了」。
		return nil, errs.CodeConflict.New("重复请求正在处理中，请稍后重试")
	}

	var result dto.SendResult
	if err := json.Unmarshal([]byte(snapshot), &result); err != nil {
		s.logger().Warn("dedup_decode_failed", "clientMsgId", clientMsgID, "err", err.Error())
		return nil, errs.CodeConflict.New("重复请求正在处理中，请稍后重试")
	}
	result.Duplicated = true
	return &result, nil
}

// checkLiving 校验直播状态。
//
// 三种情况要区分开：
//   - 明确 `1`（直播中）  → 放行
//   - 明确 `0`（已结束）  → 拒绝 409 / 80003
//   - 无状态记录（未知）  → 由 danmaku.require_living 决定：
//     false（默认）放行，true 拒绝。
//     本地单独起本服务调试时没有 live-service 发事件，若把「未知」当「未开播」，
//     整个发送链路就永远验证不了。
func (s *DanmakuService) checkLiving(ctx context.Context, roomID string) error {
	if roomID == "" {
		return nil // 点播弹幕不校验直播状态
	}
	living, known, err := s.Recent.Living(ctx, roomID)
	if err != nil {
		s.logger().Warn("live_state_unavailable", "roomId", roomID, "err", err.Error())
		return nil
	}
	if living {
		return nil
	}
	if !known && !s.Config.Danmaku.RequireLiving {
		return nil
	}
	return errs.CodeRoomNotLiving.New("直播间当前未开播")
}

// applyFilter 执行敏感词过滤，返回最终应当广播/落库的内容。
func (s *DanmakuService) applyFilter(content, uid, roomID, videoID string) (string, error) {
	decision := s.Filter.Check(content)
	switch decision.Outcome {
	case filter.OutcomeReject:
		s.logger().Warn("danmaku_filter_rejected",
			"uid", uid, "roomId", roomID, "videoId", videoID,
			"word", decision.Matched, "content", filter.Mask(content))
		return "", errs.CodeContentRejected.New("弹幕包含违规内容")
	case filter.OutcomeReplace:
		s.logger().Info("danmaku_filter_replaced",
			"uid", uid, "roomId", roomID, "videoId", videoID, "word", decision.Matched)
		return decision.Content, nil
	case filter.OutcomeLog:
		s.logger().Warn("danmaku_filter_log_only",
			"uid", uid, "roomId", roomID, "videoId", videoID,
			"word", decision.Matched, "content", filter.Mask(content))
		return content, nil
	default:
		return content, nil
	}
}

// applyRateLimit 执行双层限流。房间级被拒时不回滚用户级计数（设计文档 §7.2）。
func (s *DanmakuService) applyRateLimit(ctx context.Context, uid, roomID, videoID string) error {
	decision, err := s.Limiter.Allow(ctx, uid, resourceKey(roomID, videoID))
	if err != nil {
		s.logger().Warn("ratelimit_error", "uid", uid, "err", err.Error())
		return nil
	}
	if decision.Allowed {
		return nil
	}

	data := map[string]any{"retryAfterMs": decision.RetryAfterMs}
	if decision.Scope == ratelimit.ScopeUser {
		return errs.CodeRateLimited.WithData("发送过于频繁", data)
	}
	return errs.CodeRateLimited.WithData("当前直播间弹幕过多，请稍后再试", data)
}

// persistHot 写 Redis 热数据。写失败只告警：热数据是性能优化，
// 不是事实来源（事实来源是 MySQL），不该让发送失败。
func (s *DanmakuService) persistHot(ctx context.Context, item dto.DanmakuDTO) {
	raw := marshal(item)
	if raw == "" {
		return
	}
	var err error
	if item.RoomID != "" {
		err = s.Recent.PushRoom(ctx, item.RoomID, raw)
	} else {
		err = s.Recent.PushVideo(ctx, item.VideoID, raw)
	}
	if err != nil {
		s.logger().Warn("hot_data_write_failed",
			"roomId", item.RoomID, "videoId", item.VideoID, "err", err.Error())
	}
}

// broadcast 立即向房间推送（不等落库）。
func (s *DanmakuService) broadcast(item dto.DanmakuDTO) {
	subject := hub.SubjectKey(item.RoomID, item.VideoID)
	if subject == "" {
		return
	}
	frame, err := hub.NewFrame(hub.TypeDanmaku, item)
	if err != nil {
		s.logger().Warn("broadcast_encode_failed", "err", err.Error())
		return
	}
	if dropped := s.Hub.Broadcast(subject, frame); dropped > 0 {
		s.logger().Warn("broadcast_frames_dropped", "subject", subject, "dropped", dropped,
			"hint", "慢客户端背压：这些连接来不及消费，帧已被丢弃")
	}
}

// publishCreated 投递落库事件，失败则进 Redis 补发队列。
//
// 返回 persisted：false 表示「最终一致性暂时不保证」，
// 但**广播已经完成**，客户端体验不受影响。
func (s *DanmakuService) publishCreated(ctx context.Context, item dto.DanmakuDTO) bool {
	payload := dto.DanmakuCreatedPayload{
		RoomID:      item.RoomID,
		VideoID:     item.VideoID,
		UID:         item.UID,
		Nickname:    item.Nickname,
		Content:     item.Content,
		Color:       item.Color,
		Position:    item.Position,
		TimeMs:      item.TimeMs,
		ClientMsgID: item.ClientMsgID,
		SentAt:      item.SentAt,
	}
	ev, err := events.New(events.RoutingDanmakuCreated, trace.From(ctx), payload)
	if err != nil {
		s.logger().Error("persist_event_build_failed", "err", err.Error())
		return false
	}

	var lastErr error
	for attempt := 0; attempt <= s.Config.Danmaku.MaxPersistRetry; attempt++ {
		if lastErr = s.Bus.Publish(ctx, events.RoutingDanmakuCreated, ev); lastErr == nil {
			return true
		}
	}
	s.logger().Error("persist_event_publish_failed",
		"eventId", ev.EventID, "err", lastErr.Error(),
		"hint", "已写入 Redis 补发队列，MQ 恢复后会自动重投")

	raw, encodeErr := ev.Encode()
	if encodeErr != nil {
		s.logger().Error("persist_event_encode_failed", "eventId", ev.EventID, "err", encodeErr.Error())
		return false
	}
	if err := s.Recent.PushPending(ctx, string(raw)); err != nil {
		s.logger().Error("persist_pending_push_failed", "eventId", ev.EventID, "err", err.Error())
	}
	return false
}

// CheckJoinable 供 WebSocket 握手使用：校验房间当前是否允许建立弹幕连接。
//
// 与发送路径复用同一套直播状态判断，避免「能连上但一条也发不出去」的困惑。
func (s *DanmakuService) CheckJoinable(ctx context.Context, roomID, videoID string) error {
	return s.checkLiving(ctx, roomID)
}

// Recall 撤回弹幕（本人或 ADMIN）。
//
// 撤回**不广播**：直播场景下把一条已投屏的弹幕突然抹掉会造成视觉跳跃，
// 产品上选择「静默撤回」——客户端下次拉取历史时消失（接口文档 §3.3）。
func (s *DanmakuService) Recall(ctx context.Context, user identity.CurrentUser, danmakuID uint64) (*dto.RecallResult, error) {
	if !user.Authenticated() {
		return nil, errs.CodeUnauthorized.Plain()
	}

	item, err := s.Danmaku.GetByID(ctx, danmakuID)
	if repository.IsNotFound(err) {
		return nil, errs.CodeDanmakuNotFound.Plain()
	}
	if err != nil {
		return nil, fmt.Errorf("查询弹幕失败: %w", err)
	}

	if item.UID != user.UID && !user.IsAdmin() {
		return nil, errs.CodeForbidden.New("只能撤回自己发送的弹幕")
	}

	now := time.Now()
	if item.Status == model.StatusRecalled {
		// 幂等：重复撤回不报错，直接返回当前状态。
		return &dto.RecallResult{DanmakuID: danmakuID, Status: model.StatusRecalled, RecalledAt: now}, nil
	}

	if window := s.Config.Danmaku.RecallWindow; window > 0 {
		if time.Since(item.SentAt) > window {
			return nil, errs.CodeRecallWindowExpired.Newf(
				"超过可撤回时间窗(%ds)", int(window.Seconds()))
		}
	}

	changed, err := s.Danmaku.MarkRecalled(ctx, danmakuID)
	if err != nil {
		return nil, fmt.Errorf("撤回弹幕失败: %w", err)
	}

	if changed && item.RoomID != "" && item.ClientMsgID != "" {
		if err := s.Recent.RemoveRoomItem(ctx, item.RoomID, item.ClientMsgID); err != nil {
			s.logger().Warn("recall_hot_remove_failed",
				"danmakuId", danmakuID, "roomId", item.RoomID, "err", err.Error())
		}
	}

	s.logger().Info("danmaku_recalled",
		"danmakuId", danmakuID, "operator", user.UID, "owner", item.UID,
		"admin", user.IsAdmin(), "changed", changed)

	return &dto.RecallResult{DanmakuID: danmakuID, Status: model.StatusRecalled, RecalledAt: now}, nil
}
