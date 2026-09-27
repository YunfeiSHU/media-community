// Package trace 在 context.Context 中传递链路追踪 ID。
//
// 独立成包是为了让它成为「叶子依赖」：
// middleware 写入、service 读取，两端都只依赖本包，避免 service 反向依赖 middleware。
package trace

import "context"

type contextKey struct{}

// With 把追踪 ID 写入 context。
func With(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

// From 从 context 读取追踪 ID，不存在时返回空串。
func From(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(contextKey{}).(string); ok {
		return id
	}
	return ""
}
