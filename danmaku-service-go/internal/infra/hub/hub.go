package hub

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Backend 是广播后端抽象。
//
// 阶段一只有 memoryBackend（直接遍历本地 Room），阶段三接入多实例时
// 换成 Redis Pub/Sub 桥：本实例广播后 PUBLISH，各实例 PSUBSCRIBE 后
// 广播给本地连接。**接口先留出来，是为了让阶段三的改动不扩散到业务代码。**
type Backend interface {
	Name() string
	Broadcast(roomID string, msg []byte) int
}

// Hub 管理全部房间与连接。
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*Room

	cfg     Config
	log     *slog.Logger
	backend Backend

	dropped atomic.Int64
}

// New 构造 Hub。
func New(cfg Config, log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	if cfg.PingPeriod <= 0 {
		cfg.PingPeriod = cfg.PongWait / 2
	}
	h := &Hub{
		rooms: make(map[string]*Room),
		cfg:   cfg,
		log:   log,
	}
	h.backend = memoryBackend{hub: h}
	return h
}

// SubjectKey 把「直播房间 / 点播视频」统一成一个 Hub 房间标识。
//
// 直播与点播的 ID 空间不同（一个是 roomId，一个是 videoId），
// 加前缀后可以并存于同一个 Hub 而不互相串台。
func SubjectKey(roomID, videoID string) string {
	switch {
	case roomID != "":
		return "r:" + roomID
	case videoID != "":
		return "v:" + videoID
	default:
		return ""
	}
}

// Backend 返回当前广播后端（/readyz 展示用）。
func (h *Hub) Backend() Backend { return h.backend }

// Register 注册连接。
//
// 同时执行 FR-1.5：同一 uid 在同一房间的连接数超过上限时，
// 关掉该 uid **最旧**的连接（关闭码 4001，理由是「账号在其他地方打开」）。
// 只踢最旧而不是拒绝新连接：新连接大概率是用户当前的窗口。
func (h *Hub) Register(c *Client) {
	room := h.room(c.Subject, true)

	if c.UID != "" && h.cfg.MaxConnsPerUser > 0 {
		peers := room.ClientsByUID(c.UID)
		if len(peers) >= h.cfg.MaxConnsPerUser {
			excess := len(peers) - h.cfg.MaxConnsPerUser + 1
			for _, old := range peers[:excess] {
				old.CloseWith(CloseKickedByNewConn, "在其他地方打开")
			}
			h.log.Info("ws_kicked_duplicate_conn",
				"uid", c.UID, "subject", c.Subject, "kicked", excess)
		}
	}

	room.Add(c)
}

// Unregister 注销连接；房间空置时记录时间戳，交由 janitor 回收。
func (h *Hub) Unregister(c *Client) {
	h.mu.RLock()
	room := h.rooms[c.Subject]
	h.mu.RUnlock()
	if room == nil {
		return
	}
	room.Remove(c)
}

// Broadcast 向房间投递一帧，返回被丢弃的帧数。
func (h *Hub) Broadcast(roomID string, msg []byte) int {
	if roomID == "" || len(msg) == 0 {
		return 0
	}
	dropped := h.backend.Broadcast(roomID, msg)
	if dropped > 0 {
		h.dropped.Add(int64(dropped))
	}
	return dropped
}

// localBroadcast 是本实例的广播实现（由 memoryBackend 调用）。
func (h *Hub) localBroadcast(roomID string, msg []byte) int {
	h.mu.RLock()
	room := h.rooms[roomID]
	h.mu.RUnlock()
	if room == nil {
		// 没有连接不需要建房间：广播比连接早到是正常现象
		//（例如开播事件先于观众连接）。
		return 0
	}
	return room.Broadcast(msg)
}

// Online 返回房间在线连接数。
func (h *Hub) Online(roomID string) int {
	h.mu.RLock()
	room := h.rooms[roomID]
	h.mu.RUnlock()
	if room == nil {
		return 0
	}
	return room.Count()
}

// RoomCount 返回当前保有的房间数。
func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// ClientCount 返回当前连接总数。
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.RUnlock()

	total := 0
	for _, r := range rooms {
		total += r.Count()
	}
	return total
}

// Dropped 返回累计丢弃帧数（慢客户端背压指标）。
func (h *Hub) Dropped() int64 { return h.dropped.Load() }

// Stats 返回各房间连接数快照，供周期性日志（FR-7.2）使用。
func (h *Hub) Stats() map[string]int {
	h.mu.RLock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.RUnlock()

	out := make(map[string]int, len(rooms))
	for _, r := range rooms {
		if n := r.Count(); n > 0 {
			out[r.ID()] = n
		}
	}
	return out
}

// RunJanitor 周期性回收「空置超过宽限期」的房间。
//
// 为什么房间不立即销毁：观众进出频繁，立即销毁会导致回放热数据丢失、
// 重连时反复重建结构。60s 宽限期是内存与抖动的折中（设计文档 §4.4）。
func (h *Hub) RunJanitor(ctx context.Context) {
	interval := h.cfg.RoomIdleGrace / 2
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.reapIdleRooms()
		}
	}
}

func (h *Hub) reapIdleRooms() {
	now := time.Now()

	h.mu.Lock()
	reaped := make([]string, 0, 4)
	for id, room := range h.rooms {
		idle := room.IdleSince()
		if idle.IsZero() {
			continue
		}
		if now.Sub(idle) >= h.cfg.RoomIdleGrace {
			delete(h.rooms, id)
			reaped = append(reaped, id)
		}
	}
	h.mu.Unlock()

	if len(reaped) > 0 {
		h.log.Info("ws_rooms_reaped", "count", len(reaped), "rooms", reaped)
	}
}

// CloseAll 向所有连接广播结束消息并关闭（优雅退出）。
func (h *Hub) CloseAll(msg string, code int) {
	h.mu.RLock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.RUnlock()

	frame := MustFrame(TypeSystem, SystemData{
		Event:   EventServerShutdown,
		Message: msg,
		At:      time.Now(),
	})
	for _, r := range rooms {
		r.Broadcast(frame)
		r.CloseAll(code, msg)
	}
}

// room 查找房间；create=true 时不存在则创建。
func (h *Hub) room(id string, create bool) *Room {
	h.mu.RLock()
	room := h.rooms[id]
	h.mu.RUnlock()
	if room != nil {
		return room
	}
	if !create {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if room = h.rooms[id]; room != nil {
		return room
	}
	room = newRoom(id)
	h.rooms[id] = room
	return room
}

// memoryBackend 是单实例广播实现。
type memoryBackend struct{ hub *Hub }

// Name 实现 Backend。
func (memoryBackend) Name() string { return "memory" }

// Broadcast 实现 Backend。
func (b memoryBackend) Broadcast(roomID string, msg []byte) int {
	return b.hub.localBroadcast(roomID, msg)
}
