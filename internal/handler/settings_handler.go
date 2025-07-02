package handler

import (
	"gpt-load/internal/config"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetSettings handles the GET /api/settings request.
// It retrieves all system settings and returns them with detailed information.
func GetSettings(c *gin.Context) {
	settingsManager := config.GetSystemSettingsManager()
	currentSettings := settingsManager.GetSettings()

	// 使用新的动态元数据生成器
	settingsInfo := config.GenerateSettingsMetadata(&currentSettings)

	response.Success(c, settingsInfo)
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
