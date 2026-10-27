package repository

import (
	"context"

	"github.com/example/danmaku-service-go/internal/model"
	"gorm.io/gorm"
)

// WordRepository 定义敏感词词库操作。
type WordRepository interface {
	ListEnabled(ctx context.Context) ([]model.SensitiveWord, error)
	List(ctx context.Context) ([]model.SensitiveWord, error)
	Create(ctx context.Context, item *model.SensitiveWord) error
}

type wordRepository struct {
	db *gorm.DB
}

// NewWordRepository 构造 MySQL 实现。
func NewWordRepository(db *gorm.DB) WordRepository {
	return &wordRepository{db: db}
}

// ListEnabled 返回启用中的词条，供前缀树加载。
func (r *wordRepository) ListEnabled(ctx context.Context) ([]model.SensitiveWord, error) {
	items := make([]model.SensitiveWord, 0, 64)
	err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// List 返回全部词条（含停用），供管理接口查看。
func (r *wordRepository) List(ctx context.Context) ([]model.SensitiveWord, error) {
	items := make([]model.SensitiveWord, 0, 64)
	err := r.db.WithContext(ctx).Order("id ASC").Find(&items).Error
	return items, err
}

// Create 新增词条。
//
// 唯一键冲突在 TranslateError=true 的 GORM 连接上会返回 gorm.ErrDuplicatedKey，
// 由 service 层翻译成 40901，不在这里引入业务错误码。
func (r *wordRepository) Create(ctx context.Context, item *model.SensitiveWord) error {
	return r.db.WithContext(ctx).Create(item).Error
}
