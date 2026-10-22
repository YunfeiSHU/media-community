// Package logx 提供结构化日志。
//
// 开发模式输出文本（可读），其它模式输出 JSON（可被采集）。
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// New 按运行模式创建 logger。
func New(mode, service, version string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	var handler slog.Handler
	switch strings.ToLower(mode) {
	case "local", "dev", "debug":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler).With("service", service, "version", version)
}
