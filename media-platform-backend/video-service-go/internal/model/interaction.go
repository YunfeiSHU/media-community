package model

import "time"

// VideoLike 是视频点赞关系（media_video_like）。
//
// 唯一键 (video_id, uid) 让「重复点赞」在数据库层天然幂等：
// 插入冲突时 DoNothing，RowsAffected=0，计数就不会被重复加。
type VideoLike struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	VideoID   uint64    `gorm:"not null;uniqueIndex:uk_like_video_uid,priority:1"`
	UID       string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_like_video_uid,priority:2"`
	CreatedAt time.Time `gorm:"not null"`
}

// TableName 指定表名。
func (VideoLike) TableName() string { return "media_video_like" }

// VideoFavorite 是视频收藏关系（media_video_favorite）。
type VideoFavorite struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	VideoID   uint64    `gorm:"not null;uniqueIndex:uk_favorite_video_uid,priority:1"`
	UID       string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_favorite_video_uid,priority:2"`
	CreatedAt time.Time `gorm:"not null;index:idx_favorite_uid_created,priority:1"`
}

// TableName 指定表名。
func (VideoFavorite) TableName() string { return "media_video_favorite" }
