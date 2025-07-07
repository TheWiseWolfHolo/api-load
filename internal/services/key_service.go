package services

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/keypool"
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

// DeleteKeysResult holds the result of deleting multiple keys.
type DeleteKeysResult struct {
	DeletedCount  int   `json:"deleted_count"`
	IgnoredCount  int   `json:"ignored_count"`
	TotalInGroup  int64 `json:"total_in_group"`
}

// KeyService provides services related to API keys.
type KeyService struct {
	DB          *gorm.DB
	KeyProvider *keypool.KeyProvider
}

// NewKeyService creates a new KeyService.
func NewKeyService(db *gorm.DB, keyProvider *keypool.KeyProvider) *KeyService {
	return &KeyService{
		DB:          db,
		KeyProvider: keyProvider,
	}
}

// AddMultipleKeys handles the business logic of creating new keys from a text block.
func (s *KeyService) AddMultipleKeys(groupID uint, keysText string) (*AddKeysResult, error) {
	// 1. Parse keys from the text block
	keys := s.ParseKeysFromText(keysText)
	if len(keys) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	// 2. Get existing keys in the group for deduplication
	var existingKeys []models.APIKey
	if err := s.DB.Where("group_id = ?", groupID).Select("key_value").Find(&existingKeys).Error; err != nil {
		return nil, err
	}
	existingKeyMap := make(map[string]bool)
	for _, k := range existingKeys {
		existingKeyMap[k.KeyValue] = true
	}

	// 3. Prepare new keys for creation
	var newKeysToCreate []models.APIKey
	uniqueNewKeys := make(map[string]bool)

	for _, keyVal := range keys {
		trimmedKey := strings.TrimSpace(keyVal)
		if trimmedKey == "" {
			continue
		}
		if existingKeyMap[trimmedKey] || uniqueNewKeys[trimmedKey] {
			continue
		}
		if s.isValidKeyFormat(trimmedKey) {
			uniqueNewKeys[trimmedKey] = true
			newKeysToCreate = append(newKeysToCreate, models.APIKey{
				GroupID:  groupID,
				KeyValue: trimmedKey,
				Status:   models.KeyStatusActive,
			})
		}
	}

	if len(newKeysToCreate) == 0 {
		var totalInGroup int64
		s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalInGroup)
		return &AddKeysResult{
			AddedCount:   0,
			IgnoredCount: len(keys),
			TotalInGroup: totalInGroup,
		}, nil
	}

	// 4. Use KeyProvider to add keys, which handles DB and cache
	err := s.KeyProvider.AddKeys(groupID, newKeysToCreate)
	if err != nil {
		return nil, err
	}

	// 5. Get the new total count
	var totalInGroup int64
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalInGroup).Error; err != nil {
		return nil, err
	}

	return &AddKeysResult{
		AddedCount:   len(newKeysToCreate),
		IgnoredCount: len(keys) - len(newKeysToCreate),
		TotalInGroup: totalInGroup,
	}, nil
}

// ParseKeysFromText parses a string of keys from various formats into a string slice.
// This function is exported to be shared with the handler layer.
func (s *KeyService) ParseKeysFromText(text string) []string {
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
	return s.KeyProvider.RestoreKeys(groupID)
}

// ClearAllInvalidKeys deletes all 'inactive' keys from a group.
func (s *KeyService) ClearAllInvalidKeys(groupID uint) (int64, error) {
	return s.KeyProvider.RemoveInvalidKeys(groupID)
}

// DeleteMultipleKeys handles the business logic of deleting keys from a text block.
func (s *KeyService) DeleteMultipleKeys(groupID uint, keysText string) (*DeleteKeysResult, error) {
	// 1. Parse keys from the text block
	keysToDelete := s.ParseKeysFromText(keysText)
	if len(keysToDelete) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	// 2. Use KeyProvider to delete keys, which handles DB and cache
	deletedCount, err := s.KeyProvider.RemoveKeys(groupID, keysToDelete)
	if err != nil {
		return nil, err
	}

	ignoredCount := len(keysToDelete) - int(deletedCount)

	// 3. Get the new total count
	var totalInGroup int64
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalInGroup).Error; err != nil {
		return nil, err
	}

	return &DeleteKeysResult{
		DeletedCount: int(deletedCount),
		IgnoredCount: ignoredCount,
		TotalInGroup: totalInGroup,
	}, nil
}

// ListKeysInGroupQuery builds a query to list all keys within a specific group, filtered by status.
// It returns a GORM query builder, allowing the handler to apply pagination.
func (s *KeyService) ListKeysInGroupQuery(groupID uint, statusFilter string, searchKeyword string) *gorm.DB {
	query := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID)

	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	if searchKeyword != "" {
		// Use LIKE for fuzzy search on the key_value
		query = query.Where("key_value LIKE ?", "%"+searchKeyword+"%")
	}

	return query
}
