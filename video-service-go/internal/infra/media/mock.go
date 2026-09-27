package media

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/video-service-go/internal/infra/storage"
)

// MockTranscoder 是不依赖外部二进制的转码器实现。
//
// 它会在 MinIO 中生成结构完全正确的 HLS 目录（master.m3u8 + 各档 index.m3u8），
// 但 index.m3u8 里没有真实分片 —— 目的是让「上传 → 转码 → 发布 → 取播放地址」
// 这条业务链路在没有 FFmpeg 的环境下也能完整跑通与验收。
//
// 需要真实可播放的产物时，把 transcoder.driver 换成 real。
type MockTranscoder struct {
	store *storage.Storage
	log   *slog.Logger
}

// Name 返回驱动名。
func (t *MockTranscoder) Name() string { return "mock" }

// stepDelay 是模拟每一步转码耗时的间隔。
const stepDelay = 150 * time.Millisecond

// stepsPerProfile 是每个档位模拟的进度步数。
const stepsPerProfile = 5

// Transcode 生成占位 HLS 产物。
func (t *MockTranscoder) Transcode(ctx context.Context, req Request, progress func(int)) (*Result, error) {
	prefix := HLSObjectPrefix(req.VideoID)
	total := len(DefaultProfiles) * stepsPerProfile

	for i, p := range DefaultProfiles {
		for step := 1; step <= stepsPerProfile; step++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(stepDelay):
			}
			if progress != nil {
				progress((i*stepsPerProfile + step) * 100 / total)
			}
		}

		key := fmt.Sprintf("%s/%s/index.m3u8", prefix, p.Name)
		if err := t.store.PutBytes(ctx, key, []byte(placeholderIndexPlaylist()), "application/vnd.apple.mpegurl"); err != nil {
			return nil, fmt.Errorf("写入占位播放列表 %s 失败: %w", key, err)
		}
	}

	masterKey := prefix + "/master.m3u8"
	master := buildMasterPlaylist(func(p Profile) string { return p.Name + "/index.m3u8" })
	if err := t.store.PutBytes(ctx, masterKey, []byte(master), "application/vnd.apple.mpegurl"); err != nil {
		return nil, fmt.Errorf("写入主索引 %s 失败: %w", masterKey, err)
	}

	sources := make([]Source, 0, len(DefaultProfiles))
	for _, p := range DefaultProfiles {
		sources = append(sources, Source{
			Profile:   p.Name,
			Object:    fmt.Sprintf("%s/%s/index.m3u8", prefix, p.Name),
			Width:     p.Width,
			Height:    p.Height,
			Bandwidth: p.Bandwidth(),
		})
	}

	if progress != nil {
		progress(100)
	}
	t.log.Info("mock_transcode_done", "videoId", req.VideoID, "masterObject", masterKey)

	return &Result{
		MasterObject: masterKey,
		Duration:     estimateDuration(req.SizeBytes),
		Sources:      sources,
	}, nil
}

// placeholderIndexPlaylist 返回一个合法但无分片的 VOD 播放列表。
func placeholderIndexPlaylist() string {
	return "#EXTM3U\n" +
		"#EXT-X-VERSION:3\n" +
		"#EXT-X-TARGETDURATION:6\n" +
		"#EXT-X-MEDIA-SEQUENCE:0\n" +
		"#EXT-X-PLAYLIST-TYPE:VOD\n" +
		"#EXT-X-ENDLIST\n"
}

// estimateDuration 用文件体积粗略估算时长（约 256KB/s），保证 mock 产物有合理时长。
func estimateDuration(sizeBytes int64) int {
	const bytesPerSecond = 256 * 1024
	if sizeBytes <= 0 {
		return 10
	}
	seconds := int(sizeBytes / bytesPerSecond)
	if seconds < 10 {
		return 10
	}
	return seconds
}
