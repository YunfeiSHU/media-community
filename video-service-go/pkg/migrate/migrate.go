// Package migrate 负责启动时的数据库结构同步与基础数据初始化。
//
// 阶段一使用 GORM AutoMigrate：只加字段、不删字段，适合开发迭代。
// 阶段四接入正式的版本化迁移工具（如 golang-migrate）后再切换。
package migrate

import (
	"fmt"
	"log/slog"

	"github.com/example/video-service-go/internal/model"
	"gorm.io/gorm"
)

// AutoMigrate 同步表结构。
//
// 注意：本服务与 Java 体系共用 auth_service 库，表名统一加 media_ 前缀
// （Java 侧的表为无前缀的 account/user/product/sku/inventory/order/payment_record）。
// AutoMigrate 只操作已注册的这 5 张表，不会碰 Java 的表。
func AutoMigrate(db *gorm.DB, log *slog.Logger) error {
	tables := []any{
		&model.Video{},
		&model.Category{},
		&model.VideoTag{},
		&model.TranscodeTask{},
		&model.UploadSession{},
	}
	if err := db.AutoMigrate(tables...); err != nil {
		return fmt.Errorf("自动建表失败: %w", err)
	}
	log.Info("db_migrated", "tables", []string{"media_video", "media_video_category", "media_video_tag", "media_transcode_task", "media_upload_session"})
	return nil
}

// SeedDefaultCategories 在分类表为空时写入默认分类。
// 目的是让「接口文档里的 curl 示例（categoryId=1）」开箱即可跑通。
func SeedDefaultCategories(db *gorm.DB, log *slog.Logger) {
	var count int64
	if err := db.Model(&model.Category{}).Count(&count).Error; err != nil {
		log.Warn("seed_category_count_failed", "error", err)
		return
	}
	if count > 0 {
		return
	}

	seeds := []model.Category{
		{Name: "编程", Sort: 1},
		{Name: "生活", Sort: 2},
		{Name: "游戏", Sort: 3},
	}
	if err := db.Create(&seeds).Error; err != nil {
		log.Warn("seed_categories_failed", "error", err)
		return
	}
	log.Info("seed_categories_done", "count", len(seeds))
}
