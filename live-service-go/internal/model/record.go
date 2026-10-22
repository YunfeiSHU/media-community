package model

import "time"

// 关播原因取值。
const (
	EndReasonManual    = "manual"    // 主播调用关播接口
	EndReasonUnpublish = "unpublish" // SRS on_unpublish（断流）
	EndReasonForce     = "force"     // 创建者或管理员强制关闭
	EndReasonTimeout   = "timeout"   // 开播超时（STARTING 未收到 on_publish）
)

// Record 是直播场次记录（对应表 media_live_record）。
//
// 为什么要独立一张表而不是把历史塞进 room：
//  1. 一个房间会有很多场直播，历史是「一对多」，塞进 room 只能保留最后一次；
//  2. session_id 唯一，让 SRS on_publish 重复回调天然幂等（插入冲突即忽略）；
//  3. 历史列表是低频读、按时间倒序，独立小表不拖累 room 的写热点。
type Record struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RoomID      uint64     `gorm:"not null;index:idx_room_started,priority:1" json:"roomId"`
	SessionID   string     `gorm:"type:varchar(64);not null;uniqueIndex:uk_session" json:"sessionId"`
	StartedAt   time.Time  `gorm:"not null;index:idx_room_started,priority:2" json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt"`
	DurationSec int        `gorm:"not null;default:0" json:"durationSec"`
	PeakOnline  int        `gorm:"not null;default:0" json:"peakOnline"`
	ViewerTotal int64      `gorm:"not null;default:0" json:"viewerTotal"`
	EndReason   string     `gorm:"type:varchar(32);not null;default:''" json:"endReason"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// TableName 指定表名。
func (Record) TableName() string { return "media_live_record" }
