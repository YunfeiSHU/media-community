package repository

import (
	"context"
	"errors"
	"time"

	"github.com/example/video-service-go/internal/model"
	"gorm.io/gorm"
)

// TranscodeRepository 是转码任务的数据访问接口。
type TranscodeRepository interface {
	Create(ctx context.Context, t *model.TranscodeTask) error
	GetByID(ctx context.Context, id uint64) (*model.TranscodeTask, error)
	LatestByVideoID(ctx context.Context, videoID uint64) (*model.TranscodeTask, error)
	GetByEventID(ctx context.Context, eventID string) (*model.TranscodeTask, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	UpdateProgress(ctx context.Context, id uint64, progress int) error
	Complete(ctx context.Context, videoID, taskID uint64, playbackURL string, duration int, finishedAt time.Time) error
	MarkFailed(ctx context.Context, videoID, taskID uint64, message string, finishedAt time.Time) error
}

type transcodeRepository struct {
	db *gorm.DB
}

// NewTranscodeRepository 构造转码任务仓储。
func NewTranscodeRepository(db *gorm.DB) TranscodeRepository {
	return &transcodeRepository{db: db}
}

func (r *transcodeRepository) Create(ctx context.Context, t *model.TranscodeTask) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *transcodeRepository) GetByID(ctx context.Context, id uint64) (*model.TranscodeTask, error) {
	var t model.TranscodeTask
	err := r.db.WithContext(ctx).First(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// LatestByVideoID 取该视频最近一次转码任务。
func (r *transcodeRepository) LatestByVideoID(ctx context.Context, videoID uint64) (*model.TranscodeTask, error) {
	var t model.TranscodeTask
	err := r.db.WithContext(ctx).
		Where("video_id = ?", videoID).
		Order("id DESC").
		First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetByEventID 用于消费侧的幂等校验。
func (r *transcodeRepository) GetByEventID(ctx context.Context, eventID string) (*model.TranscodeTask, error) {
	var t model.TranscodeTask
	err := r.db.WithContext(ctx).Where("event_id = ?", eventID).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *transcodeRepository) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.TranscodeTask{}).Where("id = ?", id).Updates(fields).Error
}

func (r *transcodeRepository) UpdateProgress(ctx context.Context, id uint64, progress int) error {
	return r.db.WithContext(ctx).Model(&model.TranscodeTask{}).
		Where("id = ?", id).Update("progress", progress).Error
}

func (r *transcodeRepository) Complete(ctx context.Context, videoID, taskID uint64, playbackURL string, duration int, finishedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Video{}).Where("id = ?", videoID).
			Updates(map[string]any{
				"status":       model.VideoStatusReady,
				"playback_url": playbackURL,
				"duration":     duration,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&model.TranscodeTask{}).Where("id = ?", taskID).
			Updates(map[string]any{
				"status":      model.TranscodeStatusSuccess,
				"progress":    100,
				"message":     "",
				"finished_at": &finishedAt,
			}).Error
	})
}

func (r *transcodeRepository) MarkFailed(ctx context.Context, videoID, taskID uint64, message string, finishedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Video{}).Where("id = ?", videoID).
			Update("status", model.VideoStatusFailed).Error; err != nil {
			return err
		}
		return tx.Model(&model.TranscodeTask{}).Where("id = ?", taskID).
			Updates(map[string]any{
				"status":      model.TranscodeStatusFailed,
				"message":     message,
				"finished_at": &finishedAt,
			}).Error
	})
}
