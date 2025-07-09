package handler

import (
	"gpt-load/internal/config"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetSettings handles the GET /api/settings request.
// It retrieves all system settings, groups them by category, and returns them.
func (s *Server) GetSettings(c *gin.Context) {
	currentSettings := s.SettingsManager.GetSettings()
	settingsInfo := config.GenerateSettingsMetadata(&currentSettings)

	// Group settings by category while preserving order
	categorized := make(map[string][]models.SystemSettingInfo)
	var categoryOrder []string
	for _, s := range settingsInfo {
		if _, exists := categorized[s.Category]; !exists {
			categoryOrder = append(categoryOrder, s.Category)
		}
		categorized[s.Category] = append(categorized[s.Category], s)
	}

	// Create the response structure in the correct order
	var responseData []models.CategorizedSettings
	for _, categoryName := range categoryOrder {
		responseData = append(responseData, models.CategorizedSettings{
			CategoryName: categoryName,
			Settings:     categorized[categoryName],
		})
	}

	response.Success(c, responseData)
}

// UpdateSettings handles the PUT /api/settings request.
// It receives a key-value JSON object and updates system settings.
// After updating, it triggers a configuration reload.
func (s *Server) UpdateSettings(c *gin.Context) {
	var settingsMap map[string]any
	if err := c.ShouldBindJSON(&settingsMap); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if len(settingsMap) == 0 {
		response.Success(c, nil)
		return
	}

	// 更新配置
	if err := s.SettingsManager.UpdateSettings(settingsMap); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, err.Error()))
		return
	}

	s.SettingsManager.DisplayCurrentSettings()

	logrus.Info("Settings update request processed. Invalidation notification sent.")
	response.Success(c, gin.H{
		"message": "Settings updated successfully. Configuration will be reloaded in the background across all instances.",
	})
}
