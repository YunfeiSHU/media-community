package service

import (
	"context"
	"log/slog"

	"github.com/example/live-service-go/api/dto"
	"github.com/example/live-service-go/internal/config"
	"github.com/example/live-service-go/internal/infra/cache"
	"github.com/example/live-service-go/internal/infra/media"
	"github.com/example/live-service-go/internal/model"
	"github.com/example/live-service-go/internal/repository"
	"github.com/example/live-service-go/pkg/errs"
	"github.com/example/live-service-go/pkg/identity"
)

// RoomService 负责直播间的增删改查与只读视图。
type RoomService struct {
	deps
}

// NewRoomService 构造直播间服务。
func NewRoomService(
	cfg *config.Config,
	rooms repository.RoomRepository,
	records repository.RecordRepository,
	roomCache *cache.RoomCache,
	mediaServer media.Server,
	log *slog.Logger,
) *RoomService {
	return &RoomService{deps: deps{
		cfg: cfg, rooms: rooms, records: records, cache: roomCache, media: mediaServer, log: log,
	}}
}

// Create 创建直播间。
//
// 创建时就返回 streamKey / pushUrl，因为它们是房间的固有属性；
// 但**每次开播都会重新生成 streamKey**，所以创建时拿到的这份只在首次开播前有效。
func (s *RoomService) Create(ctx context.Context, user identity.CurrentUser, req dto.CreateRoomRequest) (*dto.CreateRoomResult, error) {
	title, err := validateTitle(req.Title)
	if err != nil {
		return nil, err
	}
	notice, err := validateOptional(req.Notice, "notice", maxNoticeLen)
	if err != nil {
		return nil, err
	}
	cover, err := validateOptional(req.CoverURL, "coverUrl", maxCoverLen)
	if err != nil {
		return nil, err
	}

	count, err := s.rooms.CountByOwner(ctx, user.UID)
	if err != nil {
		return nil, err
	}
	if count >= int64(s.cfg.Room.MaxPerOwner) {
		return nil, errs.CodeRoomLimitExceeded.Newf("直播间数量已达上限: %d", s.cfg.Room.MaxPerOwner)
	}

	streamKey, err := newStreamKey()
	if err != nil {
		return nil, err
	}

	room := &model.Room{
		OwnerID:    user.UID,
		Title:      title,
		Notice:     notice,
		CoverURL:   cover,
		CategoryID: req.CategoryID,
		StreamKey:  streamKey,
		Status:     model.RoomStatusIdle,
	}
	if err := s.rooms.Create(ctx, room); err != nil {
		return nil, err
	}

	urls := s.media.PlayURLs(room.StreamKey)
	return &dto.CreateRoomResult{
		RoomID:    room.ID,
		Title:     room.Title,
		Status:    string(room.Status),
		StreamKey: room.StreamKey,
		PushURL:   s.media.PushURL(room.StreamKey, ""),
		PlayURLs:  &dto.PlayURLs{HLS: urls.HLS, HTTPFLV: urls.HTTPFLV, WebRTC: urls.WebRTC, RTMP: urls.RTMP},
		CreatedAt: room.CreatedAt,
	}, nil
}

// Update 修改直播间元信息。
//
// 直播中允许改标题与公告（运营常态），但不提供修改 streamKey 的入口。
func (s *RoomService) Update(
	ctx context.Context, user identity.CurrentUser, roomID uint64, req dto.UpdateRoomRequest,
) (*dto.UpdateRoomResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !user.Owns(room.OwnerID) {
		return nil, errs.CodeForbidden.New("只能修改自己的直播间")
	}

	fields := make(map[string]any, 4)
	if req.Title != nil {
		title, err := validateTitle(*req.Title)
		if err != nil {
			return nil, err
		}
		fields["title"] = title
	}
	if req.Notice != nil {
		notice, err := validateOptional(*req.Notice, "notice", maxNoticeLen)
		if err != nil {
			return nil, err
		}
		fields["notice"] = notice
	}
	if req.CoverURL != nil {
		cover, err := validateOptional(*req.CoverURL, "coverUrl", maxCoverLen)
		if err != nil {
			return nil, err
		}
		fields["cover_url"] = cover
	}
	if req.CategoryID != nil {
		fields["category_id"] = *req.CategoryID
	}

	if len(fields) > 0 {
		if err := s.rooms.UpdateFields(ctx, room.ID, fields); err != nil {
			return nil, err
		}
		// 详情缓存里含标题/公告，必须失效；状态 Hash 也顺带刷新
		s.cache.DelDetail(ctx, room.ID)
		if fresh, err := s.rooms.GetByID(ctx, room.ID); err == nil {
			s.cache.SetState(ctx, fresh)
		}
	}

	return &dto.UpdateRoomResult{RoomID: room.ID, Updated: true}, nil
}

// Delete 删除直播间（软删除）。
func (s *RoomService) Delete(
	ctx context.Context, user identity.CurrentUser, roomID uint64,
) (*dto.DeleteRoomResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !user.Owns(room.OwnerID) {
		return nil, errs.CodeForbidden.New("只能删除自己的直播间")
	}
	if room.Status.InProgress() {
		return nil, errs.CodeNotLiving.New("直播中不能删除直播间，请先关播")
	}

	if err := s.rooms.Delete(ctx, room.ID); err != nil {
		return nil, err
	}
	s.cache.ClearSession(ctx, room.ID, room.StreamKey)
	return &dto.DeleteRoomResult{RoomID: room.ID, Deleted: true}, nil
}

// List 返回直播间列表。
func (s *RoomService) List(
	ctx context.Context, user identity.CurrentUser, q dto.ListQuery,
) ([]dto.RoomBrief, int64, int, int, error) {
	if q.Mine {
		if !user.Authenticated() {
			return nil, 0, 0, 0, errs.CodeUnauthorized.Plain()
		}
		q.OwnerID = user.UID
	}

	page, pageSize := normalizePaging(q.Page, q.PageSize)
	rooms, total, err := s.rooms.List(ctx, repository.RoomFilter{
		Keyword:    q.Keyword,
		CategoryID: q.CategoryID,
		OwnerID:    q.OwnerID,
		Status:     q.Status,
		LivingOnly: q.LivingOnly,
		Offset:     (page - 1) * pageSize,
		Limit:      pageSize,
	})
	if err != nil {
		return nil, 0, 0, 0, err
	}

	items := make([]dto.RoomBrief, 0, len(rooms))
	for i := range rooms {
		room := &rooms[i]
		var online int64
		if room.Status == model.RoomStatusLiving {
			// 只有正在直播的房间才需要查 Redis，避免列表接口打满 Redis
			online = s.onlineCount(ctx, room.ID)
		}
		items = append(items, dto.RoomBrief{
			RoomID:      room.ID,
			Title:       room.Title,
			Notice:      room.Notice,
			CoverURL:    room.CoverURL,
			OwnerID:     room.OwnerID,
			CategoryID:  room.CategoryID,
			Status:      string(room.Status),
			OnlineCount: online,
			ViewerTotal: room.ViewerTotal,
			PeakOnline:  room.PeakOnline,
			StartedAt:   room.LastStartedAt,
			PlayURLs:    s.playURLsPtr(room),
			CreatedAt:   room.CreatedAt,
			UpdatedAt:   room.UpdatedAt,
		})
	}
	return items, total, page, pageSize, nil
}

// Get 返回直播间详情。
//
// 可见性：游客/他人看不到 streamKey 与 pushUrl（字段直接不出现）；
// playUrls 只在 LIVING 时返回。
//
// 缓存策略：**缓存里不存 streamKey**（即使 Redis 与 Java 共用，也不该把推流凭证写进去），
// 创建者访问时再补一次主键查询。
func (s *RoomService) Get(
	ctx context.Context, user identity.CurrentUser, roomID uint64,
) (*dto.RoomDetail, error) {
	canSeeSecret := false

	if cached, ok := s.cache.GetDetail(ctx, roomID); ok {
		if cached == nil {
			return nil, errs.CodeRoomNotFound.Newf("直播间 %d 不存在", roomID)
		}
		if user.Owns(cached.OwnerID) || user.IsAdmin() {
			canSeeSecret = true
		}
		if canSeeSecret {
			room, err := s.getRoom(ctx, roomID)
			if err != nil {
				return nil, err
			}
			s.fillSecret(cached, room)
			s.fillRealtime(ctx, cached, room)
		}
		return cached, nil
	}

	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	detail := s.buildDetail(ctx, room)
	if user.Owns(room.OwnerID) || user.IsAdmin() {
		canSeeSecret = true
	}
	if canSeeSecret {
		s.fillSecret(detail, room)
	}
	s.cache.SetDetail(ctx, room.ID, detail)
	return detail, nil
}

// StreamKey 单独提供推流密钥查询：游客错误地调详情接口时字段被裁剪，
// 这个接口给出明确的语义与 403 提示。
func (s *RoomService) StreamKey(
	ctx context.Context, user identity.CurrentUser, roomID uint64,
) (*dto.StreamKeyResult, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !user.Owns(room.OwnerID) && !user.IsAdmin() {
		return nil, errs.CodeForbidden.New("只能查看自己直播间的推流密钥")
	}
	return &dto.StreamKeyResult{
		RoomID:       room.ID,
		StreamKey:    room.StreamKey,
		PushURL:      s.media.PushURL(room.StreamKey, room.CurrentSession),
		Status:       string(room.Status),
		SRSAvailable: s.srsAvailable(ctx),
	}, nil
}

// Records 返回直播历史。
func (s *RoomService) Records(
	ctx context.Context, user identity.CurrentUser, roomID uint64, page, pageSize int,
) ([]dto.RecordDTO, int64, int, int, error) {
	room, err := s.getRoom(ctx, roomID)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if !user.Owns(room.OwnerID) && !user.IsAdmin() {
		return nil, 0, 0, 0, errs.CodeForbidden.New("只能查看自己直播间的开播历史")
	}

	page, pageSize = normalizePaging(page, pageSize)
	records, total, err := s.records.ListByRoom(ctx, room.ID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	items := make([]dto.RecordDTO, 0, len(records))
	for i := range records {
		items = append(items, toRecordDTO(&records[i]))
	}
	return items, total, page, pageSize, nil
}

// ---------------------------------------------------------------- 内部

// buildDetail 由实体构造对外详情（不含 streamKey）。
func (s *RoomService) buildDetail(ctx context.Context, room *model.Room) *dto.RoomDetail {
	detail := &dto.RoomDetail{
		RoomID:       room.ID,
		Title:        room.Title,
		Notice:       room.Notice,
		CoverURL:     room.CoverURL,
		OwnerID:      room.OwnerID,
		CategoryID:   room.CategoryID,
		Status:       string(room.Status),
		SessionID:    room.CurrentSession,
		ViewerTotal:  room.ViewerTotal,
		PeakOnline:   room.PeakOnline,
		StartedAt:    room.LastStartedAt,
		EndedAt:      room.LastEndedAt,
		PlayURLs:     s.playURLsPtr(room),
		SRSAvailable: s.srsAvailable(ctx),
		CreatedAt:    room.CreatedAt,
		UpdatedAt:    room.UpdatedAt,
	}
	s.fillRealtime(ctx, detail, room)
	return detail
}

// fillRealtime 用 Redis 的实时数据覆盖详情里的在线人数 / 峰值 / 累计人次。
//
// 计算公式：累计人次 = MySQL 基数（开播前已累计）+ Redis 本次增量。
func (s *RoomService) fillRealtime(ctx context.Context, detail *dto.RoomDetail, room *model.Room) {
	detail.OnlineCount = s.onlineCount(ctx, room.ID)

	peak := s.cache.Peak(ctx, room.ID)
	if peak < int64(room.PeakOnline) {
		peak = int64(room.PeakOnline)
	}
	detail.PeakOnline = int(peak)

	if room.Status.InProgress() {
		detail.ViewerTotal = room.ViewerTotal + s.cache.Viewers(ctx, room.ID)
	}
}

// fillSecret 补齐只有创建者/管理员可见的字段。
//
// 推流地址带上本次会话 ID：SRS 回调会原样回传，用于识别旧会话的延迟回调。
func (s *RoomService) fillSecret(detail *dto.RoomDetail, room *model.Room) {
	detail.StreamKey = room.StreamKey
	detail.PushURL = s.media.PushURL(room.StreamKey, room.CurrentSession)
}
