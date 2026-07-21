package services

import (
	"api-load/internal/models"
	"fmt"
	"strings"
)

const (
	DuplicatePolicyKeep         = "keep"
	DuplicatePolicyUpdateNotes  = "update_notes"
	DuplicatePolicyUpdateStatus = "update_status"
	DuplicatePolicyOverwrite    = "overwrite"
)

type KeyImportOptions struct {
	DuplicatePolicy string
	AllowEmptyNotes bool
}

func (s *KeyService) ImportKeyRecords(groupID uint, records []KeyImportRecord, options KeyImportOptions) (*KeyImportResult, error) {
	policy := strings.TrimSpace(options.DuplicatePolicy)
	if policy == "" {
		policy = DuplicatePolicyKeep
	}
	switch policy {
	case DuplicatePolicyKeep, DuplicatePolicyUpdateNotes, DuplicatePolicyUpdateStatus, DuplicatePolicyOverwrite:
	default:
		return nil, fmt.Errorf("invalid duplicate policy: %s", policy)
	}

	result := &KeyImportResult{}
	if len(records) == 0 {
		return result, nil
	}

	hashes := make([]string, 0, len(records))
	recordByHash := make(map[string]KeyImportRecord, len(records))
	for _, record := range records {
		record.Key = strings.TrimSpace(record.Key)
		if record.Key == "" || !s.isValidKeyFormat(record.Key) {
			result.IgnoredCount++
			continue
		}
		if record.Status == "" {
			record.Status = models.KeyStatusActive
		}
		if record.Status == models.KeyStatusDisabled {
			record.Status = models.KeyStatusActive
			record.Enabled = models.Bool(false)
		}
		if record.Enabled == nil {
			record.Enabled = models.Bool(true)
		}
		if record.Priority == 0 {
			record.Priority = models.DefaultCredentialPriority
		}
		if record.Weight == 0 {
			record.Weight = models.DefaultCredentialWeight
		}
		if record.Priority < 1 || record.Priority > 1000 || record.Weight < 1 || record.Weight > 1000 {
			return nil, fmt.Errorf("priority and weight must be between 1 and 1000")
		}
		hash := s.EncryptionSvc.Hash(record.Key)
		if _, exists := recordByHash[hash]; exists {
			result.IgnoredCount++
			continue
		}
		recordByHash[hash] = record
		hashes = append(hashes, hash)
	}
	if len(hashes) == 0 {
		return result, nil
	}

	var existing []models.APIKey
	if err := s.DB.Where("group_id = ? AND key_hash IN ?", groupID, hashes).Find(&existing).Error; err != nil {
		return nil, err
	}
	existingByHash := make(map[string]models.APIKey, len(existing))
	for _, key := range existing {
		existingByHash[key.KeyHash] = key
	}

	newKeys := make([]models.APIKey, 0, len(records))
	for _, hash := range hashes {
		record := recordByHash[hash]
		if existingKey, exists := existingByHash[hash]; exists {
			result.DuplicateCount++
			updated, err := s.applyDuplicatePolicy(existingKey, record, policy, options.AllowEmptyNotes)
			if err != nil {
				return nil, err
			}
			if updated {
				result.UpdatedCount++
			} else {
				result.IgnoredCount++
			}
			continue
		}

		encryptedKey, err := s.EncryptionSvc.Encrypt(record.Key)
		if err != nil {
			return nil, err
		}
		newKeys = append(newKeys, models.APIKey{
			GroupID:  groupID,
			KeyValue: encryptedKey,
			KeyHash:  hash,
			Notes:    strings.TrimSpace(record.Notes),
			Status:   record.Status,
			Enabled:  models.Bool(models.CredentialEnabled(record.Enabled)),
			Priority: record.Priority,
			Weight:   record.Weight,
		})
	}

	if len(newKeys) > 0 {
		if err := s.KeyProvider.AddKeys(groupID, newKeys); err != nil {
			return nil, err
		}
		result.AddedCount = len(newKeys)
	}

	return result, nil
}

func (s *KeyService) applyDuplicatePolicy(existing models.APIKey, record KeyImportRecord, policy string, allowEmptyNotes bool) (bool, error) {
	switch policy {
	case DuplicatePolicyKeep:
		return false, nil
	case DuplicatePolicyUpdateNotes:
		notes := strings.TrimSpace(record.Notes)
		if notes == "" && !allowEmptyNotes {
			return false, nil
		}
		if existing.Notes == notes {
			return false, nil
		}
		existing.Notes = notes
	case DuplicatePolicyUpdateStatus:
		if existing.Status == record.Status && models.CredentialEnabled(existing.Enabled) == models.CredentialEnabled(record.Enabled) {
			return false, nil
		}
		existing.Status = record.Status
		existing.Enabled = models.Bool(models.CredentialEnabled(record.Enabled))
	case DuplicatePolicyOverwrite:
		notes := strings.TrimSpace(record.Notes)
		if existing.Notes == notes && existing.Status == record.Status &&
			models.CredentialEnabled(existing.Enabled) == models.CredentialEnabled(record.Enabled) &&
			existing.Priority == record.Priority && existing.Weight == record.Weight {
			return false, nil
		}
		existing.Notes = notes
		existing.Status = record.Status
		existing.Enabled = models.Bool(models.CredentialEnabled(record.Enabled))
		existing.Priority = record.Priority
		existing.Weight = record.Weight
	}

	if err := s.DB.Model(&models.APIKey{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"notes":    existing.Notes,
		"status":   existing.Status,
		"enabled":  models.CredentialEnabled(existing.Enabled),
		"priority": existing.Priority,
		"weight":   existing.Weight,
	}).Error; err != nil {
		return false, err
	}
	return true, s.KeyProvider.SyncKeyToStore(&existing)
}
