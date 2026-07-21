package db

import (
	"api-load/internal/models"
	"fmt"

	"gorm.io/gorm"
)

// V1_2_0_SplitCredentialEnablement converts the legacy disabled health status
// into an independent manual enablement flag and repairs scheduling defaults.
func V1_2_0_SplitCredentialEnablement(db *gorm.DB) error {
	for _, model := range []any{&models.APIKey{}, &models.UpstreamResource{}} {
		if !db.Migrator().HasTable(model) {
			continue
		}
		if err := db.Model(model).
			Where("status = ?", models.KeyStatusDisabled).
			Updates(map[string]any{"enabled": false, "status": models.KeyStatusActive}).Error; err != nil {
			return fmt.Errorf("migrate legacy disabled credentials: %w", err)
		}
		if err := db.Model(model).Where("priority <= 0").Update("priority", models.DefaultCredentialPriority).Error; err != nil {
			return fmt.Errorf("repair credential priority: %w", err)
		}
		if err := db.Model(model).Where("weight <= 0").Update("weight", models.DefaultCredentialWeight).Error; err != nil {
			return fmt.Errorf("repair credential weight: %w", err)
		}
	}
	return nil
}
