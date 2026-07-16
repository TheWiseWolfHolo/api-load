package services

import (
	"context"
	"fmt"
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
}

type ResourcePoolView struct {
	ID                   uint           `json:"id"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	Strategy             string         `json:"strategy"`
	AffinityTTLSeconds   int            `json:"affinity_ttl_seconds"`
	BusyWaitMilliseconds int            `json:"busy_wait_milliseconds"`
	ResourceCount        int            `json:"resource_count"`
	Resources            []ResourceView `json:"resources,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type ResourceView struct {
	ID                  uint       `json:"id"`
	ResourcePoolID      uint       `json:"resource_pool_id"`
	Name                string     `json:"name"`
	UpstreamURL         string     `json:"upstream_url"`
	MaskedKey           string     `json:"masked_key"`
	Status              string     `json:"status"`
	FailureCount        int64      `json:"failure_count"`
	GlobalCooldownUntil *time.Time `json:"global_cooldown_until,omitempty"`
	DisabledReason      string     `json:"disabled_reason,omitempty"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
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
	view := s.poolView(&pool, nil)
	return &view, nil
}

func (s *ResourcePoolService) ListPools(ctx context.Context) ([]ResourcePoolView, error) {
	var pools []models.ResourcePool
	if err := s.db.WithContext(ctx).Preload("Resources").Order("id desc").Find(&pools).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	views := make([]ResourcePoolView, 0, len(pools))
	for i := range pools {
		views = append(views, s.poolView(&pools[i], pools[i].Resources))
	}
	return views, nil
}

func (s *ResourcePoolService) GetPool(ctx context.Context, id uint) (*ResourcePoolView, error) {
	pool, err := s.loadPool(ctx, id, true)
	if err != nil {
		return nil, err
	}
	view := s.poolView(pool, pool.Resources)
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
		resources = append(resources, models.UpstreamResource{
			Name:        strings.TrimSpace(item.Name),
			UpstreamURL: item.UpstreamURL,
			KeyValue:    item.Key,
			Status:      models.ResourceStatusActive,
		})
	}
	if err := s.provider.AddResources(poolID, resources); err != nil {
		if strings.Contains(err.Error(), "persist upstream resources") {
			return nil, app_errors.ParseDBError(err)
		}
		return nil, resourcePoolValidationError(err.Error())
	}
	return s.ListResources(ctx, poolID)
}

func (s *ResourcePoolService) ListResources(ctx context.Context, poolID uint) ([]ResourceView, error) {
	if _, err := s.loadPool(ctx, poolID, false); err != nil {
		return nil, err
	}
	var resources []models.UpstreamResource
	if err := s.db.WithContext(ctx).Where("resource_pool_id = ?", poolID).Order("id asc").Find(&resources).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	views := make([]ResourceView, 0, len(resources))
	for i := range resources {
		views = append(views, s.resourceView(&resources[i]))
	}
	return views, nil
}

func (s *ResourcePoolService) UpdateResourceStatus(ctx context.Context, poolID, resourceID uint, status string) (*ResourceView, error) {
	resource, err := s.loadResource(ctx, poolID, resourceID)
	if err != nil {
		return nil, err
	}
	switch status {
	case models.ResourceStatusDisabled:
		resource.Status = models.ResourceStatusDisabled
		resource.DisabledReason = "disabled by administrator"
	case models.ResourceStatusActive:
		resource.Status = models.ResourceStatusActive
		resource.DisabledReason = ""
		resource.FailureCount = 0
		resource.GlobalCooldownUntil = nil
	default:
		return nil, resourcePoolValidationError("resource status must be active or disabled")
	}
	updates := map[string]any{
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

func (s *ResourcePoolService) poolView(pool *models.ResourcePool, resources []models.UpstreamResource) ResourcePoolView {
	resourceViews := make([]ResourceView, 0, len(resources))
	for i := range resources {
		resourceViews = append(resourceViews, s.resourceView(&resources[i]))
	}
	return ResourcePoolView{
		ID:                   pool.ID,
		Name:                 pool.Name,
		Description:          pool.Description,
		Strategy:             pool.Strategy,
		AffinityTTLSeconds:   pool.AffinityTTLSeconds,
		BusyWaitMilliseconds: pool.BusyWaitMilliseconds,
		ResourceCount:        len(resources),
		Resources:            resourceViews,
		CreatedAt:            pool.CreatedAt,
		UpdatedAt:            pool.UpdatedAt,
	}
}

func (s *ResourcePoolService) resourceView(resource *models.UpstreamResource) ResourceView {
	return ResourceView{
		ID:                  resource.ID,
		ResourcePoolID:      resource.ResourcePoolID,
		Name:                resource.Name,
		UpstreamURL:         resource.UpstreamURL,
		MaskedKey:           s.maskCredential(resource.KeyValue),
		Status:              resource.Status,
		FailureCount:        resource.FailureCount,
		GlobalCooldownUntil: resource.GlobalCooldownUntil,
		DisabledReason:      resource.DisabledReason,
		LastUsedAt:          resource.LastUsedAt,
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
