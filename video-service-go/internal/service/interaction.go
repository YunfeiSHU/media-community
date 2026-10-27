package service

import (
	"context"
	"log/slog"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/model"
	"github.com/example/video-service-go/internal/repository"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/identity"
)

// maxInteractionBatch 是批量查状态的上限：列表页一屏最多取这么多条。
const maxInteractionBatch = 100

// InteractionService 处理点赞 / 收藏。
type InteractionService struct {
	videos       repository.VideoRepository
	interactions repository.InteractionRepository
	log          *slog.Logger
}

// NewInteractionService 构造互动服务。
func NewInteractionService(videos repository.VideoRepository,
	interactions repository.InteractionRepository, log *slog.Logger) *InteractionService {
	return &InteractionService{videos: videos, interactions: interactions, log: log}
}

// Like 点赞（幂等：重复调用不重复计数）。
func (s *InteractionService) Like(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.InteractionState, error) {
	return s.toggle(ctx, u, id, true, true)
}

// Unlike 取消点赞。
func (s *InteractionService) Unlike(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.InteractionState, error) {
	return s.toggle(ctx, u, id, false, true)
}

// Favorite 收藏。
func (s *InteractionService) Favorite(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.InteractionState, error) {
	return s.toggle(ctx, u, id, true, false)
}

// Unfavorite 取消收藏。
func (s *InteractionService) Unfavorite(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.InteractionState, error) {
	return s.toggle(ctx, u, id, false, false)
}

func (s *InteractionService) toggle(ctx context.Context, u identity.CurrentUser, id uint64, on, like bool) (*dto.InteractionState, error) {
	if _, err := s.ensureVisible(ctx, u, id); err != nil {
		return nil, err
	}

	var changed bool
	var err error
	switch {
	case like && on:
		changed, err = s.interactions.Like(ctx, id, u.UID)
	case like && !on:
		changed, err = s.interactions.Unlike(ctx, id, u.UID)
	case !like && on:
		changed, err = s.interactions.Favorite(ctx, id, u.UID)
	default:
		changed, err = s.interactions.Unfavorite(ctx, id, u.UID)
	}
	if err != nil {
		return nil, err
	}

	states, err := s.interactions.States(ctx, u.UID, []uint64{id})
	if err != nil {
		return nil, err
	}
	state := states[id]
	s.log.Info("video_interaction", "videoId", id, "uid", u.UID, "like", like, "on", on, "changed", changed)
	return &dto.InteractionState{
		VideoID:       id,
		Liked:         state.Liked,
		Favorited:     state.Favorited,
		LikeCount:     state.LikeCount,
		FavoriteCount: state.FavoriteCount,
	}, nil
}

// Get 返回单条视频的互动状态；游客返回计数 + false。
func (s *InteractionService) Get(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.InteractionState, error) {
	if _, err := s.ensureVisible(ctx, u, id); err != nil {
		return nil, err
	}
	states, err := s.interactions.States(ctx, u.UID, []uint64{id})
	if err != nil {
		return nil, err
	}
	state := states[id]
	return &dto.InteractionState{
		VideoID:       id,
		Liked:         state.Liked,
		Favorited:     state.Favorited,
		LikeCount:     state.LikeCount,
		FavoriteCount: state.FavoriteCount,
	}, nil
}

// Batch 批量返回互动状态（列表页一次拿一屏）。
func (s *InteractionService) Batch(ctx context.Context, u identity.CurrentUser, ids []uint64) (*dto.InteractionList, error) {
	if len(ids) > maxInteractionBatch {
		return nil, errs.CodeInvalidParam.Newf("ids 最多 %d 个", maxInteractionBatch)
	}
	states, err := s.interactions.States(ctx, u.UID, ids)
	if err != nil {
		return nil, err
	}
	items := make([]dto.InteractionState, 0, len(ids))
	for _, id := range ids {
		state := states[id]
		items = append(items, dto.InteractionState{
			VideoID:       id,
			Liked:         state.Liked,
			Favorited:     state.Favorited,
			LikeCount:     state.LikeCount,
			FavoriteCount: state.FavoriteCount,
		})
	}
	return &dto.InteractionList{Items: items}, nil
}

// ensureVisible 与视频详情保持同一套可见性：未发布只对作者 / 管理员可见，其余一律 404。
func (s *InteractionService) ensureVisible(ctx context.Context, u identity.CurrentUser, id uint64) (*model.Video, error) {
	video, err := s.videos.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, errs.CodeNotFound.Plain()
		}
		return nil, err
	}
	visible := video.Status == model.VideoStatusPublished || u.Owns(video.OwnerID) || u.IsAdmin()
	if !visible {
		return nil, errs.CodeNotFound.Plain()
	}
	return video, nil
}
