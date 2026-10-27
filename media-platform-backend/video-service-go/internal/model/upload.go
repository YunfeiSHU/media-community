package model

import (
	"time"

	"gorm.io/gorm"
)

// UploadStatus 是分片上传会话状态机。
type UploadStatus string

// 上传会话状态取值。
const (
	// UploadStatusUploading 会话进行中，可继续上传分片（断点续传的依据）。
	UploadStatusUploading UploadStatus = "UPLOADING"
	// UploadStatusCompleted 分片已全部合并成对象，会话终结。
	UploadStatusCompleted UploadStatus = "COMPLETED"
	// UploadStatusAborted 用户主动取消，或超过有效期被新的初始化取代。
	UploadStatusAborted UploadStatus = "ABORTED"
)

// UploadSession 是一次分片上传的会话（对应表 media_upload_session）。
//
// 为什么要独立一张表，而不是每次由客户端带着 uploadId 求服务端：
//  1. **断点续传的锚点**：客户端刷新页面 / 换设备后手上没有 uploadId，
//     只能凭 videoId 找回「传到哪了」，这个映射必须持久化；
//  2. **业务上下文的载体**：partSize / fileSize / ext 决定「分片怎么切、还有几片」，
//     放在服务端就不需要客户端反复声明，也就不存在两端不一致；
//  3. **可审计、可清理**：能查出「谁在传什么」，也能让过期会话被 abort 掉，
//     避免 MinIO 里堆积永远合并不了的碎片。
//
// upload_id 由 MinIO 生成，服务重启后依然有效（它是对象存储侧的状态），
// 所以本表是「本地索引」，MySQL 与 MinIO 的一致性由 service 层的补偿逻辑保证。
type UploadSession struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID    uint64         `gorm:"not null;index:idx_video" json:"videoId"`
	OwnerID    string         `gorm:"type:varchar(64);not null" json:"ownerId"`
	ObjectKey  string         `gorm:"type:varchar(512);not null" json:"objectKey"`
	UploadID   string         `gorm:"column:upload_id;type:varchar(255);not null" json:"uploadId"`
	Ext        string         `gorm:"type:varchar(16);not null" json:"ext"`
	FileSize   int64          `gorm:"not null;default:0" json:"fileSize"`
	PartSize   int64          `gorm:"not null;default:0" json:"partSize"`
	TotalParts int            `gorm:"not null;default:0" json:"totalParts"`
	Status     UploadStatus   `gorm:"type:varchar(16);not null;default:'UPLOADING'" json:"status"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index:idx_deleted" json:"-"`
}

// TableName 指定表名。
func (UploadSession) TableName() string { return "media_upload_session" }

// Active 判断会话是否仍可用于续传。
func (s UploadSession) Active() bool { return s.Status == UploadStatusUploading }

// LastPartNumber 返回末片序号（totalParts 为 0 时返回 0）。
func (s UploadSession) LastPartNumber() int { return s.TotalParts }
