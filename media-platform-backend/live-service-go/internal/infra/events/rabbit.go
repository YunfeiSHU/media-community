package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/example/live-service-go/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

// 消息头常量。
const (
	headerTraceID = "x-trace-id"
	headerEventID = "x-event-id"
	headerRetry   = "x-retry"
)

// handlerTimeout 是单条消息处理的兜底超时。
// 转码本身另有 transcoder.timeout，这里取更大值只用于防止 goroutine 永久泄漏。
const handlerTimeout = 2 * time.Hour

// RabbitBus 是基于 RabbitMQ 的事件总线实现。
//
// 可靠性设计：
//   - topic exchange + 队列持久化 + 消息 Persistent
//   - 消费侧手动 ACK；处理失败按 x-retry 计数重投，超过阈值 Nack(requeue=false) 进死信队列
//   - 消费侧用 EventID 做幂等（由业务方在 handler 内实现，见 transcode worker）
type RabbitBus struct {
	conn     *amqp.Connection
	pubCh    *amqp.Channel
	exchange string
	prefetch int
	retry    int
	log      *slog.Logger

	mu        sync.Mutex // amqp.Channel 不是并发安全的，发布必须串行
	consumers []*rabbitConsumer
	closed    bool
}

type rabbitConsumer struct {
	ch   *amqp.Channel
	done chan struct{}
}

// NewRabbitBus 建立连接、声明交换机（含死信交换机）。
func NewRabbitBus(cfg config.Events, log *slog.Logger) (*RabbitBus, error) {
	conn, err := amqp.DialConfig(cfg.URL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
	})
	if err != nil {
		return nil, fmt.Errorf("连接 RabbitMQ 失败: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("创建发布 channel 失败: %w", err)
	}

	// 主交换机
	if err := pubCh.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("声明交换机 %s 失败: %w", cfg.Exchange, err)
	}
	// 死信交换机
	if err := pubCh.ExchangeDeclare(DLXNameFor(cfg.Exchange), "topic", true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("声明死信交换机 %s 失败: %w", DLXNameFor(cfg.Exchange), err)
	}

	return &RabbitBus{
		conn:     conn,
		pubCh:    pubCh,
		exchange: cfg.Exchange,
		prefetch: cfg.Prefetch,
		retry:    cfg.Retry,
		log:      log,
	}, nil
}

// Publish 发布持久化事件。
func (b *RabbitBus) Publish(ctx context.Context, routingKey string, ev Event) error {
	body, err := ev.Encode()
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return fmt.Errorf("事件总线已关闭")
	}
	return b.pubCh.PublishWithContext(ctx, b.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		MessageId:    ev.EventID,
		Headers: amqp.Table{
			headerTraceID: ev.TraceID,
			headerEventID: ev.EventID,
			headerRetry:   int32(0),
		},
		Body: body,
	})
}

// Subscribe 声明队列（含死信绑定）并启动消费 goroutine。
func (b *RabbitBus) Subscribe(routingKey string, h Handler) error {
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("创建消费 channel 失败: %w", err)
	}
	if err := ch.Qos(b.prefetch, 0, false); err != nil {
		_ = ch.Close()
		return fmt.Errorf("设置 QoS 失败: %w", err)
	}

	// owner  = 自己的交换机：用来给队列/DLQ/DLX 做命名空间（同一条事件
	//          可能被多个服务订阅，队列必须各服务独立，否则会变成竞争消费）。
	// source = **生产方**交换机：真正要绑定的对象。
	owner := b.exchange
	source := ExchangeFor(routingKey)
	dlx := DLXNameFor(owner)
	queue := QueueNameFor(owner, routingKey)
	dlq := DLQNameFor(owner, routingKey)

	// 生产方交换机可能还没被对端服务创建（启动顺序不定）；幂等声明一次，
	// 参数（topic/durable）与对方的声明完全一致，不会冲突。
	if err := ch.ExchangeDeclare(source, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("声明上游交换机 %s 失败: %w", source, err)
	}

	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("声明死信队列 %s 失败: %w", dlq, err)
	}
	if err := ch.QueueBind(dlq, routingKey, dlx, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("绑定死信队列 %s 失败: %w", dlq, err)
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange": dlx,
	}); err != nil {
		_ = ch.Close()
		return fmt.Errorf("声明队列 %s 失败: %w", queue, err)
	}
	if err := ch.QueueBind(queue, routingKey, source, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("绑定队列 %s 到交换机 %s 失败: %w", queue, source, err)
	}

	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("启动消费 %s 失败: %w", queue, err)
	}

	consumer := &rabbitConsumer{ch: ch, done: make(chan struct{})}
	b.mu.Lock()
	b.consumers = append(b.consumers, consumer)
	b.mu.Unlock()

	go b.consume(consumer, routingKey, deliveries, h)
	b.log.Info("event_subscribed", "driver", "rabbitmq", "routingKey", routingKey, "queue", queue)
	return nil
}

func (b *RabbitBus) consume(c *rabbitConsumer, routingKey string, deliveries <-chan amqp.Delivery, h Handler) {
	for {
		select {
		case <-c.done:
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			b.handleDelivery(c, routingKey, d, h)
		}
	}
}

func (b *RabbitBus) handleDelivery(c *rabbitConsumer, routingKey string, d amqp.Delivery, h Handler) {
	var ev Event
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		// 报文本身不可解析：重试没有意义，直接进死信队列
		b.log.Error("rabbit_message_decode_failed", "routingKey", routingKey, "error", err)
		_ = d.Nack(false, false)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	if err := h(ctx, ev); err != nil {
		retries := retryCount(d.Headers)
		if retries < b.retry {
			b.log.Warn("rabbit_message_retry",
				"routingKey", routingKey, "eventId", ev.EventID, "retry", retries+1, "error", err)
			if pubErr := b.republish(c.ch, routingKey, ev, retries+1); pubErr != nil {
				b.log.Error("rabbit_republish_failed", "eventId", ev.EventID, "error", pubErr)
				_ = d.Nack(false, true) // 兜底：重新入队
				return
			}
			_ = d.Ack(false)
			return
		}

		b.log.Error("rabbit_message_dead_letter",
			"routingKey", routingKey, "eventId", ev.EventID, "retry", retries, "error", err)
		_ = d.Nack(false, false)
		return
	}

	_ = d.Ack(false)
}

func (b *RabbitBus) republish(ch *amqp.Channel, routingKey string, ev Event, retry int) error {
	body, err := ev.Encode()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return ch.PublishWithContext(ctx, b.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		MessageId:    ev.EventID,
		Headers: amqp.Table{
			headerTraceID: ev.TraceID,
			headerEventID: ev.EventID,
			headerRetry:   int32(retry),
		},
		Body: body,
	})
}

// Driver 返回实现名。
func (b *RabbitBus) Driver() string { return "rabbitmq" }

// Health 检查连接是否仍然可用。
func (b *RabbitBus) Health(context.Context) error {
	if b.conn == nil || b.conn.IsClosed() {
		return fmt.Errorf("RabbitMQ 连接已关闭")
	}
	return nil
}

// Close 依次关闭消费者 channel、发布 channel 与连接。
func (b *RabbitBus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	consumers := b.consumers
	b.consumers = nil
	b.mu.Unlock()

	for _, c := range consumers {
		close(c.done)
		_ = c.ch.Close()
	}
	if b.pubCh != nil {
		_ = b.pubCh.Close()
	}
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

func retryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	switch v := headers[headerRetry].(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
