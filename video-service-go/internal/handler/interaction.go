package handler

import (
	"strconv"
	"strings"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/service"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/identity"
	"github.com/example/video-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// InteractionHandler 提供点赞 / 收藏接口。
type InteractionHandler struct {
	svc *service.InteractionService
}

// NewInteractionHandler 构造互动 handler。
func NewInteractionHandler(svc *service.InteractionService) *InteractionHandler {
	return &InteractionHandler{svc: svc}
}

// Like 处理 POST /videos/:id/like（幂等）。
func (h *InteractionHandler) Like(c *gin.Context) { h.toggle(c, true, true) }

// Unlike 处理 DELETE /videos/:id/like。
func (h *InteractionHandler) Unlike(c *gin.Context) { h.toggle(c, true, false) }

// Favorite 处理 POST /videos/:id/favorite（幂等）。
func (h *InteractionHandler) Favorite(c *gin.Context) { h.toggle(c, false, true) }

// Unfavorite 处理 DELETE /videos/:id/favorite。
func (h *InteractionHandler) Unfavorite(c *gin.Context) { h.toggle(c, false, false) }

func (h *InteractionHandler) toggle(c *gin.Context, like, on bool) {
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

	var state *dto.InteractionState
	switch {
	case like && on:
		state, err = h.svc.Like(c.Request.Context(), u, id)
	case like && !on:
		state, err = h.svc.Unlike(c.Request.Context(), u, id)
	case !like && on:
		state, err = h.svc.Favorite(c.Request.Context(), u, id)
	default:
		state, err = h.svc.Unfavorite(c.Request.Context(), u, id)
	}
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, state)
}

// Get 处理 GET /videos/:id/interactions（游客可用，返回 liked/favorited=false）。
func (h *InteractionHandler) Get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	state, err := h.svc.Get(c.Request.Context(), identity.From(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, state)
}

// Batch 处理 GET /videos/interactions?ids=1,2,3。
func (h *InteractionHandler) Batch(c *gin.Context) {
	ids, err := parseIDs(c.Query("ids"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	if len(ids) == 0 {
		response.Success(c, &dto.InteractionList{Items: []dto.InteractionState{}})
		return
	}
	result, err := h.svc.Batch(c.Request.Context(), identity.From(c), ids)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// parseIDs 解析逗号分隔的 id 列表，去重并保持顺序。
func parseIDs(raw string) ([]uint64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	seen := make(map[uint64]struct{})
	ids := make([]uint64, 0, 8)
	for _, part := range strings.Split(raw, ",") {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		id, err := strconv.ParseUint(text, 10, 64)
		if err != nil || id == 0 {
			return nil, errs.CodeInvalidParam.New("ids 必须是逗号分隔的正整数")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}
