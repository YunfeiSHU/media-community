package handler

import (
	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/service"
	"github.com/example/danmaku-service-go/pkg/errs"
	"github.com/example/danmaku-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// DanmakuHandler 提供发送与撤回接口。
type DanmakuHandler struct {
	svc *service.DanmakuService
}

// NewDanmakuHandler 构造 handler。
func NewDanmakuHandler(svc *service.DanmakuService) *DanmakuHandler {
	return &DanmakuHandler{svc: svc}
}

// Send 处理 `POST /api/v1/danmaku/send`。
//
// 身份只来自 `X-User-Id`：请求体里即使带 uid 也不会被读取（dto.SendRequest 没有该字段）。
func (h *DanmakuHandler) Send(c *gin.Context) {
	user, err := requireUser(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.SendRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	h.dispatch(c, user.UID, req)
}

// SendToRoom 处理 `POST /api/v1/danmaku/rooms/:roomId/send`。
//
// 与 Send 共用同一 service 方法，只是把 roomId 从路径取（需求文档 FR-2.1 的写法），
// 两种路径并存是为了让契约与既有调用方都能对上，不会产生两套业务逻辑。
func (h *DanmakuHandler) SendToRoom(c *gin.Context) {
	user, err := requireUser(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	roomID, err := pathParam(c, "roomId")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.SendRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	if req.RoomID == "" {
		req.RoomID = roomID
	}
	if req.VideoID != "" {
		// 路径已经指明是直播房间，请求体再带 videoId 属于自相矛盾。
		response.Fail(c, errs.CodeInvalidParam.New("roomId 与 videoId 必须二选一"))
		return
	}

	h.dispatch(c, user.UID, req)
}

func (h *DanmakuHandler) dispatch(c *gin.Context, uid string, req dto.SendRequest) {
	result, err := h.svc.Send(c.Request.Context(), dto.SendInput{
		RoomID:      req.RoomID,
		VideoID:     req.VideoID,
		UID:         uid,
		Nickname:    req.Nickname,
		Content:     req.Content,
		Color:       req.Color,
		Position:    req.Position,
		TimeMs:      req.TimeMs,
		ClientMsgID: req.ClientMsgID,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Recall 处理 `DELETE /api/v1/danmaku/:danmakuId`。
func (h *DanmakuHandler) Recall(c *gin.Context) {
	user, err := requireUser(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	id, err := pathUint(c, "danmakuId")
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.Recall(c.Request.Context(), user, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}
