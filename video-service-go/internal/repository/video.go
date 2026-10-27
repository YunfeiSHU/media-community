// Package repository 封装数据访问。只做存取，不含业务判断。
package repository

import (
	"context"
	"errors"

	"github.com/example/video-service-go/internal/model"
	"gorm.io/gorm"
)

// VideoFilter 是视频列表查询条件。
type VideoFilter struct {
	Keyword       string
	CategoryID    *uint64
	OwnerID       string
	Status        string
	OnlyPublished bool // 游客 / 普通用户强制只看已发布
	// FavoriteUID 非空时只返回该用户的收藏，并按收藏时间倒序
	FavoriteUID string
	Offset      int
	Limit       int
}

// VideoRepository 是视频与标签的数据访问接口。
type VideoRepository interface {
	Create(ctx context.Context, v *model.Video) error
	GetByID(ctx context.Context, id uint64) (*model.Video, error)
	Update(ctx context.Context, v *model.Video) error
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	Delete(ctx context.Context, id uint64) error
	List(ctx context.Context, f VideoFilter) ([]model.Video, int64, error)
	AddViewCount(ctx context.Context, id uint64, delta int64) error

	ReplaceTags(ctx context.Context, videoID uint64, tags []string) error
	TagsByVideoIDs(ctx context.Context, videoIDs []uint64) (map[uint64][]string, error)
}

type videoRepository struct {
	db *gorm.DB
}

// NewVideoRepository 构造视频仓储。
func NewVideoRepository(db *gorm.DB) VideoRepository {
	return &videoRepository{db: db}
}

func (r *videoRepository) Create(ctx context.Context, v *model.Video) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *videoRepository) GetByID(ctx context.Context, id uint64) (*model.Video, error) {
	var v model.Video
	err := r.db.WithContext(ctx).First(&v, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *videoRepository) Update(ctx context.Context, v *model.Video) error {
	return r.db.WithContext(ctx).Save(v).Error
}

// UpdateFields 只更新指定字段（避免 Save 覆盖并发改动）。
func (r *videoRepository) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.Video{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除（走 gorm.DeletedAt）。
func (r *videoRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Video{}, id).Error
}

func (r *videoRepository) List(ctx context.Context, f VideoFilter) ([]model.Video, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Video{})

	if f.Keyword != "" {
		q = q.Where("title LIKE ?", "%"+f.Keyword+"%")
	}
	if f.CategoryID != nil {
		q = q.Where("category_id = ?", *f.CategoryID)
	}
	if f.OwnerID != "" {
		q = q.Where("owner_id = ?", f.OwnerID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	} else if f.OnlyPublished {
		q = q.Where("status = ?", model.VideoStatusPublished)
	}

	if f.FavoriteUID != "" {
		// 收藏列表按「收藏时间」倒序，而不是视频发布时间
		q = q.Joins("JOIN media_video_favorite fav ON fav.video_id = media_video.id AND fav.uid = ?", f.FavoriteUID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	order := "media_video.created_at DESC, media_video.id DESC"
	if f.FavoriteUID != "" {
		order = "fav.created_at DESC, media_video.id DESC"
	}

	var items []model.Video
	err := q.Order(order).
		Offset(f.Offset).Limit(f.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// AddViewCount 原子累加播放量。
func (r *videoRepository) AddViewCount(ctx context.Context, id uint64, delta int64) error {
	return r.db.WithContext(ctx).Model(&model.Video{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", delta)).Error
}

// ReplaceTags 全量替换标签（事务内先删后插）。
func (r *videoRepository) ReplaceTags(ctx context.Context, videoID uint64, tags []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id = ?", videoID).Delete(&model.VideoTag{}).Error; err != nil {
			return err
		}
		if len(tags) == 0 {
			return nil
		}
		rows := make([]model.VideoTag, 0, len(tags))
		for _, t := range tags {
			rows = append(rows, model.VideoTag{VideoID: videoID, Tag: t})
		}
		return tx.Create(&rows).Error
	})
}

// TagsByVideoIDs 批量取标签，返回 videoID → tags，避免 N+1 查询。
func (r *videoRepository) TagsByVideoIDs(ctx context.Context, videoIDs []uint64) (map[uint64][]string, error) {
	out := make(map[uint64][]string, len(videoIDs))
	if len(videoIDs) == 0 {
		return out, nil
	}

	var rows []model.VideoTag
	err := r.db.WithContext(ctx).
		Where("video_id IN ?", videoIDs).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.VideoID] = append(out[row.VideoID], row.Tag)
	}
	return out, nil
}
