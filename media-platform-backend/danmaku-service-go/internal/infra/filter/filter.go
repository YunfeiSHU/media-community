package filter

import (
	"log/slog"
	"strings"
	"sync/atomic"
)

// Outcome 是过滤结果的动作类型。
type Outcome string

// 过滤结果取值。
const (
	// OutcomeAllow 无命中，原样放行。
	OutcomeAllow Outcome = "ALLOW"
	// OutcomeReplace 命中 REPLACE 词：内容已被替换，可放行。
	OutcomeReplace Outcome = "REPLACE"
	// OutcomeReject 命中 REJECT 词：拒绝发送。
	OutcomeReject Outcome = "REJECT"
	// OutcomeLog 命中 LOG 词：放行但记录。
	OutcomeLog Outcome = "LOG"
)

// Decision 是一次过滤的结论。
type Decision struct {
	Outcome  Outcome
	Content  string // 放行时应使用的内容（REPLACE 后可能与入参不同）
	Matched  string // 命中的词，未命中为空
	Strategy Strategy
}

// Allowed 表示是否允许继续发送。
func (d Decision) Allowed() bool { return d.Outcome != OutcomeReject }

// Filter 是敏感词过滤器。
//
// 读路径无锁：内部持有不可变 Trie，热重载时整体替换指针
// （见设计文档 §6.2：避免「重载期间匹配变慢」）。
type Filter struct {
	trie    atomic.Pointer[Trie]
	enabled bool
	def     Strategy
	log     *slog.Logger
}

// New 构造过滤器。enabled=false 时放行全部内容。
func New(enabled bool, defaultStrategy Strategy, log *slog.Logger) *Filter {
	if log == nil {
		log = slog.Default()
	}
	f := &Filter{enabled: enabled, def: defaultStrategy, log: log}
	f.trie.Store(NewTrie())
	return f
}

// Enabled 报告过滤器是否启用。
func (f *Filter) Enabled() bool { return f.enabled }

// Load 用新词库整体替换当前词库，返回加载的词数。
//
// 这是唯一的写入口，且写入的是**新构造的 Trie**，
// 旧 Trie 由 GC 回收 —— 读路径永远不会看到「半个词库」。
func (f *Filter) Load(words []Word) int {
	t := NewTrie()
	for _, w := range words {
		strategy := w.Strategy
		if strategy == "" {
			strategy = f.def
		}
		t.Insert(w.Text, strategy)
	}
	f.trie.Store(t)
	return t.Count()
}

// Count 返回当前词库词数（/readyz 用它暴露「敏感词是否加载」）。
func (f *Filter) Count() int { return f.trie.Load().Count() }

// DefaultStrategy 返回默认策略。
func (f *Filter) DefaultStrategy() Strategy { return f.def }

// Check 对内容执行过滤。
//
// 优先级：REJECT > REPLACE > LOG。
// 也就是说只要文本里出现任何一个 REJECT 词，无论是否同时命中其它策略，都拒绝
// —— 安全策略取最严格者，不做「部分替换后放行」的折中。
func (f *Filter) Check(content string) Decision {
	if !f.enabled {
		return Decision{Outcome: OutcomeAllow, Content: content}
	}

	t := f.trie.Load()
	if t.Empty() {
		// 词库为空不阻断业务：否则「词库没初始化」会变成全站无法发弹幕。
		return Decision{Outcome: OutcomeAllow, Content: content}
	}

	runes := []rune(content)
	matches := t.MatchAll(runes)
	if len(matches) == 0 {
		return Decision{Outcome: OutcomeAllow, Content: content}
	}

	for _, m := range matches {
		if m.Strategy == StrategyReject {
			return Decision{Outcome: OutcomeReject, Content: content, Matched: m.Word, Strategy: m.Strategy}
		}
	}

	var (
		out        []rune
		prev       int
		replaced   bool
		firstWord  string
		firstStrat Strategy
	)
	for _, m := range matches {
		if m.Strategy != StrategyReplace {
			continue
		}
		if !replaced {
			out = make([]rune, 0, len(runes))
			firstWord, firstStrat = m.Word, m.Strategy
		}
		out = append(out, runes[prev:m.Start]...)
		for i := m.Start; i < m.End; i++ {
			out = append(out, '*')
		}
		prev = m.End
		replaced = true
	}
	if replaced {
		out = append(out, runes[prev:]...)
		return Decision{Outcome: OutcomeReplace, Content: string(out), Matched: firstWord, Strategy: firstStrat}
	}

	first := matches[0]
	return Decision{Outcome: OutcomeLog, Content: content, Matched: first.Word, Strategy: first.Strategy}
}

// Describe 生成一行可读的命中描述，用于日志与响应头。
func (d Decision) Describe() string {
	if d.Matched == "" {
		return string(d.Outcome)
	}
	return string(d.Outcome) + ":" + d.Matched
}

// Mask 把内容的中间部分脱敏，用于日志（避免把违规内容原样写进日志）。
func Mask(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:2]) + strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-2:])
}
