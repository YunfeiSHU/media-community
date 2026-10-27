package repository

import (
	"context"
	"time"

	"github.com/example/video-service-go/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InteractionState 是单个视频的互动状态与计数。
type InteractionState struct {
	Liked         bool
	Favorited     bool
	LikeCount     int64
	FavoriteCount int64
}

// InteractionRepository 是点赞 / 收藏的数据访问接口。
type InteractionRepository interface {
	// Like 返回 changed=false 表示「已经点过」，计数不变（幂等）。
	Like(ctx context.Context, videoID uint64, uid string) (bool, error)
	Unlike(ctx context.Context, videoID uint64, uid string) (bool, error)
	Favorite(ctx context.Context, videoID uint64, uid string) (bool, error)
	Unfavorite(ctx context.Context, videoID uint64, uid string) (bool, error)
	// States 批量取状态；uid 为空时只返回计数（游客）。
	States(ctx context.Context, uid string, videoIDs []uint64) (map[uint64]InteractionState, error)
}

type interactionRepository struct {
	db *gorm.DB
}

// NewInteractionRepository 构造互动仓储。
func NewInteractionRepository(db *gorm.DB) InteractionRepository {
	return &interactionRepository{db: db}
}

func (r *interactionRepository) Like(ctx context.Context, videoID uint64, uid string) (bool, error) {
	return r.add(ctx, videoID, uid, &model.VideoLike{VideoID: videoID, UID: uid, CreatedAt: time.Now()}, "like_count")
}

func (r *interactionRepository) Favorite(ctx context.Context, videoID uint64, uid string) (bool, error) {
	return r.add(ctx, videoID, uid, &model.VideoFavorite{VideoID: videoID, UID: uid, CreatedAt: time.Now()}, "favorite_count")
}

// add 在一个事务里「插入关系 + 原子加计数」。
//
// 插入用 ON DUPLICATE KEY DO NOTHING，靠唯一键保证重复点击不会重复计数；
// RowsAffected=0 说明已存在，此时直接返回、不碰计数。
func (r *interactionRepository) add(ctx context.Context, videoID uint64, uid string, row any, column string) (bool, error) {
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		changed = true
		return tx.Model(&model.Video{}).Where("id = ?", videoID).
			UpdateColumn(column, gorm.Expr(column+" + 1")).Error
	})
	return changed, err
}

func (r *interactionRepository) Unlike(ctx context.Context, videoID uint64, uid string) (bool, error) {
	return r.remove(ctx, &model.VideoLike{}, videoID, uid, "like_count")
}

func (r *interactionRepository) Unfavorite(ctx context.Context, videoID uint64, uid string) (bool, error) {
	return r.remove(ctx, &model.VideoFavorite{}, videoID, uid, "favorite_count")
}

func (r *interactionRepository) remove(ctx context.Context, target any, videoID uint64, uid, column string) (bool, error) {
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("video_id = ? AND uid = ?", videoID, uid).Delete(target)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		changed = true
		// GREATEST 防止并发/脏数据把计数减成负数
		return tx.Model(&model.Video{}).Where("id = ?", videoID).
			UpdateColumn(column, gorm.Expr("GREATEST("+column+" - 1, 0)")).Error
	})
	return changed, err
}

func (r *interactionRepository) States(ctx context.Context, uid string, videoIDs []uint64) (map[uint64]InteractionState, error) {
	states := make(map[uint64]InteractionState, len(videoIDs))
	if len(videoIDs) == 0 {
		return states, nil
	}

	var rows []struct {
		ID            uint64
		LikeCount     int64
		FavoriteCount int64
	}
	if err := r.db.WithContext(ctx).Model(&model.Video{}).
		Select("id, like_count, favorite_count").
		Where("id IN ?", videoIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		states[row.ID] = InteractionState{LikeCount: row.LikeCount, FavoriteCount: row.FavoriteCount}
	}

	if uid == "" {
		return states, nil
	}

	var likedIDs []uint64
	if err := r.db.WithContext(ctx).Model(&model.VideoLike{}).
		Where("uid = ? AND video_id IN ?", uid, videoIDs).
		Pluck("video_id", &likedIDs).Error; err != nil {
		return nil, err
	}
	var favoritedIDs []uint64
	if err := r.db.WithContext(ctx).Model(&model.VideoFavorite{}).
		Where("uid = ? AND video_id IN ?", uid, videoIDs).
		Pluck("video_id", &favoritedIDs).Error; err != nil {
		return nil, err
	}

	for _, id := range likedIDs {
		state := states[id]
		state.Liked = true
		states[id] = state
	}
	for _, id := range favoritedIDs {
		state := states[id]
		state.Favorited = true
		states[id] = state
	}
	return states, nil
}
