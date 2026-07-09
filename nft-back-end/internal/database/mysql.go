package database

import (
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// OpenMySQL 创建 GORM MySQL 连接。
// 连接池在这里统一设置，调用方只负责传入已经校验过的 DSN。
func OpenMySQL(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}
