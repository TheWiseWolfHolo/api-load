package services

import (
	"api-load/internal/encryption"
	"api-load/internal/keypool"
	"api-load/internal/models"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	maxRequestKeys = 5000
	chunkSize      = 500
)

// AddKeysResult holds the result of adding multiple keys.
type AddKeysResult struct {
	AddedCount   int   `json:"added_count"`
	IgnoredCount int   `json:"ignored_count"`
	TotalInGroup int64 `json:"total_in_group"`
}

// DeleteKeysResult holds the result of deleting multiple keys.
type DeleteKeysResult struct {
	DeletedCount int   `json:"deleted_count"`
	IgnoredCount int   `json:"ignored_count"`
	TotalInGroup int64 `json:"total_in_group"`
}

// RestoreKeysResult holds the result of restoring multiple keys.
type RestoreKeysResult struct {
	RestoredCount int   `json:"restored_count"`
	IgnoredCount  int   `json:"ignored_count"`
	TotalInGroup  int64 `json:"total_in_group"`
}

type KeyStatusUpdateResult struct {
	ChangedCount int `json:"changed_count"`
	IgnoredCount int `json:"ignored_count"`
}

type KeyUpdateParams struct {
	Enabled  *bool
	Status   *string
	Priority *int
	Weight   *int
	Notes    *string
}

// KeyService provides services related to API keys.
type KeyService struct {
	DB            *gorm.DB
	KeyProvider   *keypool.KeyProvider
	KeyValidator  *keypool.KeyValidator
	EncryptionSvc encryption.Service
}

// NewKeyService creates a new KeyService.
func NewKeyService(db *gorm.DB, keyProvider *keypool.KeyProvider, keyValidator *keypool.KeyValidator, encryptionSvc encryption.Service) *KeyService {
	return &KeyService{
		DB:            db,
		KeyProvider:   keyProvider,
		KeyValidator:  keyValidator,
		EncryptionSvc: encryptionSvc,
	}
}

// AddMultipleKeys handles the business logic of creating new keys from a text block.
// deprecated: use KeyImportService for large imports
func (s *KeyService) AddMultipleKeys(groupID uint, keysText string) (*AddKeysResult, error) {
	keys := s.ParseKeysFromText(keysText)
	if len(keys) > maxRequestKeys {
		return nil, fmt.Errorf("batch size exceeds the limit of %d keys, got %d", maxRequestKeys, len(keys))
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	addedCount, ignoredCount, err := s.processAndCreateKeys(groupID, keys, nil)
	if err != nil {
		return nil, err
	}

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

// processAndCreateKeys is the lowest-level reusable function for adding keys.
func (s *KeyService) processAndCreateKeys(
	groupID uint,
	keys []string,
	progressCallback func(processed int),
) (addedCount int, ignoredCount int, err error) {
	// 1. Get existing key hashes in the group for deduplication
	var existingHashes []string
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Pluck("key_hash", &existingHashes).Error; err != nil {
		return 0, 0, err
	}
	existingHashMap := make(map[string]bool)
	for _, h := range existingHashes {
		existingHashMap[h] = true
	}

	// 2. Prepare new keys for creation
	var newKeysToCreate []models.APIKey
	uniqueNewKeys := make(map[string]bool)

	for _, keyVal := range keys {
		trimmedKey := strings.TrimSpace(keyVal)
		if trimmedKey == "" || uniqueNewKeys[trimmedKey] || !s.isValidKeyFormat(trimmedKey) {
			continue
		}

		// Generate hash for deduplication check
		keyHash := s.EncryptionSvc.Hash(trimmedKey)
		if existingHashMap[keyHash] {
			continue
		}

		encryptedKey, err := s.EncryptionSvc.Encrypt(trimmedKey)
		if err != nil {
			logrus.WithError(err).WithField("key", trimmedKey).Error("Failed to encrypt key, skipping")
			continue
		}

		uniqueNewKeys[trimmedKey] = true
		newKeysToCreate = append(newKeysToCreate, models.APIKey{
			GroupID:  groupID,
			KeyValue: encryptedKey,
			KeyHash:  keyHash,
			Status:   models.KeyStatusActive,
			Enabled:  models.Bool(true),
			Priority: models.DefaultCredentialPriority,
			Weight:   models.DefaultCredentialWeight,
		})
	}

	if len(newKeysToCreate) == 0 {
		return 0, len(keys), nil
	}

	// 3. Use KeyProvider to add keys in chunks
	for i := 0; i < len(newKeysToCreate); i += chunkSize {
		end := i + chunkSize
		if end > len(newKeysToCreate) {
			end = len(newKeysToCreate)
		}
		chunk := newKeysToCreate[i:end]
		if err := s.KeyProvider.AddKeys(groupID, chunk); err != nil {
			return addedCount, len(keys) - addedCount, err
		}
		addedCount += len(chunk)

		if progressCallback != nil {
			progressCallback(i + len(chunk))
		}
	}

	return addedCount, len(keys) - addedCount, nil
}

// ParseKeysFromText parses a string of keys from various formats into a string slice.
// This function is exported to be shared with the handler layer.
func (s *KeyService) ParseKeysFromText(text string) []string {
	records, err := ParseKeyImportInput(text)
	if err != nil {
		return nil
	}
	keys := make([]string, 0, len(records))
	for _, record := range records {
		keys = append(keys, record.Key)
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
	return strings.TrimSpace(key) != ""
}

// RestoreMultipleKeys handles the business logic of restoring keys from a text block.
func (s *KeyService) RestoreMultipleKeys(groupID uint, keysText string) (*RestoreKeysResult, error) {
	keysToRestore := s.ParseKeysFromText(keysText)
	if len(keysToRestore) > maxRequestKeys {
		return nil, fmt.Errorf("batch size exceeds the limit of %d keys, got %d", maxRequestKeys, len(keysToRestore))
	}
	if len(keysToRestore) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	var totalRestoredCount int64
	for i := 0; i < len(keysToRestore); i += chunkSize {
		end := i + chunkSize
		if end > len(keysToRestore) {
			end = len(keysToRestore)
		}
		chunk := keysToRestore[i:end]
		restoredCount, err := s.KeyProvider.RestoreMultipleKeys(groupID, chunk)
		if err != nil {
			return nil, err
		}
		totalRestoredCount += restoredCount
	}

	ignoredCount := len(keysToRestore) - int(totalRestoredCount)

	var totalInGroup int64
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalInGroup).Error; err != nil {
		return nil, err
	}

	return &RestoreKeysResult{
		RestoredCount: int(totalRestoredCount),
		IgnoredCount:  ignoredCount,
		TotalInGroup:  totalInGroup,
	}, nil
}

// RestoreAllInvalidKeys sets the status of all 'inactive' keys in a group to 'active'.
func (s *KeyService) RestoreAllInvalidKeys(groupID uint) (int64, error) {
	return s.KeyProvider.RestoreKeys(groupID)
}

// ClearAllInvalidKeys deletes all 'inactive' keys from a group.
func (s *KeyService) ClearAllInvalidKeys(groupID uint) (int64, error) {
	return s.KeyProvider.RemoveInvalidKeys(groupID)
}

// ClearAllKeys deletes all keys from a group.
func (s *KeyService) ClearAllKeys(groupID uint) (int64, error) {
	return s.KeyProvider.RemoveAllKeys(groupID)
}

// DeleteMultipleKeys handles the business logic of deleting keys from a text block.
func (s *KeyService) DeleteMultipleKeys(groupID uint, keysText string) (*DeleteKeysResult, error) {
	keysToDelete := s.ParseKeysFromText(keysText)
	if len(keysToDelete) > maxRequestKeys {
		return nil, fmt.Errorf("batch size exceeds the limit of %d keys, got %d", maxRequestKeys, len(keysToDelete))
	}
	if len(keysToDelete) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	var totalDeletedCount int64
	for i := 0; i < len(keysToDelete); i += chunkSize {
		end := i + chunkSize
		if end > len(keysToDelete) {
			end = len(keysToDelete)
		}
		chunk := keysToDelete[i:end]
		deletedCount, err := s.KeyProvider.RemoveKeys(groupID, chunk)
		if err != nil {
			return nil, err
		}
		totalDeletedCount += deletedCount
	}

	ignoredCount := len(keysToDelete) - int(totalDeletedCount)

	var totalInGroup int64
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalInGroup).Error; err != nil {
		return nil, err
	}

	return &DeleteKeysResult{
		DeletedCount: int(totalDeletedCount),
		IgnoredCount: ignoredCount,
		TotalInGroup: totalInGroup,
	}, nil
}

func (s *KeyService) IsValidKeyStatusFilter(status string) bool {
	switch status {
	case "", "all", models.KeyStatusActive, models.KeyStatusInvalid, models.KeyStatusDisabled:
		return true
	default:
		return false
	}
}

func (s *KeyService) IsValidKeyStatusValue(status string) bool {
	switch status {
	case models.KeyStatusActive, models.KeyStatusInvalid, models.KeyStatusDisabled:
		return true
	default:
		return false
	}
}

func (s *KeyService) SetKeyStatus(keyID uint, status string) (*KeyStatusUpdateResult, error) {
	if !s.IsValidKeyStatusValue(status) {
		return nil, fmt.Errorf("invalid key status: %s", status)
	}

	params := KeyUpdateParams{}
	switch status {
	case models.KeyStatusDisabled:
		params.Enabled = models.Bool(false)
	case models.KeyStatusActive:
		params.Enabled = models.Bool(true)
		params.Status = &status
	case models.KeyStatusInvalid:
		params.Status = &status
	}
	return s.UpdateKeys([]uint{keyID}, params)
}

func (s *KeyService) SetKeysStatus(keyIDs []uint, status string) (*KeyStatusUpdateResult, error) {
	if !s.IsValidKeyStatusValue(status) {
		return nil, fmt.Errorf("invalid key status: %s", status)
	}
	params := KeyUpdateParams{}
	switch status {
	case models.KeyStatusDisabled:
		params.Enabled = models.Bool(false)
	case models.KeyStatusActive:
		params.Enabled = models.Bool(true)
		params.Status = &status
	case models.KeyStatusInvalid:
		params.Status = &status
	}
	return s.UpdateKeys(keyIDs, params)
}

func (s *KeyService) UpdateKeys(keyIDs []uint, params KeyUpdateParams) (*KeyStatusUpdateResult, error) {
	ids := uniqueKeyIDs(keyIDs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("key_ids cannot be empty")
	}
	if params.Priority != nil && (*params.Priority < 1 || *params.Priority > 1000) {
		return nil, fmt.Errorf("priority must be between 1 and 1000")
	}
	if params.Weight != nil && (*params.Weight < 1 || *params.Weight > 1000) {
		return nil, fmt.Errorf("weight must be between 1 and 1000")
	}
	if params.Status != nil && *params.Status != models.KeyStatusActive && *params.Status != models.KeyStatusInvalid {
		return nil, fmt.Errorf("health status must be active or invalid")
	}
	if params.Notes != nil && len([]rune(*params.Notes)) > 255 {
		return nil, fmt.Errorf("notes length must be <= 255 characters")
	}

	var keys []models.APIKey
	if err := s.DB.Where("id IN ?", ids).Find(&keys).Error; err != nil {
		return nil, err
	}
	result := &KeyStatusUpdateResult{IgnoredCount: len(ids) - len(keys)}
	changedIDs := make([]uint, 0, len(keys))
	for i := range keys {
		key := &keys[i]
		changed := false
		if params.Enabled != nil && models.CredentialEnabled(key.Enabled) != *params.Enabled {
			key.Enabled = models.Bool(*params.Enabled)
			changed = true
		}
		if params.Status != nil && key.Status != *params.Status {
			key.Status = *params.Status
			changed = true
		}
		if params.Status != nil && *params.Status == models.KeyStatusActive && key.FailureCount != 0 {
			key.FailureCount = 0
			changed = true
		}
		if params.Priority != nil && key.Priority != *params.Priority {
			key.Priority = *params.Priority
			changed = true
		}
		if params.Weight != nil && key.Weight != *params.Weight {
			key.Weight = *params.Weight
			changed = true
		}
		if params.Notes != nil && key.Notes != strings.TrimSpace(*params.Notes) {
			key.Notes = strings.TrimSpace(*params.Notes)
			changed = true
		}
		if changed {
			changedIDs = append(changedIDs, key.ID)
		} else {
			result.IgnoredCount++
		}
	}
	if len(changedIDs) == 0 {
		return result, nil
	}
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		for i := range keys {
			if !containsKeyID(changedIDs, keys[i].ID) {
				continue
			}
			if err := tx.Model(&models.APIKey{}).Where("id = ?", keys[i].ID).Updates(map[string]any{
				"enabled":       models.CredentialEnabled(keys[i].Enabled),
				"status":        keys[i].Status,
				"failure_count": keys[i].FailureCount,
				"priority":      keys[i].Priority,
				"weight":        keys[i].Weight,
				"notes":         keys[i].Notes,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	changed := make([]models.APIKey, 0, len(changedIDs))
	for _, key := range keys {
		if containsKeyID(changedIDs, key.ID) {
			changed = append(changed, key)
		}
	}
	if err := s.KeyProvider.SyncKeysToStore(changed); err != nil {
		return nil, err
	}
	result.ChangedCount = len(changed)
	return result, nil
}

func uniqueKeyIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func containsKeyID(ids []uint, id uint) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// ListKeysInGroupQuery builds a query to list all keys within a specific group, filtered by status.
func (s *KeyService) ListKeysInGroupQuery(groupID uint, statusFilter string, enabledFilter *bool, searchHash string, notesKeyword string, searchKeyword string) *gorm.DB {
	query := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID)

	if statusFilter == models.KeyStatusDisabled {
		disabled := false
		enabledFilter = &disabled
	} else if statusFilter != "" && statusFilter != "all" {
		query = query.Where("status = ?", statusFilter)
		if enabledFilter == nil {
			enabled := true
			enabledFilter = &enabled
		}
	}
	if enabledFilter != nil {
		query = query.Where("enabled = ?", *enabledFilter)
	}

	notesKeyword = strings.TrimSpace(notesKeyword)
	searchKeyword = strings.TrimSpace(searchKeyword)
	if searchHash != "" && searchKeyword != "" {
		query = query.Where("key_hash = ? OR notes LIKE ?", searchHash, "%"+searchKeyword+"%")
	} else if searchHash != "" {
		query = query.Where("key_hash = ?", searchHash)
	} else if notesKeyword != "" {
		query = query.Where("notes LIKE ?", "%"+notesKeyword+"%")
	} else if searchKeyword != "" {
		query = query.Where("notes LIKE ?", "%"+searchKeyword+"%")
	}

	orderBy := "last_used_at desc, id desc"
	if s.DB.Dialector.Name() == "postgres" {
		orderBy = "last_used_at desc nulls last, id desc"
	}

	query = query.Order(orderBy)

	return query
}

// TestMultipleKeys handles a one-off validation test for multiple keys.
func (s *KeyService) TestMultipleKeys(group *models.Group, keysText string) ([]keypool.KeyTestResult, error) {
	keysToTest := s.ParseKeysFromText(keysText)
	if len(keysToTest) > maxRequestKeys {
		return nil, fmt.Errorf("batch size exceeds the limit of %d keys, got %d", maxRequestKeys, len(keysToTest))
	}
	if len(keysToTest) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	var allResults []keypool.KeyTestResult
	for i := 0; i < len(keysToTest); i += chunkSize {
		end := i + chunkSize
		if end > len(keysToTest) {
			end = len(keysToTest)
		}
		chunk := keysToTest[i:end]
		results, err := s.KeyValidator.TestMultipleKeys(group, chunk)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// StreamKeysToWriter fetches keys from the database in batches and writes them to the provided writer.
func (s *KeyService) StreamKeysToWriter(groupID uint, statusFilter string, writer io.Writer) error {
	_, err := s.ExportKeysToWriter(groupID, statusFilter, "txt", writer)
	return err
}
