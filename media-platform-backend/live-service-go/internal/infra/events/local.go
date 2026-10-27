package events

import (
	"context"
	"log/slog"
	"sync"
)

// LocalBus 是进程内事件总线，仅在开发环境或 RabbitMQ 不可用时使用。
//
// 局限（务必知晓）：
//   - 不持久化：进程重启即丢消息
//   - 不跨进程：多实例部署时各自收到不到对方的"发布"
//
// 因此启动时会打印 warning，生产环境请使用 RabbitBus。
type LocalBus struct {
	mu   sync.RWMutex
	subs map[string][]Handler
	wg   sync.WaitGroup
	log  *slog.Logger
}

// NewLocalBus 构造进程内事件总线。
func NewLocalBus(log *slog.Logger) *LocalBus {
	return &LocalBus{subs: make(map[string][]Handler), log: log}
}

// Publish 异步分发给本进程内所有订阅者。
func (b *LocalBus) Publish(ctx context.Context, routingKey string, ev Event) error {
	b.mu.RLock()
	handlers := make([]Handler, len(b.subs[routingKey]))
	copy(handlers, b.subs[routingKey])
	b.mu.RUnlock()

	// 请求结束时 ctx 会被取消，异步处理必须脱离原 ctx 的生命周期
	base := context.WithoutCancel(ctx)

	for _, h := range handlers {
		b.wg.Add(1)
		go func(handler Handler) {
			defer b.wg.Done()
			if err := handler(base, ev); err != nil {
				b.log.Error("local_bus_handler_failed",
					"routingKey", routingKey,
					"eventId", ev.EventID,
					"error", err,
				)
			}
		}(h)
	}
	return nil
}

// Subscribe 注册处理器。
func (b *LocalBus) Subscribe(routingKey string, h Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[routingKey] = append(b.subs[routingKey], h)
	return nil
}

// Driver 返回实现名。
func (b *LocalBus) Driver() string { return "local" }

// Health 进程内总线总是"健康"。
func (b *LocalBus) Health(context.Context) error { return nil }

// Close 等待在途事件处理完成。
func (b *LocalBus) Close() error {
	b.wg.Wait()
	return nil
}
