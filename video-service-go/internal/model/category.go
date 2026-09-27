package model

import "time"

// Category 是视频分类（对应表 media_video_category）。
type Category struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_category_name" json:"name"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 指定表名。
func (Category) TableName() string { return "media_video_category" }

// VideoTag 是视频标签（对应表 media_video_tag）。
// (video_id, tag) 唯一，避免同一视频重复打同一标签。
type VideoTag struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   uint64    `gorm:"not null;uniqueIndex:uk_video_tag,priority:1" json:"videoId"`
	Tag       string    `gorm:"type:varchar(32);not null;uniqueIndex:uk_video_tag,priority:2" json:"tag"`
	CreatedAt time.Time `json:"createdAt"`
}

// TableName 指定表名。
func (VideoTag) TableName() string { return "media_video_tag" }
