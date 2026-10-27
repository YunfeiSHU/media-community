// Package migrate 负责启动时的数据库结构同步。
//
// 阶段一使用 GORM AutoMigrate：只加字段、不删字段，适合开发迭代。
// 阶段四接入正式的版本化迁移工具（如 golang-migrate）后再切换。
package migrate

import (
	"fmt"
	"log/slog"

	"github.com/example/live-service-go/internal/model"
	"gorm.io/gorm"
)

// AutoMigrate 同步表结构。
//
// 注意：本服务与 Java 体系共用 auth_service 库，表名统一加 media_ 前缀
// （Java 侧的表为无前缀的 account/user/product/sku/inventory/order/payment_record）。
// AutoMigrate 只操作已注册的这两张表，不会碰 Java 的表。
func AutoMigrate(db *gorm.DB, log *slog.Logger) error {
	tables := []any{
		&model.Room{},
		&model.Record{},
	}
	if err := db.AutoMigrate(tables...); err != nil {
		return fmt.Errorf("自动建表失败: %w", err)
	}
	log.Info("db_migrated", "tables", []string{"media_live_room", "media_live_record"})
	return nil
}
