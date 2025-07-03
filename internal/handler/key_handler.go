// Package handler provides HTTP handlers for the application
package handler

import (
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CreateKeysRequest struct {
	Keys []string `json:"keys" binding:"required"`
}

// CreateKeysInGroup handles creating new keys within a specific group.
func (s *Server) CreateKeysInGroup(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var req CreateKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	var newKeys []models.APIKey
	for _, keyVal := range req.Keys {
		newKeys = append(newKeys, models.APIKey{
			GroupID:  uint(groupID),
			KeyValue: keyVal,
			Status:   "active",
		})
	}

	if err := s.DB.Create(&newKeys).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, newKeys)
}

// ListKeysInGroup handles listing all keys within a specific group.
func (s *Server) ListKeysInGroup(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var keys []models.APIKey
	if err := s.DB.Where("group_id = ?", groupID).Find(&keys).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, keys)
}

// UpdateKey handles updating a specific key.
func (s *Server) UpdateKey(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	keyID, err := strconv.Atoi(c.Param("key_id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid key ID format"))
		return
	}

	var key models.APIKey
	if err := s.DB.Where("group_id = ? AND id = ?", groupID, keyID).First(&key).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	var updateData struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	key.Status = updateData.Status
	if err := s.DB.Save(&key).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, key)
}

type DeleteKeysRequest struct {
	KeyIDs []uint `json:"key_ids" binding:"required"`
}

// DeleteKeys handles deleting one or more keys.
func (s *Server) DeleteKeys(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var req DeleteKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if len(req.KeyIDs) == 0 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "No key IDs provided"))
		return
	}

	// Start a transaction
	tx := s.DB.Begin()

	// Verify all keys belong to the specified group
	var count int64
	if err := tx.Model(&models.APIKey{}).Where("id IN ? AND group_id = ?", req.KeyIDs, groupID).Count(&count).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	if count != int64(len(req.KeyIDs)) {
		tx.Rollback()
		response.Error(c, app_errors.NewAPIError(app_errors.ErrForbidden, "One or more keys do not belong to the specified group"))
		return
	}

	// Delete the keys
	if err := tx.Where("id IN ?", req.KeyIDs).Delete(&models.APIKey{}).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	tx.Commit()
	response.Success(c, gin.H{"message": "Keys deleted successfully"})
}
