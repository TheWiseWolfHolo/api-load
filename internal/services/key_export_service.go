package services

import (
	"api-load/internal/models"
	"encoding/csv"
	"encoding/json"
	"fmt"
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
	case "txt", "jsonl", "csv":
	default:
		return nil, fmt.Errorf("invalid export format: %s", format)
	}

	query := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).
		Select("id, key_value, notes, status, enabled, priority, weight")
	switch statusFilter {
	case models.KeyStatusDisabled:
		query = query.Where("enabled = ?", false)
	case models.KeyStatusActive:
		query = query.Where("enabled = ? AND status = ?", true, models.KeyStatusActive)
	case "all":
	default:
		query = query.Where("status = ?", statusFilter)
	}

	result := &KeyExportResult{}
	var csvWriter *csv.Writer
	if format == "csv" {
		csvWriter = csv.NewWriter(writer)
		if err := csvWriter.Write([]string{"key", "notes", "status", "enabled", "priority", "weight"}); err != nil {
			return nil, err
		}
	}
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
				row := map[string]any{
					"key":      decryptedKey,
					"notes":    key.Notes,
					"status":   key.Status,
					"enabled":  models.CredentialEnabled(key.Enabled),
					"priority": key.Priority,
					"weight":   key.Weight,
				}
				data, err := json.Marshal(row)
				if err != nil {
					return err
				}
				if _, err := writer.Write(append(data, '\n')); err != nil {
					return err
				}
			case "csv":
				if err := csvWriter.Write([]string{
					decryptedKey,
					key.Notes,
					key.Status,
					fmt.Sprint(models.CredentialEnabled(key.Enabled)),
					fmt.Sprint(key.Priority),
					fmt.Sprint(key.Weight),
				}); err != nil {
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
	if csvWriter != nil {
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return nil, err
		}
	}

	return result, nil
}
