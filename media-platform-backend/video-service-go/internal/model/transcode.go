package model

import "time"

// TranscodeStatus 是转码任务状态机。
type TranscodeStatus string

// 转码任务状态取值。
const (
	TranscodeStatusPending TranscodeStatus = "PENDING"
	TranscodeStatusRunning TranscodeStatus = "RUNNING"
	TranscodeStatusSuccess TranscodeStatus = "SUCCESS"
	TranscodeStatusFailed  TranscodeStatus = "FAILED"
)

// TranscodeTask 是转码任务（对应表 media_transcode_task）。
//
// 为什么要独立一张表而不是把状态塞进 video：
//  1. 一个视频可能重试多次转码，需要保留历史（排查失败原因）
//  2. progress 会被 Worker 高频更新，单独一张小表可避免把 video 行写热
type TranscodeTask struct {
	ID         uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID    uint64          `gorm:"not null;index:idx_video" json:"videoId"`
	EventID    string          `gorm:"type:varchar(64);not null;default:'';index:idx_event" json:"eventId"`
	Status     TranscodeStatus `gorm:"type:varchar(16);not null;default:'PENDING';index:idx_status" json:"status"`
	Profiles   string          `gorm:"type:varchar(255);not null;default:''" json:"profiles"`
	Progress   int             `gorm:"not null;default:0" json:"progress"`
	Message    string          `gorm:"type:varchar(512);not null;default:''" json:"message"`
	StartedAt  *time.Time      `json:"startedAt"`
	FinishedAt *time.Time      `json:"finishedAt"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

// TableName 指定表名。
func (TranscodeTask) TableName() string { return "media_transcode_task" }
