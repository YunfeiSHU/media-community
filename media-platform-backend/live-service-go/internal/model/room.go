// Package model 定义 GORM 实体与领域枚举。
package model

import (
	"time"

	"gorm.io/gorm"
)

// RoomStatus 是直播间状态机。
//
//	IDLE ──start──► STARTING ──on_publish──► LIVING ──stop/unpublish──► ENDED
//	                    │                                             │
//	                    └── 60s 未推流 ──► FAILED ◄──────────────────┘
//
// 关键点：**start 接口只到 STARTING**，LIVING 必须由 SRS 的 on_publish 回调置入。
type RoomStatus string

// 直播间状态取值。
const (
	RoomStatusIdle     RoomStatus = "IDLE"     // 已创建，从未开播
	RoomStatusStarting RoomStatus = "STARTING" // 已下发推流地址，等待推流
	RoomStatusLiving   RoomStatus = "LIVING"   // 正在直播（由 on_publish 置入）
	RoomStatusEnded    RoomStatus = "ENDED"    // 直播结束
	RoomStatusFailed   RoomStatus = "FAILED"   // 开播失败（STARTING 超时等）
)

// Valid 判断是否为已知状态。
func (s RoomStatus) Valid() bool {
	switch s {
	case RoomStatusIdle, RoomStatusStarting, RoomStatusLiving, RoomStatusEnded, RoomStatusFailed:
		return true
	default:
		return false
	}
}

// CanStart 判断当前状态是否允许开播（STARTING/LIVING 由 service 层做幂等处理）。
func (s RoomStatus) CanStart() bool {
	return s == RoomStatusIdle || s == RoomStatusEnded || s == RoomStatusFailed
}

// InProgress 表示「正在开播流程中」（占用采集/推流资源，不允许删除）。
func (s RoomStatus) InProgress() bool {
	return s == RoomStatusStarting || s == RoomStatusLiving
}

// CanStop 判断是否处于可关播状态（其他状态在 service 层做幂等返回）。
func (s RoomStatus) CanStop() bool {
	return s == RoomStatusStarting || s == RoomStatusLiving
}

// Room 是直播间实体（对应表 media_live_room）。
//
// 表名带 media_ 前缀的原因：本服务与 Java 体系（auth/user/product/order）
// 共用同一个 MySQL 库 auth_service，带前缀才能表达归属并避免同名冲突。
type Room struct {
	ID         uint64  `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID    string  `gorm:"type:varchar(64);not null;index:idx_owner;index:idx_owner_status,priority:1" json:"ownerId"`
	Title      string  `gorm:"type:varchar(64);not null" json:"title"`
	Notice     string  `gorm:"type:varchar(512);not null;default:''" json:"notice"`
	CoverURL   string  `gorm:"type:varchar(512);not null;default:''" json:"coverUrl"`
	CategoryID *uint64 `json:"categoryId"`

	// StreamKey 每次开播都会重新生成：旧密钥在关播后即失效，
	// 避免「上次直播的密钥被人拿去重推」。
	StreamKey string `gorm:"type:varchar(64);not null;uniqueIndex:uk_stream_key" json:"streamKey"`

	Status RoomStatus `gorm:"type:varchar(16);not null;default:'IDLE';index:idx_status_updated,priority:1;index:idx_owner_status,priority:2" json:"status"`

	// CurrentSession 本次直播会话 ID，用于识别「回调是否属于本次开播」。
	CurrentSession string `gorm:"type:varchar(64);not null;default:''" json:"currentSession"`

	ViewerTotal   int64      `gorm:"not null;default:0" json:"viewerTotal"`
	PeakOnline    int        `gorm:"not null;default:0" json:"peakOnline"`
	LastStartedAt *time.Time `json:"lastStartedAt"`
	LastEndedAt   *time.Time `json:"lastEndedAt"`

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `gorm:"index:idx_status_updated,priority:2" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名。
func (Room) TableName() string { return "media_live_room" }
