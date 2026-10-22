// Package repository 封装数据访问。只做存取，不含业务判断。
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/example/live-service-go/internal/model"
	"gorm.io/gorm"
)

// RoomFilter 是直播间列表查询条件。
type RoomFilter struct {
	Keyword    string
	CategoryID *uint64
	OwnerID    string
	Status     string
	LivingOnly bool
	Offset     int
	Limit      int
}

// RoomRepository 是直播间数据访问接口。
type RoomRepository interface {
	Create(ctx context.Context, r *model.Room) error
	GetByID(ctx context.Context, id uint64) (*model.Room, error)
	GetByStreamKey(ctx context.Context, streamKey string) (*model.Room, error)

	// UpdateFields 只更新指定字段（避免 Save 覆盖并发改动）。
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error

	// Transition 是「带前置状态约束」的更新：仅当当前状态在 from 中时才写入。
	// 返回是否真的更新了一行——这是状态机并发安全的核心原语：
	// 两个并发的 on_publish 回调只有一个能把 STARTING 改成 LIVING。
	Transition(ctx context.Context, id uint64, from []model.RoomStatus, fields map[string]any) (bool, error)

	Delete(ctx context.Context, id uint64) error

	// CountByOwner 统计该主播的房间总数（用于数量上限校验）。
	CountByOwner(ctx context.Context, ownerID string) (int64, error)

	// FindLiveByOwner 找出该主播正在开播流程中的房间（STARTING/LIVING）。
	// excludeID 用于「重复开播时忽略自己」。
	FindLiveByOwner(ctx context.Context, ownerID string, excludeID uint64) (*model.Room, error)

	// ListStartingBefore 列出 STARTING 且开播时间早于 deadline 的房间，用于超时兜底。
	ListStartingBefore(ctx context.Context, deadline time.Time) ([]model.Room, error)

	List(ctx context.Context, f RoomFilter) ([]model.Room, int64, error)
}

type roomRepository struct {
	db *gorm.DB
}

// NewRoomRepository 构造直播间仓储。
func NewRoomRepository(db *gorm.DB) RoomRepository {
	return &roomRepository{db: db}
}

func (r *roomRepository) Create(ctx context.Context, room *model.Room) error {
	return r.db.WithContext(ctx).Create(room).Error
}

func (r *roomRepository) GetByID(ctx context.Context, id uint64) (*model.Room, error) {
	var room model.Room
	if err := r.db.WithContext(ctx).First(&room, id).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *roomRepository) GetByStreamKey(ctx context.Context, streamKey string) (*model.Room, error) {
	var room model.Room
	err := r.db.WithContext(ctx).Where("stream_key = ?", streamKey).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *roomRepository) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.Room{}).Where("id = ?", id).Updates(fields).Error
}

func (r *roomRepository) Transition(
	ctx context.Context, id uint64, from []model.RoomStatus, fields map[string]any,
) (bool, error) {
	if len(fields) == 0 || len(from) == 0 {
		return false, nil
	}
	res := r.db.WithContext(ctx).Model(&model.Room{}).
		Where("id = ? AND status IN ?", id, from).
		Updates(fields)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Delete 软删除（走 gorm.DeletedAt）。
func (r *roomRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Room{}, id).Error
}

func (r *roomRepository) CountByOwner(ctx context.Context, ownerID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Room{}).
		Where("owner_id = ?", ownerID).Count(&count).Error
	return count, err
}

func (r *roomRepository) FindLiveByOwner(
	ctx context.Context, ownerID string, excludeID uint64,
) (*model.Room, error) {
	q := r.db.WithContext(ctx).Model(&model.Room{}).
		Where("owner_id = ? AND status IN ?", ownerID,
			[]model.RoomStatus{model.RoomStatusStarting, model.RoomStatusLiving})
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}

	var room model.Room
	err := q.Order("id DESC").First(&room).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *roomRepository) ListStartingBefore(ctx context.Context, deadline time.Time) ([]model.Room, error) {
	var rooms []model.Room
	// 用 last_started_at（开播时刻）而不是 updated_at 判断超时：
	// 直播中改标题会刷新 updated_at，若按 updated_at 判断会让超时窗口被无限推迟。
	err := r.db.WithContext(ctx).
		Where("status = ? AND last_started_at IS NOT NULL AND last_started_at < ?",
			model.RoomStatusStarting, deadline).
		Order("id ASC").Limit(200).
		Find(&rooms).Error
	if err != nil {
		return nil, err
	}
	return rooms, nil
}

func (r *roomRepository) List(ctx context.Context, f RoomFilter) ([]model.Room, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Room{})

	if f.Keyword != "" {
		q = q.Where("title LIKE ?", "%"+f.Keyword+"%")
	}
	if f.CategoryID != nil {
		q = q.Where("category_id = ?", *f.CategoryID)
	}
	if f.OwnerID != "" {
		q = q.Where("owner_id = ?", f.OwnerID)
	}
	switch {
	case f.Status != "":
		q = q.Where("status = ?", f.Status)
	case f.LivingOnly:
		q = q.Where("status = ?", model.RoomStatusLiving)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Room
	// 排序：LIVING 优先，其次按更新时间倒序。
	// 「正在直播」的复合索引 idx_status_updated 正好覆盖 status 过滤 + updated_at 排序。
	err := q.Order("(status = 'LIVING') DESC, updated_at DESC, id DESC").
		Offset(f.Offset).Limit(f.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
