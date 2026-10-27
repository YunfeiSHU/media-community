// Package dto 定义 HTTP / WebSocket 与事件总线上的数据结构。
//
// 约定：字段名与《接口文档》完全一致（camelCase），
// 不直接复用 model，避免「数据库字段改名导致对外契约破坏」。
package dto

import "time"

// ---------------------------------------------------------------- 请求

// SendRequest 是发送弹幕的请求体（HTTP）。
//
// **没有 UID 字段**：身份只来自 `X-User-Id`。
// 客户端即使传了 uid，反序列化时也会被忽略（gin 默认不拒绝未知字段）。
type SendRequest struct {
	RoomID      string `json:"roomId"`
	VideoID     string `json:"videoId"`
	Content     string `json:"content"`
	Color       string `json:"color"`
	Position    string `json:"position"`
	TimeMs      int    `json:"timeMs"`
	ClientMsgID string `json:"clientMsgId"`
	Nickname    string `json:"nickname"`
}

// SendInput 是 service 层的发送入参（HTTP 与 WebSocket 共用）。
type SendInput struct {
	RoomID      string
	VideoID     string
	UID         string
	Nickname    string
	Content     string
	Color       string
	Position    string
	TimeMs      int
	ClientMsgID string
}

// RoomHistoryQuery 是直播历史查询参数。
type RoomHistoryQuery struct {
	Page     int
	PageSize int
	// Source: auto（第 1 页走热数据）/ hot（强制热数据）/ db（强制 MySQL）
	Source string
}

// VideoHistoryQuery 是点播历史查询参数。
type VideoHistoryQuery struct {
	FromMs int
	ToMs   int
	Limit  int
}

// CreateWordRequest 是新增敏感词的请求体。
type CreateWordRequest struct {
	Word     string `json:"word"`
	Strategy string `json:"strategy"`
	Enabled  *bool  `json:"enabled"`
}

// ---------------------------------------------------------------- 响应

// DanmakuDTO 是弹幕的对外表示。
//
// DanmakuID 为 0 表示**尚未落库**：ID 由 MySQL 自增生成，落库是异步的。
// 因此实时链路里**不能用 danmakuId 作为标识**，请用 clientMsgId + sentAt。
type DanmakuDTO struct {
	DanmakuID   uint64    `json:"danmakuId"`
	ClientMsgID string    `json:"clientMsgId"`
	RoomID      string    `json:"roomId"`
	VideoID     string    `json:"videoId"`
	UID         string    `json:"uid"`
	Nickname    string    `json:"nickname"`
	Content     string    `json:"content"`
	Color       string    `json:"color"`
	Position    string    `json:"position"`
	TimeMs      int       `json:"timeMs"`
	SentAt      time.Time `json:"sentAt"`
	Status      string    `json:"status"`
}

// SendResult 是发送接口的响应体。
//
// 嵌入 DanmakuDTO 让 JSON 扁平化（契约见《接口文档》§3.1），
// 同时它也是幂等去重时缓存下来的「首发快照」。
type SendResult struct {
	DanmakuDTO
	// Persisted 为 false 表示 MQ 投递失败、已进入 Redis 补发队列。
	Persisted bool `json:"persisted"`
	// Duplicated 为 true 表示本次是 clientMsgId 命中的重复请求。
	Duplicated bool `json:"duplicated,omitempty"`
}

// RecallResult 是撤回接口的响应体。
type RecallResult struct {
	DanmakuID  uint64    `json:"danmakuId"`
	Status     string    `json:"status"`
	RecalledAt time.Time `json:"recalledAt"`
}

// RoomHistoryResult 是直播历史响应体。
type RoomHistoryResult struct {
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Source   string       `json:"source"`
	Items    []DanmakuDTO `json:"items"`
}

// VideoHistoryResult 是点播历史响应体。
type VideoHistoryResult struct {
	VideoID string       `json:"videoId"`
	FromMs  int          `json:"fromMs"`
	ToMs    int          `json:"toMs"`
	Count   int          `json:"count"`
	Items   []DanmakuDTO `json:"items"`
}

// StatsResult 是房间弹幕统计响应体。
type StatsResult struct {
	RoomID     string     `json:"roomId"`
	Total      int64      `json:"total"`
	HotCount   int64      `json:"hotCount"`
	LastSentAt *time.Time `json:"lastSentAt"`
}

// ReloadWordsResult 是热重载响应体。
type ReloadWordsResult struct {
	Words      int       `json:"words"`
	ReloadedAt time.Time `json:"reloadedAt"`
}

// WordDTO 是敏感词的对外表示。
type WordDTO struct {
	ID       uint64 `json:"id"`
	Word     string `json:"word"`
	Strategy string `json:"strategy"`
	Enabled  bool   `json:"enabled"`
}

// WordListResult 是敏感词列表响应体。
type WordListResult struct {
	Items []WordDTO `json:"items"`
}

// ---------------------------------------------------------------- 事件载荷

// DanmakuCreatedPayload 是 `danmaku.created` 事件的载荷。
//
// 注意**没有 danmakuId**：ID 由 MySQL 自增，投递事件时还不存在。
// 消费侧靠 eventId 幂等、靠 (uid, clientMsgId) 做短时去重。
type DanmakuCreatedPayload struct {
	RoomID      string    `json:"roomId"`
	VideoID     string    `json:"videoId"`
	UID         string    `json:"uid"`
	Nickname    string    `json:"nickname"`
	Content     string    `json:"content"`
	Color       string    `json:"color"`
	Position    string    `json:"position"`
	TimeMs      int       `json:"timeMs"`
	ClientMsgID string    `json:"clientMsgId"`
	SentAt      time.Time `json:"sentAt"`
}

// LiveStartedPayload 是 `live.started` 事件的载荷（由 live-service 生产）。
type LiveStartedPayload struct {
	RoomID    string    `json:"roomId"`
	OwnerID   string    `json:"ownerId"`
	SessionID string    `json:"sessionId"`
	Title     string    `json:"title"`
	CoverURL  string    `json:"coverUrl"`
	StartedAt time.Time `json:"startedAt"`
}

// LiveStoppedPayload 是 `live.stopped` 事件的载荷（由 live-service 生产）。
type LiveStoppedPayload struct {
	RoomID      string    `json:"roomId"`
	SessionID   string    `json:"sessionId"`
	EndedAt     time.Time `json:"endedAt"`
	DurationSec int       `json:"durationSec"`
	PeakOnline  int       `json:"peakOnline"`
	ViewerTotal int       `json:"viewerTotal"`
	Reason      string    `json:"reason"`
}
