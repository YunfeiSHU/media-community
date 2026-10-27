package service

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/infra/events"
	"github.com/example/video-service-go/internal/infra/media"
	"github.com/example/video-service-go/internal/infra/storage"
	"github.com/example/video-service-go/internal/model"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/identity"
	"github.com/example/video-service-go/pkg/trace"
)

// 本文件实现大文件的分片上传与断点续传，并承载两条上传入口（一次性 / 分片）共用的收尾逻辑。
//
// 整体设计（与设计文档 §6 对应）：
//
//	init    → 在对象存储开启一次 multipart upload，把 uploadId 落库；返回当前进度快照
//	parts/N → 上传第 N 片；返回最新进度快照
//	status  → 只查进度（刷新页面后据此续传）
//	complete→ 按序合并分片 → 复用「建任务 / 改状态 / 发事件」的收尾逻辑
//	abort   → 取消并回收已上传分片
//
// **断点续传的关键取舍**：已上传分片的权威数据是「对象存储的 ListParts」，
// 服务端不做二次记账。因为分片本来就存在对象存储侧，再维护一份本地进度就等于
// 引入一个必然漂移的副本（网络重试、并发覆盖都会让它和真实状态不一致）。
// 服务端只需持久化 uploadId 这个「入口」，其余随时可以问出来。

// uploadSessionTTL 是分片上传会话的有效期。
//
// 超期后再次 init 会把旧会话 abort 掉并新建，避免对象存储里堆积
// 永远合并不了、又永远不会被列举出来的碎片。
const uploadSessionTTL = 24 * time.Hour

// ---------------------------------------------------------------- 写操作

// InitChunkedUpload 初始化分片上传；若存在参数一致的进行中会话则直接复用它。
//
// 幂等复用是断点续传的前提：客户端刷新页面后重新 init 不应产生第二个 uploadId，
// 否则之前传好的分片就白费了（对象存储里会出现两份互不相干的分片）。
func (s *VideoService) InitChunkedUpload(
	ctx context.Context, u identity.CurrentUser, id uint64, req dto.InitUploadRequest,
) (*dto.UploadSessionResult, error) {
	video, err := s.mustGetOwned(ctx, u, id)
	if err != nil {
		return nil, err
	}

	if req.FileSize <= 0 {
		return nil, errs.CodeInvalidParam.New("fileSize 必须是正整数")
	}
	maxBytes := int64(s.cfg.Upload.MaxSizeMB) * 1024 * 1024
	if req.FileSize > maxBytes {
		return nil, errs.CodeFileTooLarge.Newf("文件大小超过限制: %dMB", s.cfg.Upload.MaxSizeMB)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(req.Filename), "."))
	if ext == "" {
		return nil, errs.CodeInvalidParam.New("filename 必须带扩展名")
	}
	if !s.allowedExt(ext) {
		return nil, errs.CodeUnsupportedFileType.Newf("不支持的文件类型: %s", ext)
	}

	minPartSize := int64(s.cfg.Upload.MinPartSizeMB) * 1024 * 1024
	if req.PartSize > 0 && req.PartSize < minPartSize {
		// 小于 5MiB 的非末片会在合并阶段被对象存储拒绝（EntityTooSmall），
		// 与其让用户在传完几十片后才失败，不如在 init 就挡住。
		return nil, errs.CodeUploadPartInvalid.Newf("partSize 不得小于 %dMB", s.cfg.Upload.MinPartSizeMB)
	}
	partSize := choosePartSize(req.FileSize, req.PartSize, minPartSize, s.cfg.Upload.MaxParts)
	totalParts := totalPartsOf(req.FileSize, partSize)
	if totalParts <= 0 || totalParts > s.cfg.Upload.MaxParts {
		return nil, errs.CodeFileTooLarge.Newf("文件过大，分片数超过上限 %d", s.cfg.Upload.MaxParts)
	}

	// 幂等：状态不允许上传就直接拒绝（DRAFT 首次上传 / FAILED 重传才放行）。
	// 与一次性上传的区别在于：这里没有「复用当前转码任务」这种幂等形态，
	// 因为 init 的返回是上传会话，而转码中的视频并没有会话可返回。
	if !video.Status.CanUpload() {
		return nil, errs.CodeVideoStatusConflict.Newf("当前状态 %s 不允许上传", video.Status)
	}

	objectKey := media.OriginalObjectKey(id, ext)

	sessions, err := s.uploads.ListUploading(ctx, id)
	if err != nil {
		return nil, err
	}
	snapshot, reused, err := s.reuseSession(ctx, sessions, objectKey, ext, req.FileSize, partSize, totalParts)
	if err != nil {
		s.log.Error("list_upload_parts_failed", "videoId", id, "error", err)
		return nil, errs.CodeDependencyDown.New("查询分片进度失败")
	}
	if reused {
		return snapshot, nil
	}
	// 参数对不上或已过期的会话一律回收，否则碎片会一直占着存储空间
	s.abortSessions(ctx, sessions)

	uploadID, err := s.store.NewMultipartUpload(ctx, objectKey, contentTypeOfExt(ext))
	if err != nil {
		s.log.Error("init_multipart_failed", "videoId", id, "object", objectKey, "error", err)
		return nil, errs.CodeDependencyDown.New("初始化分片上传失败")
	}

	session := &model.UploadSession{
		VideoID:    id,
		OwnerID:    video.OwnerID,
		ObjectKey:  objectKey,
		UploadID:   uploadID,
		Ext:        ext,
		FileSize:   req.FileSize,
		PartSize:   partSize,
		TotalParts: totalParts,
		Status:     model.UploadStatusUploading,
	}
	if err := s.uploads.Create(ctx, session); err != nil {
		// 落库失败就把刚开出来的 uploadId 撤掉：否则会产生
		// 「对象存储有分片、服务端不知道 uploadId」的孤儿，谁都清理不了。
		if abortErr := s.store.AbortMultipart(ctx, objectKey, uploadID); abortErr != nil {
			s.log.Warn("abort_orphan_multipart_failed",
				"object", objectKey, "uploadId", uploadID, "error", abortErr)
		}
		return nil, err
	}

	s.log.Info("upload_session_created",
		"videoId", id, "uploadId", uploadID, "object", objectKey,
		"fileSize", req.FileSize, "partSize", partSize, "totalParts", totalParts)

	// 新会话还没有分片，这里查一次是为了走统一的快照构造路径。
	snapshot, err = s.sessionSnapshot(ctx, session)
	if err != nil {
		s.log.Error("list_upload_parts_failed", "videoId", id, "uploadId", uploadID, "error", err)
		return nil, errs.CodeDependencyDown.New("查询分片进度失败")
	}
	return snapshot, nil
}

// UploadChunk 上传单个分片并返回最新进度。
//
// body 是**原始字节流**（不是 multipart 表单）：分片本身就是二进制，
// 再包一层 multipart 只会多一次编解码和一份内存拷贝。
func (s *VideoService) UploadChunk(
	ctx context.Context, u identity.CurrentUser, id uint64, partNumber int, body io.Reader, size int64,
) (*dto.UploadPartResult, error) {
	if _, err := s.mustGetOwned(ctx, u, id); err != nil {
		return nil, err
	}

	session, err := s.currentSession(ctx, id)
	if err != nil {
		return nil, err
	}

	if partNumber < 1 || partNumber > session.TotalParts {
		return nil, errs.CodeUploadPartInvalid.Newf("partNumber 必须在 1..%d 之间", session.TotalParts)
	}
	switch {
	case size <= 0 || size > session.PartSize:
		return nil, errs.CodeUploadPartInvalid.Newf("分片大小必须在 1..%d 字节之间", session.PartSize)
	case partNumber < session.TotalParts && size != session.PartSize:
		// 只有末片可以更小。严格校验的理由：分片边界由服务端在 init 时算好并写进会话，
		// 客户端擅自改大小会让 totalParts 与实际不符，问题会推迟到合并阶段才暴露。
		return nil, errs.CodeUploadPartInvalid.Newf("第 %d 片大小必须为 %d 字节", partNumber, session.PartSize)
	}

	part, err := s.store.PutPart(ctx, session.ObjectKey, session.UploadID, partNumber, body, size)
	if err != nil {
		s.log.Error("upload_part_failed",
			"videoId", id, "uploadId", session.UploadID, "partNumber", partNumber, "error", err)
		return nil, errs.CodeDependencyDown.New("上传分片到对象存储失败")
	}

	snapshot, err := s.sessionSnapshot(ctx, session)
	if err != nil {
		s.log.Warn("list_upload_parts_failed",
			"videoId", id, "uploadId", session.UploadID, "error", err)
		return nil, errs.CodeDependencyDown.New("查询分片进度失败")
	}

	s.log.Info("upload_part_stored",
		"videoId", id, "uploadId", session.UploadID, "partNumber", partNumber,
		"size", part.Size, "uploadedCount", snapshot.UploadedCount, "totalParts", snapshot.TotalParts)

	return &dto.UploadPartResult{
		UploadSessionResult: *snapshot,
		Part: dto.UploadPartDTO{
			PartNumber: part.PartNumber,
			Size:       part.Size,
			ETag:       part.ETag,
		},
	}, nil
}

// CompleteChunkedUpload 合并全部分片，并把原片交给转码链路。
func (s *VideoService) CompleteChunkedUpload(
	ctx context.Context, u identity.CurrentUser, id uint64,
) (*dto.UploadResult, error) {
	video, err := s.mustGetOwned(ctx, u, id)
	if err != nil {
		return nil, err
	}

	session, err := s.currentSession(ctx, id)
	if err != nil {
		return nil, err
	}

	parts, err := s.store.ListParts(ctx, session.ObjectKey, session.UploadID)
	if err != nil {
		return nil, errs.CodeDependencyDown.New("查询分片进度失败")
	}
	if err := checkPartsComplete(session, parts); err != nil {
		return nil, err
	}

	if err := s.store.CompleteMultipart(ctx, session.ObjectKey, session.UploadID, parts); err != nil {
		s.log.Error("complete_multipart_failed",
			"videoId", id, "uploadId", session.UploadID, "error", err)
		return nil, errs.CodeDependencyDown.New("合并分片失败")
	}

	// 以对象存储的实际大小为准：客户端声明的 fileSize 只用于切片计划，
	// 真正落地的产物大小才是应该写进 media_video.size_bytes 的值。
	sizeBytes, err := s.store.StatSize(ctx, session.ObjectKey)
	if err != nil {
		s.log.Warn("stat_merged_object_failed",
			"videoId", id, "object", session.ObjectKey, "error", err)
		sizeBytes = session.FileSize
	}

	if err := s.uploads.UpdateFields(ctx, session.ID, map[string]any{
		"status": model.UploadStatusCompleted,
	}); err != nil {
		// 状态回写失败不影响业务：即使这条会话仍显示 UPLOADING，
		// 后续 init 会因「对象已存在/uploadId 失效」被 abort 掉。
		s.log.Warn("mark_session_completed_failed", "uploadId", session.UploadID, "error", err)
	}

	s.log.Info("upload_session_completed",
		"videoId", id, "uploadId", session.UploadID, "parts", len(parts), "sizeBytes", sizeBytes)

	return s.submitTranscode(ctx, video, session.ObjectKey, session.Ext, sizeBytes)
}

// AbortChunkedUpload 取消分片上传并回收已上传的分片。
func (s *VideoService) AbortChunkedUpload(
	ctx context.Context, u identity.CurrentUser, id uint64,
) (*dto.AbortUploadResult, error) {
	if _, err := s.mustGetOwned(ctx, u, id); err != nil {
		return nil, err
	}

	sessions, err := s.uploads.ListUploading(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, errs.CodeUploadNotFound.New("没有进行中的分片上传")
	}

	s.abortSessions(ctx, sessions)
	s.log.Info("upload_session_aborted", "videoId", id, "sessions", len(sessions))
	return &dto.AbortUploadResult{VideoID: id, Aborted: true}, nil
}

// ---------------------------------------------------------------- 读操作

// ChunkedUploadStatus 查询分片上传进度。
//
// 客户端重新进入上传页（刷新 / 换设备）时先调它：拿到的 uploadedParts 就是
// 「哪几片已经有了」，剩下的接着传即可 —— 这就是断点续传的全部机制。
func (s *VideoService) ChunkedUploadStatus(
	ctx context.Context, u identity.CurrentUser, id uint64,
) (*dto.UploadSessionResult, error) {
	if _, err := s.mustGetOwned(ctx, u, id); err != nil {
		return nil, err
	}
	session, err := s.currentSession(ctx, id)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.sessionSnapshot(ctx, session)
	if err != nil {
		s.log.Warn("list_upload_parts_failed",
			"videoId", id, "uploadId", session.UploadID, "error", err)
		return nil, errs.CodeDependencyDown.New("查询分片进度失败")
	}
	return snapshot, nil
}

// ---------------------------------------------------------------- 内部方法

// currentSession 取视频当前进行中的上传会话。
//
// 正常只会有一条；若进程崩溃等原因留下多条，取最新的一条并回收其余的，
// 保证「一个视频同一时间只有一个可用 uploadId」这个不变式。
func (s *VideoService) currentSession(ctx context.Context, videoID uint64) (*model.UploadSession, error) {
	sessions, err := s.uploads.ListUploading(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, errs.CodeUploadNotFound.New("没有进行中的分片上传，请先调用 upload/init")
	}
	if len(sessions) > 1 {
		s.abortSessions(ctx, sessions[1:])
	}
	return &sessions[0], nil
}

// reuseSession 在已有会话中找一个可续传的（参数完全一致且未过期）。
//
// 返回值语义：
//
//	(snapshot, true,  nil) 复用了这条会话
//	(nil,      false, nil) 没有可复用的会话（调用方走「回收旧会话 + 新建」）
//	(nil,      false, err) 对象存储暂时不可用 —— 必须报错退出，
//	                       否则会误把「查询失败」当「会话失效」，白白清掉已上传的分片
func (s *VideoService) reuseSession(
	ctx context.Context,
	sessions []model.UploadSession,
	objectKey, ext string,
	fileSize, partSize int64,
	totalParts int,
) (*dto.UploadSessionResult, bool, error) {
	for i := range sessions {
		session := sessions[i]
		if !sessionReusable(session, objectKey, ext, fileSize, partSize, totalParts) {
			continue
		}
		snapshot, err := s.sessionSnapshot(ctx, &session)
		if err != nil {
			// 只有「uploadId 在对象存储侧确实不存在」才认为会话废了；
			// 其它错误（超时、鉴权、5xx）原样上报，避免把可用会话误杀。
			if errors.Is(err, storage.ErrUploadNotFound) {
				s.log.Warn("upload_session_stale",
					"videoId", session.VideoID, "uploadId", session.UploadID, "error", err)
				continue
			}
			return nil, false, err
		}
		s.log.Info("upload_session_reused",
			"videoId", session.VideoID, "uploadId", session.UploadID,
			"uploadedCount", snapshot.UploadedCount, "totalParts", snapshot.TotalParts)
		return snapshot, true, nil
	}
	return nil, false, nil
}

// sessionSnapshot 读取对象存储里的真实进度并组装成对外快照。
//
// 错误保持原样返回：调用方需要区分「uploadId 失效」与「对象存储暂时不可用」。
func (s *VideoService) sessionSnapshot(
	ctx context.Context, session *model.UploadSession,
) (*dto.UploadSessionResult, error) {
	parts, err := s.store.ListParts(ctx, session.ObjectKey, session.UploadID)
	if err != nil {
		return nil, err
	}
	return buildSnapshot(session, parts), nil
}

// abortSessions 尽力回收一批会话。
//
// 对象存储侧的失败只告警：本地状态仍会标记 ABORTED，
// 这样至少不会再被当成「可续传的会话」复用。
func (s *VideoService) abortSessions(ctx context.Context, sessions []model.UploadSession) {
	for i := range sessions {
		s.abortSession(ctx, &sessions[i])
	}
}

func (s *VideoService) abortSession(ctx context.Context, session *model.UploadSession) {
	if err := s.store.AbortMultipart(ctx, session.ObjectKey, session.UploadID); err != nil {
		s.log.Warn("abort_multipart_failed",
			"uploadId", session.UploadID, "object", session.ObjectKey, "error", err)
	}
	if err := s.uploads.UpdateFields(ctx, session.ID, map[string]any{
		"status": model.UploadStatusAborted,
	}); err != nil {
		s.log.Warn("mark_session_aborted_failed", "uploadId", session.UploadID, "error", err)
	}
}

// reuseTranscodingTask 处理「已经在转码中」的幂等返回。
//
// 与一次性上传保持一致：重复提交上传不应报错，而应返回当前任务，
// 让客户端的重试是安全的。
func (s *VideoService) reuseTranscodingTask(ctx context.Context, video *model.Video) (*dto.UploadResult, bool) {
	task, err := s.tasks.LatestByVideoID(ctx, video.ID)
	if err != nil {
		return nil, false
	}
	return &dto.UploadResult{
		VideoID:   video.ID,
		Status:    string(video.Status),
		TaskID:    task.ID,
		SizeBytes: video.SizeBytes,
	}, true
}

// submitTranscode 把「原片已就位」推进为一次转码任务。
//
// 一次性上传与分片上传合并后都汇聚到这里，保证两条入口的副作用完全一致。
// 顺序不可调换：**先建任务、再改视频状态、最后发消息**。
// 最坏情况是消息投递失败导致视频卡在 TRANSCODING，可用 retry 接口兜底；
// 反过来（先发消息）消费者可能读到旧状态而误判。
func (s *VideoService) submitTranscode(
	ctx context.Context, video *model.Video, objectKey, ext string, sizeBytes int64,
) (*dto.UploadResult, error) {
	task := &model.TranscodeTask{
		VideoID:  video.ID,
		Status:   model.TranscodeStatusPending,
		Profiles: media.JoinProfileNames(),
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, err
	}

	if err := s.videos.UpdateFields(ctx, video.ID, map[string]any{
		"status":        model.VideoStatusTranscoding,
		"source_object": objectKey,
		"size_bytes":    sizeBytes,
	}); err != nil {
		return nil, err
	}
	s.cache.DelDetail(ctx, video.ID)

	event, err := events.New(events.RoutingVideoUploaded, trace.From(ctx), dto.VideoUploadedPayload{
		VideoID:      video.ID,
		OwnerID:      video.OwnerID,
		SourceObject: objectKey,
		Ext:          ext,
		SizeBytes:    sizeBytes,
		TaskID:       task.ID,
	})
	if err != nil {
		return nil, err
	}

	if err := s.bus.Publish(ctx, events.RoutingVideoUploaded, event); err != nil {
		s.log.Error("publish_video_uploaded_failed",
			"videoId", video.ID, "eventId", event.EventID, "error", err)
		return nil, errs.CodeTranscodeSubmitFail.New("投递转码事件失败，可使用转码重试接口")
	}

	// 回写 eventId，供消费侧幂等与排障
	if err := s.tasks.UpdateFields(ctx, task.ID, map[string]any{"event_id": event.EventID}); err != nil {
		s.log.Warn("save_event_id_failed", "taskId", task.ID, "error", err)
	}

	s.log.Info("video_uploaded",
		"videoId", video.ID, "taskId", task.ID, "object", objectKey,
		"sizeBytes", sizeBytes, "eventId", event.EventID)

	return &dto.UploadResult{
		VideoID:   video.ID,
		Status:    string(model.VideoStatusTranscoding),
		TaskID:    task.ID,
		SizeBytes: sizeBytes,
	}, nil
}

// ---------------------------------------------------------------- 纯函数

// choosePartSize 推导本次上传的分片大小。
//
// 规则：
//   - 下限是 S3 协议的 5MiB（非末片不得更小，否则合并时会被拒）；
//   - 客户端显式给了更大的值就以客户端为准（便于它按自己的并发度调优）；
//   - 片数超过上限时逐次翻倍，直到装得下或触及 maxParts。
func choosePartSize(fileSize, clientPartSize, minPartSize int64, maxParts int) int64 {
	size := minPartSize
	if size <= 0 {
		size = 5 << 20
	}
	if clientPartSize > size {
		size = clientPartSize
	}
	for maxParts > 0 && size > 0 && (fileSize+size-1)/size > int64(maxParts) {
		size *= 2
	}
	return size
}

// totalPartsOf 计算分片总数（向上取整）。
func totalPartsOf(fileSize, partSize int64) int {
	if fileSize <= 0 || partSize <= 0 {
		return 0
	}
	return int((fileSize + partSize - 1) / partSize)
}

// sessionReusable 判断已有会话能否直接续传。
//
// 四个参数必须全部一致（对象、扩展名、总大小、分片大小）：
// 任何一个不同，之前传好的分片的偏移量就全错了，续传会拼出损坏的文件。
func sessionReusable(
	session model.UploadSession, objectKey, ext string, fileSize, partSize int64, totalParts int,
) bool {
	if !session.Active() {
		return false
	}
	if session.ObjectKey != objectKey || session.Ext != ext ||
		session.FileSize != fileSize || session.PartSize != partSize ||
		session.TotalParts != totalParts {
		return false
	}
	return time.Since(session.CreatedAt) <= uploadSessionTTL
}

// buildSnapshot 把对象存储的分片列表投影成对外快照。
func buildSnapshot(session *model.UploadSession, parts []storage.Part) *dto.UploadSessionResult {
	items := make([]dto.UploadPartDTO, 0, len(parts))
	var uploaded int64
	for _, p := range parts {
		items = append(items, dto.UploadPartDTO{
			PartNumber: p.PartNumber,
			Size:       p.Size,
			ETag:       p.ETag,
		})
		uploaded += p.Size
	}

	return &dto.UploadSessionResult{
		VideoID:       session.VideoID,
		UploadID:      session.UploadID,
		ObjectKey:     session.ObjectKey,
		Ext:           session.Ext,
		FileSize:      session.FileSize,
		PartSize:      session.PartSize,
		TotalParts:    session.TotalParts,
		UploadedParts: items,
		UploadedBytes: uploaded,
		UploadedCount: len(items),
		Completed:     session.TotalParts > 0 && len(items) >= session.TotalParts,
	}
}

// checkPartsComplete 校验分片是否已经齐了。
//
// 用「序号集合恰好等于 1..totalParts」而不是「数量相等」来判定：
// 数量相等但序号重复/跳跃（客户端并发写同一个 partNumber）时，
// 合并出来的对象会缺字节，必须在这里挡住，而不是等转码失败再去查。
func checkPartsComplete(session *model.UploadSession, parts []storage.Part) error {
	if session.TotalParts <= 0 {
		return errs.CodeUploadPartInvalid.New("上传会话缺少分片计划，请重新初始化")
	}
	if len(parts) != session.TotalParts {
		return errs.CodeUploadIncomplete.Newf(
			"已上传 %d/%d 片，请补齐后再合并", len(parts), session.TotalParts)
	}

	seen := make(map[int]struct{}, len(parts))
	var total int64
	for _, p := range parts {
		if p.PartNumber < 1 || p.PartNumber > session.TotalParts {
			return errs.CodeUploadPartInvalid.Newf("存在越界分片 %d", p.PartNumber)
		}
		seen[p.PartNumber] = struct{}{}
		total += p.Size
	}
	if len(seen) != session.TotalParts {
		return errs.CodeUploadIncomplete.Newf(
			"分片序号存在重复，实际覆盖 %d/%d 片", len(seen), session.TotalParts)
	}
	if session.FileSize > 0 && total != session.FileSize {
		return errs.CodeUploadIncomplete.Newf(
			"分片总大小 %d 与声明的 %d 不一致", total, session.FileSize)
	}
	return nil
}
