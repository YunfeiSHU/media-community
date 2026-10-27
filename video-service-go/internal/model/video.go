// Package model 定义 GORM 实体与领域枚举。
package model

import (
	"time"

	"gorm.io/gorm"
)

// VideoStatus 是视频状态机。
type VideoStatus string

// 视频状态取值。
const (
	VideoStatusDraft       VideoStatus = "DRAFT"       // 草稿：仅有元数据
	VideoStatusTranscoding VideoStatus = "TRANSCODING" // 原片已上传，转码中
	VideoStatusReady       VideoStatus = "READY"       // 转码完成，可发布
	VideoStatusPublished   VideoStatus = "PUBLISHED"   // 已发布，游客可见
	VideoStatusOffline     VideoStatus = "OFFLINE"     // 已下架
	VideoStatusFailed      VideoStatus = "FAILED"      // 转码失败
)

// CanUpload 判断当前状态是否允许上传原片（DRAFT 首次上传，FAILED 重传）。
func (s VideoStatus) CanUpload() bool {
	return s == VideoStatusDraft || s == VideoStatusFailed
}

// CanPublish 判断当前状态是否允许发布（READY 首发，OFFLINE 重新上架）。
func (s VideoStatus) CanPublish() bool {
	return s == VideoStatusReady || s == VideoStatusOffline
}

// VisibleToAll 判断游客是否可见。
func (s VideoStatus) VisibleToAll() bool {
	return s == VideoStatusPublished
}

// Valid 判断是否为已知状态。
func (s VideoStatus) Valid() bool {
	switch s {
	case VideoStatusDraft, VideoStatusTranscoding, VideoStatusReady,
		VideoStatusPublished, VideoStatusOffline, VideoStatusFailed:
		return true
	default:
		return false
	}
}

// Video 是视频元数据实体（对应表 media_video）。
//
// 表名带 media_ 前缀的原因：本服务与 Java 体系（auth/user/product/order）
// 共用同一个 MySQL 库 auth_service，带前缀才能表达归属并避免同名冲突。
type Video struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Title        string         `gorm:"type:varchar(128);not null" json:"title"`
	Description  string         `gorm:"type:varchar(1024);not null;default:''" json:"description"`
	OwnerID      string         `gorm:"type:varchar(64);not null;index:idx_owner_created,priority:1" json:"ownerId"`
	CategoryID   *uint64        `gorm:"index:idx_category" json:"categoryId"`
	CoverURL     string         `gorm:"type:varchar(512);not null;default:''" json:"coverUrl"`
	SourceObject string         `gorm:"type:varchar(512);not null;default:''" json:"sourceObject"`
	PlaybackURL  string         `gorm:"type:varchar(512);not null;default:''" json:"playbackUrl"`
	Duration     int            `gorm:"not null;default:0" json:"duration"`
	SizeBytes    int64          `gorm:"not null;default:0" json:"sizeBytes"`
	Status       VideoStatus    `gorm:"type:varchar(16);not null;default:'DRAFT';index:idx_status_created,priority:1" json:"status"`
	ViewCount    int64          `gorm:"not null;default:0" json:"viewCount"`
	// 互动计数：由点赞/收藏操作在同一事务里原子增减。
	// 没有单独建计数表，是为了让列表页一次查询就能拿到（详情/列表 DTO 都要带）。
	LikeCount     int64          `gorm:"not null;default:0" json:"likeCount"`
	FavoriteCount int64          `gorm:"not null;default:0" json:"favoriteCount"`
	PublishedAt   *time.Time     `json:"publishedAt"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index:idx_deleted" json:"-"`
}

// TableName 指定表名。
func (Video) TableName() string { return "media_video" }
