package ratelimit

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// New(nil, ...) 会走进程内降级实现，因此这些测试不依赖 Redis。
func TestUserLevelLimit(t *testing.T) {
	l := New(nil, 2, 100, time.Second, testLogger())
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		decision, err := l.Allow(ctx, "u-1", "room-1")
		if err != nil {
			t.Fatalf("第 %d 次判定出错: %v", i+1, err)
		}
		if !decision.Allowed {
			t.Fatalf("第 %d 次应当放行", i+1)
		}
	}

	decision, err := l.Allow(ctx, "u-1", "room-1")
	if err != nil {
		t.Fatalf("判定出错: %v", err)
	}
	if decision.Allowed {
		t.Fatal("超过用户级上限时应拒绝")
	}
	if decision.Scope != ScopeUser {
		t.Fatalf("拒绝层级应为 user，实际 %s", decision.Scope)
	}
	if decision.RetryAfterMs <= 0 {
		t.Fatalf("拒绝时应给出 retryAfterMs，实际 %d", decision.RetryAfterMs)
	}

	// 其他用户不受影响（限流 key 按 uid 隔离）。
	if d, _ := l.Allow(ctx, "u-2", "room-1"); !d.Allowed {
		t.Fatal("不同用户不应被 u-1 的计数影响")
	}
}

func TestRoomLevelLimitAndScope(t *testing.T) {
	// 用户上限很高，确保命中的一定是房间级。
	l := New(nil, 1000, 3, time.Second, testLogger())
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if d, _ := l.Allow(ctx, "u", "room-1"); !d.Allowed {
			t.Fatalf("第 %d 次应当放行", i+1)
		}
	}

	decision, _ := l.Allow(ctx, "u", "room-1")
	if decision.Allowed {
		t.Fatal("超过房间级上限时应拒绝")
	}
	if decision.Scope != ScopeRoom {
		t.Fatalf("拒绝层级应为 room，实际 %s", decision.Scope)
	}

	// **被房间级拒绝时不应回滚用户级计数**：换个房间仍然可以发。
	if d, _ := l.Allow(ctx, "u", "room-2"); !d.Allowed {
		t.Fatal("房间级拒绝不应影响其他房间")
	}
}

func TestWindowRollsOver(t *testing.T) {
	l := New(nil, 1, 100, 50*time.Millisecond, testLogger())
	ctx := context.Background()

	if d, _ := l.Allow(ctx, "u", "room"); !d.Allowed {
		t.Fatal("首次应当放行")
	}
	if d, _ := l.Allow(ctx, "u", "room"); d.Allowed {
		t.Fatal("同一窗口内第二次应被拒绝")
	}

	time.Sleep(80 * time.Millisecond) // 越过窗口
	if d, _ := l.Allow(ctx, "u", "room"); !d.Allowed {
		t.Fatal("新窗口应当重新放行")
	}
}

func TestEmptyUIDSkipsUserLevel(t *testing.T) {
	l := New(nil, 1, 100, time.Second, testLogger())
	ctx := context.Background()

	// uid 为空（理论上发送路径已挡住）时不应把所有人算成一个用户。
	for i := 0; i < 5; i++ {
		if d, _ := l.Allow(ctx, "", "room"); !d.Allowed {
			t.Fatalf("第 %d 次不应被用户级限流拒绝", i+1)
		}
	}
}
