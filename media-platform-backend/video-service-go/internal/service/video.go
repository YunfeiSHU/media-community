// Package service 承载业务规则、状态机与事务边界。
//
// 约束：本包不感知 *gin.Context（HTTP 细节留在 handler），只依赖 context.Context。
package service

import (
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/config"
	"github.com/example/video-service-go/internal/infra/cache"
	"github.com/example/video-service-go/internal/infra/events"
	"github.com/example/video-service-go/internal/infra/media"
	"github.com/example/video-service-go/internal/infra/storage"
	"github.com/example/video-service-go/internal/model"
	"github.com/example/video-service-go/internal/repository"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/identity"
	"github.com/example/video-service-go/pkg/trace"
)

// 领域约束常量。
const (
	maxTitleLen       = 128
	maxDescriptionLen = 1024
	maxTagLen         = 32
	maxTagCount       = 10
	defaultPageSize   = 20
	maxPageSize       = 100
)

// VideoService 是点播域的核心业务实现。
type VideoService struct {
	cfg     *config.Config
	videos  repository.VideoRepository
	cats    repository.CategoryRepository
	tasks   repository.TranscodeRepository
	uploads repository.UploadSessionRepository
	store   *storage.Storage
	bus     events.Bus
	cache   *cache.VideoCache
	log     *slog.Logger
}

// NewVideoService 构造视频服务。
func NewVideoService(
	cfg *config.Config,
	videos repository.VideoRepository,
	cats repository.CategoryRepository,
	tasks repository.TranscodeRepository,
	uploads repository.UploadSessionRepository,
	store *storage.Storage,
	bus events.Bus,
	videoCache *cache.VideoCache,
	log *slog.Logger,
) *VideoService {
	return &VideoService{
		cfg:     cfg,
		videos:  videos,
		cats:    cats,
		tasks:   tasks,
		uploads: uploads,
		store:   store,
		bus:     bus,
		cache:   videoCache,
		log:     log,
	}
}

// ---------------------------------------------------------------- 写操作

// CreateVideo 创建视频草稿。
func (s *VideoService) CreateVideo(ctx context.Context, u identity.CurrentUser, req dto.CreateVideoRequest) (*dto.CreateVideoResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errs.CodeInvalidParam.New("title 不能为空")
	}
	if utf8.RuneCountInString(title) > maxTitleLen {
		return nil, errs.CodeInvalidParam.Newf("title 长度不能超过 %d 个字符", maxTitleLen)
	}

	description := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(description) > maxDescriptionLen {
		return nil, errs.CodeInvalidParam.Newf("description 长度不能超过 %d 个字符", maxDescriptionLen)
	}

	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return nil, err
	}

	coverURL, err := NormalizeCoverURL(req.CoverURL)
	if err != nil {
		return nil, err
	}

	if req.CategoryID != nil {
		if err := s.ensureCategoryExists(ctx, *req.CategoryID); err != nil {
			return nil, err
		}
	}

	video := &model.Video{
		Title:       title,
		Description: description,
		OwnerID:     u.UID,
		CategoryID:  req.CategoryID,
		CoverURL:    coverURL,
		Status:      model.VideoStatusDraft,
	}
	if err := s.videos.Create(ctx, video); err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		if err := s.videos.ReplaceTags(ctx, video.ID, tags); err != nil {
			return nil, err
		}
	}

	s.log.Info("video_created", "videoId", video.ID, "uid", u.UID)
	return &dto.CreateVideoResult{VideoID: video.ID, Status: string(video.Status)}, nil
}

// UpdateVideo 修改视频元数据（仅创建者）。
func (s *VideoService) UpdateVideo(ctx context.Context, u identity.CurrentUser, id uint64, req dto.UpdateVideoRequest) (*dto.UpdateVideoResult, error) {
	if _, err := s.mustGetOwned(ctx, u, id); err != nil {
		return nil, err
	}

	fields := make(map[string]any)

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, errs.CodeInvalidParam.New("title 不能为空")
		}
		if utf8.RuneCountInString(title) > maxTitleLen {
			return nil, errs.CodeInvalidParam.Newf("title 长度不能超过 %d 个字符", maxTitleLen)
		}
		fields["title"] = title
	}

	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if utf8.RuneCountInString(description) > maxDescriptionLen {
			return nil, errs.CodeInvalidParam.Newf("description 长度不能超过 %d 个字符", maxDescriptionLen)
		}
		fields["description"] = description
	}

	if req.CategoryID != nil {
		if err := s.ensureCategoryExists(ctx, *req.CategoryID); err != nil {
			return nil, err
		}
		fields["category_id"] = *req.CategoryID
	}

	if req.CoverURL != nil {
		coverURL, err := NormalizeCoverURL(*req.CoverURL)
		if err != nil {
			return nil, err
		}
		fields["cover_url"] = coverURL
	}

	if len(fields) > 0 {
		if err := s.videos.UpdateFields(ctx, id, fields); err != nil {
			return nil, err
		}
	}

	if req.Tags != nil {
		tags, err := normalizeTags(*req.Tags)
		if err != nil {
			return nil, err
		}
		if err := s.videos.ReplaceTags(ctx, id, tags); err != nil {
			return nil, err
		}
	}

	s.cache.DelDetail(ctx, id)
	return &dto.UpdateVideoResult{VideoID: id, Updated: true}, nil
}

// DeleteVideo 软删除视频（仅创建者），并异步清理对象存储产物。
func (s *VideoService) DeleteVideo(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.DeleteVideoResult, error) {
	if _, err := s.mustGetOwned(ctx, u, id); err != nil {
		return nil, err
	}

	if err := s.videos.Delete(ctx, id); err != nil {
		return nil, err
	}
	s.cache.DelDetail(ctx, id)

	// 对象存储清理失败不应影响删除结果：用独立 context，避免请求结束时被取消
	go s.cleanupObjects(context.WithoutCancel(ctx), id)

	s.log.Info("video_deleted", "videoId", id, "uid", u.UID)
	return &dto.DeleteVideoResult{VideoID: id, Deleted: true}, nil
}

// Upload 一次性上传原片并投递转码事件（小文件走这条，大文件请用分片上传）。
func (s *VideoService) Upload(ctx context.Context, u identity.CurrentUser, id uint64, fileHeader *multipart.FileHeader) (*dto.UploadResult, error) {
	video, err := s.mustGetOwned(ctx, u, id)
	if err != nil {
		return nil, err
	}

	maxBytes := int64(s.cfg.Upload.MaxSizeMB) * 1024 * 1024
	if fileHeader.Size > maxBytes {
		return nil, errs.CodeFileTooLarge.Newf("文件大小超过限制: %dMB", s.cfg.Upload.MaxSizeMB)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileHeader.Filename), "."))
	if !s.allowedExt(ext) {
		return nil, errs.CodeUnsupportedFileType.Newf("不支持的文件类型: %s", ext)
	}

	// 幂等：已在上传/转码中，直接返回当前任务
	if video.Status == model.VideoStatusTranscoding {
		if result, ok := s.reuseTranscodingTask(ctx, video); ok {
			return result, nil
		}
	}
	if !video.Status.CanUpload() {
		return nil, errs.CodeVideoStatusConflict.Newf("当前状态 %s 不允许上传", video.Status)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, errs.CodeInvalidParam.New("读取上传文件失败")
	}
	defer func() { _ = file.Close() }()

	objectKey := media.OriginalObjectKey(id, ext)
	if err := s.store.Put(ctx, objectKey, file, fileHeader.Size, contentTypeOfExt(ext)); err != nil {
		s.log.Error("upload_object_failed", "videoId", id, "error", err)
		return nil, errs.CodeDependencyDown.New("上传到对象存储失败")
	}

	return s.submitTranscode(ctx, video, objectKey, ext, fileHeader.Size)
}

// Publish 发布视频（仅创建者）。
func (s *VideoService) Publish(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.PublishResult, error) {
	video, err := s.mustGetOwned(ctx, u, id)
	if err != nil {
		return nil, err
	}
	if !video.Status.CanPublish() {
		return nil, errs.CodeVideoStatusConflict.Newf("当前状态 %s 不允许发布", video.Status)
	}
	if video.PlaybackURL == "" {
		return nil, errs.CodeVideoStatusConflict.New("转码产物不存在，无法发布")
	}

	now := time.Now()
	if err := s.videos.UpdateFields(ctx, id, map[string]any{
		"status":       model.VideoStatusPublished,
		"published_at": &now,
	}); err != nil {
		return nil, err
	}
	s.cache.DelDetail(ctx, id)

	s.log.Info("video_published", "videoId", id, "uid", u.UID)
	return &dto.PublishResult{
		VideoID:     id,
		Status:      string(model.VideoStatusPublished),
		PublishedAt: &now,
	}, nil
}

// Offline 下架视频（创建者或 ADMIN）。
func (s *VideoService) Offline(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.PublishResult, error) {
	if !u.Authenticated() {
		return nil, errs.CodeUnauthorized.Plain()
	}

	video, err := s.videos.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	// 治理动作允许 ADMIN 越权，但普通用户只能操作自己的视频
	if !u.Owns(video.OwnerID) && !u.IsAdmin() {
		return nil, errs.CodeForbidden.New("只能下架自己创建的视频")
	}
	if video.Status != model.VideoStatusPublished {
		return nil, errs.CodeVideoStatusConflict.Newf("当前状态 %s 不允许下架", video.Status)
	}

	if err := s.videos.UpdateFields(ctx, id, map[string]any{"status": model.VideoStatusOffline}); err != nil {
		return nil, err
	}
	s.cache.DelDetail(ctx, id)

	s.log.Info("video_offline", "videoId", id, "operator", u.UID)
	return &dto.PublishResult{VideoID: id, Status: string(model.VideoStatusOffline)}, nil
}

// RetryTranscode 重建转码任务并重新投递事件（仅创建者，且状态为 FAILED）。
func (s *VideoService) RetryTranscode(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.TranscodeInfo, error) {
	video, err := s.mustGetOwned(ctx, u, id)
	if err != nil {
		return nil, err
	}
	if video.Status != model.VideoStatusFailed {
		return nil, errs.CodeVideoStatusConflict.Newf("当前状态 %s 不允许重试转码", video.Status)
	}
	if video.SourceObject == "" {
		return nil, errs.CodeVideoStatusConflict.New("原片不存在，请重新上传")
	}

	task := &model.TranscodeTask{
		VideoID:  id,
		Status:   model.TranscodeStatusPending,
		Profiles: media.JoinProfileNames(),
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, err
	}
	if err := s.videos.UpdateFields(ctx, id, map[string]any{"status": model.VideoStatusTranscoding}); err != nil {
		return nil, err
	}
	s.cache.DelDetail(ctx, id)

	ext := strings.TrimPrefix(filepath.Ext(video.SourceObject), ".")
	event, err := events.New(events.RoutingVideoUploaded, trace.From(ctx), dto.VideoUploadedPayload{
		VideoID:      id,
		OwnerID:      video.OwnerID,
		SourceObject: video.SourceObject,
		Ext:          ext,
		SizeBytes:    video.SizeBytes,
		TaskID:       task.ID,
	})
	if err != nil {
		return nil, err
	}
	if err := s.bus.Publish(ctx, events.RoutingVideoUploaded, event); err != nil {
		return nil, errs.CodeTranscodeSubmitFail.New("投递转码事件失败")
	}
	if err := s.tasks.UpdateFields(ctx, task.ID, map[string]any{"event_id": event.EventID}); err != nil {
		s.log.Warn("save_event_id_failed", "taskId", task.ID, "error", err)
	}

	s.log.Info("video_transcode_retried", "videoId", id, "taskId", task.ID)
	return toTranscodeInfo(id, task), nil
}

// ---------------------------------------------------------------- 读操作

// GetVideo 查询视频详情（带缓存）。
func (s *VideoService) GetVideo(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.VideoDetail, error) {
	if cached, hit := s.cache.GetDetail(ctx, id); hit {
		if cached == nil {
			return nil, errs.CodeNotFound.Plain()
		}
		if !s.canView(u, cached.OwnerID, model.VideoStatus(cached.Status)) {
			return nil, errs.CodeNotFound.Plain()
		}
		return maskForViewer(u, cached), nil
	}

	video, err := s.videos.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			s.cache.SetNil(ctx, id)
			return nil, errs.CodeNotFound.Plain()
		}
		return nil, err
	}
	if !s.canView(u, video.OwnerID, video.Status) {
		return nil, errs.CodeNotFound.Plain()
	}

	detail, err := s.buildDetail(ctx, video)
	if err != nil {
		return nil, err
	}
	// 缓存完整详情（含只有所有者可见的字段），返回前按调用者裁剪
	s.cache.SetDetail(ctx, id, detail)
	return maskForViewer(u, detail), nil
}

// ListVideos 分页查询视频列表。
func (s *VideoService) ListVideos(ctx context.Context, u identity.CurrentUser, q dto.ListQuery) ([]dto.VideoBrief, int64, error) {
	page, pageSize := normalizePaging(q.Page, q.PageSize)

	filter := repository.VideoFilter{
		Keyword:    strings.TrimSpace(q.Keyword),
		CategoryID: q.CategoryID,
		Offset:     (page - 1) * pageSize,
		Limit:      pageSize,
	}

	switch {
	case q.Favorite:
		// 收藏列表只返回「当前登录用户」的收藏；没有 favoriteUid 这类参数，
		// 因此无法借此查看别人的收藏。
		if !u.Authenticated() {
			return nil, 0, errs.CodeUnauthorized.Plain()
		}
		filter.FavoriteUID = u.UID
	case q.Mine:
		if !u.Authenticated() {
			return nil, 0, errs.CodeUnauthorized.Plain()
		}
		filter.OwnerID = u.UID
	case q.OwnerID != "":
		filter.OwnerID = strings.TrimSpace(q.OwnerID)
	}

	// 只有「查自己」或「管理员」才能看到非公开状态；其余一律强制只看 PUBLISHED
	canSeeAll := u.IsAdmin() || (u.Authenticated() && filter.OwnerID == u.UID)
	if canSeeAll {
		filter.Status = strings.TrimSpace(q.Status)
	} else {
		filter.OnlyPublished = true
	}

	items, total, err := s.videos.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 {
		return []dto.VideoBrief{}, total, nil
	}

	ids := make([]uint64, 0, len(items))
	categoryIDs := make([]uint64, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
		if items[i].CategoryID != nil {
			categoryIDs = append(categoryIDs, *items[i].CategoryID)
		}
	}

	categoryNames, err := s.cats.NamesByIDs(ctx, categoryIDs)
	if err != nil {
		s.log.Warn("load_category_names_failed", "error", err)
		categoryNames = map[uint64]string{}
	}
	tagMap, err := s.videos.TagsByVideoIDs(ctx, ids)
	if err != nil {
		s.log.Warn("load_tags_failed", "error", err)
		tagMap = map[uint64][]string{}
	}

	briefs := make([]dto.VideoBrief, 0, len(items))
	for i := range items {
		briefs = append(briefs, toBrief(&items[i], categoryNames, tagMap))
	}
	return briefs, total, nil
}

// Play 获取播放信息并累加播放量。
func (s *VideoService) Play(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.PlayInfo, error) {
	detail, err := s.GetVideo(ctx, u, id)
	if err != nil {
		return nil, err
	}
	if detail.PlaybackURL == "" {
		return nil, errs.CodeVideoStatusConflict.Newf("视频尚未转码完成，当前状态 %s", detail.Status)
	}

	// 播放量先进 Redis 缓冲，由 ViewFlusher 定时回写 MySQL
	s.cache.IncrView(ctx, id)

	return &dto.PlayInfo{
		VideoID:     id,
		Title:       detail.Title,
		CoverURL:    detail.CoverURL,
		Duration:    detail.Duration,
		Format:      "hls",
		PlaybackURL: detail.PlaybackURL,
		Sources:     s.buildSources(id),
	}, nil
}

// TranscodeStatus 查询转码任务状态（创建者或 ADMIN）。
func (s *VideoService) TranscodeStatus(ctx context.Context, u identity.CurrentUser, id uint64) (*dto.TranscodeInfo, error) {
	if !u.Authenticated() {
		return nil, errs.CodeUnauthorized.Plain()
	}

	video, err := s.videos.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	if !u.Owns(video.OwnerID) && !u.IsAdmin() {
		return nil, errs.CodeForbidden.New("只能查看自己视频的转码状态")
	}

	task, err := s.tasks.LatestByVideoID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, errs.CodeTaskNotFound.Plain()
		}
		return nil, err
	}
	return toTranscodeInfo(id, task), nil
}

// ---------------------------------------------------------------- 内部方法

// canView 判断当前调用者能否看到该视频。
func (s *VideoService) canView(u identity.CurrentUser, ownerID string, status model.VideoStatus) bool {
	if status.VisibleToAll() {
		return true
	}
	return u.Owns(ownerID) || u.IsAdmin()
}

func (s *VideoService) mustGetOwned(ctx context.Context, u identity.CurrentUser, id uint64) (*model.Video, error) {
	video, err := s.videos.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	if err := u.RequireOwner(video.OwnerID); err != nil {
		return nil, err
	}
	return video, nil
}

func (s *VideoService) ensureCategoryExists(ctx context.Context, id uint64) error {
	if _, err := s.cats.GetByID(ctx, id); err != nil {
		if repository.IsNotFound(err) {
			return errs.CodeCategoryNotFound.Plain()
		}
		return err
	}
	return nil
}

func (s *VideoService) buildDetail(ctx context.Context, video *model.Video) (*dto.VideoDetail, error) {
	categoryNames := map[uint64]string{}
	if video.CategoryID != nil {
		names, err := s.cats.NamesByIDs(ctx, []uint64{*video.CategoryID})
		if err != nil {
			s.log.Warn("load_category_names_failed", "error", err)
		} else {
			categoryNames = names
		}
	}

	tagMap, err := s.videos.TagsByVideoIDs(ctx, []uint64{video.ID})
	if err != nil {
		s.log.Warn("load_tags_failed", "error", err)
		tagMap = map[uint64][]string{}
	}

	return &dto.VideoDetail{
		VideoBrief:   toBrief(video, categoryNames, tagMap),
		PlaybackURL:  video.PlaybackURL,
		SizeBytes:    video.SizeBytes,
		SourceObject: video.SourceObject,
	}, nil
}

// buildSources 由档位配置推导各码率播放地址。
//
// 之所以「推导」而不是「存库」：转码是全档位成功才置 READY 的，
// 档位定义来自同一份 media.DefaultProfiles，推导结果与实际产物天然一致，
// 省掉一张关联表与一次查询。
func (s *VideoService) buildSources(videoID uint64) []dto.PlaySource {
	prefix := media.HLSObjectPrefix(videoID)
	sources := make([]dto.PlaySource, 0, len(media.DefaultProfiles))
	for _, p := range media.DefaultProfiles {
		sources = append(sources, dto.PlaySource{
			Profile:   p.Name,
			URL:       s.store.URL(fmt.Sprintf("%s/%s/index.m3u8", prefix, p.Name)),
			Width:     p.Width,
			Height:    p.Height,
			Bandwidth: p.Bandwidth(),
		})
	}
	return sources
}

func (s *VideoService) allowedExt(ext string) bool {
	for _, allowed := range s.cfg.Upload.AllowedExt {
		if strings.EqualFold(strings.TrimSpace(allowed), ext) {
			return true
		}
	}
	return false
}

func (s *VideoService) cleanupObjects(ctx context.Context, videoID uint64) {
	prefixes := []string{
		fmt.Sprintf("video/original/%d", videoID),
		media.HLSObjectPrefix(videoID),
		fmt.Sprintf("video/cover/%d", videoID),
	}
	for _, prefix := range prefixes {
		if err := s.store.RemovePrefix(ctx, prefix); err != nil {
			s.log.Warn("cleanup_objects_failed", "videoId", videoID, "prefix", prefix, "error", err)
		}
	}
}

// ---------------------------------------------------------------- 辅助函数

// maskForViewer 按调用者裁剪字段：sourceObject 只对所有者 / ADMIN 可见。
func maskForViewer(u identity.CurrentUser, detail *dto.VideoDetail) *dto.VideoDetail {
	if u.Owns(detail.OwnerID) || u.IsAdmin() {
		return detail
	}
	masked := *detail
	masked.SourceObject = ""
	return &masked
}

func toBrief(video *model.Video, categoryNames map[uint64]string, tagMap map[uint64][]string) dto.VideoBrief {
	brief := dto.VideoBrief{
		ID:          video.ID,
		Title:       video.Title,
		Description: video.Description,
		OwnerID:     video.OwnerID,
		CategoryID:  video.CategoryID,
		CoverURL:    video.CoverURL,
		Duration:    video.Duration,
		Status:      string(video.Status),
		ViewCount:   video.ViewCount,
		PublishedAt: video.PublishedAt,
		CreatedAt:   video.CreatedAt,
		UpdatedAt:   video.UpdatedAt,
		Tags:        []string{},
	}
	if video.CategoryID != nil {
		brief.CategoryName = categoryNames[*video.CategoryID]
	}
	if tags, ok := tagMap[video.ID]; ok && len(tags) > 0 {
		brief.Tags = tags
	}
	return brief
}

func toTranscodeInfo(videoID uint64, task *model.TranscodeTask) *dto.TranscodeInfo {
	return &dto.TranscodeInfo{
		VideoID:    videoID,
		TaskID:     task.ID,
		Status:     string(task.Status),
		Progress:   task.Progress,
		Profiles:   media.SplitProfileNames(task.Profiles),
		Message:    task.Message,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
	}
}

func mapNotFound(err error) error {
	if repository.IsNotFound(err) {
		return errs.CodeNotFound.Plain()
	}
	return err
}

func normalizeTags(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, tag := range raw {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > maxTagLen {
			return nil, errs.CodeInvalidParam.Newf("标签 %s 长度不能超过 %d 个字符", tag, maxTagLen)
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}

	if len(out) > maxTagCount {
		return nil, errs.CodeInvalidParam.Newf("标签数量不能超过 %d 个", maxTagCount)
	}
	return out, nil
}

func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// NormalizePaging 暴露给 handler 用于回填分页信息。
func NormalizePaging(page, pageSize int) (int, int) { return normalizePaging(page, pageSize) }

func contentTypeOfExt(ext string) string {
	switch strings.ToLower(ext) {
	case "mp4":
		return "video/mp4"
	case "mov":
		return "video/quicktime"
	case "mkv":
		return "video/x-matroska"
	case "avi":
		return "video/x-msvideo"
	case "flv":
		return "video/x-flv"
	case "webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}
