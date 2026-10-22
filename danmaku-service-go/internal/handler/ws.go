package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/config"
	"github.com/example/danmaku-service-go/internal/infra/hub"
	"github.com/example/danmaku-service-go/internal/service"
	"github.com/example/danmaku-service-go/pkg/errs"
	"github.com/example/danmaku-service-go/pkg/identity"
	"github.com/example/danmaku-service-go/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// sendTimeout 是单条 WebSocket 发送的业务处理超时。
//
// WebSocket 消息没有请求级 context（握手请求的 context 在升级后就失效了），
// 因此每条消息自己造一个带超时的 context，避免 MySQL/Redis 卡住时 goroutine 泄漏。
const sendTimeout = 5 * time.Second

// WSHandler 处理 WebSocket 握手、协议帧与优雅关闭。
type WSHandler struct {
	cfg      *config.Config
	hub      *hub.Hub
	svc      *service.DanmakuService
	log      *slog.Logger
	upgrader websocket.Upgrader
	hubCfg   hub.Config
}

// NewWSHandler 构造 WebSocket handler。
func NewWSHandler(cfg *config.Config, h *hub.Hub, svc *service.DanmakuService, log *slog.Logger) *WSHandler {
	if log == nil {
		log = slog.Default()
	}
	hubCfg := hub.Config{
		ReadBufferSize:  cfg.WS.ReadBufferSize,
		WriteBufferSize: cfg.WS.WriteBufferSize,
		WriteWait:       cfg.WS.WriteWait,
		PongWait:        cfg.WS.PongWait,
		PingPeriod:      cfg.WS.PingPeriod,
		MaxConnsPerUser: cfg.WS.MaxConnsPerUser,
		MaxFrameBytes:   cfg.WS.MaxFrameBytes,
		RoomIdleGrace:   cfg.WS.RoomIdleGrace,
	}
	return &WSHandler{
		cfg:    cfg,
		hub:    h,
		svc:    svc,
		log:    log,
		hubCfg: hubCfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:   cfg.WS.ReadBufferSize,
			WriteBufferSize:  cfg.WS.WriteBufferSize,
			HandshakeTimeout: 10 * time.Second,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					// 非浏览器客户端（脚本 / 播放器）不发 Origin；不做来源校验。
					return true
				}
				return cfg.WS.OriginAllowed(origin)
			},
		},
	}
}

// Serve 处理 `GET /ws/danmaku?roomId=&videoId=&uid=&nickname=`。
//
// uid 允许从 query 传入：**浏览器的 WebSocket 无法自定义请求头**，
// 生产环境应强制 WSS 并由网关校验 JWT 后再放行升级；
// 若网关注入了 `X-User-Id`，这里以请求头为准（比 query 可信）。
func (h *WSHandler) Serve(c *gin.Context) {
	roomID := strings.TrimSpace(c.Query("roomId"))
	videoID := strings.TrimSpace(c.Query("videoId"))
	uid := strings.TrimSpace(c.Query("uid"))
	nickname := strings.TrimSpace(c.Query("nickname"))

	if headerUID := strings.TrimSpace(c.GetHeader(identity.HeaderUserID)); headerUID != "" {
		uid = headerUID
	}

	if roomID == "" && videoID == "" {
		h.renderPreUpgradeError(c, errs.CodeInvalidParam.New("必须提供 roomId 或 videoId"))
		return
	}
	if roomID != "" && videoID != "" {
		h.renderPreUpgradeError(c, errs.CodeInvalidParam.New("roomId 与 videoId 只能提供一个"))
		return
	}

	if err := h.svc.CheckJoinable(c.Request.Context(), roomID, videoID); err != nil {
		h.renderPreUpgradeError(c, err)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 失败时它自己已经写过 HTTP 响应，这里只记日志。
		h.log.Warn("ws_upgrade_failed", "roomId", roomID, "videoId", videoID, "err", err.Error())
		return
	}

	subject := hub.SubjectKey(roomID, videoID)
	client := hub.NewClient(h.hub, conn, hub.ClientMeta{
		ID:       newClientID(),
		Subject:  subject,
		RoomID:   roomID,
		VideoID:  videoID,
		UID:      uid,
		Nickname: nickname,
		ReadOnly: uid == "",
	}, h.hubCfg, h.onMessage, h.log)

	h.hub.Register(client)

	// welcome 必须在 Register 之后：它的 onlineCount 要包含本连接。
	hotCount := int64(0)
	if roomID != "" {
		if n, err := h.svc.RoomHotCount(c.Request.Context(), roomID); err == nil {
			hotCount = n
		}
	}
	client.SendFrame(hub.TypeWelcome, hub.WelcomeData{
		ClientID:    client.ID,
		RoomID:      roomID,
		VideoID:     videoID,
		UID:         uid,
		ReadOnly:    client.ReadOnly,
		OnlineCount: h.hub.Online(subject),
		HotCount:    hotCount,
		ServerTime:  time.Now(),
	})

	h.log.Info("ws_connected",
		"clientId", client.ID, "roomId", roomID, "videoId", videoID,
		"uid", uid, "readOnly", client.ReadOnly, "online", h.hub.Online(subject))

	// Serve 阻塞在 readPump 上，直到连接出错 / 被关闭（含被踢）。
	client.Serve()

	h.hub.Unregister(client)
	client.CloseWith(hub.CloseNormalClosure, "连接结束")

	h.log.Info("ws_disconnected",
		"clientId", client.ID, "subject", subject,
		"dropped", client.Dropped(), "durationSec", int(time.Since(client.ConnectedAt).Seconds()))
}

// onMessage 处理客户端帧。
//
// 业务错误一律回 `error` 帧而**不关连接**（设计文档 §10.2）：
// 一条弹幕被限流不代表连接坏了，关连接只会引发重连风暴。
func (h *WSHandler) onMessage(client *hub.Client, frame hub.Frame) {
	switch frame.Type {
	case hub.TypePing:
		client.SendFrame(hub.TypePong, hub.PongData{ServerTime: time.Now()})

	case hub.TypeDanmaku:
		if client.ReadOnly {
			client.SendError(errs.CodeUnauthorized.Code, "游客不能发送弹幕")
			return
		}

		var in hub.InboundDanmaku
		if err := frame.Bind(&in); err != nil {
			client.SendError(errs.CodeInvalidParam.Code, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()

		// roomId / videoId 以握手参数为准，帧内不传 —— 防跨房间注入。
		result, err := h.svc.Send(ctx, dto.SendInput{
			RoomID:      client.RoomID,
			VideoID:     client.VideoID,
			UID:         client.UID,
			Nickname:    client.Nickname,
			Content:     in.Content,
			Color:       in.Color,
			Position:    in.Position,
			TimeMs:      in.TimeMs,
			ClientMsgID: in.ClientMsgID,
		})
		if err != nil {
			h.sendErrorFrame(client, err)
			return
		}
		// 发送者自己会通过房间广播收到这条弹幕（与其他人完全同一条），
		// 因此这里不再单独回包，客户端只需处理 danmaku 帧。
		_ = result

	default:
		client.SendError(errs.CodeInvalidParam.Code, "不支持的帧类型: "+frame.Type)
	}
}

// sendErrorFrame 把业务错误编码成 error 帧（带上 retryAfterMs）。
func (h *WSHandler) sendErrorFrame(client *hub.Client, err error) {
	be, ok := errs.AsBizError(err)
	if !ok {
		h.log.Error("ws_send_failed", "clientId", client.ID, "err", err.Error())
		client.SendFrame(hub.TypeError, hub.ErrorData{
			Code:    errs.CodeInternal.Code,
			Message: errs.CodeInternal.Message,
		})
		return
	}

	data := hub.ErrorData{Code: be.Code.Code, Message: be.Message()}
	if extra, ok := be.ExtraData().(map[string]any); ok {
		if ms, ok := extra["retryAfterMs"].(int64); ok {
			data.RetryAfterMs = ms
		}
	}
	client.SendFrame(hub.TypeError, data)
}

// renderPreUpgradeError 在升级前返回统一响应体（HTTP 400/403 等，不升级连接）。
func (h *WSHandler) renderPreUpgradeError(c *gin.Context, err error) {
	be, ok := errs.AsBizError(err)
	if !ok {
		c.JSON(http.StatusInternalServerError, response.Body{
			Code: errs.CodeInternal.Code, Message: errs.CodeInternal.Message, Data: nil,
		})
		return
	}
	c.JSON(be.Code.HTTPStatus, response.Body{
		Code: be.Code.Code, Message: be.Message(), Data: be.ExtraData(),
	})
}

// newClientID 生成形如 `c-8f2a1b` 的连接标识（仅用于日志与 welcome 帧）。
func newClientID() string {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "c-000000"
	}
	return "c-" + hex.EncodeToString(buf)
}
