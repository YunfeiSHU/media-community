package handler

import (
	"github.com/example/danmaku-service-go/internal/service"
	"github.com/example/danmaku-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// HeaderTruncated 标注「结果因硬上限被截断」。
//
// 为什么不返回 200 + 部分数据了事：静默截断会让调用方以为「这就是全部」。
const HeaderTruncated = "X-Result-Truncated"

// HistoryHandler 提供历史查询与统计接口（均无需登录）。
type HistoryHandler struct {
	svc *service.DanmakuService
}

// NewHistoryHandler 构造 handler。
func NewHistoryHandler(svc *service.DanmakuService) *HistoryHandler {
	return &HistoryHandler{svc: svc}
}

// RoomHistory 处理 `GET /api/v1/danmaku/rooms/:roomId/history`。
func (h *HistoryHandler) RoomHistory(c *gin.Context) {
	roomID, err := pathParam(c, "roomId")
	if err != nil {
		response.Fail(c, err)
		return
	}

	query, err := parseRoomHistoryQuery(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.RoomHistory(c.Request.Context(), roomID, query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// VideoHistory 处理 `GET /api/v1/danmaku/videos/:videoId/history`。
func (h *HistoryHandler) VideoHistory(c *gin.Context) {
	videoID, err := pathParam(c, "videoId")
	if err != nil {
		response.Fail(c, err)
		return
	}

	query, err := parseVideoHistoryQuery(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, truncated, err := h.svc.VideoHistory(c.Request.Context(), videoID, query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if truncated {
		// 必须在写响应体之前设置。
		c.Header(HeaderTruncated, "true")
	}
	response.Success(c, result)
}

// Stats 处理 `GET /api/v1/danmaku/stats?roomId=`。
func (h *HistoryHandler) Stats(c *gin.Context) {
	roomID := c.Query("roomId")
	result, err := h.svc.Stats(c.Request.Context(), roomID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}
