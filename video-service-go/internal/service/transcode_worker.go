package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/config"
	"github.com/example/video-service-go/internal/infra/cache"
	"github.com/example/video-service-go/internal/infra/events"
	"github.com/example/video-service-go/internal/infra/media"
	"github.com/example/video-service-go/internal/infra/storage"
	"github.com/example/video-service-go/internal/model"
	"github.com/example/video-service-go/internal/repository"
	"github.com/example/video-service-go/pkg/trace"
)

// idemKeyPrefix 是消息幂等键前缀。
const idemKeyPrefix = "mq:idem:"

// maxMessageLen 是入库的失败原因长度上限（对应 video_transcode_task.message 的长度）。
const maxMessageLen = 500

type eventIdempotency interface {
	ClaimEvent(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	ReleaseEvent(ctx context.Context, key string) error
}

// TranscodeWorker 消费 video.uploaded 事件并执行转码。
//
// 与 API 进程同进程运行，便于阶段一单机演示；
// 阶段二可直接以 cmd/worker 独立部署（共用同一份 internal 代码）。
type TranscodeWorker struct {
	cfg         *config.Config
	transcodes  repository.TranscodeRepository
	bus         events.Bus
	store       *storage.Storage
	transcoder  media.Transcoder
	cache       *cache.VideoCache
	idempotency eventIdempotency
	log         *slog.Logger
	sem         chan struct{}
}

// NewTranscodeWorker 构造转码 Worker。
func NewTranscodeWorker(
	cfg *config.Config,
	transcodes repository.TranscodeRepository,
	bus events.Bus,
	store *storage.Storage,
	transcoder media.Transcoder,
	videoCache *cache.VideoCache,
	log *slog.Logger,
) *TranscodeWorker {
	return &TranscodeWorker{
		cfg:         cfg,
		transcodes:  transcodes,
		bus:         bus,
		store:       store,
		transcoder:  transcoder,
		cache:       videoCache,
		idempotency: videoCache,
		log:         log,
		sem:         make(chan struct{}, cfg.Transcoder.Concurrency),
	}
}

// Register 把自身注册为 video.uploaded 的消费者。
func (w *TranscodeWorker) Register() error {
	return w.bus.Subscribe(events.RoutingVideoUploaded, w.Handle)
}

// Handle 处理一条 video.uploaded 事件。
//
// 幂等策略：先用 Redis SETNX 抢占 eventId；只有「成功」才保留键。
// 失败时释放键，从而允许 MQ 的重投真正重新执行（否则重试会被幂等挡掉，形同虚设）。
func (w *TranscodeWorker) Handle(ctx context.Context, ev events.Event) error {
	var payload dto.VideoUploadedPayload
	if err := ev.Decode(&payload); err != nil {
		w.log.Error("transcode_event_decode_failed", "eventId", ev.EventID, "error", err)
		return err
	}

	claimed, err := w.claim(ctx, ev.EventID)
	if err != nil {
		return err
	}
	if !claimed {
		w.log.Info("transcode_event_duplicated", "eventId", ev.EventID, "videoId", payload.VideoID)
		return nil
	}

	succeeded := false
	defer func() {
		if !succeeded {
			w.release(context.WithoutCancel(ctx), ev.EventID)
		}
	}()

	// 并发闸门：避免多个大文件同时转码打满 CPU
	w.sem <- struct{}{}
	defer func() { <-w.sem }()

	if err := w.run(ctx, payload); err != nil {
		w.log.Error("transcode_failed",
			"videoId", payload.VideoID, "taskId", payload.TaskID, "eventId", ev.EventID, "error", err)
		w.markFailed(context.WithoutCancel(ctx), payload, err)
		return err
	}

	succeeded = true
	return nil
}

func (w *TranscodeWorker) run(ctx context.Context, payload dto.VideoUploadedPayload) error {
	startedAt := time.Now()
	if err := w.transcodes.UpdateFields(ctx, payload.TaskID, map[string]any{
		"status":     model.TranscodeStatusRunning,
		"progress":   0,
		"message":    "",
		"started_at": &startedAt,
	}); err != nil {
		return err
	}

	reporter := newProgressReporter(w.transcodes, payload.TaskID, w.log)

	result, err := w.transcoder.Transcode(ctx, media.Request{
		VideoID:      payload.VideoID,
		SourceObject: payload.SourceObject,
		SizeBytes:    payload.SizeBytes,
	}, reporter.report)
	reporter.flush()
	if err != nil {
		return err
	}

	playbackURL := w.store.URL(result.MasterObject)
	finishedAt := time.Now()

	// 跨表终态写入放在同一事务：避免出现「task=SUCCESS 但 video 仍为 TRANSCODING」
	err = w.transcodes.Complete(ctx, payload.VideoID, payload.TaskID, playbackURL, result.Duration, finishedAt)
	if err != nil {
		return err
	}

	w.cache.DelDetail(ctx, payload.VideoID)
	w.publishTranscoded(ctx, payload, result, playbackURL, "")

	w.log.Info("transcode_succeeded",
		"videoId", payload.VideoID,
		"taskId", payload.TaskID,
		"duration", result.Duration,
		"masterObject", result.MasterObject,
		"costMs", time.Since(startedAt).Milliseconds(),
	)
	return nil
}

func (w *TranscodeWorker) markFailed(ctx context.Context, payload dto.VideoUploadedPayload, cause error) {
	finishedAt := time.Now()
	message := truncateMessage(cause.Error())

	err := w.transcodes.MarkFailed(ctx, payload.VideoID, payload.TaskID, message, finishedAt)
	if err != nil {
		w.log.Error("mark_transcode_failed_error", "videoId", payload.VideoID, "error", err)
	}

	w.cache.DelDetail(ctx, payload.VideoID)
	w.publishTranscoded(ctx, payload, nil, "", message)
}

// publishTranscoded 发布转码完成事件。
// 该事件失败只告警，不影响转码结果本身（阶段二会接入通知 / 统计消费者）。
func (w *TranscodeWorker) publishTranscoded(
	ctx context.Context,
	payload dto.VideoUploadedPayload,
	result *media.Result,
	playbackURL, message string,
) {
	out := dto.VideoTranscodedPayload{
		VideoID:     payload.VideoID,
		TaskID:      payload.TaskID,
		Success:     result != nil,
		PlaybackURL: playbackURL,
		Message:     message,
	}
	if result != nil {
		out.Duration = result.Duration
	}

	ev, err := events.New(events.RoutingVideoTranscoded, trace.From(ctx), out)
	if err != nil {
		w.log.Error("build_video_transcoded_event_failed", "error", err)
		return
	}
	if err := w.bus.Publish(ctx, events.RoutingVideoTranscoded, ev); err != nil {
		w.log.Warn("publish_video_transcoded_failed", "videoId", payload.VideoID, "error", err)
	}
}

func (w *TranscodeWorker) claim(ctx context.Context, eventID string) (bool, error) {
	ok, err := w.idempotency.ClaimEvent(ctx, idemKeyPrefix+eventID, "1", w.cfg.Events.IdempotentTTL)
	if err != nil {
		// Redis 故障时不放行：宁可重试，也不要重复转码
		return false, err
	}
	return ok, nil
}

func (w *TranscodeWorker) release(ctx context.Context, eventID string) {
	if err := w.idempotency.ReleaseEvent(ctx, idemKeyPrefix+eventID); err != nil {
		w.log.Warn("idempotent_release_failed", "eventId", eventID, "error", err)
	}
}

// ---------------------------------------------------------------- 进度上报

// progressReporter 把转码进度节流写入数据库。
// 转码器可能每秒回调多次，直接写库会把 task 行写热，因此按 5% 步长节流。
type progressReporter struct {
	transcodes repository.TranscodeRepository
	taskID     uint64
	log        *slog.Logger

	mu    sync.Mutex
	last  int
	dirty bool
}

func newProgressReporter(transcodes repository.TranscodeRepository, taskID uint64, log *slog.Logger) *progressReporter {
	return &progressReporter{transcodes: transcodes, taskID: taskID, last: -1, log: log}
}

func (p *progressReporter) report(pct int) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	p.mu.Lock()
	if pct-p.last < 5 && pct != 100 {
		p.mu.Unlock()
		return
	}
	p.last = pct
	p.dirty = true
	p.mu.Unlock()

	p.flush()
}

// flush 把当前进度落库。
func (p *progressReporter) flush() {
	p.mu.Lock()
	if !p.dirty {
		p.mu.Unlock()
		return
	}
	pct := p.last
	p.dirty = false
	p.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := p.transcodes.UpdateProgress(ctx, p.taskID, pct); err != nil {
		p.log.Warn("update_transcode_progress_failed", "taskId", p.taskID, "progress", pct, "error", err)
	}
}

func truncateMessage(s string) string {
	if len(s) <= maxMessageLen {
		return s
	}
	return s[:maxMessageLen]
}
