// Package model 定义与数据库表一一对应的持久化结构。
//
// 表名统一带 `media_` 前缀：本服务与 Java 体系（auth/user/product/order）
// **共用同一个 MySQL 库 auth_service**，Java 侧用的是无前缀的通用名
// （account / user / product / sku / inventory / order / payment_record），
// 共库场景必须靠前缀表达归属。完整 DDL（幂等，可手工执行）见
// `deploy/db/go/media-tables.sql`（仓库根的 deploy 目录下）。
package model

import "time"

// 弹幕状态。
const (
	// StatusNormal 正常展示。
	StatusNormal = "NORMAL"
	// StatusRecalled 已撤回（软删，保留审计）。
	StatusRecalled = "RECALLED"
)

// 弹幕显示位置。
const (
	PositionScroll = "SCROLL"
	PositionTop    = "TOP"
	PositionBottom = "BOTTOM"
)

// DefaultColor 是默认弹幕颜色。
const DefaultColor = "#FFFFFF"

// Danmaku 对应表 media_danmaku。
//
// 设计取舍（详见设计文档 §3.2）：
//   - room_id 与 video_id 互斥共存：一套模型统一直播弹幕与点播弹幕；
//   - sent_at 用 DATETIME(3)：同秒内多条弹幕要能稳定排序；
//   - client_msg_id **不加唯一索引**：唯一索引会让 MQ 重投变成报错而非幂等跳过，
//     去重改在 Redis 做短时窗口。
type Danmaku struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement;index:idx_room_id_id,priority:2" json:"id"`
	RoomID      string    `gorm:"column:room_id;size:64;not null;default:'';index:idx_room_id_id,priority:1;index:idx_room_sent,priority:1" json:"roomId"`
	VideoID     string    `gorm:"column:video_id;size:64;not null;default:'';index:idx_video_time,priority:1" json:"videoId"`
	UID         string    `gorm:"column:uid;size:64;not null;index:idx_uid_sent,priority:1" json:"uid"`
	Nickname    string    `gorm:"column:nickname;size:64;not null;default:''" json:"nickname"`
	Content     string    `gorm:"column:content;size:200;not null" json:"content"`
	Color       string    `gorm:"column:color;size:9;not null;default:'#FFFFFF'" json:"color"`
	Position    string    `gorm:"column:position;size:16;not null;default:'SCROLL'" json:"position"`
	TimeMs      int       `gorm:"column:time_ms;not null;default:0;index:idx_video_time,priority:2" json:"timeMs"`
	ClientMsgID string    `gorm:"column:client_msg_id;size:64;not null;default:''" json:"clientMsgId"`
	Status      string    `gorm:"column:status;size:16;not null;default:'NORMAL'" json:"status"`
	SentAt      time.Time `gorm:"column:sent_at;type:datetime(3);not null;index:idx_room_sent,priority:2;index:idx_uid_sent,priority:2" json:"sentAt"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

// TableName 固定表名，避免 GORM 复数化规则带来的意外。
func (Danmaku) TableName() string { return "media_danmaku" }

// Normal 表示该弹幕是否处于可见状态。
func (d Danmaku) Normal() bool { return d.Status == StatusNormal }

// IsLive 表示这条弹幕属于直播场景。
func (d Danmaku) IsLive() bool { return d.RoomID != "" }
