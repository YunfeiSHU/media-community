// Package migrate 负责建表。
//
// 与 Java 体系共用同一个 `auth_service` 库，因此这里只 AutoMigrate 本服务的
// `media_*` 表，绝不触碰 Java 的表（`account` / `user` / `product` / `order` …）。
// 等价的幂等 DDL（可手工执行 / 给 DBA 审核）见 `deploy/db/go/media-tables.sql`。
package migrate

import (
	"log/slog"

	"github.com/example/danmaku-service-go/internal/model"
	"gorm.io/gorm"
)

// AutoMigrate 创建/补齐本服务的表结构。
func AutoMigrate(db *gorm.DB, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}

	if err := db.AutoMigrate(
		&model.Danmaku{},
		&model.SensitiveWord{},
	); err != nil {
		return err
	}

	log.Info("schema_migrated",
		"tables", []string{"media_danmaku", "media_sensitive_word"},
		"database", "auth_service（与 Java 共用）")
	return nil
}
