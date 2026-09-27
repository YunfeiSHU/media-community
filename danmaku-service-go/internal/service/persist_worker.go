package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/infra/events"
	"github.com/example/danmaku-service-go/internal/model"
)

// 补发队列的批量参数。
const (
	pendingBatchSize = 200
	// pendingMaxBatches 限制单次重放的批次数，避免一次重放把进程拖住太久。
	pendingMaxBatches = 50
	// republishInterval 是补发任务的执行间隔。
	republishInterval = 10 * time.Second
)

// PersistWorker 消费 `danmaku.created` 事件，把弹幕落库。
//
// 这是**持久化链路**的唯一出口：实时链路（广播）与它完全解耦，
// 因此 MySQL 慢 / 挂只会影响「历史可查性」，不影响观众观感。
type PersistWorker struct {
	Deps
}

// NewPersistWorker 构造落库 worker。
func NewPersistWorker(d Deps) *PersistWorker { return &PersistWorker{Deps: d} }

// Register 订阅落库事件。应在启动时调用一次。
func (w *PersistWorker) Register() error {
	return w.Bus.Subscribe(events.RoutingDanmakuCreated, w.Handle)
}

// Handle 处理一条待落库事件。
//
// 幂等与错误处理要点：
//   - 先占位 Redis 幂等键（eventId），重投直接被跳过；
//   - 若落库失败，必须**回滚幂等键**并返回 error，让 MQ 重投/进死信，
//     否则就是「标记已处理但实际没写库」的静默丢数据；
//   - 脏数据（缺 uid / 内容）直接丢弃且不返回 error：重投一万次也不会变好，
//     只会把队列堵死。
func (w *PersistWorker) Handle(ctx context.Context, ev events.Event) error {
	if ev.EventType != "" && ev.EventType != events.RoutingDanmakuCreated {
		return nil
	}

	first, err := w.Recent.MarkEvent(ctx, ev.EventID, w.Config.Events.IdempotentTTL)
	if err != nil {
		// 幂等键不可用：放行并继续落库（重复风险由 (uid, clientMsgId) 短时去重兜底）。
		w.logger().Warn("idem_mark_failed", "eventId", ev.EventID, "err", err.Error())
	}
	if !first {
		w.logger().Debug("persist_skipped_duplicate", "eventId", ev.EventID)
		return nil
	}

	var payload dto.DanmakuCreatedPayload
	if err := ev.Decode(&payload); err != nil {
		w.logger().Error("persist_payload_decode_failed", "eventId", ev.EventID, "err", err.Error())
		return nil
	}
	if payload.UID == "" || payload.Content == "" || (payload.RoomID == "" && payload.VideoID == "") {
		w.logger().Warn("persist_payload_invalid", "eventId", ev.EventID,
			"roomId", payload.RoomID, "videoId", payload.VideoID, "uid", payload.UID)
		return nil
	}

	sentAt := payload.SentAt
	if sentAt.IsZero() {
		sentAt = time.Now()
	}

	item := model.Danmaku{
		RoomID:      payload.RoomID,
		VideoID:     payload.VideoID,
		UID:         payload.UID,
		Nickname:    payload.Nickname,
		Content:     payload.Content,
		Color:       payload.Color,
		Position:    payload.Position,
		TimeMs:      payload.TimeMs,
		ClientMsgID: payload.ClientMsgID,
		Status:      model.StatusNormal,
		SentAt:      sentAt,
		CreatedAt:   time.Now(),
	}

	if err := w.Danmaku.Create(ctx, &item); err != nil {
		// 回滚幂等键，让重投真的能重试。
		if clearErr := w.Recent.ClearEvent(ctx, ev.EventID); clearErr != nil {
			w.logger().Error("idem_clear_failed", "eventId", ev.EventID, "err", clearErr.Error())
		}
		return fmt.Errorf("写入弹幕失败: %w", err)
	}

	w.logger().Info("danmaku_persisted",
		"danmakuId", item.ID,
		"roomId", item.RoomID, "videoId", item.VideoID,
		"uid", item.UID, "clientMsgId", item.ClientMsgID, "eventId", ev.EventID)
	return nil
}

// RunRepublisher 周期性重放补发队列（MQ 恢复后自动补投）。
//
// 关于「重投成功后如何确认已落库」：这里**不做删除确认**，
// 由消费侧的幂等（eventId + 短时 clientMsgId 去重）兜住「重投成功但本进程崩了」
// 造成的重复投递 —— 重复会被幂等跳过，业务无害（设计文档 §8.3）。
func (w *PersistWorker) RunRepublisher(ctx context.Context) {
	ticker := time.NewTicker(republishInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Bus.Health(ctx); err != nil {
				continue // MQ 还没恢复，不刷日志
			}
			count, err := w.FlushPending(ctx)
			if err != nil {
				w.logger().Warn("pending_republish_partial", "republished", count, "err", err.Error())
				continue
			}
			if count > 0 {
				w.logger().Info("pending_republished", "count", count)
			}
		}
	}
}

// FlushPending 尽力把补发队列重投回 MQ，返回成功重投的条数。
func (w *PersistWorker) FlushPending(ctx context.Context) (int, error) {
	if err := w.Bus.Health(ctx); err != nil {
		return 0, fmt.Errorf("事件总线不可用: %w", err)
	}

	total := 0
	for batch := 0; batch < pendingMaxBatches; batch++ {
		raw, err := w.Recent.PopPending(ctx, pendingBatchSize)
		if err != nil {
			return total, err
		}
		if len(raw) == 0 {
			return total, nil
		}

		failed := make([]string, 0, 4)
		for _, line := range raw {
			var ev events.Event
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				// 队列里的坏数据直接丢弃：留着只会每 10s 重试一次永远失败。
				w.logger().Warn("pending_item_corrupted", "err", err.Error())
				continue
			}
			if err := w.Bus.Publish(ctx, events.RoutingDanmakuCreated, ev); err != nil {
				failed = append(failed, line)
				continue
			}
			total++
		}

		if len(failed) > 0 {
			if err := w.Recent.RequeuePending(ctx, failed); err != nil {
				w.logger().Error("pending_requeue_failed", "count", len(failed), "err", err.Error())
			}
			return total, fmt.Errorf("%d 条重投失败，已放回队列", len(failed))
		}
	}
	return total, nil
}

// PendingLen 返回补发队列长度（健康检查 / 监控用）。
func (w *PersistWorker) PendingLen(ctx context.Context) int64 {
	n, err := w.Recent.PendingLen(ctx)
	if err != nil {
		return 0
	}
	return n
}
