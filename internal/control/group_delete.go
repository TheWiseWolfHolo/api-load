package control

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	app_errors "gpt-load/internal/platform/errors"
	"gpt-load/internal/storage/models"
)

type AccessKeyReferenceSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type GroupInUseData struct {
	AccessKeys []AccessKeyReferenceSummary `json:"access_keys"`
}

func detachGroupReferences(
	tx *gorm.DB,
	groupID uint,
) error {
	type accessKeyFilterRow struct {
		ID      uint
		Name    string
		Filters []byte
	}
	var rows []accessKeyFilterRow
	if err := tx.Table("access_keys").
		Select("id", "name", "filters").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return app_errors.ParseDBError(err)
	}
	for _, row := range rows {
		filters, err := decodeStoredAccessKeyFilters(row.Filters)
		if err != nil {
			return fmt.Errorf(
				"decode access key %d filters for group delete: %w",
				row.ID,
				app_errors.ErrInternalServer,
			)
		}
		remaining := make([]uint, 0, len(filters.Groups))
		found := false
		for _, referencedID := range filters.Groups {
			if referencedID == groupID {
				found = true
			} else {
				remaining = append(remaining, referencedID)
			}
		}
		if !found {
			continue
		}
		filters.Groups = remaining
		if len(remaining) == 0 {
			// Empty explicit scope must never become unrestricted access.
			filters.GroupsRestricted = true
		}
		encoded, err := encodeStoredAccessKeyFilters(filters)
		if err != nil {
			return fmt.Errorf("encode access key %d filters for group delete: %w", row.ID, err)
		}
		updates := map[string]any{"filters": models.JSON(encoded)}
		if len(remaining) == 0 {
			updates["status"] = "disabled"
		}
		if err := tx.Model(&models.AccessKey{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return app_errors.ParseDBError(err)
		}
	}
	return nil
}

func (s *Service) DeleteGroup(ctx context.Context, groupID uint) error {
	if groupID == 0 {
		return app_errors.ErrBadRequest
	}
	var deletedCredentialIDs []uint
	providerReferencesChanged := false
	_, err := s.writeGroupConfig(ctx, func(tx *gorm.DB) error {
		var group models.Group
		if err := tx.Where("id = ?", groupID).Take(&group).Error; err != nil {
			return app_errors.ParseDBError(err)
		}
		var groupModels []GroupModel
		if err := decodeGroupDiscoveryJSON(group.Models, &groupModels); err != nil {
			return fmt.Errorf("decode group %d models: %w", groupID, app_errors.ErrInternalServer)
		}
		providerReferencesChanged = len(groupModels) > 0
		if err := detachGroupReferences(tx, groupID); err != nil {
			return err
		}
		if err := tx.Model(&models.Credential{}).
			Where("group_id = ?", groupID).
			Order("id ASC").
			Pluck("id", &deletedCredentialIDs).Error; err != nil {
			return app_errors.ParseDBError(err)
		}
		if err := tx.Delete(&group).Error; err != nil {
			return app_errors.ParseDBError(err)
		}
		return nil
	}, func() error {
		s.registry.RemoveGroup(groupID)
		for _, credentialID := range deletedCredentialIDs {
			if _, exists := s.registry.EncryptedCredentialData(credentialID); exists {
				return fmt.Errorf("deleted Registry credential %d remains", credentialID)
			}
			s.retireCredentialRuntime(credentialID)
		}
		return nil
	})
	if err != nil {
		return withControlOperationContext(err, groupID, 0)
	}
	if providerReferencesChanged && s.catalogSync != nil {
		s.catalogSync.RequestGroupSync()
	}
	return nil
}
