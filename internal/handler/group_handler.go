// Package handler provides HTTP handlers for the application
package handler

import (
	"encoding/json"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateGroup handles the creation of a new group.
func (s *Server) CreateGroup(c *gin.Context) {
	var group models.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validation
	if group.Name == "" {
		response.Error(c, http.StatusBadRequest, "Group name is required")
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

// GetGroup handles getting a single group by its ID.
func (s *Server) GetGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid group ID")
		return
	}

	var group models.Group
	if err := s.DB.Preload("APIKeys").First(&group, id).Error; err != nil {
		response.Error(c, http.StatusNotFound, "Group not found")
		return
	}

	response.Success(c, group)
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
	if err := s.DB.Preload("APIKeys").First(&updatedGroup, id).Error; err != nil {
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
