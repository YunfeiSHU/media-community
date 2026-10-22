package hub

import (
	"sort"
	"sync"
	"time"
)

// Room 是「一个房间（或一个点播视频）」的连接集合。
//
// 点播弹幕复用同一结构：Hub 的 key 用 `v:{videoId}`，
// 广播语义与直播完全一致（只是没有「开播/关播」概念）。
type Room struct {
	id string

	mu        sync.RWMutex
	clients   map[*Client]struct{}
	lastEmpty time.Time

	dropped int64
}

func newRoom(id string) *Room {
	return &Room{id: id, clients: make(map[*Client]struct{})}
}

// ID 返回房间标识。
func (r *Room) ID() string { return r.id }

// Add 加入一个连接。
func (r *Room) Add(c *Client) {
	r.mu.Lock()
	r.clients[c] = struct{}{}
	r.lastEmpty = time.Time{}
	r.mu.Unlock()
}

// Remove 移除一个连接，返回剩余连接数。
func (r *Room) Remove(c *Client) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
	if len(r.clients) == 0 {
		r.lastEmpty = time.Now()
	}
	return len(r.clients)
}

// Count 返回当前连接数。
func (r *Room) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// IdleSince 返回「空置起始时间」；当前非空时返回零值。
func (r *Room) IdleSince() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.clients) > 0 {
		return time.Time{}
	}
	return r.lastEmpty
}

// Dropped 返回该房间累计丢弃的帧数（慢客户端背压打点）。
func (r *Room) Dropped() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dropped
}

// ClientsByUID 返回同一 uid 的已连接客户端，按「连接建立顺序」升序（用于多端连接数限制）。
//
// 排序用 Seq 而不是 ConnectedAt：后者的时间戳在粗粒度时钟平台（Windows 约 15.6ms）
// 上可能完全相同，会让调用方「取最旧的 N 条」拿到随机结果。
func (r *Room) ClientsByUID(uid string) []*Client {
	if uid == "" {
		return nil
	}
	r.mu.RLock()
	out := make([]*Client, 0, 4)
	for c := range r.clients {
		if c.UID == uid {
			out = append(out, c)
		}
	}
	r.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// Broadcast 把消息投递给房间内所有连接，返回被丢弃的帧数。
//
// 两个关键点（设计文档 §4.3）：
//  1. **先快照再遍历**：避免长时间持读锁，否则 Register/Unregister 的写锁会被饿死；
//  2. **非阻塞投递**：send channel 满直接丢弃并计数，绝不让慢客户端阻塞整个房间。
func (r *Room) Broadcast(msg []byte) int {
	snapshot := r.snapshot()
	dropped := 0
	for _, c := range snapshot {
		if !c.Send(msg) {
			dropped++
		}
	}
	if dropped > 0 {
		r.mu.Lock()
		r.dropped += int64(dropped)
		r.mu.Unlock()
	}
	return dropped
}

// CloseAll 关闭房间内所有连接（服务端优雅退出 / 房间被强制关闭）。
func (r *Room) CloseAll(code int, reason string) {
	for _, c := range r.snapshot() {
		c.CloseWith(code, reason)
	}
}

func (r *Room) snapshot() []*Client {
	r.mu.RLock()
	out := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		out = append(out, c)
	}
	r.mu.RUnlock()
	return out
}
