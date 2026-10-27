// Package filter 实现基于前缀树（Trie）的敏感词匹配。
//
// 为什么自己写而不引第三方库：核心只需「插入 + 扫描匹配」两个操作、零外部依赖，
// 而且本项目最需要说清楚的是**归一化**与**策略**这两件事，
// 用第三方库反而看不清。真实生产的内容审核（AI 语义/图像）是阶段四的事，
// 这里不假装已经解决（见设计文档 §6.3）。
package filter

import (
	"strings"
	"unicode"
)

// Strategy 是命中敏感词后的处置策略。
type Strategy string

// 策略取值，与 `media_sensitive_word.strategy` 字段一致。
const (
	// StrategyReject 直接拒绝：不入库、不广播（默认）。
	StrategyReject Strategy = "REJECT"
	// StrategyReplace 把命中片段替换为等长的 `*` 后放行。
	StrategyReplace Strategy = "REPLACE"
	// StrategyLog 放行但记录日志（用于灰度观察新词）。
	StrategyLog Strategy = "LOG"
)

// ParseStrategy 把字符串归一化为策略枚举，未知取值回退到 fallback。
func ParseStrategy(raw string, fallback Strategy) Strategy {
	switch Strategy(strings.ToUpper(strings.TrimSpace(raw))) {
	case StrategyReject:
		return StrategyReject
	case StrategyReplace:
		return StrategyReplace
	case StrategyLog:
		return StrategyLog
	default:
		return fallback
	}
}

// Word 是词库中的一条记录。
type Word struct {
	Text     string
	Strategy Strategy
}

// Match 描述一次命中：下标是**原始文本的 rune 下标**（左闭右开）。
//
// 之所以用原始下标而不是归一化后的下标：REPLACE 策略要按位替换用户原文，
// 若用归一化文本的下标，替换结果会把用户原本的空格/全角字符一起吃掉。
type Match struct {
	Start    int
	End      int
	Word     string
	Strategy Strategy
}

// node 是前缀树节点。
type node struct {
	children map[rune]*node
	word     string
	strategy Strategy
}

// Trie 是敏感词前缀树。
//
// 并发约定：构建完成后**只读**，由 Filter 用 atomic.Pointer 整体替换，
// 因此读路径完全无锁。
type Trie struct {
	root  *node
	count int
}

// NewTrie 构造空前缀树。
func NewTrie() *Trie {
	return &Trie{root: &node{children: make(map[rune]*node)}}
}

// Insert 插入一个敏感词。空词或全分隔符的词会被忽略。
func (t *Trie) Insert(word string, strategy Strategy) {
	folded := []rune(Fold(word))
	if len(folded) == 0 {
		return
	}
	cur := t.root
	for _, r := range folded {
		if isSeparator(r) {
			// 词条自身含分隔符（如 "广 告"）时按归一化结果入库，
			// 运行时扫描也会跳过分隔符，两边语义一致。
			continue
		}
		next, ok := cur.children[r]
		if !ok {
			next = &node{children: make(map[rune]*node)}
			cur.children[r] = next
		}
		cur = next
	}
	if cur.word == "" {
		t.count++
	}
	cur.word = string(folded)
	cur.strategy = strategy
}

// Count 返回已加载的敏感词数量。
func (t *Trie) Count() int {
	if t == nil {
		return 0
	}
	return t.count
}

// Empty 表示词库为空（此时应放行全部内容，避免误伤）。
func (t *Trie) Empty() bool { return t.Count() == 0 }

// MatchAll 扫描文本，返回**互不重叠**的全部命中（按起始位置升序）。
//
// 实现要点：
//  1. 对每个非分隔符起点向后走树，跳过中间的分隔符（空格 / * / .），
//     因此 "f u*c.k" 能命中 "fuck"；
//  2. 记录「走过的最深的成词节点」，即该起点上的最长命中；
//  3. 命中后从命中结束位置继续扫描，避免重叠命中导致重复替换。
func (t *Trie) MatchAll(text []rune) []Match {
	if t == nil || t.root == nil || t.count == 0 {
		return nil
	}

	var out []Match
	for i := 0; i < len(text); {
		if isSeparator(text[i]) {
			i++
			continue
		}

		cur := t.root
		end := -1
		var word string
		var strategy Strategy

		for j := i; j < len(text); j++ {
			if isSeparator(text[j]) {
				// 分隔符只是噪声：跳过它，但不移动树节点。
				continue
			}
			next, ok := cur.children[FoldRune(text[j])]
			if !ok {
				break
			}
			cur = next
			if cur.word != "" {
				end = j + 1
				word = cur.word
				strategy = cur.strategy
			}
		}

		if end > 0 {
			out = append(out, Match{Start: i, End: end, Word: word, Strategy: strategy})
			i = end
			continue
		}
		i++
	}
	return out
}

// isSeparator 判断是否为「用户用来绕过匹配」的装饰性字符。
//
// 只收窄到空格与几种最常见的插入符号：过宽的集合（如所有标点）
// 会把 "a-b" 与 "ab" 混为一谈，带来误伤。
func isSeparator(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '*', '.', '-', '_', '~', '|':
		return true
	default:
		return false
	}
}

// FoldRune 做单字符归一化：全角转半角 + 转小写。
func FoldRune(r rune) rune {
	switch {
	case r >= 0xFF01 && r <= 0xFF5E:
		// 全角 ASCII 区
		r -= 0xFEE0
	case r == 0x3000:
		// 全角空格
		r = ' '
	}
	return unicode.ToLower(r)
}

// Fold 做整串归一化：全角转半角 + 转小写 + 去掉装饰字符。
//
// 繁简转换**没有**实现（设计文档 §14 已如实标注该缺口）。
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		r = FoldRune(r)
		if isSeparator(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
