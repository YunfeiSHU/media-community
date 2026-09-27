package filter

import (
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestTrieMatchAllSkipsNoiseAndFoldsWidth(t *testing.T) {
	trie := NewTrie()
	trie.Insert("badword", StrategyReject)
	trie.Insert("测试词", StrategyReject)

	cases := []struct {
		name      string
		text      string
		wantHit   bool
		wantStart int
		wantEnd   int
	}{
		{"原样", "xxbadwordxx", true, 2, 9},
		{"中间插空格", "xxbad wordxx", true, 2, 10},
		{"中间插星号与点", "xxb*a.d*w*o*r*dxx", true, 2, 15},
		{"大小写混写", "xxBaDwOrDxx", true, 2, 9},
		{"全角字符", "xxｂａｄｗｏｒｄxx", true, 2, 9},
		{"中文词", "这里测试词出现", true, 2, 5},
		{"无命中", "everything is fine", false, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matches := trie.MatchAll([]rune(tc.text))
			if !tc.wantHit {
				if len(matches) != 0 {
					t.Fatalf("期望无命中，实际 %+v", matches)
				}
				return
			}
			if len(matches) != 1 {
				t.Fatalf("期望 1 个命中，实际 %d 个: %+v", len(matches), matches)
			}
			if matches[0].Start != tc.wantStart || matches[0].End != tc.wantEnd {
				t.Fatalf("命中区间错误：want [%d,%d) got [%d,%d)",
					tc.wantStart, tc.wantEnd, matches[0].Start, matches[0].End)
			}
		})
	}
}

func TestTrieMatchAllLongestAndNonOverlapping(t *testing.T) {
	trie := NewTrie()
	trie.Insert("ab", StrategyReject)
	trie.Insert("abcd", StrategyReject)

	matches := trie.MatchAll([]rune("zabcdz"))
	if len(matches) != 1 {
		t.Fatalf("期望 1 个命中（最长且不重叠），实际 %+v", matches)
	}
	if matches[0].Word != "abcd" {
		t.Fatalf("期望命中 abcd（最长），实际 %q", matches[0].Word)
	}
	if matches[0].Start != 1 || matches[0].End != 5 {
		t.Fatalf("区间错误: %+v", matches[0])
	}
}

func TestFilterRejectTakesPrecedence(t *testing.T) {
	f := New(true, StrategyReject, testLogger())
	f.Load([]Word{
		{Text: "badword", Strategy: StrategyReject},
		{Text: "广告词", Strategy: StrategyReplace},
	})

	// 同时命中 REJECT 与 REPLACE 时必须拒绝，而不是「替换后放行」。
	decision := f.Check("badword 与 广告词 同时出现")
	if decision.Outcome != OutcomeReject {
		t.Fatalf("期望 REJECT，实际 %s", decision.Outcome)
	}
	if decision.Matched != "badword" {
		t.Fatalf("期望匹配到 badword，实际 %q", decision.Matched)
	}
}

func TestFilterReplaceOnlyTouchesMatchedSpan(t *testing.T) {
	f := New(true, StrategyReject, testLogger())
	f.Load([]Word{{Text: "广告词示例", Strategy: StrategyReplace}})

	decision := f.Check("这里广告词示例出现")
	if decision.Outcome != OutcomeReplace {
		t.Fatalf("期望 REPLACE，实际 %s", decision.Outcome)
	}
	if decision.Content != "这里*****出现" {
		t.Fatalf("替换结果错误: %q", decision.Content)
	}
}

func TestFilterLogOnlyAllowsContent(t *testing.T) {
	f := New(true, StrategyReject, testLogger())
	f.Load([]Word{{Text: "测试词", Strategy: StrategyLog}})

	decision := f.Check("这里测试词出现")
	if decision.Outcome != OutcomeLog {
		t.Fatalf("期望 LOG，实际 %s", decision.Outcome)
	}
	if !decision.Allowed() {
		t.Fatal("LOG 策略必须放行")
	}
	if decision.Content != "这里测试词出现" {
		t.Fatalf("LOG 不应修改内容: %q", decision.Content)
	}
}

func TestFilterEmptyLibraryAllowsEverything(t *testing.T) {
	f := New(true, StrategyReject, testLogger())
	f.Load(nil)

	if f.Count() != 0 {
		t.Fatalf("期望词库为空，实际 %d", f.Count())
	}
	if decision := f.Check("任意内容都放行"); decision.Outcome != OutcomeAllow {
		t.Fatalf("词库为空时必须放行，实际 %s", decision.Outcome)
	}
}

func TestFilterDisabledAllowsEverything(t *testing.T) {
	f := New(false, StrategyReject, testLogger())
	f.Load([]Word{{Text: "badword", Strategy: StrategyReject}})

	if decision := f.Check("badword"); decision.Outcome != OutcomeAllow {
		t.Fatalf("过滤器关闭时必须放行，实际 %s", decision.Outcome)
	}
}

// TestFilterReloadUnderConcurrentRead 用 `go test -race` 验证
// 「热重载时整体替换指针」确实让读路径无锁且无数据竞争。
func TestFilterReloadUnderConcurrentRead(t *testing.T) {
	f := New(true, StrategyReject, testLogger())
	f.Load([]Word{{Text: "旧词", Strategy: StrategyReject}})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = f.Check("这里可能含旧词或新词")
					_ = f.Count()
				}
			}
		}()
	}

	for i := 0; i < 50; i++ {
		f.Load([]Word{{Text: "新词", Strategy: StrategyReplace}})
		f.Load([]Word{{Text: "旧词", Strategy: StrategyReject}})
	}
	close(stop)
	wg.Wait()
}

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Ａ Ｂ C":     "abc",
		"广 告*词.示-例": "广告词示例",
		"MiXeD":     "mixed",
	}
	for in, want := range cases {
		if got := Fold(in); got != want {
			t.Fatalf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskKeepsEdgeRunes(t *testing.T) {
	if got := Mask("广告词示例"); got != "广告*示例" {
		t.Fatalf("Mask 结果错误: %q", got)
	}
	if got := Mask("ab"); !strings.Contains(got, "*") {
		t.Fatalf("短内容也应脱敏: %q", got)
	}
}
