// Package events 提供事件总线抽象与两种实现（RabbitMQ / 进程内）。
//
// 为什么要抽象：转码是典型的异步长耗时任务，生产环境必须走真实 MQ；
// 但本地开发时「没起 RabbitMQ 就完全跑不起来」是糟糕的体验，
// 因此提供 LocalBus 作为降级实现（日志会显式告警其不持久化）。
package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// 路由键常量。
const (
	RoutingVideoUploaded   = "video.uploaded"
	RoutingVideoTranscoded = "video.transcoded"
)

// 交换机前缀：所有业务交换机都长这样。
const ExchangePrefix = "go.media."

// ExchangeFor 由路由键推导**生产方**交换机。
//
// 约定：路由键形如 "<域>.<事件>"，域就是交换机后缀。
//
//	video.uploaded   → go.media.video
//	live.started     → go.media.live
//	danmaku.created  → go.media.danmaku
//
// **订阅时必须绑定到生产方的交换机，而不是自己那个。** 否则会永远收不到
// 别的服务发的事件：danmaku 订阅 live.started 却把队列绑在 go.media.danmaku 上，
// 而 live-service 是发到 go.media.live 的，两边就像活在平行世界 ——
// 日志里一切正常（队列有消费者、绑定也成功），只是永远没有流量。
func ExchangeFor(routingKey string) string {
	domain := routingKey
	if i := strings.Index(routingKey, "."); i > 0 {
		domain = routingKey[:i]
	}
	return ExchangePrefix + domain
}

// Event 是所有事件的统一信封。
//
// EventID 是幂等键：MQ 至少一次投递语义下，消费侧必须据此去重。
type Event struct {
	EventID    string          `json:"eventId"`
	EventType  string          `json:"eventType"`
	OccurredAt time.Time       `json:"occurredAt"`
	TraceID    string          `json:"traceId"`
	Payload    json.RawMessage `json:"payload"`
}

// New 构造事件，payload 会被序列化为 JSON。
func New(eventType, traceID string, payload any) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("序列化事件载荷失败: %w", err)
	}
	return Event{
		EventID:    uuid.NewString(),
		EventType:  eventType,
		OccurredAt: time.Now(),
		TraceID:    traceID,
		Payload:    raw,
	}, nil
}

// Decode 把载荷反序列化到 v。
func (e Event) Decode(v any) error {
	if len(e.Payload) == 0 {
		return fmt.Errorf("事件 %s 载荷为空", e.EventID)
	}
	return json.Unmarshal(e.Payload, v)
}

// Encode 把事件序列化为 JSON（发送前调用）。
func (e Event) Encode() ([]byte, error) {
	return json.Marshal(e)
}
