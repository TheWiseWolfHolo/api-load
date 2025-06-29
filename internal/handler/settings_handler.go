package handler

import (
	"gpt-load/internal/db"
	"gpt-load/internal/models"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// GetSettings handles the GET /api/settings request.
// It retrieves all system settings from the database and returns them as a key-value map.
func GetSettings(c *gin.Context) {
	var settings []models.SystemSetting
	if err := db.DB.Find(&settings).Error; err != nil {
		response.InternalError(c, "Failed to retrieve settings")
		return
	}

	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s.SettingValue
	}

	response.Success(c, settingsMap)
}

// UpdateSettings handles the PUT /api/settings request.
// It receives a key-value JSON object and updates or creates settings in the database.
func UpdateSettings(c *gin.Context) {
	var settingsMap map[string]string
	if err := c.ShouldBindJSON(&settingsMap); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	var settingsToUpdate []models.SystemSetting
	for key, value := range settingsMap {
		settingsToUpdate = append(settingsToUpdate, models.SystemSetting{
			SettingKey:   key,
			SettingValue: value,
		})
	}

	if len(settingsToUpdate) == 0 {
		response.Success(c, nil)
		return
	}

	// Using OnConflict to perform an "upsert" operation.
	// If a setting with the same key exists, it will be updated. Otherwise, a new one will be created.
	if err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "setting_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"setting_value"}),
	}).Create(&settingsToUpdate).Error; err != nil {
		response.InternalError(c, "Failed to update settings")
		return
	}

	response.Success(c, nil)
}