package events

import "context"

// Handler 是事件处理器。返回 error 会触发重试（RabbitBus）或仅记录日志（LocalBus）。
type Handler func(ctx context.Context, ev Event) error

// Bus 是事件总线抽象。
type Bus interface {
	// Publish 发布事件到指定路由键。
	Publish(ctx context.Context, routingKey string, ev Event) error
	// Subscribe 订阅路由键。同一路由键可注册多个处理器。
	Subscribe(routingKey string, h Handler) error
	// Driver 返回实现名（rabbitmq | local）。
	Driver() string
	// Health 用于 /readyz 探针。
	Health(ctx context.Context) error
	// Close 释放资源。
	Close() error
}

// QueueNameFor 由「消费方 exchange + 路由键」推导队列名。
//
// **must 带 exchange 前缀**：多个服务会订阅同一个 routing key
// （danmaku 与 live 都要听 live.started/live.stopped），如果队列名只由
// routingKey 推导，两个服务就会撞同一个队列名；而各自的 DLX 又是按自己那个
// exchange 命名的，于是后启动的服务会拿到
// `406 PRECONDITION_FAILED - inequivalent arg 'x-dead-letter-exchange'`。
// 带上 exchange 前缀后，每个服务有自己的队列、自己的重试与死信，
// 同一个事件会分别投递到各服务的队列（互不影响）。
func QueueNameFor(exchange, routingKey string) string {
	return exchange + "." + routingKey + ".q"
}

// DLQNameFor 由「消费方 exchange + 路由键」推导死信队列名，与 QueueNameFor 配套。
func DLQNameFor(exchange, routingKey string) string {
	return exchange + "." + routingKey + ".dlq"
}

// DLXNameFor 由交换机名推导死信交换机名。
func DLXNameFor(exchange string) string { return exchange + ".dlx" }
