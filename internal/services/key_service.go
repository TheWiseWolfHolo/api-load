package services

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/models"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// AddKeysResult holds the result of adding multiple keys.
type AddKeysResult struct {
	AddedCount   int   `json:"added_count"`
	IgnoredCount int   `json:"ignored_count"`
	TotalInGroup int64 `json:"total_in_group"`
}

// KeyService provides services related to API keys.
type KeyService struct {
	DB *gorm.DB
}

// NewKeyService creates a new KeyService.
func NewKeyService(db *gorm.DB) *KeyService {
	return &KeyService{DB: db}
}

// AddMultipleKeys handles the business logic of creating new keys from a text block.
func (s *KeyService) AddMultipleKeys(groupID uint, keysText string) (*AddKeysResult, error) {
	// 1. Parse keys from the text block
	keys := s.parseKeysFromText(keysText)
	if len(keys) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	// 2. Get the group information for validation
	var group models.Group
	if err := s.DB.First(&group, groupID).Error; err != nil {
		return nil, fmt.Errorf("failed to find group: %w", err)
	}

	// 3. Get existing keys in the group for deduplication
	var existingKeys []models.APIKey
	if err := s.DB.Where("group_id = ?", groupID).Select("key_value").Find(&existingKeys).Error; err != nil {
		return nil, err
	}
	existingKeyMap := make(map[string]bool)
	for _, k := range existingKeys {
		existingKeyMap[k.KeyValue] = true
	}

	// 4. Prepare new keys with basic validation only
	var newKeysToCreate []models.APIKey
	uniqueNewKeys := make(map[string]bool)

	for _, keyVal := range keys {
		trimmedKey := strings.TrimSpace(keyVal)
		if trimmedKey == "" {
			continue
		}

		// Check if key already exists
		if existingKeyMap[trimmedKey] || uniqueNewKeys[trimmedKey] {
			continue
		}

		// 通用验证：只做基础格式检查，不做渠道特定验证
		if s.isValidKeyFormat(trimmedKey) {
			uniqueNewKeys[trimmedKey] = true
			newKeysToCreate = append(newKeysToCreate, models.APIKey{
				GroupID:  groupID,
				KeyValue: trimmedKey,
				Status:   "active",
			})
		}
	}

	addedCount := len(newKeysToCreate)
	// 更准确的忽略计数：包括重复的和无效的
	ignoredCount := len(keys) - addedCount

	// 5. Insert new keys if any
	if addedCount > 0 {
		if err := s.DB.Create(&newKeysToCreate).Error; err != nil {
			return nil, err
		}
	}

	// 6. Get the new total count
	var totalInGroup int64
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalInGroup).Error; err != nil {
		return nil, err
	}

	return &AddKeysResult{
		AddedCount:   addedCount,
		IgnoredCount: ignoredCount,
		TotalInGroup: totalInGroup,
	}, nil
}

func (s *KeyService) parseKeysFromText(text string) []string {
	var keys []string

	// First, try to parse as a JSON array of strings
	if json.Unmarshal([]byte(text), &keys) == nil && len(keys) > 0 {
		return s.filterValidKeys(keys)
	}

	// 通用解析：通过分隔符分割文本，不使用复杂的正则表达式
	delimiters := regexp.MustCompile(`[\s,;|\n\r\t]+`)
	splitKeys := delimiters.Split(strings.TrimSpace(text), -1)

	for _, key := range splitKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}

	return s.filterValidKeys(keys)
}

// filterValidKeys validates and filters potential API keys
func (s *KeyService) filterValidKeys(keys []string) []string {
	var validKeys []string
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if s.isValidKeyFormat(key) {
			validKeys = append(validKeys, key)
		}
	}
	return validKeys
}

// isValidKeyFormat performs basic validation on key format
func (s *KeyService) isValidKeyFormat(key string) bool {
	if len(key) < 4 || len(key) > 1000 {
		return false
	}

	if key == "" ||
		strings.TrimSpace(key) == "" {
		return false
	}

	validChars := regexp.MustCompile(`^[a-zA-Z0-9_\-./+=:]+$`)
	return validChars.MatchString(key)
}

// RestoreAllInvalidKeys sets the status of all 'inactive' keys in a group to 'active'.
func (s *KeyService) RestoreAllInvalidKeys(groupID uint) (int64, error) {
	result := s.DB.Model(&models.APIKey{}).Where("group_id = ? AND status = ?", groupID, "inactive").Update("status", "active")
	return result.RowsAffected, result.Error
}

// ClearAllInvalidKeys deletes all 'inactive' keys from a group.
func (s *KeyService) ClearAllInvalidKeys(groupID uint) (int64, error) {
	result := s.DB.Where("group_id = ? AND status = ?", groupID, "inactive").Delete(&models.APIKey{})
	return result.RowsAffected, result.Error
}

// DeleteSingleKey deletes a specific key from a group.
func (s *KeyService) DeleteSingleKey(groupID, keyID uint) (int64, error) {
	result := s.DB.Where("group_id = ? AND id = ?", groupID, keyID).Delete(&models.APIKey{})
	return result.RowsAffected, result.Error
}

// ExportKeys returns a list of keys for a group, filtered by status.
func (s *KeyService) ExportKeys(groupID uint, filter string) ([]string, error) {
	query := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID)

	switch filter {
	case "valid":
		query = query.Where("status = ?", "active")
	case "invalid":
		query = query.Where("status = ?", "inactive")
	case "all":
		// No status filter needed
	default:
		return nil, fmt.Errorf("invalid filter value. Use 'all', 'valid', or 'invalid'")
	}

	var keys []string
	if err := query.Pluck("key_value", &keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// ListKeysInGroup lists all keys within a specific group, filtered by status.
func (s *KeyService) ListKeysInGroup(groupID uint, statusFilter string) ([]models.APIKey, error) {
	var keys []models.APIKey
	query := s.DB.Where("group_id = ?", groupID)

	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	if err := query.Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}
