// Package handler provides HTTP handlers for the application
package handler

import (
	"encoding/json"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
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

// CreateGroup handles the creation of a new group.
func (s *Server) CreateGroup(c *gin.Context) {
	var group models.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validation
	if !isValidGroupName(group.Name) {
		response.Error(c, http.StatusBadRequest, "Invalid group name format. Use lowercase letters and underscores, and do not start with an underscore.")
		return
	}
	if len(group.Upstreams) == 0 {
		response.Error(c, http.StatusBadRequest, "At least one upstream is required")
		return
	}
	if group.ChannelType == "" {
		response.Error(c, http.StatusBadRequest, "Channel type is required")
		return
	}

	if err := s.DB.Create(&group).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create group")
		return
	}

	response.Success(c, group)
}

// ListGroups handles listing all groups.
func (s *Server) ListGroups(c *gin.Context) {
	var groups []models.Group
	if err := s.DB.Order("sort asc, id desc").Find(&groups).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list groups")
		return
	}
	response.Success(c, groups)
}

// UpdateGroup handles updating an existing group.
func (s *Server) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid group ID")
		return
	}

	var group models.Group
	if err := s.DB.First(&group, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "Group not found")
		return
	}

	var updateData models.Group
	if err := c.ShouldBindJSON(&updateData); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate group name if it's being updated
	if updateData.Name != "" && !isValidGroupName(updateData.Name) {
		response.Error(c, http.StatusBadRequest, "Invalid group name format. Use lowercase letters and underscores, and do not start with an underscore.")
		return
	}

	// Use a transaction to ensure atomicity
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}

	// Convert updateData to a map to ensure zero values (like Sort: 0) are updated
	var updateMap map[string]interface{}
	updateBytes, _ := json.Marshal(updateData)
	if err := json.Unmarshal(updateBytes, &updateMap); err != nil {
		response.Error(c, http.StatusBadRequest, "Failed to process update data")
		return
	}

	// If config is being updated, it needs to be marshalled to JSON string for GORM
	if config, ok := updateMap["config"]; ok {
		if configMap, isMap := config.(map[string]interface{}); isMap {
			configJSON, err := json.Marshal(configMap)
			if err != nil {
				response.Error(c, http.StatusBadRequest, "Failed to process config data")
				return
			}
			updateMap["config"] = string(configJSON)
		}
	}

	// Handle upstreams field specifically
	if upstreams, ok := updateMap["upstreams"]; ok {
		if upstreamsSlice, isSlice := upstreams.([]interface{}); isSlice {
			upstreamsJSON, err := json.Marshal(upstreamsSlice)
			if err != nil {
				response.Error(c, http.StatusBadRequest, "Failed to process upstreams data")
				return
			}
			updateMap["upstreams"] = string(upstreamsJSON)
		}
	}

	// Remove fields that are not actual columns or should not be updated from the map
	delete(updateMap, "id")
	delete(updateMap, "api_keys")
	delete(updateMap, "created_at")
	delete(updateMap, "updated_at")

	// Use Updates with a map to only update provided fields, including zero values
	if err := tx.Model(&group).Updates(updateMap).Error; err != nil {
		tx.Rollback()
		response.Error(c, http.StatusInternalServerError, "Failed to update group")
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		response.Error(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	// Re-fetch the group to return the updated data
	var updatedGroup models.Group
	if err := s.DB.First(&updatedGroup, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "Failed to fetch updated group data")
		return
	}

	response.Success(c, updatedGroup)
}

// DeleteGroup handles deleting a group.
func (s *Server) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid group ID")
		return
	}

	// Use a transaction to ensure atomicity
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to start transaction")
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
		response.Error(c, http.StatusInternalServerError, "Failed to delete associated API keys")
		return
	}

	if result := tx.Delete(&models.Group{}, id); result.Error != nil {
		tx.Rollback()
		response.Error(c, http.StatusInternalServerError, "Failed to delete group")
		return
	} else if result.RowsAffected == 0 {
		tx.Rollback()
		response.Error(c, http.StatusNotFound, "Group not found")
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		response.Error(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	response.Success(c, gin.H{"message": "Group and associated keys deleted successfully"})
}
