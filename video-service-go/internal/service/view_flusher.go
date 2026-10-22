package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/example/video-service-go/internal/infra/cache"
	"github.com/example/video-service-go/internal/repository"
)

// ViewFlusher 定时把 Redis 中的播放量缓冲回写到 MySQL。
//
// 之所以这样设计：播放是最高频的读，如果每次播放都 UPDATE video.view_count，
// 热点视频会把同一行写热并产生大量行锁；改为 Redis INCR 后批量回写，
// 用「最终一致」换取写放大与锁竞争的消除。
type ViewFlusher struct {
	cache    *cache.VideoCache
	videos   repository.VideoRepository
	interval time.Duration
	log      *slog.Logger
}

// NewViewFlusher 构造播放量回写器。
func NewViewFlusher(videoCache *cache.VideoCache, videos repository.VideoRepository, interval time.Duration, log *slog.Logger) *ViewFlusher {
	return &ViewFlusher{cache: videoCache, videos: videos, interval: interval, log: log}
}

// Run 阻塞运行，直到 ctx 被取消；退出前会做最后一次回写。
func (f *ViewFlusher) Run(ctx context.Context) {
	f.log.Info("view_flusher_started", "interval", f.interval.String())

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			f.Flush(context.WithoutCancel(ctx))
			f.log.Info("view_flusher_stopped")
			return
		case <-ticker.C:
			f.Flush(ctx)
		}
	}
}

// Flush 执行一次回写。
func (f *ViewFlusher) Flush(ctx context.Context) {
	deltas, err := f.cache.FlushViews(ctx)
	if err != nil {
		f.log.Error("flush_views_failed", "error", err)
		return
	}
	if len(deltas) == 0 {
		return
	}

	var succeeded, failed int
	for videoID, delta := range deltas {
		if err := f.videos.AddViewCount(ctx, videoID, delta); err != nil {
			f.log.Warn("flush_view_count_failed", "videoId", videoID, "delta", delta, "error", err)
			failed++
			continue
		}
		succeeded++
	}

	f.log.Info("view_flushed", "videos", len(deltas), "succeeded", succeeded, "failed", failed)
}
