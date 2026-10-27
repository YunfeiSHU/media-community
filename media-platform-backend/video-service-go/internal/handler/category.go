package handler

import (
	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/service"
	"github.com/example/video-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// CategoryHandler 处理分类相关 HTTP 请求。
type CategoryHandler struct {
	svc *service.CategoryService
}

// NewCategoryHandler 构造分类 handler。
func NewCategoryHandler(svc *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

// List 查询分类列表（公开）。
func (h *CategoryHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto.CategoryList{Items: items})
}

// Create 新建分类（仅 ADMIN，鉴权由路由上的 AdminOnly 中间件完成）。
func (h *CategoryHandler) Create(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	category, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, category)
}
