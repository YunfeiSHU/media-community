package repository

import (
	"context"

	"github.com/example/video-service-go/internal/model"
	"gorm.io/gorm"
)

// UploadSessionRepository 定义分片上传会话的数据访问操作。
type UploadSessionRepository interface {
	Create(ctx context.Context, s *model.UploadSession) error
	GetByUploadID(ctx context.Context, uploadID string) (*model.UploadSession, error)
	// ListUploading 返回某视频全部进行中的会话（按 id 倒序，最新在前）。
	//
	// 之所以返回列表而不是「最新一条」：正常流程下最多只有一条 UPLOADING，
	// 但进程崩溃 / 客户端中途改文件大小都会留下孤儿会话，
	// service 需要把它们逐个 abort 掉，否则 MinIO 里会堆积永远合并不了的碎片。
	ListUploading(ctx context.Context, videoID uint64) ([]model.UploadSession, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
}

type uploadSessionRepository struct {
	db *gorm.DB
}

// NewUploadSessionRepository 构造分片上传会话仓储。
func NewUploadSessionRepository(db *gorm.DB) UploadSessionRepository {
	return &uploadSessionRepository{db: db}
}

func (r *uploadSessionRepository) Create(ctx context.Context, s *model.UploadSession) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *uploadSessionRepository) GetByUploadID(ctx context.Context, uploadID string) (*model.UploadSession, error) {
	var s model.UploadSession
	if err := r.db.WithContext(ctx).First(&s, "upload_id = ?", uploadID).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *uploadSessionRepository) ListUploading(ctx context.Context, videoID uint64) ([]model.UploadSession, error) {
	var items []model.UploadSession
	err := r.db.WithContext(ctx).
		Where("video_id = ? AND status = ?", videoID, model.UploadStatusUploading).
		Order("id DESC").
		Find(&items).Error
	return items, err
}

// UpdateFields 只更新指定字段（避免 Save 覆盖并发改动）。
func (r *uploadSessionRepository) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.UploadSession{}).Where("id = ?", id).Updates(fields).Error
}
