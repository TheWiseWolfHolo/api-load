package services

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"api-load/internal/encryption"
	app_errors "api-load/internal/errors"
	"api-load/internal/models"
	"api-load/internal/resourcepool"

	"gorm.io/gorm"
)

const (
	defaultResourcePoolStrategy = "round_robin"
	defaultAffinityTTLSeconds   = 3600
	defaultBusyWaitMilliseconds = 2000
)

type ResourcePoolService struct {
	db            *gorm.DB
	provider      *resourcepool.Provider
	encryptionSvc encryption.Service
}

func NewResourcePoolService(db *gorm.DB, provider *resourcepool.Provider, encryptionSvc encryption.Service) *ResourcePoolService {
	return &ResourcePoolService{db: db, provider: provider, encryptionSvc: encryptionSvc}
}

type ResourcePoolCreateParams struct {
	Name                 string
	Description          string
	Strategy             string
	AffinityTTLSeconds   int
	BusyWaitMilliseconds int
}

type ResourcePoolUpdateParams struct {
	Name                 *string
	Description          *string
	AffinityTTLSeconds   *int
	BusyWaitMilliseconds *int
}

type ResourceCreateParams struct {
	Name        string `json:"name"`
	UpstreamURL string `json:"upstream_url"`
	Key         string `json:"key"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Weight      int    `json:"weight,omitempty"`
}

type ResourceUpdateParams struct {
	Name        string
	UpstreamURL string
	Key         *string
	Enabled     *bool
	Status      *string
	Priority    *int
	Weight      *int
}

type ResourceBatchUpdateParams struct {
	Enabled  *bool
	Status   *string
	Priority *int
	Weight   *int
}

type ResourceListParams struct {
	Page     int
	PageSize int
	Search   string
	Status   string
	Enabled  *bool
}

type ResourcePagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type ResourceListView struct {
	Items      []ResourceView     `json:"items"`
	Pagination ResourcePagination `json:"pagination"`
}

type BulkResourceStatusResult struct {
	RequestedCount int `json:"requested_count"`
	MatchedCount   int `json:"matched_count"`
	UpdatedCount   int `json:"updated_count"`
}

type BulkResourceDeleteResult struct {
	RequestedIDCount  int `json:"requested_id_count"`
	RequestedKeyCount int `json:"requested_key_count"`
	MatchedCount      int `json:"matched_count"`
	DeletedCount      int `json:"deleted_count"`
	BlockedCount      int `json:"blocked_count"`
	MissingKeyCount   int `json:"missing_key_count"`
}

type ResourcePoolView struct {
	ID                   uint      `json:"id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	Strategy             string    `json:"strategy"`
	AffinityTTLSeconds   int       `json:"affinity_ttl_seconds"`
	BusyWaitMilliseconds int       `json:"busy_wait_milliseconds"`
	ResourceCount        int       `json:"resource_count"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type ResourceView struct {
	ID                  uint       `json:"id"`
	ResourcePoolID      uint       `json:"resource_pool_id"`
	Name                string     `json:"name"`
	UpstreamURL         string     `json:"upstream_url"`
	MaskedKey           string     `json:"masked_key"`
	Status              string     `json:"status"`
	Enabled             bool       `json:"enabled"`
	Priority            int        `json:"priority"`
	Weight              int        `json:"weight"`
	RequestCount        int64      `json:"request_count"`
	TotalFailureCount   int64      `json:"total_failure_count"`
	FailureCount        int64      `json:"failure_count"`
	GlobalCooldownUntil *time.Time `json:"global_cooldown_until,omitempty"`
	DisabledReason      string     `json:"disabled_reason,omitempty"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (s *ResourcePoolService) CreatePool(ctx context.Context, params ResourcePoolCreateParams) (*ResourcePoolView, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, resourcePoolValidationError("resource pool name is required")
	}
	strategy := strings.TrimSpace(params.Strategy)
	if strategy == "" {
		strategy = defaultResourcePoolStrategy
	}
	if strategy != defaultResourcePoolStrategy {
		return nil, resourcePoolValidationError("only round_robin resource pool strategy is supported")
	}
	ttl := params.AffinityTTLSeconds
	if ttl == 0 {
		ttl = defaultAffinityTTLSeconds
	}
	busyWait := params.BusyWaitMilliseconds
	if busyWait == 0 {
		busyWait = defaultBusyWaitMilliseconds
	}
	if err := validateResourcePoolTiming(ttl, busyWait); err != nil {
		return nil, err
	}

	pool := models.ResourcePool{
		Name:                 name,
		Description:          strings.TrimSpace(params.Description),
		Strategy:             strategy,
		AffinityTTLSeconds:   ttl,
		BusyWaitMilliseconds: busyWait,
	}
	if err := s.db.WithContext(ctx).Create(&pool).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	if err := s.provider.SyncPoolToStore(&pool); err != nil {
		_ = s.db.WithContext(ctx).Delete(&pool).Error
		return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	view := s.poolView(&pool, 0)
	return &view, nil
}

func (s *ResourcePoolService) ListPools(ctx context.Context) ([]ResourcePoolView, error) {
	var pools []models.ResourcePool
	if err := s.db.WithContext(ctx).Order("id desc").Find(&pools).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	counts, err := s.resourceCounts(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]ResourcePoolView, 0, len(pools))
	for i := range pools {
		views = append(views, s.poolView(&pools[i], counts[pools[i].ID]))
	}
	return views, nil
}

func (s *ResourcePoolService) GetPool(ctx context.Context, id uint) (*ResourcePoolView, error) {
	pool, err := s.loadPool(ctx, id, false)
	if err != nil {
		return nil, err
	}
	count, err := s.resourceCount(ctx, id)
	if err != nil {
		return nil, err
	}
	view := s.poolView(pool, count)
	return &view, nil
}

func (s *ResourcePoolService) UpdatePool(ctx context.Context, id uint, params ResourcePoolUpdateParams) (*ResourcePoolView, error) {
	pool, err := s.loadPool(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if params.Name != nil {
		name := strings.TrimSpace(*params.Name)
		if name == "" {
			return nil, resourcePoolValidationError("resource pool name is required")
		}
		pool.Name = name
	}
	if params.Description != nil {
		pool.Description = strings.TrimSpace(*params.Description)
	}
	if params.AffinityTTLSeconds != nil {
		pool.AffinityTTLSeconds = *params.AffinityTTLSeconds
	}
	if params.BusyWaitMilliseconds != nil {
		pool.BusyWaitMilliseconds = *params.BusyWaitMilliseconds
	}
	if err := validateResourcePoolTiming(pool.AffinityTTLSeconds, pool.BusyWaitMilliseconds); err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Save(pool).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	if err := s.provider.SyncPoolToStore(pool); err != nil {
		return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	return s.GetPool(ctx, id)
}

func (s *ResourcePoolService) DeletePool(ctx context.Context, id uint) error {
	pool, err := s.loadPool(ctx, id, true)
	if err != nil {
		return err
	}
	var groupCount int64
	if err := s.db.WithContext(ctx).Model(&models.Group{}).Where("resource_pool_id = ?", id).Count(&groupCount).Error; err != nil {
		return app_errors.ParseDBError(err)
	}
	if groupCount > 0 {
		return app_errors.NewAPIError(app_errors.ErrResourceInUse, "resource pool is still referenced by groups")
	}
	var objectCount int64
	if err := s.db.WithContext(ctx).Model(&models.UpstreamObjectBinding{}).Where("resource_pool_id = ?", id).Count(&objectCount).Error; err != nil {
		return app_errors.ParseDBError(err)
	}
	if objectCount > 0 {
		return app_errors.NewAPIError(app_errors.ErrResourceInUse, "resource pool still owns upstream batch or file objects")
	}

	removed := make([]models.UpstreamResource, 0, len(pool.Resources))
	for i := range pool.Resources {
		if err := s.provider.RemoveResourceFromStore(&pool.Resources[i]); err != nil {
			for j := range removed {
				_ = s.provider.SyncResourceToStore(&removed[j])
			}
			return app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
		}
		removed = append(removed, pool.Resources[i])
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		s.restoreResourcesToStore(removed)
		return app_errors.ErrDatabase
	}
	if err := tx.Where("resource_pool_id = ?", id).Delete(&models.UpstreamResource{}).Error; err != nil {
		tx.Rollback()
		s.restoreResourcesToStore(removed)
		return app_errors.ParseDBError(err)
	}
	if err := tx.Delete(&models.ResourcePool{}, id).Error; err != nil {
		tx.Rollback()
		s.restoreResourcesToStore(removed)
		return app_errors.ParseDBError(err)
	}
	if err := tx.Commit().Error; err != nil {
		s.restoreResourcesToStore(removed)
		return app_errors.ParseDBError(err)
	}
	if err := s.provider.RemovePoolFromStore(id); err != nil {
		return app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	return nil
}

func (s *ResourcePoolService) AddResources(ctx context.Context, poolID uint, params []ResourceCreateParams) ([]ResourceView, error) {
	if _, err := s.loadPool(ctx, poolID, false); err != nil {
		return nil, err
	}
	if len(params) == 0 {
		return nil, resourcePoolValidationError("at least one resource is required")
	}
	resources := make([]models.UpstreamResource, 0, len(params))
	for _, item := range params {
		priority := item.Priority
		if priority == 0 {
			priority = models.DefaultCredentialPriority
		}
		weight := item.Weight
		if weight == 0 {
			weight = models.DefaultCredentialWeight
		}
		if priority < 1 || priority > 1000 || weight < 1 || weight > 1000 {
			return nil, resourcePoolValidationError("priority and weight must be between 1 and 1000")
		}
		enabled := item.Enabled
		if enabled == nil {
			enabled = models.Bool(true)
		}
		resources = append(resources, models.UpstreamResource{
			Name:        strings.TrimSpace(item.Name),
			UpstreamURL: item.UpstreamURL,
			KeyValue:    item.Key,
			Status:      models.ResourceStatusActive,
			Enabled:     enabled,
			Priority:    priority,
			Weight:      weight,
		})
	}
	created, err := s.provider.AddResources(poolID, resources)
	if err != nil {
		if strings.Contains(err.Error(), "persist upstream resources") {
			return nil, app_errors.ParseDBError(err)
		}
		return nil, resourcePoolValidationError(err.Error())
	}
	views := make([]ResourceView, 0, len(created))
	for i := range created {
		views = append(views, s.resourceView(&created[i]))
	}
	return views, nil
}

func (s *ResourcePoolService) ListResources(ctx context.Context, poolID uint, params ResourceListParams) (*ResourceListView, error) {
	if _, err := s.loadPool(ctx, poolID, false); err != nil {
		return nil, err
	}
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	status := strings.TrimSpace(params.Status)
	if status != "" && status != models.ResourceStatusActive && status != models.ResourceStatusInvalid && status != models.ResourceStatusDisabled {
		return nil, resourcePoolValidationError("invalid resource status filter")
	}

	query := s.db.WithContext(ctx).Model(&models.UpstreamResource{}).Where("resource_pool_id = ?", poolID)
	enabledFilter := params.Enabled
	if status == models.ResourceStatusDisabled {
		disabled := false
		enabledFilter = &disabled
	} else if status != "" {
		query = query.Where("status = ?", status)
		if enabledFilter == nil {
			enabled := true
			enabledFilter = &enabled
		}
	}
	if enabledFilter != nil {
		query = query.Where("enabled = ?", *enabledFilter)
	}
	if search := strings.TrimSpace(params.Search); search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR upstream_url LIKE ? OR key_hash = ?", like, like, s.encryptionSvc.Hash(search))
	}
	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	var resources []models.UpstreamResource
	if err := query.Order("id asc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&resources).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	views := make([]ResourceView, 0, len(resources))
	for i := range resources {
		views = append(views, s.resourceView(&resources[i]))
	}
	return &ResourceListView{
		Items: views,
		Pagination: ResourcePagination{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: int(math.Ceil(float64(totalItems) / float64(pageSize))),
		},
	}, nil
}

func (s *ResourcePoolService) UpdateResource(ctx context.Context, poolID, resourceID uint, params ResourceUpdateParams) (*ResourceView, error) {
	resource, err := s.loadResource(ctx, poolID, resourceID)
	if err != nil {
		return nil, err
	}
	if params.Key != nil && strings.TrimSpace(*params.Key) == "" {
		return nil, resourcePoolValidationError("replacement key cannot be empty")
	}
	if params.Priority != nil && (*params.Priority < 1 || *params.Priority > 1000) {
		return nil, resourcePoolValidationError("priority must be between 1 and 1000")
	}
	if params.Weight != nil && (*params.Weight < 1 || *params.Weight > 1000) {
		return nil, resourcePoolValidationError("weight must be between 1 and 1000")
	}
	if params.Status != nil && *params.Status != models.ResourceStatusActive && *params.Status != models.ResourceStatusInvalid {
		return nil, resourcePoolValidationError("health status must be active or invalid")
	}
	if err := s.provider.UpdateResource(resource, params.Name, params.UpstreamURL, params.Key); err != nil {
		if strings.Contains(err.Error(), "persist upstream resource") {
			return nil, app_errors.ParseDBError(err)
		}
		return nil, resourcePoolValidationError(err.Error())
	}
	updates := make(map[string]any)
	if params.Enabled != nil {
		resource.Enabled = models.Bool(*params.Enabled)
		updates["enabled"] = *params.Enabled
	}
	if params.Status != nil {
		resource.Status = *params.Status
		updates["status"] = *params.Status
		if *params.Status == models.ResourceStatusActive {
			resource.FailureCount = 0
			updates["failure_count"] = 0
		}
	}
	if params.Priority != nil {
		resource.Priority = *params.Priority
		updates["priority"] = *params.Priority
	}
	if params.Weight != nil {
		resource.Weight = *params.Weight
		updates["weight"] = *params.Weight
	}
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(resource).Updates(updates).Error; err != nil {
			return nil, app_errors.ParseDBError(err)
		}
		if err := s.provider.SyncResourceToStore(resource); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
		}
	}
	view := s.resourceView(resource)
	return &view, nil
}

func (s *ResourcePoolService) BulkUpdateResourceStatus(ctx context.Context, poolID uint, resourceIDs []uint, status string) (*BulkResourceStatusResult, error) {
	switch status {
	case models.ResourceStatusDisabled:
		return s.BulkUpdateResources(ctx, poolID, resourceIDs, ResourceBatchUpdateParams{Enabled: models.Bool(false)})
	case models.ResourceStatusActive:
		active := models.ResourceStatusActive
		return s.BulkUpdateResources(ctx, poolID, resourceIDs, ResourceBatchUpdateParams{Enabled: models.Bool(true), Status: &active})
	default:
		return nil, resourcePoolValidationError("resource status must be active or disabled")
	}
}

func (s *ResourcePoolService) BulkUpdateResources(ctx context.Context, poolID uint, resourceIDs []uint, params ResourceBatchUpdateParams) (*BulkResourceStatusResult, error) {
	if _, err := s.loadPool(ctx, poolID, false); err != nil {
		return nil, err
	}
	ids := uniqueResourceIDs(resourceIDs)
	if len(ids) == 0 {
		return nil, resourcePoolValidationError("at least one resource ID is required")
	}
	if params.Priority != nil && (*params.Priority < 1 || *params.Priority > 1000) {
		return nil, resourcePoolValidationError("priority must be between 1 and 1000")
	}
	if params.Weight != nil && (*params.Weight < 1 || *params.Weight > 1000) {
		return nil, resourcePoolValidationError("weight must be between 1 and 1000")
	}
	if params.Status != nil && *params.Status != models.ResourceStatusActive && *params.Status != models.ResourceStatusInvalid {
		return nil, resourcePoolValidationError("health status must be active or invalid")
	}
	updates := make(map[string]any)
	if params.Enabled != nil {
		updates["enabled"] = *params.Enabled
	}
	if params.Status != nil {
		updates["status"] = *params.Status
		if *params.Status == models.ResourceStatusActive {
			updates["failure_count"] = 0
		}
	}
	if params.Priority != nil {
		updates["priority"] = *params.Priority
	}
	if params.Weight != nil {
		updates["weight"] = *params.Weight
	}
	if len(updates) == 0 {
		return nil, resourcePoolValidationError("at least one field is required")
	}
	result := &BulkResourceStatusResult{RequestedCount: len(ids)}
	var resources []models.UpstreamResource
	if err := s.db.WithContext(ctx).Where("resource_pool_id = ? AND id IN ?", poolID, ids).Find(&resources).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	result.MatchedCount = len(resources)
	if len(resources) == 0 {
		return result, nil
	}
	matchedIDs := make([]uint, 0, len(resources))
	for _, resource := range resources {
		matchedIDs = append(matchedIDs, resource.ID)
	}
	dbResult := s.db.WithContext(ctx).Model(&models.UpstreamResource{}).Where("resource_pool_id = ? AND id IN ?", poolID, matchedIDs).Updates(updates)
	if dbResult.Error != nil {
		return nil, app_errors.ParseDBError(dbResult.Error)
	}
	result.UpdatedCount = int(dbResult.RowsAffected)
	if err := s.db.WithContext(ctx).Where("resource_pool_id = ? AND id IN ?", poolID, matchedIDs).Find(&resources).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	if err := s.provider.SyncResourcesToStore(resources); err != nil {
		return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	return result, nil
}

func (s *ResourcePoolService) BulkDeleteResources(ctx context.Context, poolID uint, resourceIDs []uint, keys []string) (*BulkResourceDeleteResult, error) {
	if _, err := s.loadPool(ctx, poolID, false); err != nil {
		return nil, err
	}
	ids := uniqueResourceIDs(resourceIDs)
	keyHashes := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keyHashes[s.encryptionSvc.Hash(trimmed)] = struct{}{}
		}
	}
	result := &BulkResourceDeleteResult{RequestedIDCount: len(ids), RequestedKeyCount: len(keyHashes)}
	if len(ids) == 0 && len(keyHashes) == 0 {
		return nil, resourcePoolValidationError("at least one resource ID or key is required")
	}

	hashes := make([]string, 0, len(keyHashes))
	for hash := range keyHashes {
		hashes = append(hashes, hash)
	}
	query := s.db.WithContext(ctx).Where("resource_pool_id = ?", poolID)
	switch {
	case len(ids) > 0 && len(hashes) > 0:
		query = query.Where("id IN ? OR key_hash IN ?", ids, hashes)
	case len(ids) > 0:
		query = query.Where("id IN ?", ids)
	default:
		query = query.Where("key_hash IN ?", hashes)
	}
	var resources []models.UpstreamResource
	if err := query.Order("id asc").Find(&resources).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	result.MatchedCount = len(resources)
	matchedHashes := make(map[string]struct{}, len(resources))
	resourceIDsFound := make([]uint, 0, len(resources))
	for i := range resources {
		matchedHashes[resources[i].KeyHash] = struct{}{}
		resourceIDsFound = append(resourceIDsFound, resources[i].ID)
	}
	for hash := range keyHashes {
		if _, ok := matchedHashes[hash]; !ok {
			result.MissingKeyCount++
		}
	}

	blocked := make(map[uint]struct{})
	if len(resourceIDsFound) > 0 {
		var bindings []models.UpstreamObjectBinding
		if err := s.db.WithContext(ctx).Where("resource_id IN ?", resourceIDsFound).Find(&bindings).Error; err != nil {
			return nil, app_errors.ParseDBError(err)
		}
		for i := range bindings {
			blocked[bindings[i].ResourceID] = struct{}{}
		}
	}
	for i := range resources {
		resource := &resources[i]
		if _, isBlocked := blocked[resource.ID]; isBlocked {
			result.BlockedCount++
			continue
		}
		if err := s.provider.RemoveResourceFromStore(resource); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
		}
		if err := s.db.WithContext(ctx).Delete(resource).Error; err != nil {
			_ = s.provider.SyncResourceToStore(resource)
			return nil, app_errors.ParseDBError(err)
		}
		result.DeletedCount++
	}
	if result.DeletedCount > 0 {
		if err := s.provider.RefreshScheduler(poolID); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
		}
	}
	return result, nil
}

func (s *ResourcePoolService) UpdateResourceStatus(ctx context.Context, poolID, resourceID uint, status string) (*ResourceView, error) {
	resource, err := s.loadResource(ctx, poolID, resourceID)
	if err != nil {
		return nil, err
	}
	switch status {
	case models.ResourceStatusDisabled:
		resource.Enabled = models.Bool(false)
		resource.DisabledReason = "disabled by administrator"
	case models.ResourceStatusActive:
		resource.Enabled = models.Bool(true)
		resource.Status = models.ResourceStatusActive
		resource.DisabledReason = ""
		resource.FailureCount = 0
		resource.GlobalCooldownUntil = nil
	default:
		return nil, resourcePoolValidationError("resource status must be active or disabled")
	}
	updates := map[string]any{
		"enabled":               models.CredentialEnabled(resource.Enabled),
		"status":                resource.Status,
		"disabled_reason":       resource.DisabledReason,
		"failure_count":         resource.FailureCount,
		"global_cooldown_until": resource.GlobalCooldownUntil,
	}
	if err := s.db.WithContext(ctx).Model(resource).Updates(updates).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	if err := s.provider.SyncResourceToStore(resource); err != nil {
		return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	view := s.resourceView(resource)
	return &view, nil
}

func (s *ResourcePoolService) DeleteResource(ctx context.Context, poolID, resourceID uint) error {
	resource, err := s.loadResource(ctx, poolID, resourceID)
	if err != nil {
		return err
	}
	var objectCount int64
	if err := s.db.WithContext(ctx).Model(&models.UpstreamObjectBinding{}).Where("resource_id = ?", resourceID).Count(&objectCount).Error; err != nil {
		return app_errors.ParseDBError(err)
	}
	if objectCount > 0 {
		return app_errors.NewAPIError(app_errors.ErrResourceInUse, "resource still owns upstream batch or file objects; disable it instead")
	}
	if err := s.provider.RemoveResourceFromStore(resource); err != nil {
		return app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	if err := s.db.WithContext(ctx).Delete(resource).Error; err != nil {
		_ = s.provider.SyncResourceToStore(resource)
		return app_errors.ParseDBError(err)
	}
	if err := s.provider.RefreshScheduler(poolID); err != nil {
		return app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	return nil
}

func (s *ResourcePoolService) loadPool(ctx context.Context, id uint, preloadResources bool) (*models.ResourcePool, error) {
	if id == 0 {
		return nil, app_errors.ErrResourceNotFound
	}
	db := s.db.WithContext(ctx)
	if preloadResources {
		db = db.Preload("Resources")
	}
	var pool models.ResourcePool
	if err := db.First(&pool, id).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	return &pool, nil
}

func (s *ResourcePoolService) loadResource(ctx context.Context, poolID, resourceID uint) (*models.UpstreamResource, error) {
	var resource models.UpstreamResource
	if err := s.db.WithContext(ctx).Where("id = ? AND resource_pool_id = ?", resourceID, poolID).First(&resource).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	return &resource, nil
}

func (s *ResourcePoolService) poolView(pool *models.ResourcePool, resourceCount int) ResourcePoolView {
	return ResourcePoolView{
		ID:                   pool.ID,
		Name:                 pool.Name,
		Description:          pool.Description,
		Strategy:             pool.Strategy,
		AffinityTTLSeconds:   pool.AffinityTTLSeconds,
		BusyWaitMilliseconds: pool.BusyWaitMilliseconds,
		ResourceCount:        resourceCount,
		CreatedAt:            pool.CreatedAt,
		UpdatedAt:            pool.UpdatedAt,
	}
}

func (s *ResourcePoolService) resourceCount(ctx context.Context, poolID uint) (int, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UpstreamResource{}).Where("resource_pool_id = ?", poolID).Count(&count).Error; err != nil {
		return 0, app_errors.ParseDBError(err)
	}
	return int(count), nil
}

func (s *ResourcePoolService) resourceCounts(ctx context.Context) (map[uint]int, error) {
	type countRow struct {
		ResourcePoolID uint
		Count          int
	}
	var rows []countRow
	if err := s.db.WithContext(ctx).Model(&models.UpstreamResource{}).
		Select("resource_pool_id, COUNT(*) AS count").
		Group("resource_pool_id").
		Scan(&rows).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	counts := make(map[uint]int, len(rows))
	for _, row := range rows {
		counts[row.ResourcePoolID] = row.Count
	}
	return counts, nil
}

func uniqueResourceIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	unique := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func (s *ResourcePoolService) resourceView(resource *models.UpstreamResource) ResourceView {
	return ResourceView{
		ID:                  resource.ID,
		ResourcePoolID:      resource.ResourcePoolID,
		Name:                resource.Name,
		UpstreamURL:         resource.UpstreamURL,
		MaskedKey:           s.maskCredential(resource.KeyValue),
		Status:              resource.Status,
		Enabled:             models.CredentialEnabled(resource.Enabled),
		Priority:            resource.Priority,
		Weight:              resource.Weight,
		RequestCount:        resource.RequestCount,
		TotalFailureCount:   resource.TotalFailureCount,
		FailureCount:        resource.FailureCount,
		GlobalCooldownUntil: resource.GlobalCooldownUntil,
		DisabledReason:      resource.DisabledReason,
		LastUsedAt:          resource.LastUsedAt,
		LastSuccessAt:       resource.LastSuccessAt,
		LastFailureAt:       resource.LastFailureAt,
		CreatedAt:           resource.CreatedAt,
		UpdatedAt:           resource.UpdatedAt,
	}
}

func (s *ResourcePoolService) maskCredential(encrypted string) string {
	if s.encryptionSvc == nil {
		return "****"
	}
	plain, err := s.encryptionSvc.Decrypt(encrypted)
	if err != nil || len(plain) <= 4 {
		return "****"
	}
	return "****" + plain[len(plain)-4:]
}

func (s *ResourcePoolService) restoreResourcesToStore(resources []models.UpstreamResource) {
	for i := range resources {
		_ = s.provider.SyncResourceToStore(&resources[i])
	}
}

func validateResourcePoolTiming(ttlSeconds, busyWaitMilliseconds int) error {
	if ttlSeconds < 60 || ttlSeconds > 7*24*60*60 {
		return resourcePoolValidationError("affinity_ttl_seconds must be between 60 and 604800")
	}
	if busyWaitMilliseconds < 0 || busyWaitMilliseconds > 10000 {
		return resourcePoolValidationError("busy_wait_milliseconds must be between 0 and 10000")
	}
	return nil
}

func resourcePoolValidationError(message string) error {
	return app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("resource pool validation failed: %s", message))
}
