// Package dto 定义 HTTP 出入参与跨服务事件载荷。
//
// 统一约定：请求体用驼峰，响应体用驼峰；id 一律数字（前端安全范围内）或字符串。
package dto

import "time"

// ---------------------------------------------------------------- 请求

// CreateRoomRequest 创建直播间。
type CreateRoomRequest struct {
	Title      string  `json:"title"`
	Notice     string  `json:"notice"`
	CoverURL   string  `json:"coverUrl"`
	CategoryID *uint64 `json:"categoryId"`
}

// UpdateRoomRequest 修改直播间；指针字段为 nil 表示不修改。
type UpdateRoomRequest struct {
	Title      *string `json:"title"`
	Notice     *string `json:"notice"`
	CoverURL   *string `json:"coverUrl"`
	CategoryID *uint64 `json:"categoryId"`
}

// ListQuery 直播间列表查询条件。
type ListQuery struct {
	Keyword    string
	CategoryID *uint64
	OwnerID    string
	Status     string
	LivingOnly bool
	Mine       bool
	Page       int
	PageSize   int
}

// HeartbeatRequest 观众心跳。
type HeartbeatRequest struct {
	// SessionID 游客标识；已登录用户可省略（用 uid 作为 memberKey）。
	SessionID string `json:"sessionId"`
}

// ForceCloseRequest 强制关闭直播间。
type ForceCloseRequest struct {
	Reason string `json:"reason"`
}

// ---------------------------------------------------------------- 响应

// PlayURLs 各协议播放地址。
type PlayURLs struct {
	HLS     string `json:"hls"`
	HTTPFLV string `json:"httpFlv"`
	WebRTC  string `json:"webrtc"`
	RTMP    string `json:"rtmp"`
}

// CreateRoomResult 创建直播间结果。
type CreateRoomResult struct {
	RoomID    uint64    `json:"roomId"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	StreamKey string    `json:"streamKey"`
	PushURL   string    `json:"pushUrl"`
	PlayURLs  *PlayURLs `json:"playUrls"`
	CreatedAt time.Time `json:"createdAt"`
}

// UpdateRoomResult 修改直播间结果。
type UpdateRoomResult struct {
	RoomID  uint64 `json:"roomId"`
	Updated bool   `json:"updated"`
}

// DeleteRoomResult 删除直播间结果。
type DeleteRoomResult struct {
	RoomID  uint64 `json:"roomId"`
	Deleted bool   `json:"deleted"`
}

// RoomBrief 是列表项。
type RoomBrief struct {
	RoomID      uint64     `json:"roomId"`
	Title       string     `json:"title"`
	Notice      string     `json:"notice"`
	CoverURL    string     `json:"coverUrl"`
	OwnerID     string     `json:"ownerId"`
	CategoryID  *uint64    `json:"categoryId"`
	Status      string     `json:"status"`
	OnlineCount int64      `json:"onlineCount"`
	ViewerTotal int64      `json:"viewerTotal"`
	PeakOnline  int        `json:"peakOnline"`
	StartedAt   *time.Time `json:"startedAt"`
	PlayURLs    *PlayURLs  `json:"playUrls"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// RoomDetail 是详情。
//
// StreamKey / PushURL 仅创建者与 ADMIN 可见（其余情况因 omitempty 直接不出现）；
// PlayURLs 仅在 LIVING 时返回，避免前端误播空流。
type RoomDetail struct {
	RoomID       uint64     `json:"roomId"`
	Title        string     `json:"title"`
	Notice       string     `json:"notice"`
	CoverURL     string     `json:"coverUrl"`
	OwnerID      string     `json:"ownerId"`
	CategoryID   *uint64    `json:"categoryId"`
	Status       string     `json:"status"`
	SessionID    string     `json:"sessionId"`
	OnlineCount  int64      `json:"onlineCount"`
	ViewerTotal  int64      `json:"viewerTotal"`
	PeakOnline   int        `json:"peakOnline"`
	StartedAt    *time.Time `json:"startedAt"`
	EndedAt      *time.Time `json:"endedAt"`
	StreamKey    string     `json:"streamKey,omitempty"`
	PushURL      string     `json:"pushUrl,omitempty"`
	PlayURLs     *PlayURLs  `json:"playUrls"`
	SRSAvailable bool       `json:"srsAvailable"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// StartResult 开播结果。
type StartResult struct {
	RoomID       uint64    `json:"roomId"`
	Status       string    `json:"status"`
	SessionID    string    `json:"sessionId"`
	StreamKey    string    `json:"streamKey"`
	PushURL      string    `json:"pushUrl"`
	PlayURLs     *PlayURLs `json:"playUrls"`
	SRSAvailable bool      `json:"srsAvailable"`
	StartedAt    time.Time `json:"startedAt"`
	Hint         string    `json:"hint"`
}

// StopResult 关播结果。
//
// EndedAt 用指针：幂等返回（房间本来就不在直播中）时没有真实结束时刻，
// 输出 null 比输出零值时间更诚实。
type StopResult struct {
	RoomID       uint64     `json:"roomId"`
	Status       string     `json:"status"`
	SessionID    string     `json:"sessionId"`
	EndedAt      *time.Time `json:"endedAt"`
	DurationSec  int        `json:"durationSec"`
	PeakOnline   int        `json:"peakOnline"`
	ViewerTotal  int64      `json:"viewerTotal"`
	EndReason    string     `json:"endReason"`
	StreamKicked bool       `json:"streamKicked"`
}

// ForceCloseResult 强制关闭结果。
type ForceCloseResult struct {
	RoomID    uint64 `json:"roomId"`
	Status    string `json:"status"`
	EndReason string `json:"endReason"`
	Reason    string `json:"reason"`
}

// StreamKeyResult 推流密钥查询结果。
type StreamKeyResult struct {
	RoomID       uint64 `json:"roomId"`
	StreamKey    string `json:"streamKey"`
	PushURL      string `json:"pushUrl"`
	Status       string `json:"status"`
	SRSAvailable bool   `json:"srsAvailable"`
}

// PlayResult 播放地址结果。
type PlayResult struct {
	RoomID      uint64    `json:"roomId"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	OnlineCount int64     `json:"onlineCount"`
	Format      []string  `json:"format"`
	PlayURLs    *PlayURLs `json:"playUrls"`
}

// HeartbeatResult 心跳结果。
type HeartbeatResult struct {
	RoomID      uint64 `json:"roomId"`
	OnlineCount int64  `json:"onlineCount"`
	MemberKey   string `json:"memberKey"`
	FirstEnter  bool   `json:"firstEnter"`
	TTLSec      int    `json:"ttlSec"`
}

// OnlineResult 在线人数（轻量轮询，只查 Redis）。
type OnlineResult struct {
	RoomID      uint64 `json:"roomId"`
	OnlineCount int64  `json:"onlineCount"`
	PeakOnline  int    `json:"peakOnline"`
	ViewerTotal int64  `json:"viewerTotal"`
}

// RecordDTO 是直播历史项。
type RecordDTO struct {
	RecordID    uint64     `json:"recordId"`
	SessionID   string     `json:"sessionId"`
	StartedAt   time.Time  `json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt"`
	DurationSec int        `json:"durationSec"`
	PeakOnline  int        `json:"peakOnline"`
	ViewerTotal int64      `json:"viewerTotal"`
	EndReason   string     `json:"endReason"`
}

// HookRequest 是 SRS http_hooks 回调报文（只取用得到的字段）。
type HookRequest struct {
	Action   string `json:"action"`
	ClientID string `json:"client_id"`
	IP       string `json:"ip"`
	Vhost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
	Param    string `json:"param"`
}

// HookResponse 是回调响应，SRS 约定 code != 0 表示拒绝/断开该流。
type HookResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------- 事件载荷

// LiveStartedPayload 是 live.started 的载荷。
//
// **跨服务契约：roomId 一律是字符串。** 房间在 danmaku / 网关 / Redis key 里
// 都是字符串形态，事件里再用数字会让消费侧 `json: cannot unmarshal number into
// Go struct field ... of type string` 直接炸掉（重试到死信）。发布时用
// strconv.FormatUint 转一次即可，不要图省事直接塞 uint64。
type LiveStartedPayload struct {
	RoomID    string    `json:"roomId"`
	OwnerID   string    `json:"ownerId"`
	SessionID string    `json:"sessionId"`
	Title     string    `json:"title"`
	CoverURL  string    `json:"coverUrl"`
	StartedAt time.Time `json:"startedAt"`
}

// LiveStoppedPayload 是 live.stopped 的载荷。
//
// roomId 的字符串约定同 LiveStartedPayload。
type LiveStoppedPayload struct {
	RoomID      string    `json:"roomId"`
	SessionID   string    `json:"sessionId"`
	EndedAt     time.Time `json:"endedAt"`
	DurationSec int       `json:"durationSec"`
	PeakOnline  int       `json:"peakOnline"`
	ViewerTotal int64     `json:"viewerTotal"`
	Reason      string    `json:"reason"`
}
