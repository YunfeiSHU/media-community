package handler

import (
	"net/http"

	"github.com/example/live-service-go/api/dto"
	"github.com/example/live-service-go/internal/service"
	"github.com/example/live-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// HeaderSessionID 是游客会话标识（服务端按 IP+UA 兜底之外的第二选择）。
const HeaderSessionID = "X-Session-Id"

// HookHandler 提供 SRS http_hooks 回调接口。
//
// 安全边界：这些接口在网关白名单里（SRS 无法携带 JWT），
// 因此不依赖身份头，校验完全落在 streamKey（32 位随机 hex，不可枚举）
// + 可选的 hook_secret 上。
type HookHandler struct {
	svc *service.LiveService
}

// NewHookHandler 构造 handler。
func NewHookHandler(svc *service.LiveService) *HookHandler {
	return &HookHandler{svc: svc}
}

// OnPublish POST /api/v1/live/hooks/on-publish
func (h *HookHandler) OnPublish(c *gin.Context) {
	req, ok := bindHook(c)
	if !ok {
		return
	}
	renderHook(c, h.svc.OnPublish(c.Request.Context(), req))
}

// OnUnpublish POST /api/v1/live/hooks/on-unpublish
func (h *HookHandler) OnUnpublish(c *gin.Context) {
	req, ok := bindHook(c)
	if !ok {
		return
	}
	renderHook(c, h.svc.OnUnpublish(c.Request.Context(), req))
}

// OnPlay POST /api/v1/live/hooks/on-play
func (h *HookHandler) OnPlay(c *gin.Context) {
	req, ok := bindHook(c)
	if !ok {
		return
	}
	renderHook(c, h.svc.OnPlay(c.Request.Context(), req))
}

// OnStop POST /api/v1/live/hooks/on-stop
func (h *HookHandler) OnStop(c *gin.Context) {
	req, ok := bindHook(c)
	if !ok {
		return
	}
	renderHook(c, h.svc.OnStop(c.Request.Context(), req))
}

// bindHook 解析 SRS 回调报文。
//
// 注意：SRS 的报文里有 client_id / stream_url 这类下划线字段，
// 用 ShouldBindJSON 而不是 BindJSON，字段不匹配也不会报错，
// 保证「回调解析失败」不会变成 SRS 侧的错误日志噪音。
func bindHook(c *gin.Context) (dto.HookRequest, bool) {
	var req dto.HookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		renderHook(c, &dto.HookResponse{Code: 0, Message: "ignored"})
		return req, false
	}
	return req, true
}

// renderHook 渲染回调响应。
//
// 与普通接口不同：SRS 约定**响应体的顶层 code** 决定是否放行该推流，
// code != 0 时 SRS 会拒绝连接 / 断开该流。
// 因此这里不能复用统一响应的「成功恒为 0」语义，而是把业务码直接放在顶层，
// 同时 HTTP 状态码保持 200（否则 SRS 会当成网络故障）。
func renderHook(c *gin.Context, resp *dto.HookResponse) {
	if resp == nil {
		resp = &dto.HookResponse{Code: 0, Message: "success"}
	}
	c.JSON(http.StatusOK, response.Body{Code: resp.Code, Message: resp.Message, Data: nil})
}
