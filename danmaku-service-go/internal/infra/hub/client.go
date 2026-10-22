package hub

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/danmaku-service-go/pkg/errs"
	"github.com/gorilla/websocket"
)

// maxProtoErrors 是允许的连续协议解析失败次数，超过即关闭连接。
//
// 设计文档 §10.2 的选择：单条弹幕被拒（限流/敏感词）不关连接
// ——否则客户端会重连风暴；只有「帧根本解析不了」才关。
const maxProtoErrors = 3

// MessageHandler 处理客户端帧。实现方负责把业务错误编码成 error 帧。
type MessageHandler func(c *Client, frame Frame)

// Config 是 Hub 与单连接的行为参数（由 config.WS 转换而来）。
type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	WriteWait       time.Duration
	PongWait        time.Duration
	PingPeriod      time.Duration
	MaxConnsPerUser int
	MaxFrameBytes   int64
	RoomIdleGrace   time.Duration
}

// ClientMeta 是握手阶段确定的连接属性。
//
//   - Subject：Hub 内部的房间标识（由 hub.SubjectKey 生成，直播 r:{roomId} / 点播 v:{videoId}）
//   - RoomID / VideoID：业务上的真实 ID，用于落库与业务校验
//   - UID 为空表示游客：**只收不发**（发送请求会被回 error 帧）
type ClientMeta struct {
	ID       string
	Subject  string
	RoomID   string
	VideoID  string
	UID      string
	Nickname string
	ReadOnly bool
}

// Client 是一条 WebSocket 连接。
//
// 并发模型（设计文档 §4.2）：每条连接两个 goroutine
//   - readPump：阻塞读帧 → 交 handler
//   - writePump：从 send channel 取消息写出
//
// `gorilla/websocket` **不允许并发调用 ReadMessage 与 WriteMessage**，
// 读写分离天然满足该约束；而「广播只往带缓冲的 send 里投递」保证了
// 「任一慢客户端不拖慢整个房间」。
type Client struct {
	ClientMeta
	ConnectedAt time.Time

	// Seq 是连接建立顺序（进程内单调递增），用于判定「谁更旧」。
	//
	// 为什么不用 ConnectedAt 排序：Windows 上 time.Now() 的粒度约为 15.6ms，
	// 同一时刻注册的多条连接会拿到**完全相同**的时间戳，
	// 于是「同 uid 超过上限就踢最旧连接」的排序退化成随机。
	// ConnectedAt 保留仅用于展示与日志。
	Seq uint64

	conn    *websocket.Conn
	cfg     Config
	hub     *Hub
	handler MessageHandler
	log     *slog.Logger

	send chan []byte

	dropped   atomic.Int64
	protoErrs int // 仅 readPump goroutine 访问

	closeOnce   sync.Once
	closeCh     chan struct{}
	closeMu     sync.Mutex
	closeCode   int
	closeReason string
	closeSet    bool // 关闭码是否被显式设置过（默认 1001 不算）
}

// connSeq 为每条新连接分配单调递增序号，是 Client.Seq 的唯一来源。
var connSeq atomic.Uint64

// NewClient 构造连接对象。
//
// conn 允许为 nil：此时 Send / 广播仍然可用（消息只进 send 缓冲），
// 这让 Hub 的广播与背压可以脱离真实网络做 `-race` 单元测试。
// **Serve 在 conn == nil 时会阻塞等待关闭，禁止在生产路径这样用。**
func NewClient(h *Hub, conn *websocket.Conn, meta ClientMeta, cfg Config, handler MessageHandler, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{
		ClientMeta:  meta,
		ConnectedAt: time.Now(),
		Seq:         connSeq.Add(1),
		conn:        conn,
		cfg:         cfg,
		hub:         h,
		handler:     handler,
		log:         log.With("clientId", meta.ID, "subject", meta.Subject, "uid", meta.UID),
		send:        make(chan []byte, cfg.WriteBufferSize),
		closeCh:     make(chan struct{}),
		closeCode:   CloseGoingAway,
	}
}

// Send 非阻塞投递一帧。
//
// 返回 false 表示缓冲区已满、该帧被丢弃（调用方累计丢弃计数）。
// **这里绝不能用阻塞发送**：一个慢客户端会拖垮整个房间的广播。
func (c *Client) Send(msg []byte) bool {
	if len(msg) == 0 {
		return true
	}
	select {
	case <-c.closeCh:
		return true // 已关闭不算丢弃
	default:
	}

	select {
	case c.send <- msg:
		return true
	default:
		c.dropped.Add(1)
		return false
	}
}

// SendFrame 序列化并投递一帧。
func (c *Client) SendFrame(typ string, data any) bool {
	raw, err := NewFrame(typ, data)
	if err != nil {
		c.log.Warn("ws_frame_encode_failed", "type", typ, "err", err.Error())
		return false
	}
	return c.Send(raw)
}

// SendError 回一帧 error（不关闭连接）。
func (c *Client) SendError(code int, message string) bool {
	return c.SendFrame(TypeError, ErrorData{Code: code, Message: message})
}

// SendErrorRetry 回一帧带 retryAfterMs 的 error（限流场景）。
func (c *Client) SendErrorRetry(code int, message string, retryAfterMs int64) bool {
	return c.SendFrame(TypeError, ErrorData{Code: code, Message: message, RetryAfterMs: retryAfterMs})
}

// Dropped 返回该连接累计被丢弃的帧数。
func (c *Client) Dropped() int64 { return c.dropped.Load() }

// Close 以默认关闭码关闭连接。
func (c *Client) Close() { c.CloseWith(0, "") }

// CloseWith 关闭连接。code 为 0 表示沿用已有关闭码。
//
// **先到者胜**：一旦关闭码被显式设置过（连续协议错误 1003、被新连接踢掉 4001、
// 服务端重启 1001），后面的调用不会覆盖它。这是必须的 —— readPump 因协议错误
// CloseWith(1003) 后返回，HTTP handler 收尾时还会调用 CloseWith(1000, "连接结束")，
// 若不保护，客户端就只能看到 1000，真正的关闭原因（1003 / 4001）就丢了。
//
// 注意不能用「closeCode == 0」做判据：NewClient 把默认值设成了 1001。
//
// 只关闭 closeCh，真正的 conn.Close 由 writePump 在发出 Close 帧后执行
// —— 这样客户端能收到关闭原因（如 4001「在其他地方打开」）。
func (c *Client) CloseWith(code int, reason string) {
	c.closeMu.Lock()
	if code != 0 && !c.closeSet {
		c.closeCode, c.closeReason, c.closeSet = code, reason, true
	}
	c.closeMu.Unlock()

	c.closeOnce.Do(func() { close(c.closeCh) })
}

// Serve 启动读写 goroutine，并在读循环结束后返回。
//
// 调用方（HTTP handler）在 Serve 返回后必须 Unregister + CloseWith。
func (c *Client) Serve() {
	go c.writePump()
	c.readPump()
}

func (c *Client) closing() (int, string) {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	return c.closeCode, c.closeReason
}

func (c *Client) writePump() {
	ticker := time.NewTicker(c.cfg.PingPeriod)
	defer func() {
		ticker.Stop()
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}()

	for {
		select {
		case msg := <-c.send:
			if err := c.write(websocket.TextMessage, msg); err != nil {
				c.log.Debug("ws_write_failed", "err", err.Error())
				return
			}
		case <-ticker.C:
			// 服务端主动 Ping：客户端回 Pong 会刷新读超时。
			if err := c.write(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.closeCh:
			code, reason := c.closing()
			if c.conn != nil {
				_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait))
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
			}
			return
		}
	}
}

func (c *Client) write(messageType int, data []byte) error {
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait))
	return c.conn.WriteMessage(messageType, data)
}

func (c *Client) readPump() {
	if c.conn == nil {
		// 测试模式：没有真实连接，等关闭信号即可。
		<-c.closeCh
		return
	}

	c.conn.SetReadLimit(c.cfg.MaxFrameBytes)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, CloseKickedByNewConn) {
				c.log.Debug("ws_read_closed", "err", err.Error())
			}
			return
		}

		// 任何入站帧都算保活（客户端可以只发业务帧而不发 ping）。
		_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))

		frame, err := DecodeFrame(data)
		if err != nil {
			c.protoErrs++
			c.SendError(errs.CodeInvalidParam.Code, "帧格式不正确")
			if c.protoErrs >= maxProtoErrors {
				c.CloseWith(CloseUnsupportedData, "连续协议错误")
				return
			}
			continue
		}
		c.protoErrs = 0

		if c.handler != nil {
			c.handler(c, frame)
		}
	}
}
