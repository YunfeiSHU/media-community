// Package repository 是数据访问层：只做 SQL，不含业务判断。
//
// 约定：查询不到记录时返回 gorm.ErrRecordNotFound 原样错误，
// 由 service 层翻译成业务错误码（保持 repository 与 errs 解耦）。
package repository

import (
	"context"
	"time"

	"github.com/example/danmaku-service-go/internal/model"
	"gorm.io/gorm"
)

// DanmakuRepository 定义弹幕持久化操作。
type DanmakuRepository interface {
	Create(ctx context.Context, item *model.Danmaku) error
	GetByID(ctx context.Context, id uint64) (*model.Danmaku, error)
	MarkRecalled(ctx context.Context, id uint64) (bool, error)
	CountByRoom(ctx context.Context, roomID string) (int64, error)
	LastSentAt(ctx context.Context, roomID string) (*time.Time, error)
	ListByRoom(ctx context.Context, roomID string, offset, limit int) ([]model.Danmaku, error)
	ListByVideoRange(ctx context.Context, videoID string, fromMs, toMs, limit int) ([]model.Danmaku, error)
}

type danmakuRepository struct {
	db *gorm.DB
}

// NewDanmakuRepository 构造 MySQL 实现。
func NewDanmakuRepository(db *gorm.DB) DanmakuRepository {
	return &danmakuRepository{db: db}
}

func (r *danmakuRepository) Create(ctx context.Context, item *model.Danmaku) error {
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *danmakuRepository) GetByID(ctx context.Context, id uint64) (*model.Danmaku, error) {
	var item model.Danmaku
	if err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// MarkRecalled 幂等地把状态置为 RECALLED，返回是否真的发生了变更。
func (r *danmakuRepository) MarkRecalled(ctx context.Context, id uint64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.Danmaku{}).
		Where("id = ? AND status <> ?", id, model.StatusRecalled).
		Update("status", model.StatusRecalled)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// CountByRoom 统计房间可见弹幕总数（撤回的不计入）。
//
// 热数据（Redis）只有最近 N 条，若直接用它当 total，
// 前端分页器会显示出错误的总页数（见接口文档 §4.1 的说明）。
func (r *danmakuRepository) CountByRoom(ctx context.Context, roomID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.Danmaku{}).
		Where("room_id = ? AND status = ?", roomID, model.StatusNormal).
		Count(&total).Error
	return total, err
}

// LastSentAt 返回房间最后一条可见弹幕的时间（无数据时返回 nil）。
func (r *danmakuRepository) LastSentAt(ctx context.Context, roomID string) (*time.Time, error) {
	var row struct {
		Last *time.Time
	}
	err := r.db.WithContext(ctx).
		Model(&model.Danmaku{}).
		Select("MAX(sent_at) AS last").
		Where("room_id = ? AND status = ?", roomID, model.StatusNormal).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return row.Last, nil
}

// ListByRoom 按 sent_at 倒序分页（走 idx_room_sent）。
func (r *danmakuRepository) ListByRoom(ctx context.Context, roomID string, offset, limit int) ([]model.Danmaku, error) {
	items := make([]model.Danmaku, 0, limit)
	err := r.db.WithContext(ctx).
		Where("room_id = ? AND status = ?", roomID, model.StatusNormal).
		Order("sent_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error
	return items, err
}

// ListByVideoRange 按 time_ms 升序返回区间内弹幕（走 idx_video_time）。
//
// 点播弹幕必须按 time_ms 升序：客户端要在播放到对应时刻时投放，
// 乱序返回意味着客户端还要自己排序。
func (r *danmakuRepository) ListByVideoRange(ctx context.Context, videoID string, fromMs, toMs, limit int) ([]model.Danmaku, error) {
	query := r.db.WithContext(ctx).
		Where("video_id = ? AND status = ?", videoID, model.StatusNormal).
		Where("time_ms >= ?", fromMs)
	if toMs > 0 {
		query = query.Where("time_ms <= ?", toMs)
	}

	items := make([]model.Danmaku, 0, limit)
	err := query.
		Order("time_ms ASC, id ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}
