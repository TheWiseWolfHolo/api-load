// Package handler provides HTTP handlers for the application
package handler

import (
	"encoding/json"
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// isValidGroupName checks if the group name is valid.
func isValidGroupName(name string) bool {
	if name == "" {
		return false
	}
	// 允许使用小写字母、数字和下划线，长度在 3 到 30 个字符之间
	match, _ := regexp.MatchString("^[a-z0-9_]{3,30}$", name)
	return match
}

// validateAndCleanConfig validates the group config against the GroupConfig struct.
func validateAndCleanConfig(configMap map[string]any) (map[string]any, error) {
	if configMap == nil {
		return nil, nil
	}

	configBytes, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}

	var validatedConfig models.GroupConfig
	if err := json.Unmarshal(configBytes, &validatedConfig); err != nil {
		return nil, err
	}

	// 验证配置项的合理范围
	if validatedConfig.BlacklistThreshold != nil && *validatedConfig.BlacklistThreshold < 0 {
		return nil, fmt.Errorf("blacklist_threshold must be >= 0")
	}
	if validatedConfig.MaxRetries != nil && (*validatedConfig.MaxRetries < 0 || *validatedConfig.MaxRetries > 10) {
		return nil, fmt.Errorf("max_retries must be between 0 and 10")
	}
	if validatedConfig.RequestTimeout != nil && (*validatedConfig.RequestTimeout < 1 || *validatedConfig.RequestTimeout > 3600) {
		return nil, fmt.Errorf("request_timeout must be between 1 and 3600 seconds")
	}
	if validatedConfig.KeyValidationIntervalMinutes != nil && (*validatedConfig.KeyValidationIntervalMinutes < 5 || *validatedConfig.KeyValidationIntervalMinutes > 1440) {
		return nil, fmt.Errorf("key_validation_interval_minutes must be between 5 and 1440 minutes")
	}

	// Marshal back to a map to remove any fields not in GroupConfig
	validatedBytes, err := json.Marshal(validatedConfig)
	if err != nil {
		return nil, err
	}

	var cleanedMap map[string]any
	if err := json.Unmarshal(validatedBytes, &cleanedMap); err != nil {
		return nil, err
	}

	return cleanedMap, nil
}

// CreateGroup handles the creation of a new group.
func (s *Server) CreateGroup(c *gin.Context) {
	var group models.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// Validation
	if !isValidGroupName(group.Name) {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid group name format. Use 3-30 lowercase letters, numbers, and underscores."))
		return
	}
	if len(group.Upstreams) == 0 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "At least one upstream is required"))
		return
	}
	if group.ChannelType == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Channel type is required"))
		return
	}
	if group.TestModel == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Test model is required"))
		return
	}

	cleanedConfig, err := validateAndCleanConfig(group.Config)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid config format"))
		return
	}
	group.Config = cleanedConfig

	if err := s.DB.Create(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, group)
}

// ListGroups handles listing all groups.
func (s *Server) ListGroups(c *gin.Context) {
	var groups []models.Group
	if err := s.DB.Order("sort asc, id desc").Find(&groups).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}
	response.Success(c, groups)
}

// GroupUpdateRequest defines the payload for updating a group.
// Using a dedicated struct avoids issues with zero values being ignored by GORM's Update.
type GroupUpdateRequest struct {
	Name           string          `json:"name"`
	DisplayName    string          `json:"display_name"`
	Description    string          `json:"description"`
	Upstreams      json.RawMessage `json:"upstreams"`
	ChannelType    string          `json:"channel_type"`
	Sort           *int            `json:"sort"`
	TestModel      string          `json:"test_model"`
	ParamOverrides map[string]any  `json:"param_overrides"`
	Config         map[string]any  `json:"config"`
}

// UpdateGroup handles updating an existing group.
func (s *Server) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var group models.Group
	if err := s.DB.First(&group, id).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	var req GroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// Start a transaction
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer tx.Rollback() // Rollback on panic

	// Apply updates from the request
	if req.Name != "" {
		if !isValidGroupName(req.Name) {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid group name format."))
			return
		}
		group.Name = req.Name
	}
	if req.DisplayName != "" {
		group.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		group.Description = req.Description
	}
	if req.Upstreams != nil {
		group.Upstreams = datatypes.JSON(req.Upstreams)
	}
	if req.ChannelType != "" {
		group.ChannelType = req.ChannelType
	}
	if req.Sort != nil {
		group.Sort = *req.Sort
	}
	if req.TestModel != "" {
		group.TestModel = req.TestModel
	}
	if req.ParamOverrides != nil {
		group.ParamOverrides = req.ParamOverrides
	}
	if req.Config != nil {
		cleanedConfig, err := validateAndCleanConfig(req.Config)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid config format"))
			return
		}
		group.Config = cleanedConfig
	}

	// Save the updated group object
	if err := tx.Save(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	if err := tx.Commit().Error; err != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	response.Success(c, group)
}

// DeleteGroup handles deleting a group.
func (s *Server) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	// Use a transaction to ensure atomicity
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Also delete associated API keys
	if err := tx.Where("group_id = ?", id).Delete(&models.APIKey{}).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	if result := tx.Delete(&models.Group{}, id); result.Error != nil {
		tx.Rollback()
		response.Error(c, app_errors.ParseDBError(result.Error))
		return
	} else if result.RowsAffected == 0 {
		tx.Rollback()
		response.Error(c, app_errors.ErrResourceNotFound)
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	response.Success(c, gin.H{"message": "Group and associated keys deleted successfully"})
}
