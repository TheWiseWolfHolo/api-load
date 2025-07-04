package handler

import (
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// validateGroupID validates and parses group ID from request parameter
func validateGroupID(c *gin.Context) (uint, error) {
	groupIDStr := c.Param("id")
	if groupIDStr == "" {
		return 0, fmt.Errorf("group ID is required")
	}

	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil || groupID <= 0 {
		return 0, fmt.Errorf("invalid group ID format")
	}

	return uint(groupID), nil
}

// validateKeyID validates and parses key ID from request parameter
func validateKeyID(c *gin.Context) (uint, error) {
	keyIDStr := c.Param("key_id")
	if keyIDStr == "" {
		return 0, fmt.Errorf("key ID is required")
	}

	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil || keyID <= 0 {
		return 0, fmt.Errorf("invalid key ID format")
	}

	return uint(keyID), nil
}

// validateKeysText validates the keys text input
func validateKeysText(keysText string) error {
	if strings.TrimSpace(keysText) == "" {
		return fmt.Errorf("keys text cannot be empty")
	}

	if len(keysText) > 1024*1024 { // 1MB limit
		return fmt.Errorf("keys text is too large (max 1MB)")
	}

	return nil
}

// findGroupByID is a helper function to find a group by its ID.
func (s *Server) findGroupByID(c *gin.Context, groupID int) (*models.Group, bool) {
	var group models.Group
	if err := s.DB.First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, app_errors.ErrResourceNotFound)
		} else {
			response.Error(c, app_errors.ParseDBError(err))
		}
		return nil, false
	}
	return &group, true
}

// AddMultipleKeysRequest defines the payload for adding multiple keys from a text block.
type AddMultipleKeysRequest struct {
	KeysText string `json:"keys_text" binding:"required"`
}

// AddMultipleKeys handles creating new keys from a text block within a specific group.
func (s *Server) AddMultipleKeys(c *gin.Context) {
	groupID, err := validateGroupID(c)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, err.Error()))
		return
	}

	var req AddMultipleKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if err := validateKeysText(req.KeysText); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
		return
	}

	result, err := s.KeyService.AddMultipleKeys(groupID, req.KeysText)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, result)
}

// ListKeysInGroup handles listing all keys within a specific group.
func (s *Server) ListKeysInGroup(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID"))
		return
	}

	statusFilter := c.Query("status")
	if statusFilter != "" && statusFilter != "active" && statusFilter != "inactive" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid status filter"))
		return
	}

	keys, err := s.KeyService.ListKeysInGroup(uint(groupID), statusFilter)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, keys)
}

// DeleteSingleKey handles deleting a specific key.
func (s *Server) DeleteSingleKey(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID"))
		return
	}

	keyID, err := strconv.Atoi(c.Param("key_id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid key ID"))
		return
	}

	rowsAffected, err := s.KeyService.DeleteSingleKey(uint(groupID), uint(keyID))
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}
	if rowsAffected == 0 {
		response.Error(c, app_errors.ErrResourceNotFound)
		return
	}

	response.Success(c, gin.H{"message": "Key deleted successfully"})
}

// TestSingleKey handles a one-off validation test for a single key.
func (s *Server) TestSingleKey(c *gin.Context) {
	keyID, err := validateKeyID(c)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, err.Error()))
		return
	}

	isValid, validationErr := s.KeyValidatorService.TestSingleKeyByID(c.Request.Context(), keyID)
	if validationErr != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadGateway, validationErr.Error()))
		return
	}

	if isValid {
		response.Success(c, gin.H{"success": true, "message": "Key is valid."})
	} else {
		response.Success(c, gin.H{"success": false, "message": "Key is invalid or has insufficient quota."})
	}
}

// ValidateGroupKeys initiates a manual validation task for all keys in a group.
func (s *Server) ValidateGroupKeys(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID"))
		return
	}

	group, ok := s.findGroupByID(c, groupID)
	if !ok {
		return
	}

	taskStatus, err := s.KeyManualValidationService.StartValidationTask(group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrTaskInProgress, err.Error()))
		return
	}

	response.Success(c, taskStatus)
}

// RestoreAllInvalidKeys sets the status of all 'inactive' keys in a group to 'active'.
func (s *Server) RestoreAllInvalidKeys(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID"))
		return
	}

	rowsAffected, err := s.KeyService.RestoreAllInvalidKeys(uint(groupID))
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, gin.H{"message": fmt.Sprintf("%d keys restored.", rowsAffected)})
}

// ClearAllInvalidKeys deletes all 'inactive' keys from a group.
func (s *Server) ClearAllInvalidKeys(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID"))
		return
	}

	rowsAffected, err := s.KeyService.ClearAllInvalidKeys(uint(groupID))
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, gin.H{"message": fmt.Sprintf("%d invalid keys cleared.", rowsAffected)})
}

// ExportKeys returns a list of keys for a group, filtered by status.
func (s *Server) ExportKeys(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID"))
		return
	}

	filter := c.DefaultQuery("filter", "all")
	keys, err := s.KeyService.ExportKeys(uint(groupID), filter)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
		return
	}

	response.Success(c, gin.H{"keys": keys})
}
