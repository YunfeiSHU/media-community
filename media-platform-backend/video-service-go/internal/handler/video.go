package handler

import (
	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/service"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/identity"
	"github.com/example/video-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// VideoHandler 处理视频相关 HTTP 请求。
type VideoHandler struct {
	svc *service.VideoService
}

// NewVideoHandler 构造视频 handler。
func NewVideoHandler(svc *service.VideoService) *VideoHandler {
	return &VideoHandler{svc: svc}
}

// Create 创建视频草稿。
func (h *VideoHandler) Create(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.CreateVideoRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.CreateVideo(c.Request.Context(), u, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Update 修改视频元数据。
func (h *VideoHandler) Update(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.UpdateVideoRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.UpdateVideo(c.Request.Context(), u, id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Delete 软删除视频。
func (h *VideoHandler) Delete(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.DeleteVideo(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// List 分页查询视频列表。
func (h *VideoHandler) List(c *gin.Context) {
	q, err := parseListQuery(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	items, total, err := h.svc.ListVideos(c.Request.Context(), identity.From(c), q)
	if err != nil {
		response.Fail(c, err)
		return
	}

	page, pageSize := service.NormalizePaging(q.Page, q.PageSize)
	response.Success(c, response.NewPage(items, total, page, pageSize))
}

// Get 查询视频详情。
func (h *VideoHandler) Get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	detail, err := h.svc.GetVideo(c.Request.Context(), identity.From(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, detail)
}

// Upload 上传原片。
func (h *VideoHandler) Upload(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.CodeInvalidParam.New("缺少上传文件字段 file"))
		return
	}

	result, err := h.svc.Upload(c.Request.Context(), u, id, fileHeader)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Publish 发布视频。
func (h *VideoHandler) Publish(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.Publish(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Offline 下架视频。
func (h *VideoHandler) Offline(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.Offline(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Play 获取播放信息。
func (h *VideoHandler) Play(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	info, err := h.svc.Play(c.Request.Context(), identity.From(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, info)
}

// TranscodeStatus 查询转码任务状态。
func (h *VideoHandler) TranscodeStatus(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	info, err := h.svc.TranscodeStatus(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, info)
}

// RetryTranscode 重试转码。
func (h *VideoHandler) RetryTranscode(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	info, err := h.svc.RetryTranscode(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, info)
}
