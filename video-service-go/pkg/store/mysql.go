// Package store 负责与外部存储中间件建立连接。
package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/example/video-service-go/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// NewMySQL 建立 GORM 连接并做一次 Ping 校验。
//
// 失败即返回错误（而非懒连接），让「配置写错」在启动阶段就暴露，
// 而不是等到第一个请求才 500。
func NewMySQL(cfg config.MySQL) (*gorm.DB, error) {
	level := gormlogger.Warn
	switch strings.ToLower(cfg.LogLevel) {
	case "silent":
		level = gormlogger.Silent
	case "error":
		level = gormlogger.Error
	case "info":
		level = gormlogger.Info
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(level),
		SkipDefaultTransaction: true,
		// 让唯一键冲突变成 gorm.ErrDuplicatedKey，避免上层去匹配驱动错误字符串
		TranslateError: true,
		NowFunc:        func() time.Time { return time.Now().Local() },
	})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 sql.DB 失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("MySQL Ping 失败: %w", err)
	}
	return db, nil
}
