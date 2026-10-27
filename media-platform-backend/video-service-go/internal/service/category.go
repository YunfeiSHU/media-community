package service

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/infra/cache"
	"github.com/example/video-service-go/internal/model"
	"github.com/example/video-service-go/internal/repository"
	"github.com/example/video-service-go/pkg/errs"
)

// maxCategoryNameLen 是分类名长度上限。
const maxCategoryNameLen = 64

// CategoryService 是分类业务实现。
type CategoryService struct {
	cats  repository.CategoryRepository
	cache *cache.VideoCache
	log   *slog.Logger
}

// NewCategoryService 构造分类服务。
func NewCategoryService(cats repository.CategoryRepository, videoCache *cache.VideoCache, log *slog.Logger) *CategoryService {
	return &CategoryService{cats: cats, cache: videoCache, log: log}
}

// List 查询分类列表（带缓存）。
func (s *CategoryService) List(ctx context.Context) ([]dto.CategoryDTO, error) {
	if items, hit := s.cache.GetCategories(ctx); hit {
		return items, nil
	}

	rows, err := s.cats.List(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]dto.CategoryDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.CategoryDTO{ID: row.ID, Name: row.Name, Sort: row.Sort})
	}

	s.cache.SetCategories(ctx, items)
	return items, nil
}

// Create 新建分类（仅 ADMIN）。
func (s *CategoryService) Create(ctx context.Context, req dto.CreateCategoryRequest) (*dto.CategoryDTO, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errs.CodeInvalidParam.New("name 不能为空")
	}
	if utf8.RuneCountInString(name) > maxCategoryNameLen {
		return nil, errs.CodeInvalidParam.Newf("name 长度不能超过 %d 个字符", maxCategoryNameLen)
	}

	category := &model.Category{Name: name, Sort: req.Sort}
	if err := s.cats.Create(ctx, category); err != nil {
		if repository.IsDuplicated(err) {
			return nil, errs.CodeConflict.Newf("分类名 %s 已存在", name)
		}
		return nil, err
	}

	s.cache.DelCategories(ctx)
	s.log.Info("category_created", "categoryId", category.ID, "name", category.Name)

	return &dto.CategoryDTO{ID: category.ID, Name: category.Name, Sort: category.Sort}, nil
}
