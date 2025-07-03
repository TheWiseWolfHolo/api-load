package handler

import (
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetSettings handles the GET /api/settings request.
// It retrieves all system settings, groups them by category, and returns them.
func GetSettings(c *gin.Context) {
	settingsManager := config.GetSystemSettingsManager()
	currentSettings := settingsManager.GetSettings()
	settingsInfo := config.GenerateSettingsMetadata(&currentSettings)

	// Group settings by category
	categorized := make(map[string][]models.SystemSettingInfo)
	for _, s := range settingsInfo {
		categorized[s.Category] = append(categorized[s.Category], s)
	}

	// Create the response structure
	var responseData []models.CategorizedSettings
	for categoryName, settings := range categorized {
		responseData = append(responseData, models.CategorizedSettings{
			CategoryName: categoryName,
			Settings:     settings,
		})
	}

	response.Success(c, responseData)
}

// UpdateSettings handles the PUT /api/settings request.
// It receives a key-value JSON object and updates system settings.
// After updating, it triggers a configuration reload.
func UpdateSettings(c *gin.Context) {
	var settingsMap map[string]string
	if err := c.ShouldBindJSON(&settingsMap); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if len(settingsMap) == 0 {
		response.Success(c, nil)
		return
	}

	settingsManager := config.GetSystemSettingsManager()

	// 更新配置
	if err := settingsManager.UpdateSettings(settingsMap); err != nil {
		response.InternalError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 重载系统配置
	if err := settingsManager.LoadFromDatabase(); err != nil {
		logrus.Errorf("Failed to reload system settings: %v", err)
		response.InternalError(c, "Failed to reload system settings")
		return
	}

	settingsManager.DisplayCurrentSettings()

	logrus.Info("Configuration reloaded successfully via API")
	response.Success(c, gin.H{
		"message": "Configuration reloaded successfully",
		"timestamp": gin.H{
			"reloaded_at": "now",
		},
	})

	response.Success(c, gin.H{
		"message": "Settings updated successfully. Configuration reloaded.",
	})
}
