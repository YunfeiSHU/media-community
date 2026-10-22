// Package service 承载直播间的业务规则、状态机与事务边界。
//
// 分层纪律：本包不感知 *gin.Context，也不直接依赖 gorm.DB；
// 数据访问一律通过 repository，缓存通过 cache，媒体服务通过 media。
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/example/live-service-go/api/dto"
	"github.com/example/live-service-go/internal/config"
	"github.com/example/live-service-go/internal/infra/cache"
	"github.com/example/live-service-go/internal/infra/media"
	"github.com/example/live-service-go/internal/model"
	"github.com/example/live-service-go/internal/repository"
	"github.com/example/live-service-go/pkg/errs"
)

// 字段长度限制，与《需求文档》/ DDL 保持一致。
const (
	maxTitleLen  = 64
	maxNoticeLen = 512
	maxCoverLen  = 512

	defaultPageSize = 20
	maxPageSize     = 100
)

// deps 是各 Service 共享的依赖集合。
//
// 用嵌入而不是每个 Service 重复声明：三个 Service（房间 / 开播 / 统计）
// 面向同一批依赖，拆开只会让构造函数变长。
type deps struct {
	cfg     *config.Config
	rooms   repository.RoomRepository
	records repository.RecordRepository
	cache   *cache.RoomCache
	media   media.Server
	log     *slog.Logger
}

// getRoom 读取直播间，找不到时返回 70005。
func (d *deps) getRoom(ctx context.Context, id uint64) (*model.Room, error) {
	room, err := d.rooms.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, errs.CodeRoomNotFound.Newf("直播间 %d 不存在", id)
		}
		return nil, err
	}
	return room, nil
}

// srsAvailable 探测媒体服务是否可用。
//
// 探测失败不视为错误（开播仍然成功，只是主播推流时会失败），
// 因此这里只返回布尔值，把错误吞进日志。
func (d *deps) srsAvailable(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return d.media.Health(probeCtx) == nil
}

// onlineCount 读取实时在线人数（Redis）。
func (d *deps) onlineCount(ctx context.Context, roomID uint64) int64 {
	return d.cache.Online(ctx, roomID)
}

// playURLsPtr 仅在状态允许时返回播放地址，避免前端误播空流。
func (d *deps) playURLsPtr(room *model.Room) *dto.PlayURLs {
	if room.Status != model.RoomStatusLiving {
		return nil
	}
	urls := d.media.PlayURLs(room.StreamKey)
	return &dto.PlayURLs{HLS: urls.HLS, HTTPFLV: urls.HTTPFLV, WebRTC: urls.WebRTC, RTMP: urls.RTMP}
}

// newStreamKey 生成推流密钥：16 字节随机数 → 32 位 hex。
//
// 用 crypto/rand 而不是 math/rand：密钥是唯一的推流凭证，
// 可预测就等于「任何人都能把你的房间推成黑屏」。
func newStreamKey() (string, error) { return randomHex(16) }

// newSessionID 生成会话 ID：8 字节随机数 → 16 位 hex。
func newSessionID() (string, error) { return randomHex(8) }

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// normalizePaging 归一化分页参数。
func normalizePaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// validateTitle 校验标题。
func validateTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", errs.CodeInvalidParam.New("title 不能为空")
	}
	if len([]rune(trimmed)) > maxTitleLen {
		return "", errs.CodeInvalidParam.Newf("title 长度不能超过 %d", maxTitleLen)
	}
	return trimmed, nil
}

// validateOptional 校验可选的文本字段（按字符数而非字节数）。
func validateOptional(value, field string, max int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len([]rune(trimmed)) > max {
		return "", errs.CodeInvalidParam.Newf("%s 长度不能超过 %d", field, max)
	}
	return trimmed, nil
}

// toRecordDTO 把实体转成对外 DTO。
func toRecordDTO(rec *model.Record) dto.RecordDTO {
	return dto.RecordDTO{
		RecordID:    rec.ID,
		SessionID:   rec.SessionID,
		StartedAt:   rec.StartedAt,
		EndedAt:     rec.EndedAt,
		DurationSec: rec.DurationSec,
		PeakOnline:  rec.PeakOnline,
		ViewerTotal: rec.ViewerTotal,
		EndReason:   rec.EndReason,
	}
}
