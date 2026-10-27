package handler

import (
	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/service"
	"github.com/example/danmaku-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// AdminHandler 提供敏感词管理接口（必须 ADMIN）。
//
// 路由上已经挂了 middleware.AdminOnly()，这里仍然再校验一次：
// 管理类接口的鉴权是「纵深防御」的典型场景，多一次判断的成本可以忽略。
type AdminHandler struct {
	svc *service.DanmakuService
}

// NewAdminHandler 构造 handler。
func NewAdminHandler(svc *service.DanmakuService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// ReloadWords 处理 `POST /api/v1/danmaku/admin/reload-words`。
func (h *AdminHandler) ReloadWords(c *gin.Context) {
	if _, err := requireAdmin(c); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.ReloadWords(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// ListWords 处理 `GET /api/v1/danmaku/admin/words`。
func (h *AdminHandler) ListWords(c *gin.Context) {
	if _, err := requireAdmin(c); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.ListWords(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// CreateWord 处理 `POST /api/v1/danmaku/admin/words`（新增后自动热重载）。
func (h *AdminHandler) CreateWord(c *gin.Context) {
	if _, err := requireAdmin(c); err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.CreateWordRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.CreateWord(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}
