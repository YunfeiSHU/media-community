package media

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/example/video-service-go/internal/config"
	"github.com/example/video-service-go/internal/infra/storage"
)

// FFmpegTranscoder 调用本机 ffmpeg 生成多码率 HLS。
//
// 前置：机器上已安装 ffmpeg 且可执行（Windows 上需加入 PATH，或配置 transcoder.ffmpeg_path 绝对路径）。
type FFmpegTranscoder struct {
	cfg   config.Transcoder
	store *storage.Storage
	log   *slog.Logger
}

// Name 返回驱动名。
func (t *FFmpegTranscoder) Name() string { return "real" }

// Transcode 下载原片 → 逐档转码 → 上传 HLS 产物。
func (t *FFmpegTranscoder) Transcode(ctx context.Context, req Request, progress func(int)) (*Result, error) {
	if _, err := exec.LookPath(t.cfg.FFmpegPath); err != nil {
		return nil, fmt.Errorf("找不到 ffmpeg 可执行文件 %q，请安装或改用 transcoder.driver=mock: %w", t.cfg.FFmpegPath, err)
	}

	workDir := filepath.Join(t.cfg.WorkDir, fmt.Sprintf("video-%d", req.VideoID))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建转码工作目录失败: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(workDir); rmErr != nil {
			t.log.Warn("cleanup_work_dir_failed", "dir", workDir, "error", rmErr)
		}
	}()

	// 1) 下载原片
	ext := filepath.Ext(req.SourceObject)
	if ext == "" {
		ext = ".mp4"
	}
	srcPath := filepath.Join(workDir, "source"+ext)
	t.log.Info("transcode_download_source", "videoId", req.VideoID, "object", req.SourceObject)
	if err := t.store.Download(ctx, req.SourceObject, srcPath); err != nil {
		return nil, err
	}

	// 2) 探测时长（用于进度换算；失败不阻断，只是进度不准）
	totalSec, err := t.probeDuration(ctx, srcPath)
	if err != nil {
		t.log.Warn("ffmpeg_probe_duration_failed", "videoId", req.VideoID, "error", err)
	}

	// 3) 逐档转码
	outDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建 HLS 输出目录失败: %w", err)
	}

	prefix := HLSObjectPrefix(req.VideoID)
	sources := make([]Source, 0, len(DefaultProfiles))

	for i, p := range DefaultProfiles {
		profileDir := filepath.Join(outDir, p.Name)
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return nil, fmt.Errorf("创建档位目录失败: %w", err)
		}

		index := i
		if err := t.runProfile(ctx, srcPath, profileDir, p, totalSec, func(pct int) {
			if progress == nil {
				return
			}
			progress((index*100 + pct) / len(DefaultProfiles))
		}); err != nil {
			return nil, fmt.Errorf("转码档位 %s 失败: %w", p.Name, err)
		}

		sources = append(sources, Source{
			Profile:   p.Name,
			Object:    fmt.Sprintf("%s/%s/index.m3u8", prefix, p.Name),
			Width:     p.Width,
			Height:    p.Height,
			Bandwidth: p.Bandwidth(),
		})
	}

	// 4) 生成主索引
	masterPath := filepath.Join(outDir, "master.m3u8")
	master := buildMasterPlaylist(func(p Profile) string { return p.Name + "/index.m3u8" })
	if err := os.WriteFile(masterPath, []byte(master), 0o644); err != nil {
		return nil, fmt.Errorf("写入主索引失败: %w", err)
	}

	// 5) 上传 HLS 目录
	if err := t.uploadDir(ctx, outDir, prefix); err != nil {
		return nil, err
	}

	if progress != nil {
		progress(100)
	}
	t.log.Info("ffmpeg_transcode_done", "videoId", req.VideoID, "prefix", prefix, "durationSec", totalSec)

	return &Result{
		MasterObject: prefix + "/master.m3u8",
		Duration:     int(totalSec + 0.5),
		Sources:      sources,
	}, nil
}

// runProfile 对单个档位执行一次 ffmpeg，并通过 -progress 解析进度。
func (t *FFmpegTranscoder) runProfile(
	ctx context.Context,
	src, outDir string,
	p Profile,
	totalSec float64,
	onProgress func(int),
) error {
	ctx, cancel := context.WithTimeout(ctx, t.cfg.Timeout)
	defer cancel()

	args := []string{
		"-hide_banner", "-y",
		"-i", src,
		"-vf", fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", p.Width, p.Height),
		"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "main",
		"-b:v", fmt.Sprintf("%dk", p.VideoKbps),
		"-maxrate", fmt.Sprintf("%dk", p.VideoKbps*11/10),
		"-bufsize", fmt.Sprintf("%dk", p.VideoKbps*2),
		"-c:a", "aac", "-b:a", fmt.Sprintf("%dk", p.AudioKbps), "-ar", "48000", "-ac", "2",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outDir, "%d.ts"),
		"-progress", "pipe:1", "-nostats",
		filepath.Join(outDir, "index.m3u8"),
	}

	cmd := exec.CommandContext(ctx, t.cfg.FFmpegPath, args...)
	cmd.Dir = outDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 ffmpeg stdout 失败: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 ffmpeg 失败: %w", err)
	}

	if totalSec > 0 && onProgress != nil {
		go parseOutTime(stdout, totalSec, onProgress)
	} else {
		go func() { _, _ = io.Copy(io.Discard, stdout) }()
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w (stderr: %s)", err, truncate(stderr.String(), 600))
	}
	return nil
}

// probeDuration 用 ffmpeg 自身的输出解析时长（避免额外依赖 ffprobe）。
func (t *FFmpegTranscoder) probeDuration(ctx context.Context, src string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.cfg.FFmpegPath, "-hide_banner", "-i", src)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// 没有指定输出文件，ffmpeg 必然返回非 0，这里只关心 stderr 里的 Duration 行
	_ = cmd.Run()

	matches := durationPattern.FindStringSubmatch(stderr.String())
	if len(matches) != 4 {
		return 0, fmt.Errorf("未能从 ffmpeg 输出中解析时长")
	}
	hours, _ := strconv.ParseFloat(matches[1], 64)
	minutes, _ := strconv.ParseFloat(matches[2], 64)
	seconds, _ := strconv.ParseFloat(matches[3], 64)
	return hours*3600 + minutes*60 + seconds, nil
}

// uploadDir 把本地 HLS 目录整体上传到对象存储，保持相对路径。
func (t *FFmpegTranscoder) uploadDir(ctx context.Context, localDir, keyPrefix string) error {
	return filepath.WalkDir(localDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		key := keyPrefix + "/" + filepath.ToSlash(rel)

		if err := t.store.PutFile(ctx, key, path, contentTypeOf(path)); err != nil {
			return err
		}
		return nil
	})
}

// durationPattern 匹配 ffmpeg stderr 中的 "Duration: 00:01:23.45"。
var durationPattern = regexp.MustCompile(`Duration:\s*(\d+):(\d+):(\d+\.\d+)`)

// outTimePattern 匹配 `-progress` 输出中的 "out_time=00:01:23.456789"。
const outTimePrefix = "out_time="

// parseOutTime 从 ffmpeg -progress 输出流中解析已处理时长并换算百分比。
func parseOutTime(r io.Reader, totalSec float64, onProgress func(int)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, outTimePrefix) {
			continue
		}
		secs, ok := parseClock(strings.TrimPrefix(line, outTimePrefix))
		if !ok || totalSec <= 0 {
			continue
		}
		pct := int(secs / totalSec * 100)
		if pct < 0 {
			pct = 0
		}
		if pct > 99 {
			pct = 99
		}
		onProgress(pct)
	}
}

// parseClock 解析 "HH:MM:SS.micro" 为秒。
func parseClock(s string) (float64, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 3 {
		return 0, false
	}
	hours, err1 := strconv.ParseFloat(parts[0], 64)
	minutes, err2 := strconv.ParseFloat(parts[1], 64)
	seconds, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return hours*3600 + minutes*60 + seconds, true
}

// contentTypeOf 按扩展名推断 MIME，供 HLS 播放器正确解析。
func contentTypeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".mp4":
		return "video/mp4"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
