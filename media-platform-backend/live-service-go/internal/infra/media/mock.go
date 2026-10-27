package media

import (
	"context"

	"github.com/example/live-service-go/internal/config"
)

// MockServer 是无需 SRS 的占位实现。
//
// 它照样返回**真实可用的地址格式**（只是没人监听），这样前端/主播端可以照常联调；
// 状态推进则靠 `/api/v1/live/dev/publish/{streamKey}` 模拟回调。
type MockServer struct {
	urls
}

// NewMock 构造 mock 媒体服务。
func NewMock(cfg config.Media) *MockServer {
	return &MockServer{urls: urls{cfg: cfg}}
}

// Name 返回实现名。
func (m *MockServer) Name() string { return "mock" }

// KickOff 在 mock 下无事可做（返回 nil 让上层认为「已断开」）。
func (m *MockServer) KickOff(_ context.Context, _ string) error { return nil }

// Health mock 下恒健康：本地没起 SRS 时 /readyz 不应永远是红的。
func (m *MockServer) Health(_ context.Context) error { return nil }
