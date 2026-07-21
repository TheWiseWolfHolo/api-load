package services

import (
	"api-load/internal/models"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"gorm.io/gorm"
)

type ResourceExportResult struct {
	ExportedCount int `json:"exported_count"`
	ErrorCount    int `json:"error_count"`
}

func (s *ResourcePoolService) ExportResourcesToWriter(ctx context.Context, poolID uint, status string, enabled *bool, content, format string, writer io.Writer) (*ResourceExportResult, error) {
	if _, err := s.loadPool(ctx, poolID, false); err != nil {
		return nil, err
	}
	if content == "" {
		content = "full"
	}
	if content != "full" && content != "keys" {
		return nil, resourcePoolValidationError("export content must be full or keys")
	}
	if format == "" {
		if content == "keys" {
			format = "txt"
		} else {
			format = "jsonl"
		}
	}
	if content == "keys" && format != "txt" {
		return nil, resourcePoolValidationError("keys-only export uses txt format")
	}
	if content == "full" && format != "jsonl" && format != "csv" {
		return nil, resourcePoolValidationError("full export uses jsonl or csv format")
	}

	query := s.db.WithContext(ctx).Model(&models.UpstreamResource{}).Where("resource_pool_id = ?", poolID)
	if status == models.ResourceStatusDisabled {
		disabled := false
		enabled = &disabled
	} else if status != "" && status != "all" {
		if status != models.ResourceStatusActive && status != models.ResourceStatusInvalid {
			return nil, resourcePoolValidationError("invalid resource status filter")
		}
		query = query.Where("status = ?", status)
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	if status == models.ResourceStatusActive && enabled == nil {
		query = query.Where("enabled = ?", true)
	}

	result := &ResourceExportResult{}
	var csvWriter *csv.Writer
	if format == "csv" {
		csvWriter = csv.NewWriter(writer)
		if err := csvWriter.Write([]string{"name", "upstream_url", "key", "enabled", "priority", "weight"}); err != nil {
			return nil, err
		}
	}
	var resources []models.UpstreamResource
	err := query.Order("priority asc, id asc").FindInBatches(&resources, chunkSize, func(tx *gorm.DB, batch int) error {
		for _, resource := range resources {
			key, err := s.encryptionSvc.Decrypt(resource.KeyValue)
			if err != nil {
				result.ErrorCount++
				continue
			}
			switch format {
			case "txt":
				if _, err := fmt.Fprintln(writer, key); err != nil {
					return err
				}
			case "jsonl":
				row := map[string]any{
					"name": resource.Name, "upstream_url": resource.UpstreamURL, "key": key,
					"enabled":  models.CredentialEnabled(resource.Enabled),
					"priority": resource.Priority, "weight": resource.Weight,
				}
				encoded, err := json.Marshal(row)
				if err != nil {
					return err
				}
				if _, err := writer.Write(append(encoded, '\n')); err != nil {
					return err
				}
			case "csv":
				if err := csvWriter.Write([]string{
					resource.Name, resource.UpstreamURL, key,
					fmt.Sprint(models.CredentialEnabled(resource.Enabled)), fmt.Sprint(resource.Priority), fmt.Sprint(resource.Weight),
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
