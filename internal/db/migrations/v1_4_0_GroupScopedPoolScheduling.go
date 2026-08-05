package db

import (
	"api-load/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// V1_4_0_GroupScopedPoolScheduling preserves the effective behavior of
// pre-v1.4 resource pools. Pool-bound groups previously always used affinity
// even when their hidden group config happened to contain round_robin.
func V1_4_0_GroupScopedPoolScheduling(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Group{}) {
		return nil
	}
	var groups []models.Group
	if err := db.Where("resource_pool_id IS NOT NULL").Find(&groups).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for i := range groups {
			config := groups[i].Config
			if config == nil {
				config = datatypes.JSONMap{}
			}
			if config["key_selection_strategy"] == "sticky" {
				continue
			}
			config["key_selection_strategy"] = "sticky"
			if err := tx.Model(&models.Group{}).Where("id = ?", groups[i].ID).Update("config", config).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
