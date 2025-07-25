package db

import (
	"gorm.io/gorm"
)

func MigrateDatabase(db *gorm.DB) error {
	// v1.0.13 修复请求日志数据
	if err := V1_0_13_FixRequestLogs(db); err != nil {
		return err
	}
	// v1.0.16 增加 key_value 字段长度
	if err := V1_0_16_IncreaseKeyValueLength(db); err != nil {
		return err
	}
	return nil
}
