// Package db 数据库连接。测试用 glebarez/sqlite（纯 Go 无 CGO）。
package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"fitness/internal/config"
)

// OpenMySQL 打开生产/开发 MySQL 连接
func OpenMySQL(cfg config.DBConfig) (*gorm.DB, error) {
	gormLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             500 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
	})
	d, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{Logger: gormLogger})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败（%s:%d/%s）: %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	sqlDB, err := d.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(30)
	// 5 分钟回收连接：开发库走 VPN 隧道，长连接易被中间层静默掐断（表现为间歇性查询报错）
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	return d, nil
}

// OpenSQLite 测试用内存库（AutoMigrate 建表，不依赖 migrations/）。
// 每次调用生成独立命名空间，避免 cache=shared 内存库在测试间串数据；
// 同一返回值内的连接池共享同一份库。
func OpenSQLite() (*gorm.DB, error) {
	dsn := fmt.Sprintf("file:memdb_%d?mode=memory&cache=shared", time.Now().UnixNano())
	d, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return d, nil
}
