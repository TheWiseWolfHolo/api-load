// Package handler provides HTTP handlers for the application
package handler

import (
	"encoding/json"
	"fmt"
	"net/url"

	"gpt-load/internal/config"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gpt-load/internal/channel"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

// isValidChannelType checks if the channel type is valid by checking against the registered channels.
func isValidChannelType(channelType string) bool {
	channels := channel.GetChannels()
	for _, t := range channels {
		if t == channelType {
			return true
		}
	}
	return false
}

// UpstreamDefinition defines the structure for an upstream in the request.
type UpstreamDefinition struct {
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

// validateAndCleanUpstreams validates and cleans the upstreams JSON.
func validateAndCleanUpstreams(upstreams json.RawMessage) (datatypes.JSON, error) {
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("upstreams field is required")
	}

	var defs []UpstreamDefinition
	if err := json.Unmarshal(upstreams, &defs); err != nil {
		return nil, fmt.Errorf("invalid format for upstreams: %w", err)
	}

	if len(defs) == 0 {
		return nil, fmt.Errorf("at least one upstream is required")
	}

	for i := range defs {
		defs[i].URL = strings.TrimSpace(defs[i].URL)
		if defs[i].URL == "" {
			return nil, fmt.Errorf("upstream URL cannot be empty")
		}
		// Basic URL format validation
		if !strings.HasPrefix(defs[i].URL, "http://") && !strings.HasPrefix(defs[i].URL, "https://") {
			return nil, fmt.Errorf("invalid URL format for upstream: %s", defs[i].URL)
		}
		if defs[i].Weight <= 0 {
			return nil, fmt.Errorf("upstream weight must be a positive integer")
		}
	}

	cleanedUpstreams, err := json.Marshal(defs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cleaned upstreams: %w", err)
	}

	return cleanedUpstreams, nil
}

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

	// Strict check for unknown fields
	var cleanedMap map[string]any
	if err := json.Unmarshal(configBytes, &cleanedMap); err != nil {
		return nil, err
	}

	val := reflect.ValueOf(validatedConfig)
	typ := val.Type()
	validFields := make(map[string]bool)
	for i := 0; i < typ.NumField(); i++ {
		jsonTag := typ.Field(i).Tag.Get("json")
		fieldName := strings.Split(jsonTag, ",")[0]
		if fieldName != "" && fieldName != "-" {
			validFields[fieldName] = true
		}
	}

	for key := range configMap {
		if !validFields[key] {
			return nil, fmt.Errorf("unknown config field: '%s'", key)
		}
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

	// Marshal back to a map to ensure consistency
	validatedBytes, err := json.Marshal(validatedConfig)
	if err != nil {
		return nil, err
	}
	var finalMap map[string]any
	if err := json.Unmarshal(validatedBytes, &finalMap); err != nil {
		return nil, err
	}

	return finalMap, nil
}

// CreateGroup handles the creation of a new group.
func (s *Server) CreateGroup(c *gin.Context) {
	var req models.Group
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// Data Cleaning and Validation
	name := strings.TrimSpace(req.Name)
	if !isValidGroupName(name) {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid group name format. Use 3-30 lowercase letters, numbers, and underscores."))
		return
	}

	channelType := strings.TrimSpace(req.ChannelType)
	if !isValidChannelType(channelType) {
		supported := strings.Join(channel.GetChannels(), ", ")
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid channel type. Supported types are: %s", supported)))
		return
	}

	testModel := strings.TrimSpace(req.TestModel)
	if testModel == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Test model is required"))
		return
	}

	cleanedUpstreams, err := validateAndCleanUpstreams(json.RawMessage(req.Upstreams))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
		return
	}

	cleanedConfig, err := validateAndCleanConfig(req.Config)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid config format: %v", err)))
		return
	}

	group := models.Group{
		Name:           name,
		DisplayName:    strings.TrimSpace(req.DisplayName),
		Description:    strings.TrimSpace(req.Description),
		Upstreams:      cleanedUpstreams,
		ChannelType:    channelType,
		Sort:           req.Sort,
		TestModel:      testModel,
		ParamOverrides: req.ParamOverrides,
		Config:         cleanedConfig,
	}

	if err := s.DB.Create(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, s.newGroupResponse(&group))
}

// ListGroups handles listing all groups.
func (s *Server) ListGroups(c *gin.Context) {
	var groups []models.Group
	if err := s.DB.Order("sort asc, id desc").Find(&groups).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	var groupResponses []GroupResponse
	for i := range groups {
		groupResponses = append(groupResponses, *s.newGroupResponse(&groups[i]))
	}

	response.Success(c, groupResponses)
}

// GroupUpdateRequest defines the payload for updating a group.
// Using a dedicated struct avoids issues with zero values being ignored by GORM's Update.
type GroupUpdateRequest struct {
	Name           *string         `json:"name,omitempty"`
	DisplayName    *string         `json:"display_name,omitempty"`
	Description    *string         `json:"description,omitempty"`
	Upstreams      json.RawMessage `json:"upstreams"`
	ChannelType    *string         `json:"channel_type,omitempty"`
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

	// Apply updates from the request, with cleaning and validation
	if req.Name != nil {
		cleanedName := strings.TrimSpace(*req.Name)
		if !isValidGroupName(cleanedName) {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid group name format. Name is required and must be 3-30 lowercase letters, numbers, or underscores."))
			return
		}
		group.Name = cleanedName
	}

	if req.DisplayName != nil {
		group.DisplayName = strings.TrimSpace(*req.DisplayName)
	}

	if req.Description != nil {
		group.Description = strings.TrimSpace(*req.Description)
	}

	if req.Upstreams != nil {
		cleanedUpstreams, err := validateAndCleanUpstreams(req.Upstreams)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
			return
		}
		group.Upstreams = cleanedUpstreams
	}

	if req.ChannelType != nil {
		cleanedChannelType := strings.TrimSpace(*req.ChannelType)
		if !isValidChannelType(cleanedChannelType) {
			supported := strings.Join(channel.GetChannels(), ", ")
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid channel type. Supported types are: %s", supported)))
			return
		}
		group.ChannelType = cleanedChannelType
	}
	if req.Sort != nil {
		group.Sort = *req.Sort
	}
	if req.TestModel != "" {
		cleanedTestModel := strings.TrimSpace(req.TestModel)
		if cleanedTestModel == "" {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Test model cannot be empty or just spaces."))
			return
		}
		group.TestModel = cleanedTestModel
	}
	if req.ParamOverrides != nil {
		group.ParamOverrides = req.ParamOverrides
	}
	if req.Config != nil {
		cleanedConfig, err := validateAndCleanConfig(req.Config)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid config format: %v", err)))
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

	response.Success(c, s.newGroupResponse(&group))
}

// GroupResponse defines the structure for a group response, excluding sensitive or large fields.
type GroupResponse struct {
	ID              uint              `json:"id"`
	Name            string            `json:"name"`
	Endpoint        string            `json:"endpoint"`
	DisplayName     string            `json:"display_name"`
	Description     string            `json:"description"`
	Upstreams       datatypes.JSON    `json:"upstreams"`
	ChannelType     string            `json:"channel_type"`
	Sort            int               `json:"sort"`
	TestModel       string            `json:"test_model"`
	ParamOverrides  datatypes.JSONMap `json:"param_overrides"`
	Config          datatypes.JSONMap `json:"config"`
	LastValidatedAt *time.Time        `json:"last_validated_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// newGroupResponse creates a new GroupResponse from a models.Group.
func (s *Server) newGroupResponse(group *models.Group) *GroupResponse {
	appURL := s.SettingsManager.GetAppUrl()
	endpoint := ""
	if appURL != "" {
		u, err := url.Parse(appURL)
		if err == nil {
			u.Path = strings.TrimRight(u.Path, "/") + "/proxy/" + group.Name
			endpoint = u.String()
		}
	}

	return &GroupResponse{
		ID:              group.ID,
		Name:            group.Name,
		Endpoint:        endpoint,
		DisplayName:     group.DisplayName,
		Description:     group.Description,
		Upstreams:       group.Upstreams,
		ChannelType:     group.ChannelType,
		Sort:            group.Sort,
		TestModel:       group.TestModel,
		ParamOverrides:  group.ParamOverrides,
		Config:          group.Config,
		LastValidatedAt: group.LastValidatedAt,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
	}
}

// DeleteGroup handles deleting a group.
func (s *Server) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	// First, get all API keys for this group to clean up from memory store
	var apiKeys []models.APIKey
	if err := s.DB.Where("group_id = ?", id).Find(&apiKeys).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Extract key IDs for memory store cleanup
	var keyIDs []uint
	for _, key := range apiKeys {
		keyIDs = append(keyIDs, key.ID)
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

	// First check if the group exists
	var group models.Group
	if err := tx.First(&group, id).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Delete associated API keys first due to foreign key constraint
	if err := tx.Where("group_id = ?", id).Delete(&models.APIKey{}).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	// Then delete the group
	if err := tx.Delete(&models.Group{}, id).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Clean up memory store (Redis) within the transaction to ensure atomicity
	// If Redis cleanup fails, the entire transaction will be rolled back
	if len(keyIDs) > 0 {
		if err := s.KeyService.KeyProvider.RemoveKeysFromStore(uint(id), keyIDs); err != nil {
			tx.Rollback()
			logrus.WithFields(logrus.Fields{
				"groupID":  id,
				"keyCount": len(keyIDs),
				"error":    err,
			}).Error("Failed to remove keys from memory store, rolling back transaction")

			response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase,
				"Failed to delete group: unable to clean up cache"))
			return
		}
	}

	// Commit the transaction only if both DB and Redis operations succeed
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	response.Success(c, gin.H{"message": "Group and associated keys deleted successfully"})
}

// ConfigOption represents a single configurable option for a group.
type ConfigOption struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultValue any    `json:"default_value"`
}

// GetGroupConfigOptions returns a list of available configuration options for groups.
func (s *Server) GetGroupConfigOptions(c *gin.Context) {
	var options []ConfigOption

	// 1. Get all system setting definitions from the struct tags
	defaultSettings := config.DefaultSystemSettings()
	settingDefinitions := config.GenerateSettingsMetadata(&defaultSettings)
	defMap := make(map[string]models.SystemSettingInfo)
	for _, def := range settingDefinitions {
		defMap[def.Key] = def
	}

	// 2. Get current system setting values
	currentSettings := s.SettingsManager.GetSettings()
	currentSettingsValue := reflect.ValueOf(currentSettings)
	currentSettingsType := currentSettingsValue.Type()
	jsonToFieldMap := make(map[string]string)
	for i := 0; i < currentSettingsType.NumField(); i++ {
		field := currentSettingsType.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag != "" {
			jsonToFieldMap[jsonTag] = field.Name
		}
	}

	// 3. Iterate over GroupConfig fields to maintain order and build the response
	groupConfigType := reflect.TypeOf(models.GroupConfig{})

	for i := 0; i < groupConfigType.NumField(); i++ {
		field := groupConfigType.Field(i)
		jsonTag := field.Tag.Get("json")
		key := strings.Split(jsonTag, ",")[0]

		if key == "" || key == "-" {
			continue
		}

		if definition, ok := defMap[key]; ok {
			var defaultValue any
			if fieldName, ok := jsonToFieldMap[key]; ok {
				defaultValue = currentSettingsValue.FieldByName(fieldName).Interface()
			}

			option := ConfigOption{
				Key:          key,
				Name:         definition.Name,
				Description:  definition.Description,
				DefaultValue: defaultValue,
			}
			options = append(options, option)
		}
	}

	response.Success(c, options)
}
