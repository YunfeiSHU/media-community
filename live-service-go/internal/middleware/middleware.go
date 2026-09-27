// Package middleware 提供横切关注点。
package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/example/live-service-go/pkg/errs"
	"github.com/example/live-service-go/pkg/identity"
	"github.com/example/live-service-go/pkg/response"
	"github.com/example/live-service-go/pkg/trace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 请求上下文键。
const (
	CtxRequestID = "requestId"
	CtxUID       = "uid"
)

// HeaderRequestID 是链路追踪头。
const HeaderRequestID = "X-Request-Id"

// RequestID 读取或生成请求 ID，注入 gin.Context / 请求 context 并回写响应头。
//
// 同时写入 request context（trace 包）是为了让 service 层在发布 MQ 事件时
// 能带上 traceId，实现跨服务的链路串联。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(HeaderRequestID)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set(CtxRequestID, rid)
		c.Header(HeaderRequestID, rid)
		c.Request = c.Request.WithContext(trace.With(c.Request.Context(), rid))
		c.Next()
	}
}

// GetRequestID 返回当前请求 ID。
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(CtxRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Logger 输出访问日志。
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latencyMs", time.Since(start).Milliseconds(),
			"clientIp", c.ClientIP(),
			"requestId", GetRequestID(c),
		}
		if uid := identity.From(c).UID; uid != "" {
			attrs = append(attrs, "uid", uid)
		}
		if errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String(); errMsg != "" {
			attrs = append(attrs, "errors", errMsg)
		}

		if c.Writer.Status() >= http.StatusInternalServerError {
			log.Error("http_request", attrs...)
			return
		}
		log.Info("http_request", attrs...)
	}
}

// Recovery 捕获 panic，返回 500/50000，并打印堆栈。
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		log.Error("panic_recovered",
			"error", recovered,
			"path", c.Request.URL.Path,
			"requestId", GetRequestID(c),
			"stack", string(debug.Stack()),
		)
		if !c.Writer.Written() {
			response.Failure(c, errs.CodeInternal.New("服务内部异常"))
		}
		c.Abort()
	})
}

// ErrorHandler 统一渲染 handler 通过 response.Fail 登记的错误。
func ErrorHandler(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}

		err := c.Errors.Last().Err
		if _, ok := errs.AsBizError(err); !ok {
			// 非业务错误：详细信息只进日志，响应体不透出内部细节
			log.Error("unhandled_error",
				"error", err,
				"path", c.Request.URL.Path,
				"requestId", GetRequestID(c),
			)
		}
		response.Failure(c, err)
	}
}

// CORS 允许跨域：HLS 播放与 WebSocket 握手都需要。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		h.Set("Access-Control-Allow-Headers",
			"Content-Type,Authorization,X-Request-Id,X-User-Id,X-User-Role,Range")
		h.Set("Access-Control-Expose-Headers", "X-Request-Id,Content-Length,Content-Range")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth 要求必须登录（缺 X-User-Id → 401/40101）。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := identity.Require(c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		c.Set(CtxUID, u.UID)
		c.Next()
	}
}

// AdminOnly 要求必须为管理员。
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := identity.RequireAdmin(c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		c.Set(CtxUID, u.UID)
		c.Next()
	}
}
