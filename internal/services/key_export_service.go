package services

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/models"
	"io"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type KeyExportResult struct {
	ExportedCount int `json:"exported_count"`
	ErrorCount    int `json:"error_count"`
}

func (s *KeyService) ExportKeysToWriter(groupID uint, statusFilter string, format string, writer io.Writer) (*KeyExportResult, error) {
	if statusFilter == "" {
		statusFilter = "all"
	}
	if !s.IsValidKeyStatusFilter(statusFilter) {
		return nil, fmt.Errorf("invalid status filter: %s", statusFilter)
	}
	if format == "" {
		format = "txt"
	}
	switch format {
	case "txt", "jsonl":
	default:
		return nil, fmt.Errorf("invalid export format: %s", format)
	}

	query := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Select("id, key_value, notes, status")
	if statusFilter != "all" {
		query = query.Where("status = ?", statusFilter)
	}

	result := &KeyExportResult{}
	var keys []models.APIKey
	err := query.FindInBatches(&keys, chunkSize, func(tx *gorm.DB, batch int) error {
		for _, key := range keys {
			decryptedKey, err := s.EncryptionSvc.Decrypt(key.KeyValue)
			if err != nil {
				result.ErrorCount++
				logrus.WithError(err).WithField("key_id", key.ID).Error("Failed to decrypt key for export, skipping")
				continue
			}

			switch format {
			case "txt":
				if _, err := writer.Write([]byte(decryptedKey + "\n")); err != nil {
					return err
				}
			case "jsonl":
				row := map[string]string{
					"key":    decryptedKey,
					"notes":  key.Notes,
					"status": key.Status,
				}
				data, err := json.Marshal(row)
				if err != nil {
					return err
				}
				if _, err := writer.Write(append(data, '\n')); err != nil {
					return err
				}
			}
			result.ExportedCount++
		}
		return nil
	}).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}
