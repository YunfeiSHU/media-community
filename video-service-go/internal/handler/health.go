package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// HealthCheck 是一项依赖健康检查。
type HealthCheck struct {
	Name string
	Fn   func(ctx context.Context) (detail string, err error)
}

// HealthHandler 提供存活 / 就绪探针。
type HealthHandler struct {
	service string
	version string
	checks  []HealthCheck
	timeout time.Duration
}

// NewHealthHandler 构造健康检查 handler。
func NewHealthHandler(service, version string, timeout time.Duration, checks []HealthCheck) *HealthHandler {
	return &HealthHandler{service: service, version: version, checks: checks, timeout: timeout}
}

type checkResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Detail    string `json:"detail"`
}

// Healthz 存活探针：只表示进程还在，不检查下游依赖。
func (h *HealthHandler) Healthz(c *gin.Context) {
	response.Success(c, gin.H{
		"status":  "UP",
		"service": h.service,
		"version": h.version,
	})
}

// Readyz 就绪探针：逐项检查下游依赖，任一失败即 503。
func (h *HealthHandler) Readyz(c *gin.Context) {
	results := make([]checkResult, 0, len(h.checks))
	overall := "UP"

	for _, check := range h.checks {
		ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
		start := time.Now()
		detail, err := check.Fn(ctx)
		cancel()

		item := checkResult{
			Name:      check.Name,
			Status:    "UP",
			LatencyMs: time.Since(start).Milliseconds(),
			Detail:    detail,
		}
		if err != nil {
			item.Status = "DOWN"
			item.Detail = err.Error()
			overall = "DOWN"
		}
		results = append(results, item)
	}

	payload := gin.H{"status": overall, "checks": results}
	if overall == "DOWN" {
		c.JSON(http.StatusServiceUnavailable, response.Body{
			Code:    errs.CodeDependencyDown.Code,
			Message: errs.CodeDependencyDown.Message,
			Data:    payload,
		})
		return
	}
	response.Success(c, payload)
}
