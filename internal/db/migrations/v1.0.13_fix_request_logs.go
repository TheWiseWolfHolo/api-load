package db

import (
	"gpt-load/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func V1_0_13_FixRequestLogs(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 如果有key_id，就执行修复
		if !tx.Migrator().HasColumn(&models.RequestLog{}, "key_id") {
			return nil
		}

		logrus.Info("Old schema detected. Starting data migration for request_logs...")

		if !tx.Migrator().HasColumn(&models.RequestLog{}, "group_name") {
			logrus.Info("Adding 'group_name' column to request_logs table...")
			if err := tx.Migrator().AddColumn(&models.RequestLog{}, "group_name"); err != nil {
				return err
			}
		}
		if !tx.Migrator().HasColumn(&models.RequestLog{}, "key_value") {
			logrus.Info("Adding 'key_value' column to request_logs table...")
			if err := tx.Migrator().AddColumn(&models.RequestLog{}, "key_value"); err != nil {
				return err
			}
		}

		type OldRequestLog struct {
			ID      string
			KeyID   uint `gorm:"column:key_id"`
			GroupID uint
		}

		batchSize := 1000
		for i := 0; ; i++ {
			logrus.Infof("Processing batch %d...", i+1)
			var oldLogs []OldRequestLog

			result := tx.Model(&models.RequestLog{}).
				Select("id", "key_id", "group_id").
				Where("key_value IS NULL OR group_name IS NULL").
				Limit(batchSize).
				Find(&oldLogs)

			if result.Error != nil {
				return result.Error
			}

			if len(oldLogs) == 0 {
				logrus.Info("All batches processed.")
				break
			}

			keyIDMap := make(map[uint]bool)
			groupIDMap := make(map[uint]bool)
			for _, logEntry := range oldLogs {
				if logEntry.KeyID > 0 {
					keyIDMap[logEntry.KeyID] = true
				}
				if logEntry.GroupID > 0 {
					groupIDMap[logEntry.GroupID] = true
				}
			}

			var apiKeys []models.APIKey
			if len(keyIDMap) > 0 {
				var keyIDs []uint
				for id := range keyIDMap {
					keyIDs = append(keyIDs, id)
				}
				if err := tx.Model(&models.APIKey{}).Where("id IN ?", keyIDs).Find(&apiKeys).Error; err != nil {
					return err
				}
			}
			keyValueMapping := make(map[uint]string)
			for _, key := range apiKeys {
				keyValueMapping[key.ID] = key.KeyValue
			}

			var groups []models.Group
			if len(groupIDMap) > 0 {
				var groupIDs []uint
				for id := range groupIDMap {
					groupIDs = append(groupIDs, id)
				}
				if err := tx.Model(&models.Group{}).Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
					return err
				}
			}
			groupNameMapping := make(map[uint]string)
			for _, group := range groups {
				groupNameMapping[group.ID] = group.Name
			}

			for _, logEntry := range oldLogs {
				groupName, gExists := groupNameMapping[logEntry.GroupID]
				if !gExists {
					logrus.Warnf("Log ID %s: Could not find Group for group_id %d. Setting group_name to empty string.", logEntry.ID, logEntry.GroupID)
				}

				keyValue, kExists := keyValueMapping[logEntry.KeyID]
				if !kExists {
					logrus.Warnf("Log ID %s: Could not find APIKey for key_id %d. Setting key_value to empty string.", logEntry.ID, logEntry.KeyID)
				}

				updates := map[string]any{
					"group_name": groupName,
					"key_value":  keyValue,
				}
				if err := tx.Model(&models.RequestLog{}).Where("id = ?", logEntry.ID).UpdateColumns(updates).Error; err != nil {
					logrus.WithError(err).Errorf("Failed to update log entry with ID: %s", logEntry.ID)
					continue
				}
			}
			logrus.Infof("Successfully updated %d log entries in batch %d.", len(oldLogs), i+1)
		}

		logrus.Info("Data migration complete. Dropping 'key_id' column from request_logs table...")
		if err := tx.Migrator().DropColumn(&models.RequestLog{}, "key_id"); err != nil {
			logrus.WithError(err).Warn("Failed to drop 'key_id' column. This can be done manually.")
		}

		logrus.Info("Database migration successful!")
		return nil
	})
}
