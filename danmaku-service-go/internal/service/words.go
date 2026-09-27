package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/internal/infra/filter"
	"github.com/example/danmaku-service-go/internal/model"
	"github.com/example/danmaku-service-go/internal/repository"
	"github.com/example/danmaku-service-go/pkg/errs"
)

// maxWordLen 与 `media_sensitive_word.word` 的列长度一致。
const maxWordLen = 64

// LoadWords 从 MySQL 加载启用中的词库并整体替换前缀树，返回词数。
//
// 调用时机：启动时预热、`reload-words` 手动触发、新增词后自动触发。
func (s *DanmakuService) LoadWords(ctx context.Context) (int, error) {
	rows, err := s.Words.ListEnabled(ctx)
	if err != nil {
		return 0, fmt.Errorf("加载敏感词词库失败: %w", err)
	}

	words := make([]filter.Word, 0, len(rows))
	for _, row := range rows {
		words = append(words, filter.Word{
			Text:     row.Word,
			Strategy: filter.ParseStrategy(row.Strategy, s.Filter.DefaultStrategy()),
		})
	}

	count := s.Filter.Load(words)
	if count == 0 {
		// 词库为空时**不阻断**（放行全部内容）：否则「词库没初始化」
		// 会变成「全站无法发弹幕」，故障面被放大。
		s.logger().Warn("sensitive_word_empty",
			"hint", "词库为空，当前不过滤任何内容；请通过 /api/v1/danmaku/admin/words 添加")
	} else {
		s.logger().Info("sensitive_word_loaded", "words", count)
	}
	return count, nil
}

// ReloadWords 手动热重载（ADMIN）。
func (s *DanmakuService) ReloadWords(ctx context.Context) (*dto.ReloadWordsResult, error) {
	count, err := s.LoadWords(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.ReloadWordsResult{Words: count, ReloadedAt: time.Now()}, nil
}

// ListWords 返回全部词条（含停用），供管理端查看。
func (s *DanmakuService) ListWords(ctx context.Context) (*dto.WordListResult, error) {
	rows, err := s.Words.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询敏感词失败: %w", err)
	}
	items := make([]dto.WordDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.WordDTO{
			ID:       row.ID,
			Word:     row.Word,
			Strategy: row.Strategy,
			Enabled:  row.Enabled,
		})
	}
	return &dto.WordListResult{Items: items}, nil
}

// CreateWord 新增敏感词，成功后**自动热重载**（无需再手动调 reload-words）。
func (s *DanmakuService) CreateWord(ctx context.Context, req dto.CreateWordRequest) (*dto.WordDTO, error) {
	word := trim(req.Word)
	if word == "" {
		return nil, errs.CodeInvalidParam.New("word 不能为空")
	}
	if utf8.RuneCountInString(word) > maxWordLen {
		return nil, errs.CodeInvalidParam.Newf("word 长度不能超过 %d 个字符", maxWordLen)
	}

	raw := strings.ToUpper(trim(req.Strategy))
	strategy := s.Filter.DefaultStrategy()
	if raw != "" {
		switch filter.Strategy(raw) {
		case filter.StrategyReject, filter.StrategyReplace, filter.StrategyLog:
			strategy = filter.Strategy(raw)
		default:
			// 与 ParseStrategy 的「静默回退」不同：管理接口收到非法值必须报错，
			// 否则运营以为设成了 REPLACE，实际却按默认策略生效。
			return nil, errs.CodeInvalidParam.New("strategy 仅支持 REJECT / REPLACE / LOG")
		}
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now()
	row := model.SensitiveWord{
		Word:      word,
		Strategy:  string(strategy),
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Words.Create(ctx, &row); err != nil {
		if repository.IsDuplicated(err) {
			return nil, errs.CodeConflict.Newf("敏感词已存在: %s", word)
		}
		return nil, fmt.Errorf("新增敏感词失败: %w", err)
	}

	if _, err := s.LoadWords(ctx); err != nil {
		// 词已经落库，只是热重载失败：不能因为缓存刷新失败就把请求判为失败，
		// 否则运营会重复提交（虽然会被 40901 挡住，但体验很差）。
		s.logger().Warn("reload_after_create_failed", "word", word, "err", err.Error())
	}

	return &dto.WordDTO{
		ID:       row.ID,
		Word:     row.Word,
		Strategy: row.Strategy,
		Enabled:  row.Enabled,
	}, nil
}
