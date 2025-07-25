package db

import (
	"fmt"

	"gorm.io/gorm"
)

// V1_0_16_IncreaseKeyValueLength migrates the key_value column length.
func V1_0_16_IncreaseKeyValueLength(db *gorm.DB) error {
	if err := alterColumnType(db, "api_keys", "key_value", "varchar(1024)"); err != nil {
		return fmt.Errorf("failed to migrate api_keys table: %w", err)
	}

	if err := alterColumnType(db, "request_logs", "key_value", "varchar(1024)"); err != nil {
		return fmt.Errorf("failed to migrate request_logs table: %w", err)
	}

	return nil
}

func alterColumnType(db *gorm.DB, tableName, columnName, newType string) error {
	var currentType string
	switch db.Dialector.Name() {
	case "sqlite":
		return nil
	case "mysql":
		err := db.Raw("SELECT COLUMN_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?",
			db.Migrator().CurrentDatabase(), tableName, columnName).Scan(&currentType).Error
		if err != nil {
			return err
		}
		if currentType == newType {
			return nil
		}
		return db.Exec(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s", tableName, columnName, newType)).Error
	case "postgres":
		err := db.Raw("SELECT data_type FROM information_schema.columns WHERE table_name = ? AND column_name = ?",
			tableName, columnName).Scan(&currentType).Error
		if err != nil {
			return err
		}
		return db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s", tableName, columnName, newType)).Error
	default:
		return nil
	}
}
