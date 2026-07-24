package services

import (
	"api-load/internal/models"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"gorm.io/gorm"
)

// ResourceExportStatusCooling 是导出过滤的虚拟状态:status 仍为 active、
// 但 global_cooldown_until 在未来的资源。它们当下不可用,不随 active 导出。
const ResourceExportStatusCooling = "cooling"

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
	now := time.Now()
	switch status {
	case models.ResourceStatusDisabled:
		disabled := false
		enabled = &disabled
	case models.ResourceStatusActive:
		// 冷却中的资源 status 仍是 active,但当下不可调度,不算可用 key。
		query = query.Where("status = ?", models.ResourceStatusActive).
			Where("global_cooldown_until IS NULL OR global_cooldown_until <= ?", now)
	case ResourceExportStatusCooling:
		query = query.Where("status = ? AND global_cooldown_until IS NOT NULL AND global_cooldown_until > ?",
			models.ResourceStatusActive, now)
	case models.ResourceStatusInvalid:
		query = query.Where("status = ?", status)
	case "", "all":
	default:
		return nil, resourcePoolValidationError("invalid resource status filter")
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	statusImpliesEnabled := status == models.ResourceStatusActive ||
		status == models.ResourceStatusInvalid || status == ResourceExportStatusCooling
	if statusImpliesEnabled && enabled == nil {
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
