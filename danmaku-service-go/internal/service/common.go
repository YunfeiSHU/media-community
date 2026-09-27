// Package service 承载业务规则：校验、过滤、限流、广播、异步落库。
//
// 分层原则：handler 只做「取参数 + 调 service + 渲染」，
// repository 只做 SQL，跨存储的编排全部落在这一层。
package service

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/config"
	"github.com/example/danmaku-service-go/internal/infra/cache"
	"github.com/example/danmaku-service-go/internal/infra/events"
	"github.com/example/danmaku-service-go/internal/infra/filter"
	"github.com/example/danmaku-service-go/internal/infra/hub"
	"github.com/example/danmaku-service-go/internal/infra/ratelimit"
	"github.com/example/danmaku-service-go/internal/model"
	"github.com/example/danmaku-service-go/internal/repository"
	"github.com/example/danmaku-service-go/pkg/errs"
)

// 分页与查询上限。
const (
	defaultPageSize   = 50
	maxPageSize       = 200
	defaultVideoLimit = 2000
	maxVideoLimit     = 5000
	maxNicknameLen    = 64
)

// colorPattern 校验 #RRGGBB 或 #AARRGGBB。
//
// 严格校验不只是「数据洁癖」：颜色值最终会进入前端样式，
// 宽松放行等于把样式注入的口子留给别人（非功能性需求里的安全项）。
var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$`)

// Deps 汇集服务层依赖，避免构造函数参数爆炸。
type Deps struct {
	Config  *config.Config
	Danmaku repository.DanmakuRepository
	Words   repository.WordRepository
	Recent  *cache.RecentCache
	Limiter *ratelimit.Limiter
	Filter  *filter.Filter
	Hub     *hub.Hub
	Bus     events.Bus
	Log     *slog.Logger
}

func (d Deps) logger() *slog.Logger {
	if d.Log == nil {
		return slog.Default()
	}
	return d.Log
}

// ---------------------------------------------------------------- 校验

// normalizeColor 归一化颜色：缺省为白色，格式错误直接拒绝。
func normalizeColor(raw string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return model.DefaultColor, nil
	}
	if !colorPattern.MatchString(value) {
		return "", errs.CodeInvalidParam.New("color 必须是 #RRGGBB 格式")
	}
	return value, nil
}

// normalizePosition 归一化显示位置。
func normalizePosition(raw string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return model.PositionScroll, nil
	}
	switch value {
	case model.PositionScroll, model.PositionTop, model.PositionBottom:
		return value, nil
	default:
		return "", errs.CodeInvalidParam.New("position 仅支持 SCROLL / TOP / BOTTOM")
	}
}

// displayName 计算展示昵称：缺省时回退到 uid。
func displayName(nickname, uid string) string {
	name := strings.TrimSpace(nickname)
	if name == "" {
		name = uid
	}
	runes := []rune(name)
	if len(runes) > maxNicknameLen {
		name = string(runes[:maxNicknameLen])
	}
	return name
}

// normalizePaging 收紧分页参数，防止 pageSize=1000000 把库拖垮。
func normalizePaging(page, pageSize int, def, max int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = def
	}
	if pageSize > max {
		pageSize = max
	}
	return page, pageSize
}

// resourceKey 是限流用的资源标识（按房间 / 视频隔离计数）。
func resourceKey(roomID, videoID string) string {
	switch {
	case roomID != "":
		return roomID
	case videoID != "":
		return "v:" + videoID
	default:
		return ""
	}
}

// ---------------------------------------------------------------- 转换

// toDTO 把持久化模型转成对外表示。
func toDTO(item model.Danmaku) dto.DanmakuDTO {
	return dto.DanmakuDTO{
		DanmakuID:   item.ID,
		ClientMsgID: item.ClientMsgID,
		RoomID:      item.RoomID,
		VideoID:     item.VideoID,
		UID:         item.UID,
		Nickname:    item.Nickname,
		Content:     item.Content,
		Color:       item.Color,
		Position:    item.Position,
		TimeMs:      item.TimeMs,
		SentAt:      item.SentAt,
		Status:      item.Status,
	}
}

// toDTOList 批量转换。
func toDTOList(items []model.Danmaku) []dto.DanmakuDTO {
	out := make([]dto.DanmakuDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toDTO(item))
	}
	return out
}

// marshal 序列化辅助；失败时返回空串（调用方按「热数据写不进去」处理）。
func marshal(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

// trim 是 strings.TrimSpace 的短别名，让密集的校验代码保持可读。
func trim(s string) string { return strings.TrimSpace(s) }
