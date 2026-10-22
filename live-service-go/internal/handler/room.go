package handler

import (
	"github.com/example/live-service-go/api/dto"
	"github.com/example/live-service-go/internal/service"
	"github.com/example/live-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// RoomHandler 提供直播间管理接口。
type RoomHandler struct {
	svc *service.RoomService
}

// NewRoomHandler 构造 handler。
func NewRoomHandler(svc *service.RoomService) *RoomHandler {
	return &RoomHandler{svc: svc}
}

// Create POST /api/v1/live/rooms
func (h *RoomHandler) Create(c *gin.Context) {
	user, err := requireUser(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.CreateRoomRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.Create(c.Request.Context(), user, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Update PUT /api/v1/live/rooms/:id
func (h *RoomHandler) Update(c *gin.Context) {
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

	var req dto.UpdateRoomRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.Update(c.Request.Context(), user, id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Delete DELETE /api/v1/live/rooms/:id
func (h *RoomHandler) Delete(c *gin.Context) {
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

	result, err := h.svc.Delete(c.Request.Context(), user, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// List GET /api/v1/live/rooms
func (h *RoomHandler) List(c *gin.Context) {
	query, err := parseRoomListQuery(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	items, total, page, pageSize, err := h.svc.List(c.Request.Context(), currentUser(c), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, response.NewPage(items, total, page, pageSize))
}

// Get GET /api/v1/live/rooms/:id
func (h *RoomHandler) Get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	detail, err := h.svc.Get(c.Request.Context(), currentUser(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, detail)
}

// StreamKey GET /api/v1/live/rooms/:id/stream-key
func (h *RoomHandler) StreamKey(c *gin.Context) {
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

	result, err := h.svc.StreamKey(c.Request.Context(), user, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Records GET /api/v1/live/rooms/:id/records
func (h *RoomHandler) Records(c *gin.Context) {
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
	page, pageSize, err := parsePaging(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	items, total, page, pageSize, err := h.svc.Records(c.Request.Context(), user, id, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, response.NewPage(items, total, page, pageSize))
}
