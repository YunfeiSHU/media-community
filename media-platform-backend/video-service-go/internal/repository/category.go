package repository

import (
	"context"
	"errors"

	"github.com/example/video-service-go/internal/model"
	"gorm.io/gorm"
)

// CategoryRepository 是分类的数据访问接口。
type CategoryRepository interface {
	Create(ctx context.Context, c *model.Category) error
	GetByID(ctx context.Context, id uint64) (*model.Category, error)
	List(ctx context.Context) ([]model.Category, error)
	NamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository 构造分类仓储。
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(ctx context.Context, c *model.Category) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *categoryRepository) GetByID(ctx context.Context, id uint64) (*model.Category, error) {
	var c model.Category
	err := r.db.WithContext(ctx).First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepository) List(ctx context.Context) ([]model.Category, error) {
	var items []model.Category
	err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}

// NamesByIDs 批量取分类名，返回 id → name。
func (r *categoryRepository) NamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	var rows []model.Category
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}
