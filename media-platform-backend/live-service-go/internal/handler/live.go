package handler

import (
	"strings"

	"github.com/example/live-service-go/api/dto"
	"github.com/example/live-service-go/internal/service"
	"github.com/example/live-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// LiveHandler 提供开播 / 关播 / 播放 / 在线人数接口。
//
// 它同时持有 LiveService（生命周期）与 StatsService（在线人数统计）：
// 两者在 HTTP 层面同属「直播间实时能力」，拆成两个 handler 只会让路由注册变碎。
type LiveHandler struct {
	live  *service.LiveService
	stats *service.StatsService
}

// NewLiveHandler 构造 handler。
func NewLiveHandler(live *service.LiveService, stats *service.StatsService) *LiveHandler {
	return &LiveHandler{live: live, stats: stats}
}

// Start POST /api/v1/live/rooms/:id/start
func (h *LiveHandler) Start(c *gin.Context) {
	user, err := requireUser(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.live.Start(c.Request.Context(), user, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Stop POST /api/v1/live/rooms/:id/stop
func (h *LiveHandler) Stop(c *gin.Context) {
	user, err := requireUser(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.live.Stop(c.Request.Context(), user, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// ForceClose POST /api/v1/live/rooms/:id/force-close
func (h *LiveHandler) ForceClose(c *gin.Context) {
	user, err := requireUser(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.ForceCloseRequest
	if err := bindJSONOptional(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.live.ForceClose(c.Request.Context(), user, id, strings.TrimSpace(req.Reason))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Play GET /api/v1/live/rooms/:id/play
func (h *LiveHandler) Play(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.live.Play(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Heartbeat POST /api/v1/live/rooms/:id/heartbeat
//
// 游客身份可以来自请求头 X-Session-Id，也可以来自请求体 sessionId。
func (h *LiveHandler) Heartbeat(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.HeartbeatRequest
	if err := bindJSONOptional(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	guest := req.SessionID
	if guest == "" {
		guest = c.GetHeader(HeaderSessionID)
	}

	result, err := h.stats.Heartbeat(c.Request.Context(), id, currentUser(c).UID, guest)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Online GET /api/v1/live/rooms/:id/online
func (h *LiveHandler) Online(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.stats.Online(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// DevPublish POST /api/v1/live/dev/publish/:streamKey（仅 debug 模式注册）
func (h *LiveHandler) DevPublish(c *gin.Context) {
	streamKey, err := pathStreamKey(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sessionID := c.Query("session")

	resp := h.live.DevPublish(c.Request.Context(), streamKey, sessionID)
	renderHook(c, resp)
}

// DevUnpublish POST /api/v1/live/dev/unpublish/:streamKey（仅 debug 模式注册）
func (h *LiveHandler) DevUnpublish(c *gin.Context) {
	streamKey, err := pathStreamKey(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	resp := h.live.DevUnpublish(c.Request.Context(), streamKey)
	renderHook(c, resp)
}
