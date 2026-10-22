package repository

import (
	"context"

	"github.com/example/live-service-go/internal/model"
	"gorm.io/gorm"
)

// RecordRepository 是直播场次记录的数据访问接口。
type RecordRepository interface {
	Create(ctx context.Context, rec *model.Record) error
	GetBySessionID(ctx context.Context, sessionID string) (*model.Record, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	ListByRoom(ctx context.Context, roomID uint64, offset, limit int) ([]model.Record, int64, error)
}

type recordRepository struct {
	db *gorm.DB
}

// NewRecordRepository 构造场次记录仓储。
func NewRecordRepository(db *gorm.DB) RecordRepository {
	return &recordRepository{db: db}
}

func (r *recordRepository) Create(ctx context.Context, rec *model.Record) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *recordRepository) GetBySessionID(ctx context.Context, sessionID string) (*model.Record, error) {
	var rec model.Record
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *recordRepository) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.Record{}).Where("id = ?", id).Updates(fields).Error
}

func (r *recordRepository) ListByRoom(
	ctx context.Context, roomID uint64, offset, limit int,
) ([]model.Record, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Record{}).Where("room_id = ?", roomID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Record
	err := q.Order("started_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
