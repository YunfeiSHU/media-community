package service

import (
	"context"
	"fmt"
	"time"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/infra/events"
	"github.com/example/danmaku-service-go/internal/infra/hub"
)

// LiveBridge 订阅 live-service 的事件，把「直播间是否直播中」同步到本服务。
//
// 事件消费契约（接口文档 §8）：
//
//	live.started → SET danmaku:live:{roomId}="1"；清理上一轮热数据；广播 LIVE_STARTED
//	live.stopped → SET danmaku:live:{roomId}="0"（**写 0 而不是删键**）；广播 LIVE_ENDED；补落库
//
// 键值只有 "1"/"0" 两种确定状态；键不存在才代表「状态未知」（本地单独跑 danmaku
// 没有 live-service 发事件时就是这种情形，require_living=false 下放行以便调试）。
type LiveBridge struct {
	Deps
	persist *PersistWorker
}

// NewLiveBridge 构造桥接器；persist 用于 live.stopped 时的最后一轮补落库。
func NewLiveBridge(d Deps, persist *PersistWorker) *LiveBridge {
	return &LiveBridge{Deps: d, persist: persist}
}

// Register 订阅直播事件。启动时调用一次。
func (b *LiveBridge) Register() error {
	if err := b.Bus.Subscribe(events.RoutingLiveStarted, b.onStarted); err != nil {
		return err
	}
	return b.Bus.Subscribe(events.RoutingLiveStopped, b.onStopped)
}

func (b *LiveBridge) onStarted(ctx context.Context, ev events.Event) error {
	var payload dto.LiveStartedPayload
	if err := ev.Decode(&payload); err != nil {
		return fmt.Errorf("解析 live.started 载荷失败: %w", err)
	}
	if payload.RoomID == "" {
		return nil
	}

	if err := b.Recent.SetLiving(ctx, payload.RoomID, true); err != nil {
		b.logger().Warn("live_state_set_failed", "roomId", payload.RoomID, "err", err.Error())
	}
	// 清理上一轮热数据：否则观众进来会看到上一场直播的弹幕。
	if err := b.Recent.DelRoom(ctx, payload.RoomID); err != nil {
		b.logger().Warn("live_hot_clear_failed", "roomId", payload.RoomID, "err", err.Error())
	}

	b.broadcastSystem(payload.RoomID, hub.EventLiveStarted, "直播已开始")
	b.logger().Info("live_started_received",
		"roomId", payload.RoomID, "ownerId", payload.OwnerID,
		"sessionId", payload.SessionID, "eventId", ev.EventID)
	return nil
}

func (b *LiveBridge) onStopped(ctx context.Context, ev events.Event) error {
	var payload dto.LiveStoppedPayload
	if err := ev.Decode(&payload); err != nil {
		return fmt.Errorf("解析 live.stopped 载荷失败: %w", err)
	}
	if payload.RoomID == "" {
		return nil
	}

	// 标记为「明确已结束」（写 "0"，不是删键）。
	// 删键会让状态退化成 known=false（未知），在 require_living=false 下就变成放行，
	// 关播后还能继续刷弹幕 —— 与需求文档 FR-6.3「未开播的房间拒绝发送」相反。
	if err := b.Recent.SetLiving(ctx, payload.RoomID, false); err != nil {
		b.logger().Warn("live_state_set_failed", "roomId", payload.RoomID, "err", err.Error())
	}

	// 房间保留（观众可能还在看结束画面），只广播系统消息。
	b.broadcastSystem(payload.RoomID, hub.EventLiveEnded, "直播已结束")
	b.logger().Info("live_stopped_received",
		"roomId", payload.RoomID, "sessionId", payload.SessionID,
		"durationSec", payload.DurationSec, "peakOnline", payload.PeakOnline, "eventId", ev.EventID)

	// 触发一次「补落库」：若之前 MQ 抖动过，这里把补发队列排空。
	// 不去重投「热数据里的全部弹幕」——那会制造重复行（client_msg_id 无唯一索引），
	// 收益只是心理安慰，代价是数据污染。
	if b.persist != nil {
		if n, err := b.persist.FlushPending(ctx); err != nil {
			b.logger().Warn("live_stop_flush_partial", "roomId", payload.RoomID, "republished", n, "err", err.Error())
		} else if n > 0 {
			b.logger().Info("live_stop_flushed", "roomId", payload.RoomID, "republished", n)
		}
	}
	return nil
}

func (b *LiveBridge) broadcastSystem(roomID, event, message string) {
	frame, err := hub.NewFrame(hub.TypeSystem, hub.SystemData{
		Event:   event,
		Message: message,
		At:      time.Now(),
	})
	if err != nil {
		b.logger().Warn("system_frame_encode_failed", "event", event, "err", err.Error())
		return
	}
	b.Hub.Broadcast(hub.SubjectKey(roomID, ""), frame)
}
