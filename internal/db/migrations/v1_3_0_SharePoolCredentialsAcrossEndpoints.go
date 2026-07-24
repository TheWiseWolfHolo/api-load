package db

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"api-load/internal/models"

	"gorm.io/gorm"
)

// V1_3_0_SharePoolCredentialsAcrossEndpoints separates protocol-specific base
// URLs from credentials. Existing URL+key rows are converted into shared keys,
// while bound groups receive an endpoint matching their channel type.
func V1_3_0_SharePoolCredentialsAcrossEndpoints(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var pools []models.ResourcePool
		if err := tx.Find(&pools).Error; err != nil {
			return err
		}
		for i := range pools {
			if err := migrateResourcePoolEndpoints(tx, pools[i].ID); err != nil {
				return fmt.Errorf("migrate resource pool %d endpoints: %w", pools[i].ID, err)
			}
			if err := deduplicateResourcePoolCredentials(tx, pools[i].ID); err != nil {
				return fmt.Errorf("migrate resource pool %d credentials: %w", pools[i].ID, err)
			}
		}
		return nil
	})
}

func migrateResourcePoolEndpoints(tx *gorm.DB, poolID uint) error {
	var groups []models.Group
	if err := tx.Where("resource_pool_id = ?", poolID).Order("id asc").Find(&groups).Error; err != nil {
		return err
	}
	var resources []models.UpstreamResource
	if err := tx.Where("resource_pool_id = ?", poolID).Order("id asc").Find(&resources).Error; err != nil {
		return err
	}

	urlCounts := make(map[string]int)
	for _, resource := range resources {
		baseURL := strings.TrimRight(strings.TrimSpace(resource.UpstreamURL), "/")
		if baseURL != "" {
			urlCounts[baseURL]++
		}
	}
	type countedURL struct {
		URL   string
		Count int
	}
	orderedURLs := make([]countedURL, 0, len(urlCounts))
	for baseURL, count := range urlCounts {
		orderedURLs = append(orderedURLs, countedURL{URL: baseURL, Count: count})
	}
	sort.Slice(orderedURLs, func(i, j int) bool {
		if orderedURLs[i].Count == orderedURLs[j].Count {
			return orderedURLs[i].URL < orderedURLs[j].URL
		}
		return orderedURLs[i].Count > orderedURLs[j].Count
	})

	channels := make(map[string]struct{})
	for _, group := range groups {
		if group.GroupType != "aggregate" && strings.TrimSpace(group.ChannelType) != "" {
			channels[group.ChannelType] = struct{}{}
		}
	}
	channelNames := make([]string, 0, len(channels))
	for channelType := range channels {
		channelNames = append(channelNames, channelType)
	}
	sort.Strings(channelNames)
	if len(channelNames) == 0 && len(orderedURLs) > 0 {
		// The old schema did not record a protocol on the pool itself. Preserve
		// every URL as an explicitly unassigned endpoint for an administrator to
		// classify later instead of guessing a channel type or losing data.
		channelNames = append(channelNames, "legacy")
	}

	preferredEndpoint := make(map[string]uint)
	for _, channelType := range channelNames {
		var existing []models.ResourcePoolEndpoint
		if err := tx.Where("resource_pool_id = ? AND channel_type = ?", poolID, channelType).Order("id asc").Find(&existing).Error; err != nil {
			return err
		}
		for _, endpoint := range existing {
			if models.CredentialEnabled(endpoint.Enabled) && preferredEndpoint[channelType] == 0 {
				preferredEndpoint[channelType] = endpoint.ID
			}
		}
		for index, entry := range orderedURLs {
			var endpoint models.ResourcePoolEndpoint
			err := tx.Where("resource_pool_id = ? AND channel_type = ? AND base_url = ?", poolID, channelType, entry.URL).
				First(&endpoint).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if err == gorm.ErrRecordNotFound {
				endpointName, nameErr := uniqueMigrationEndpointName(tx, poolID, fmt.Sprintf("migrated-%s-%d", channelType, index+1))
				if nameErr != nil {
					return nameErr
				}
				endpoint = models.ResourcePoolEndpoint{
					ResourcePoolID: poolID,
					Name:           endpointName,
					ChannelType:    channelType,
					BaseURL:        entry.URL,
					Enabled:        models.Bool(true),
				}
				if err := tx.Create(&endpoint).Error; err != nil {
					return err
				}
			}
			if index == 0 {
				preferredEndpoint[channelType] = endpoint.ID
			}
		}
	}

	for i := range groups {
		group := &groups[i]
		if group.ResourceEndpointID != nil && *group.ResourceEndpointID > 0 {
			continue
		}
		endpointID := preferredEndpoint[group.ChannelType]
		if endpointID == 0 {
			continue
		}
		if err := tx.Model(group).Update("resource_endpoint_id", endpointID).Error; err != nil {
			return err
		}
		group.ResourceEndpointID = &endpointID
	}

	for _, group := range groups {
		if group.ResourceEndpointID == nil || *group.ResourceEndpointID == 0 {
			continue
		}
		if err := tx.Model(&models.UpstreamObjectBinding{}).
			Where("group_id = ? AND resource_pool_id = ? AND resource_endpoint_id = 0", group.ID, poolID).
			Update("resource_endpoint_id", *group.ResourceEndpointID).Error; err != nil {
			return err
		}
	}
	return nil
}

func uniqueMigrationEndpointName(tx *gorm.DB, poolID uint, base string) (string, error) {
	for suffix := 0; ; suffix++ {
		name := base
		if suffix > 0 {
			name = fmt.Sprintf("%s-%d", base, suffix+1)
		}
		var count int64
		if err := tx.Model(&models.ResourcePoolEndpoint{}).
			Where("resource_pool_id = ? AND name = ?", poolID, name).
			Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return name, nil
		}
	}
}

func deduplicateResourcePoolCredentials(tx *gorm.DB, poolID uint) error {
	var resources []models.UpstreamResource
	if err := tx.Where("resource_pool_id = ?", poolID).Order("id asc").Find(&resources).Error; err != nil {
		return err
	}
	grouped := make(map[string][]models.UpstreamResource)
	for _, resource := range resources {
		identity := strings.TrimSpace(resource.KeyHash)
		if identity == "" {
			identity = strings.TrimSpace(resource.IdentityHash)
		}
		grouped[identity] = append(grouped[identity], resource)
	}
	for identity, duplicates := range grouped {
		if identity == "" || len(duplicates) == 0 {
			continue
		}
		canonical := duplicates[0]
		for _, duplicate := range duplicates[1:] {
			mergeResourceState(&canonical, &duplicate)
			if err := tx.Model(&models.RequestLog{}).Where("resource_id = ?", duplicate.ID).Update("resource_id", canonical.ID).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.UpstreamObjectBinding{}).Where("resource_id = ?", duplicate.ID).Update("resource_id", canonical.ID).Error; err != nil {
				return err
			}
			if err := tx.Delete(&models.UpstreamResource{}, duplicate.ID).Error; err != nil {
				return err
			}
		}
		canonical.UpstreamURL = ""
		canonical.IdentityHash = identity
		if err := tx.Save(&canonical).Error; err != nil {
			return err
		}
	}
	return nil
}

func mergeResourceState(target, source *models.UpstreamResource) {
	if !models.CredentialEnabled(source.Enabled) {
		target.Enabled = models.Bool(false)
	}
	if source.Status == models.ResourceStatusInvalid {
		target.Status = models.ResourceStatusInvalid
	}
	if target.DisabledReason == "" {
		target.DisabledReason = source.DisabledReason
	}
	if source.Priority < target.Priority {
		target.Priority = source.Priority
	}
	if source.Weight > target.Weight {
		target.Weight = source.Weight
	}
	target.RequestCount += source.RequestCount
	target.TotalFailureCount += source.TotalFailureCount
	target.FailureCount += source.FailureCount
	target.GlobalCooldownUntil = laterTime(target.GlobalCooldownUntil, source.GlobalCooldownUntil)
	target.LastUsedAt = laterTime(target.LastUsedAt, source.LastUsedAt)
	target.LastSuccessAt = laterTime(target.LastSuccessAt, source.LastSuccessAt)
	target.LastFailureAt = laterTime(target.LastFailureAt, source.LastFailureAt)
}

func laterTime(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || left.After(*right) {
		return left
	}
	return right
}
