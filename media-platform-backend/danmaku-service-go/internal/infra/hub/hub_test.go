package hub

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		ReadBufferSize:  1024,
		WriteBufferSize: 2, // 故意很小，方便验证「满则丢弃」
		WriteWait:       time.Second,
		PongWait:        time.Minute,
		PingPeriod:      time.Second,
		MaxConnsPerUser: 3,
		MaxFrameBytes:   4096,
		RoomIdleGrace:   time.Hour,
	}
}

func testHub(cfg Config) *Hub {
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newTestClient(h *Hub, cfg Config, id, subject, uid string) *Client {
	return NewClient(h, nil, ClientMeta{ID: id, Subject: subject, RoomID: "1", UID: uid}, cfg, nil, nil)
}

func TestBroadcastOnlyReachesSameRoom(t *testing.T) {
	cfg := testConfig()
	h := testHub(cfg)

	a := newTestClient(h, cfg, "a", "r:1", "")
	b := newTestClient(h, cfg, "b", "r:1", "")
	c := newTestClient(h, cfg, "c", "r:2", "")
	h.Register(a)
	h.Register(b)
	h.Register(c)

	if dropped := h.Broadcast("r:1", []byte("hello")); dropped != 0 {
		t.Fatalf("广播不应有丢弃，实际 %d", dropped)
	}

	if len(a.send) != 1 || len(b.send) != 1 {
		t.Fatalf("同房间连接应各收到 1 帧，实际 a=%d b=%d", len(a.send), len(b.send))
	}
	if len(c.send) != 0 {
		t.Fatalf("不同房间不应收到广播，实际 %d", len(c.send))
	}
	if got := h.Online("r:1"); got != 2 {
		t.Fatalf("Online 应为 2，实际 %d", got)
	}
}

// TestSlowClientIsDroppedNotBlocking 是「背压」这条非功能性需求的直接验证：
// 一个不消费消息的慢客户端只能让自己丢帧，不能让广播变慢或阻塞。
func TestSlowClientIsDroppedNotBlocking(t *testing.T) {
	cfg := testConfig()
	h := testHub(cfg)

	slow := newTestClient(h, cfg, "slow", "r:1", "")
	fast := newTestClient(h, cfg, "fast", "r:1", "")
	h.Register(slow)
	h.Register(fast)

	// slow 的缓冲只有 2，且没人消费；fast 每轮都被消费掉。
	for i := 0; i < 5; i++ {
		h.Broadcast("r:1", []byte("frame"))
		<-fast.send
	}

	if got := slow.Dropped(); got != 3 {
		t.Fatalf("慢客户端应丢弃 3 帧，实际 %d", got)
	}
	if got := fast.Dropped(); got != 0 {
		t.Fatalf("正常客户端不应丢弃，实际 %d", got)
	}
	if got := h.Dropped(); got != 3 {
		t.Fatalf("Hub 累计丢弃应为 3，实际 %d", got)
	}
}

func TestRegisterKicksOldestConnectionOfSameUser(t *testing.T) {
	cfg := testConfig()
	h := testHub(cfg)

	first := newTestClient(h, cfg, "c1", "r:1", "u-1")
	h.Register(first)
	for i := 0; i < 2; i++ {
		h.Register(newTestClient(h, cfg, "c2", "r:1", "u-1"))
	}

	// 第 4 条连接到达 → 最旧的（first）应被踢。
	h.Register(newTestClient(h, cfg, "c4", "r:1", "u-1"))

	select {
	case <-first.closeCh:
	default:
		t.Fatal("超过 max_conns_per_user 时最旧连接应被关闭")
	}
	if code, reason := first.closing(); code != CloseKickedByNewConn {
		t.Fatalf("关闭码应为 %d，实际 %d (%s)", CloseKickedByNewConn, code, reason)
	}

	// 不同 uid 不受影响。
	other := newTestClient(h, cfg, "other", "r:1", "u-2")
	h.Register(other)
	select {
	case <-other.closeCh:
		t.Fatal("不同 uid 的连接不应被踢")
	default:
	}
}

func TestUnregisterAndReapIdleRooms(t *testing.T) {
	cfg := testConfig()
	cfg.RoomIdleGrace = 20 * time.Millisecond
	h := testHub(cfg)

	c := newTestClient(h, cfg, "c1", "r:1", "")
	h.Register(c)
	if h.RoomCount() != 1 {
		t.Fatalf("应有 1 个房间，实际 %d", h.RoomCount())
	}
	h.Unregister(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.RunJanitor(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.RoomCount() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("空置房间应在宽限期后被回收，实际仍有 %d 个", h.RoomCount())
}

// TestHubConcurrentAccess 用 `go test -race` 覆盖注册/注销/广播并发。
func TestHubConcurrentAccess(t *testing.T) {
	cfg := testConfig()
	cfg.WriteBufferSize = 16
	h := testHub(cfg)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			subject := "r:1"
			if n%2 == 0 {
				subject = "r:2"
			}
			for j := 0; j < 40; j++ {
				c := newTestClient(h, cfg, "c", subject, "u")
				h.Register(c)
				h.Broadcast(subject, []byte("x"))
				_ = h.Online(subject)
				_ = h.ClientCount()
				h.Unregister(c)
				c.CloseWith(CloseNormalClosure, "done")
			}
		}(i)
	}
	wg.Wait()

	if got := h.ClientCount(); got != 0 {
		t.Fatalf("全部注销后连接数应为 0，实际 %d", got)
	}
}

func TestSubjectKeySeparatesLiveAndVideo(t *testing.T) {
	if got := SubjectKey("5001", ""); got != "r:5001" {
		t.Fatalf("直播标识错误: %s", got)
	}
	if got := SubjectKey("", "1001"); got != "v:1001" {
		t.Fatalf("点播标识错误: %s", got)
	}
	if got := SubjectKey("5001", "1001"); got != "r:5001" {
		t.Fatalf("同时存在时应优先 roomId: %s", got)
	}
	if got := SubjectKey("", ""); got != "" {
		t.Fatalf("都为空时应返回空串: %s", got)
	}
}

// TestCloseWithKeepsFirstNonZeroCode 守住一个真实踩过的坑：
// readPump 因连续协议错误 CloseWith(1003) 后返回，HTTP handler 收尾时
// 还会无脑 CloseWith(1000, "连接结束")。若后者覆盖前者，客户端永远只会
// 看到 1000，既看不到 1003，也会掩盖「被新连接踢掉」的 4001。
func TestCloseWithKeepsFirstNonZeroCode(t *testing.T) {
	cfg := testConfig()

	t.Run("协议错误优先于收尾的正常关闭", func(t *testing.T) {
		c := newTestClient(testHub(cfg), cfg, "a", "r:1", "u1")
		c.CloseWith(CloseUnsupportedData, "连续协议错误")
		c.CloseWith(CloseNormalClosure, "连接结束")

		code, reason := c.closing()
		if code != CloseUnsupportedData || reason != "连续协议错误" {
			t.Fatalf("关闭码应保持 1003/连续协议错误，实际 %d/%s", code, reason)
		}
	})

	t.Run("被踢的 4001 优先于收尾的正常关闭", func(t *testing.T) {
		c := newTestClient(testHub(cfg), cfg, "b", "r:1", "u1")
		c.CloseWith(CloseKickedByNewConn, "在其他地方打开了")
		c.CloseWith(CloseNormalClosure, "连接结束")

		if code, _ := c.closing(); code != CloseKickedByNewConn {
			t.Fatalf("关闭码应保持 4001，实际 %d", code)
		}
	})

	t.Run("没有既有关闭码时收尾的 1000 生效", func(t *testing.T) {
		c := newTestClient(testHub(cfg), cfg, "c", "r:1", "u1")
		c.CloseWith(CloseNormalClosure, "连接结束")

		if code, _ := c.closing(); code != CloseNormalClosure {
			t.Fatalf("关闭码应为 1000，实际 %d", code)
		}
	})

	t.Run("code 为 0 时沿用已有关闭码", func(t *testing.T) {
		c := newTestClient(testHub(cfg), cfg, "d", "r:1", "u1")
		c.CloseWith(CloseGoingAway, "服务端重启")
		c.Close() // Close 等价于 CloseWith(0, "")

		if code, _ := c.closing(); code != CloseGoingAway {
			t.Fatalf("关闭码应沿用 1001，实际 %d", code)
		}
	})

	t.Run("没人设置过时用默认的 1001", func(t *testing.T) {
		c := newTestClient(testHub(cfg), cfg, "e", "r:1", "u1")
		c.Close()

		if code, _ := c.closing(); code != CloseGoingAway {
			t.Fatalf("默认关闭码应为 1001，实际 %d", code)
		}
	})
}
