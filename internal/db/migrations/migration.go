package db

import (
	"gorm.io/gorm"
)

func MigrateDatabase(db *gorm.DB) error {
	// v1.0.13 修复请求日志数据
	return V1_0_13_FixRequestLogs(db)
}
