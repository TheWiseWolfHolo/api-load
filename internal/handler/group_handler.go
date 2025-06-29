// Package handler provides HTTP handlers for the application
package handler

import (
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

	if err := s.DB.Create(&group).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create group")
		return
	}

	response.Success(c, group)
}

// ListGroups handles listing all groups.
func (s *Server) ListGroups(c *gin.Context) {
	var groups []models.Group
	if err := s.DB.Find(&groups).Error; err != nil {
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

	// We only allow updating certain fields
	group.Name = updateData.Name
	group.Description = updateData.Description
	group.ChannelType = updateData.ChannelType
	group.Config = updateData.Config

	if err := s.DB.Save(&group).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update group")
		return
	}

	response.Success(c, group)
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

	// Also delete associated API keys
	if err := tx.Where("group_id = ?", id).Delete(&models.APIKey{}).Error; err != nil {
		tx.Rollback()
		response.Error(c, http.StatusInternalServerError, "Failed to delete associated API keys")
		return
	}

	if err := tx.Delete(&models.Group{}, id).Error; err != nil {
		tx.Rollback()
		response.Error(c, http.StatusInternalServerError, "Failed to delete group")
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		response.Error(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	response.Success(c, gin.H{"message": "Group and associated keys deleted successfully"})
}