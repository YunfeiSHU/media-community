package media

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/example/video-service-go/internal/config"
	"github.com/example/video-service-go/internal/infra/storage"
)

// Request 是一次转码请求。
type Request struct {
	VideoID      uint64
	SourceObject string // 原片在 MinIO 中的 key
	SizeBytes    int64
}

// Source 是单个档位的产物描述。
type Source struct {
	Profile   string
	Object    string // 该档位 index.m3u8 的 key
	Width     int
	Height    int
	Bandwidth int
}

// Result 是转码产物汇总。
type Result struct {
	MasterObject string // master.m3u8 的 key
	Duration     int    // 视频时长（秒）
	Sources      []Source
}

// Transcoder 是转码器抽象。
type Transcoder interface {
	// Name 返回驱动名（mock | real）。
	Name() string
	// Transcode 执行转码，progress 回调参数为 0..100。
	Transcode(ctx context.Context, req Request, progress func(int)) (*Result, error)
}

// New 按配置构造转码器。
func New(cfg config.Transcoder, store *storage.Storage, log *slog.Logger) (Transcoder, error) {
	switch cfg.Driver {
	case "real":
		return &FFmpegTranscoder{cfg: cfg, store: store, log: log}, nil
	case "mock":
		return &MockTranscoder{store: store, log: log}, nil
	default:
		return nil, fmt.Errorf("未知的转码驱动: %s", cfg.Driver)
	}
}

// HLSObjectPrefix 返回某视频 HLS 产物在对象存储中的前缀。
func HLSObjectPrefix(videoID uint64) string {
	return fmt.Sprintf("video/hls/%d", videoID)
}

// OriginalObjectKey 返回原片在对象存储中的 key。
func OriginalObjectKey(videoID uint64, ext string) string {
	return fmt.Sprintf("video/original/%d/source.%s", videoID, ext)
}

// CoverObjectKey 返回封面在对象存储中的 key。
func CoverObjectKey(videoID uint64, ext string) string {
	return fmt.Sprintf("video/cover/%d.%s", videoID, ext)
}
