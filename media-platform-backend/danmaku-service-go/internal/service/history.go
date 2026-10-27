package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/pkg/errs"
)

// 查询来源。
const (
	sourceAuto = "auto"
	sourceHot  = "hot"
	sourceDB   = "db"
)

// RoomHistory 返回直播弹幕历史（按 sentAt 倒序分页）。
//
// 冷热分离策略（设计文档 §9.1）：
//
//	page=1 且 pageSize ≤ 热数据量 → 读 Redis List
//	否则                        → 读 MySQL（走 idx_room_sent）
//
// **不做「Redis + MySQL 合并」**：合并需要处理「哪些已在 Redis、哪些不在」的
// 去重与排序，逻辑复杂且容易出 bug，收益只是第一页更快。
// 简单规则已覆盖绝大多数真实请求（用户只看最新一屏）。
func (s *DanmakuService) RoomHistory(ctx context.Context, roomID string, query dto.RoomHistoryQuery) (*dto.RoomHistoryResult, error) {
	roomID = trim(roomID)
	if roomID == "" {
		return nil, errs.CodeInvalidParam.New("roomId 不能为空")
	}

	page, pageSize := normalizePaging(query.Page, query.PageSize, defaultPageSize, maxPageSize)
	source := normalizeSource(query.Source)

	// total 始终取 MySQL 计数：热数据只有最近 N 条，
	// 用它当总数会让前端分页器显示错误的总页数（接口文档 §4.1）。
	total, err := s.Danmaku.CountByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("统计房间弹幕数失败: %w", err)
	}

	if useHot(source, page, pageSize, s.Recent.HotSize()) {
		items, err := s.readHot(ctx, roomID, page, pageSize)
		if err != nil {
			s.logger().Warn("hot_history_read_failed",
				"roomId", roomID, "err", err.Error(), "hint", "回退到 MySQL")
		} else if len(items) > 0 || source == sourceHot {
			return &dto.RoomHistoryResult{
				Total:    total,
				Page:     page,
				PageSize: pageSize,
				Source:   sourceHot,
				Items:    items,
			}, nil
		}
		// auto 且热数据为空 → 继续走 MySQL（例如热数据已过 TTL）
	}

	items, err := s.Danmaku.ListByRoom(ctx, roomID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, fmt.Errorf("查询弹幕历史失败: %w", err)
	}
	return &dto.RoomHistoryResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Source:   sourceDB,
		Items:    toDTOList(items),
	}, nil
}

// readHot 从热数据读取并反序列化。
//
// 单条 JSON 损坏时跳过而不是整体失败：热数据是可重建的缓存，
// 一条脏数据不该让整个历史接口 500。
func (s *DanmakuService) readHot(ctx context.Context, roomID string, page, pageSize int) ([]dto.DanmakuDTO, error) {
	raw, err := s.Recent.ListRoom(ctx, roomID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]dto.DanmakuDTO, 0, len(raw))
	for _, line := range raw {
		var item dto.DanmakuDTO
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			s.logger().Warn("hot_item_decode_failed", "roomId", roomID, "err", err.Error())
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// normalizeSource 把 source 参数归一化，未知取值按 auto 处理。
func normalizeSource(raw string) string {
	switch strings.ToLower(trim(raw)) {
	case sourceHot:
		return sourceHot
	case sourceDB:
		return sourceDB
	default:
		return sourceAuto
	}
}

// useHot 判断本次是否走热数据。
func useHot(source string, page, pageSize int, hotSize int64) bool {
	switch source {
	case sourceHot:
		return true
	case sourceDB:
		return false
	default:
		// auto：只有第一页且请求量不超过热数据容量时才走 Redis。
		return page == 1 && int64(pageSize) <= hotSize
	}
}

// VideoHistory 返回点播弹幕历史（按 timeMs 升序，供播放器按时间轴投放）。
//
// 返回值中的 bool 表示是否因硬上限被截断（handler 会写 X-Result-Truncated 响应头）。
func (s *DanmakuService) VideoHistory(ctx context.Context, videoID string, query dto.VideoHistoryQuery) (*dto.VideoHistoryResult, bool, error) {
	videoID = trim(videoID)
	if videoID == "" {
		return nil, false, errs.CodeInvalidParam.New("videoId 不能为空")
	}
	if query.FromMs < 0 || query.ToMs < 0 {
		return nil, false, errs.CodeInvalidParam.New("fromMs / toMs 不能为负数")
	}
	if query.ToMs > 0 && query.ToMs < query.FromMs {
		return nil, false, errs.CodeInvalidParam.New("toMs 不能小于 fromMs")
	}

	limit := query.Limit
	if limit <= 0 {
		limit = defaultVideoLimit
	}
	if limit > maxVideoLimit {
		limit = maxVideoLimit
	}

	// 多取一条用于判断「是否被截断」：这样既不用 COUNT，
	// 也不会因为正好等于 limit 而误报截断。
	items, err := s.Danmaku.ListByVideoRange(ctx, videoID, query.FromMs, query.ToMs, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("查询点播弹幕失败: %w", err)
	}

	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}

	result := &dto.VideoHistoryResult{
		VideoID: videoID,
		FromMs:  query.FromMs,
		ToMs:    query.ToMs,
		Count:   len(items),
		Items:   toDTOList(items),
	}
	return result, truncated, nil
}

// Stats 返回房间弹幕统计。
func (s *DanmakuService) Stats(ctx context.Context, roomID string) (*dto.StatsResult, error) {
	roomID = trim(roomID)
	if roomID == "" {
		return nil, errs.CodeInvalidParam.New("roomId 不能为空")
	}

	total, err := s.Danmaku.CountByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("统计房间弹幕数失败: %w", err)
	}

	hotCount, err := s.Recent.RoomLen(ctx, roomID)
	if err != nil {
		s.logger().Warn("hot_count_failed", "roomId", roomID, "err", err.Error())
	}

	lastSentAt, err := s.Danmaku.LastSentAt(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("查询最后弹幕时间失败: %w", err)
	}

	return &dto.StatsResult{
		RoomID:     roomID,
		Total:      total,
		HotCount:   hotCount,
		LastSentAt: lastSentAt,
	}, nil
}
