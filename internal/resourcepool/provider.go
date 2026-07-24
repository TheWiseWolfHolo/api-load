package resourcepool

import (
	"api-load/internal/encryption"
	app_errors "api-load/internal/errors"
	"api-load/internal/keypool"
	"api-load/internal/models"
	"api-load/internal/scheduler"
	"api-load/internal/store"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const DefaultAffinityTTL = time.Hour

const (
	resourceIdentityLookupBatchSize = 500
	resourceInsertBatchSize         = 100
)

type SelectionRequest struct {
	Route              string
	Affinity           string
	ExcludeResourceIDs []uint
	AffinityTTL        time.Duration
}

type PoolConfig struct {
	AffinityTTL         time.Duration
	BusyWait            time.Duration
	AutoRestoreSchedule string
}

type Provider struct {
	db             *gorm.DB
	store          store.Store
	encryptionSvc  encryption.Service
	weightedPicker *scheduler.SmoothPicker
}

func NewProvider(db *gorm.DB, cacheStore store.Store, encryptionSvc encryption.Service) *Provider {
	return &Provider{db: db, store: cacheStore, encryptionSvc: encryptionSvc, weightedPicker: scheduler.NewSmoothPicker()}
}

func (p *Provider) LoadResourcesFromDB() error {
	var pools []models.ResourcePool
	if err := p.db.Find(&pools).Error; err != nil {
		return fmt.Errorf("load resource pools: %w", err)
	}
	var resources []models.UpstreamResource
	if err := p.db.Find(&resources).Error; err != nil {
		return fmt.Errorf("load upstream resources: %w", err)
	}
	var endpoints []models.ResourcePoolEndpoint
	if err := p.db.Find(&endpoints).Error; err != nil {
		return fmt.Errorf("load resource pool endpoints: %w", err)
	}

	for _, pool := range pools {
		if err := p.store.Delete(activeResourcesKey(pool.ID)); err != nil {
			return fmt.Errorf("clear active resources for pool %d: %w", pool.ID, err)
		}
		if err := p.SyncPoolToStore(&pool); err != nil {
			return err
		}
	}
	for i := range resources {
		resource := &resources[i]
		if err := p.syncResourceToStore(resource); err != nil {
			return err
		}
	}
	for i := range endpoints {
		if err := p.SyncEndpointToStore(&endpoints[i]); err != nil {
			return err
		}
	}
	for _, pool := range pools {
		if err := p.syncSchedulerSnapshot(pool.ID); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provider) SyncPoolToStore(pool *models.ResourcePool) error {
	if pool == nil || pool.ID == 0 {
		return errors.New("resource pool ID is required")
	}
	return p.store.HSet(poolKey(pool.ID), map[string]any{
		"affinity_ttl_seconds":   pool.AffinityTTLSeconds,
		"busy_wait_milliseconds": pool.BusyWaitMilliseconds,
		"auto_restore_schedule":  pool.AutoRestoreSchedule,
	})
}

func (p *Provider) RemovePoolFromStore(poolID uint) error {
	if poolID == 0 {
		return errors.New("resource pool ID is required")
	}
	if err := p.store.Delete(activeResourcesKey(poolID)); err != nil {
		return fmt.Errorf("delete active resource list for pool %d: %w", poolID, err)
	}
	if err := p.store.Delete(poolKey(poolID)); err != nil {
		return fmt.Errorf("delete resource pool config %d: %w", poolID, err)
	}
	if err := p.store.Delete(resourceSchedulerSnapshotKey(poolID)); err != nil {
		return fmt.Errorf("delete resource scheduler snapshot %d: %w", poolID, err)
	}
	return nil
}

func (p *Provider) GetPoolConfig(poolID uint) (PoolConfig, error) {
	if poolID == 0 {
		return PoolConfig{}, errors.New("resource pool ID is required")
	}
	details, err := p.store.HGetAll(poolKey(poolID))
	if err != nil || len(details) == 0 {
		var pool models.ResourcePool
		if dbErr := p.db.First(&pool, poolID).Error; dbErr != nil {
			return PoolConfig{}, dbErr
		}
		if syncErr := p.SyncPoolToStore(&pool); syncErr != nil {
			return PoolConfig{}, syncErr
		}
		details = map[string]string{
			"affinity_ttl_seconds":   strconv.Itoa(pool.AffinityTTLSeconds),
			"busy_wait_milliseconds": strconv.Itoa(pool.BusyWaitMilliseconds),
			"auto_restore_schedule":  pool.AutoRestoreSchedule,
		}
	}
	ttlSeconds, _ := strconv.Atoi(details["affinity_ttl_seconds"])
	if ttlSeconds <= 0 {
		ttlSeconds = int(DefaultAffinityTTL / time.Second)
	}
	busyWaitMilliseconds, _ := strconv.Atoi(details["busy_wait_milliseconds"])
	if busyWaitMilliseconds < 0 {
		busyWaitMilliseconds = 0
	}
	return PoolConfig{
		AffinityTTL:         time.Duration(ttlSeconds) * time.Second,
		BusyWait:            time.Duration(busyWaitMilliseconds) * time.Millisecond,
		AutoRestoreSchedule: strings.TrimSpace(details["auto_restore_schedule"]),
	}, nil
}

// AddResources persists shared credentials. Credentials are encrypted at rest
// and deduplicated by key identity within the pool.
func (p *Provider) AddResources(poolID uint, resources []models.UpstreamResource) ([]models.UpstreamResource, error) {
	if poolID == 0 {
		return nil, errors.New("resource pool ID is required")
	}
	if len(resources) == 0 {
		return []models.UpstreamResource{}, nil
	}
	prepared := make([]models.UpstreamResource, 0, len(resources))
	seenIdentities := make(map[string]struct{}, len(resources))
	for i := range resources {
		resource := resources[i]
		resource.ResourcePoolID = poolID
		if err := p.prepareResource(&resource, resource.KeyValue); err != nil {
			return nil, fmt.Errorf("resource %d: %w", i, err)
		}
		if resource.Status == "" {
			resource.Status = models.ResourceStatusActive
		}
		if _, duplicate := seenIdentities[resource.IdentityHash]; duplicate {
			continue
		}
		seenIdentities[resource.IdentityHash] = struct{}{}
		prepared = append(prepared, resource)
	}

	identityHashes := make([]string, 0, len(prepared))
	for i := range prepared {
		identityHashes = append(identityHashes, prepared[i].IdentityHash)
	}
	existingIdentityHashes := make([]string, 0)
	for start := 0; start < len(identityHashes); start += resourceIdentityLookupBatchSize {
		end := min(start+resourceIdentityLookupBatchSize, len(identityHashes))
		var batch []string
		if err := p.db.Model(&models.UpstreamResource{}).
			Where("resource_pool_id = ? AND identity_hash IN ?", poolID, identityHashes[start:end]).
			Pluck("identity_hash", &batch).Error; err != nil {
			return nil, fmt.Errorf("persist upstream resources: load existing identities: %w", err)
		}
		existingIdentityHashes = append(existingIdentityHashes, batch...)
	}
	existingIdentities := make(map[string]struct{}, len(existingIdentityHashes))
	for _, identityHash := range existingIdentityHashes {
		existingIdentities[identityHash] = struct{}{}
	}
	newResources := prepared[:0]
	for i := range prepared {
		if _, exists := existingIdentities[prepared[i].IdentityHash]; exists {
			continue
		}
		newResources = append(newResources, prepared[i])
	}
	prepared = newResources
	if len(prepared) == 0 {
		return []models.UpstreamResource{}, nil
	}

	if err := p.db.CreateInBatches(&prepared, resourceInsertBatchSize).Error; err != nil {
		return nil, fmt.Errorf("persist upstream resources: %w", err)
	}
	for i := range prepared {
		if err := p.syncResourceToStore(&prepared[i]); err != nil {
			return nil, err
		}
	}
	if err := p.syncSchedulerSnapshot(poolID); err != nil {
		return nil, err
	}
	return prepared, nil
}

// UpdateResource changes the user-managed fields of one shared credential.
// A nil key keeps the existing credential.
func (p *Provider) UpdateResource(resource *models.UpstreamResource, name string, key *string) error {
	if resource == nil || resource.ID == 0 || resource.ResourcePoolID == 0 {
		return errors.New("resource and pool IDs are required")
	}
	plainKey := ""
	if key == nil {
		decrypted, err := p.encryptionSvc.Decrypt(resource.KeyValue)
		if err != nil {
			return fmt.Errorf("decrypt existing resource key: %w", err)
		}
		plainKey = decrypted
	} else {
		plainKey = *key
	}

	updated := *resource
	updated.Name = strings.TrimSpace(name)
	if err := p.prepareResource(&updated, plainKey); err != nil {
		return err
	}
	if err := p.db.Save(&updated).Error; err != nil {
		return fmt.Errorf("persist upstream resource: %w", err)
	}
	if err := p.SyncResourceToStore(&updated); err != nil {
		return err
	}
	*resource = updated
	return nil
}

func (p *Provider) prepareResource(resource *models.UpstreamResource, plainKey string) error {
	plainKey = strings.TrimSpace(plainKey)
	if plainKey == "" {
		return errors.New("key is required")
	}
	resource.KeyHash = p.encryptionSvc.Hash(plainKey)
	resource.IdentityHash = resource.KeyHash
	encryptedKey, err := p.encryptionSvc.Encrypt(plainKey)
	if err != nil {
		return fmt.Errorf("encrypt key: %w", err)
	}
	resource.KeyValue = encryptedKey
	return nil
}

func (p *Provider) SyncEndpointToStore(endpoint *models.ResourcePoolEndpoint) error {
	if endpoint == nil || endpoint.ID == 0 || endpoint.ResourcePoolID == 0 {
		return errors.New("endpoint and pool IDs are required")
	}
	return p.store.HSet(endpointKey(endpoint.ID), map[string]any{
		"resource_pool_id": endpoint.ResourcePoolID,
		"name":             endpoint.Name,
		"channel_type":     endpoint.ChannelType,
		"base_url":         endpoint.BaseURL,
		"enabled":          models.CredentialEnabled(endpoint.Enabled),
	})
}

func (p *Provider) RemoveEndpointFromStore(endpointID uint) error {
	if endpointID == 0 {
		return errors.New("endpoint ID is required")
	}
	return p.store.Delete(endpointKey(endpointID))
}

// ResolveEndpoint validates that the selected endpoint belongs to the pool,
// matches the group's protocol, and is currently enabled.
func (p *Provider) ResolveEndpoint(poolID, endpointID uint, channelType string) (*models.ResourcePoolEndpoint, error) {
	if poolID == 0 || endpointID == 0 {
		return nil, errors.New("resource pool endpoint is required")
	}
	details, err := p.store.HGetAll(endpointKey(endpointID))
	if err != nil || len(details) == 0 {
		var endpoint models.ResourcePoolEndpoint
		if dbErr := p.db.Where("id = ? AND resource_pool_id = ?", endpointID, poolID).First(&endpoint).Error; dbErr != nil {
			return nil, dbErr
		}
		if syncErr := p.SyncEndpointToStore(&endpoint); syncErr != nil {
			return nil, syncErr
		}
		if !models.CredentialEnabled(endpoint.Enabled) {
			return nil, errors.New("resource pool endpoint is disabled")
		}
		if endpoint.ChannelType != channelType {
			return nil, fmt.Errorf("resource pool endpoint uses %s, group uses %s", endpoint.ChannelType, channelType)
		}
		return &endpoint, nil
	}
	cachedPoolID, _ := strconv.ParseUint(details["resource_pool_id"], 10, 64)
	if uint(cachedPoolID) != poolID {
		return nil, errors.New("resource pool endpoint belongs to a different pool")
	}
	if details["enabled"] != "true" && details["enabled"] != "1" {
		return nil, errors.New("resource pool endpoint is disabled")
	}
	if details["channel_type"] != channelType {
		return nil, fmt.Errorf("resource pool endpoint uses %s, group uses %s", details["channel_type"], channelType)
	}
	return &models.ResourcePoolEndpoint{
		ID:             endpointID,
		ResourcePoolID: poolID,
		Name:           details["name"],
		ChannelType:    details["channel_type"],
		BaseURL:        details["base_url"],
		Enabled:        models.Bool(true),
	}, nil
}

func (p *Provider) SyncResourceToStore(resource *models.UpstreamResource) error {
	if err := p.syncResourceToStore(resource); err != nil {
		return err
	}
	return p.syncSchedulerSnapshot(resource.ResourcePoolID)
}

func (p *Provider) SyncResourcesToStore(resources []models.UpstreamResource) error {
	poolIDs := make(map[uint]struct{})
	for i := range resources {
		if err := p.syncResourceToStore(&resources[i]); err != nil {
			return err
		}
		poolIDs[resources[i].ResourcePoolID] = struct{}{}
	}
	for poolID := range poolIDs {
		if err := p.syncSchedulerSnapshot(poolID); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provider) syncResourceToStore(resource *models.UpstreamResource) error {
	if resource == nil || resource.ID == 0 || resource.ResourcePoolID == 0 {
		return errors.New("resource and pool IDs are required")
	}
	if err := p.store.HSet(resourceKey(resource.ID), resourceToMap(resource)); err != nil {
		return fmt.Errorf("cache resource %d: %w", resource.ID, err)
	}
	listKey := activeResourcesKey(resource.ResourcePoolID)
	if err := p.store.LRem(listKey, 0, resource.ID); err != nil {
		return fmt.Errorf("remove stale resource %d from pool %d: %w", resource.ID, resource.ResourcePoolID, err)
	}
	if models.CredentialEnabled(resource.Enabled) && resource.Status == models.ResourceStatusActive {
		if err := p.store.LPush(listKey, resource.ID); err != nil {
			return fmt.Errorf("add resource %d to pool %d: %w", resource.ID, resource.ResourcePoolID, err)
		}
	}
	return nil
}

// RemoveResourceFromStore removes a physical resource from scheduling. Stale
// affinity entries are harmless: selection discards them when the resource
// hash no longer exists and immediately chooses another resource.
func (p *Provider) RemoveResourceFromStore(resource *models.UpstreamResource) error {
	if resource == nil || resource.ID == 0 || resource.ResourcePoolID == 0 {
		return errors.New("resource and pool IDs are required")
	}
	if err := p.store.LRem(activeResourcesKey(resource.ResourcePoolID), 0, resource.ID); err != nil {
		return fmt.Errorf("remove resource %d from pool %d: %w", resource.ID, resource.ResourcePoolID, err)
	}
	if err := p.store.Delete(resourceKey(resource.ID)); err != nil {
		return fmt.Errorf("delete cached resource %d: %w", resource.ID, err)
	}
	return nil
}

func (p *Provider) SelectResource(poolID uint, req SelectionRequest) (*models.UpstreamResource, error) {
	if poolID == 0 {
		return nil, app_errors.ErrNoActiveKeys
	}
	excluded := make(map[uint]struct{}, len(req.ExcludeResourceIDs))
	for _, id := range req.ExcludeResourceIDs {
		excluded[id] = struct{}{}
	}

	if req.Affinity != "" {
		if raw, err := p.store.Get(affinityKey(poolID, req.Affinity)); err == nil {
			if id, parseErr := strconv.ParseUint(string(raw), 10, 64); parseErr == nil {
				if _, skip := excluded[uint(id)]; !skip {
					if resource, loadErr := p.resourceFromStore(uint(id)); loadErr == nil && p.isSelectable(resource, req.Route) {
						return resource, nil
					}
				}
			} else {
				_ = p.store.Delete(affinityKey(poolID, req.Affinity))
			}
		}
	}

	resource, err := p.selectWeightedResource(poolID, req.Route, excluded)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func (p *Provider) SelectBoundResource(poolID, resourceID uint, route string) (*models.UpstreamResource, error) {
	if poolID == 0 || resourceID == 0 {
		return nil, app_errors.ErrNoActiveKeys
	}
	resource, err := p.resourceFromStore(resourceID)
	if err != nil {
		var stored models.UpstreamResource
		if dbErr := p.db.Where("id = ? AND resource_pool_id = ?", resourceID, poolID).First(&stored).Error; dbErr != nil {
			return nil, app_errors.ErrNoActiveKeys
		}
		if syncErr := p.SyncResourceToStore(&stored); syncErr != nil {
			return nil, syncErr
		}
		resource, err = p.resourceFromStore(resourceID)
		if err != nil {
			return nil, err
		}
	}
	if resource.ResourcePoolID != poolID || !p.isSelectable(resource, route) {
		return nil, app_errors.ErrNoActiveKeys
	}
	return resource, nil
}

func (p *Provider) FindObjectBinding(ctx context.Context, groupID uint, objectType, objectID string) (*models.UpstreamObjectBinding, error) {
	var binding models.UpstreamObjectBinding
	result := p.db.WithContext(ctx).
		Where("group_id = ? AND object_type = ? AND object_id = ?", groupID, objectType, strings.TrimSpace(objectID)).
		Limit(1).
		Find(&binding)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &binding, nil
}

func (p *Provider) BindObject(ctx context.Context, binding models.UpstreamObjectBinding) error {
	binding.ObjectType = strings.TrimSpace(binding.ObjectType)
	binding.ObjectID = strings.TrimSpace(binding.ObjectID)
	if binding.GroupID == 0 || binding.ResourcePoolID == 0 || binding.ResourceID == 0 || binding.ObjectID == "" {
		return errors.New("complete upstream object ownership is required")
	}
	if binding.ObjectType != models.UpstreamObjectTypeBatch && binding.ObjectType != models.UpstreamObjectTypeFile {
		return fmt.Errorf("unsupported upstream object type %q", binding.ObjectType)
	}

	existing, err := p.FindObjectBinding(ctx, binding.GroupID, binding.ObjectType, binding.ObjectID)
	if err == nil {
		if existing.ResourcePoolID != binding.ResourcePoolID || existing.ResourceID != binding.ResourceID {
			return fmt.Errorf("upstream %s %s is already owned by resource %d", binding.ObjectType, binding.ObjectID, existing.ResourceID)
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := p.db.WithContext(ctx).Create(&binding).Error; err != nil {
		// A concurrent observer may have inserted the same binding. Reload and
		// verify that it did not silently move to another physical credential.
		existing, reloadErr := p.FindObjectBinding(ctx, binding.GroupID, binding.ObjectType, binding.ObjectID)
		if reloadErr == nil && existing.ResourcePoolID == binding.ResourcePoolID && existing.ResourceID == binding.ResourceID {
			return nil
		}
		return err
	}
	return nil
}

func (p *Provider) RefreshAffinity(poolID uint, affinity string, ttl time.Duration) error {
	if affinity == "" {
		return nil
	}
	key := affinityKey(poolID, affinity)
	raw, err := p.store.Get(key)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = DefaultAffinityTTL
	}
	return p.store.Set(key, raw, ttl)
}

// BindAffinity persists a resource choice only after the corresponding
// upstream attempt has succeeded. Failed fallback attempts must not move a
// conversation away from its last known-good resource.
func (p *Provider) BindAffinity(poolID uint, affinity string, resourceID uint, ttl time.Duration) error {
	if poolID == 0 || resourceID == 0 || affinity == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = DefaultAffinityTTL
	}
	return p.store.Set(
		affinityKey(poolID, affinity),
		[]byte(strconv.FormatUint(uint64(resourceID), 10)),
		ttl,
	)
}

// HandleFailure disables a physical resource after every upstream failure
// except 404. A failed URL+key pair must leave both protocol routes together;
// only an explicitly configured quota auto-restore schedule may cool and
// restore it later.
func (p *Provider) HandleFailure(resource *models.UpstreamResource, _ string, statusCode int, message string, _ http.Header) error {
	if resource == nil {
		return nil
	}
	if statusCode == http.StatusNotFound {
		return nil
	}
	if err := p.recordHealthFailure(resource); err != nil {
		return err
	}
	if statusCode == http.StatusPaymentRequired || keypool.IsQuotaOrBillingFailure(message, nil) {
		return p.handleQuotaExhausted(resource, message)
	}
	return p.MarkInvalid(resource, message)
}

// handleQuotaExhausted applies the pool's auto-restore policy to quota and
// billing failures. Without a schedule (the default) the resource is marked
// invalid and stays out of rotation until it is validated or re-enabled
// manually; with a schedule the resource cools down until the next restore
// point and then re-enters rotation lazily via isSelectable.
func (p *Provider) handleQuotaExhausted(resource *models.UpstreamResource, message string) error {
	config, err := p.GetPoolConfig(resource.ResourcePoolID)
	if err != nil {
		return fmt.Errorf("load pool config for quota failure on resource %d: %w", resource.ID, err)
	}
	if config.AutoRestoreSchedule == "" {
		return p.MarkInvalid(resource, message)
	}
	until, err := keypool.NextAutoRestoreTime(config.AutoRestoreSchedule, time.Now())
	if err != nil {
		logrus.WithFields(logrus.Fields{"poolID": resource.ResourcePoolID, "error": err}).
			Warn("Invalid resource pool auto_restore_schedule, marking quota-exhausted resource invalid")
		return p.MarkInvalid(resource, message)
	}
	return p.SetGlobalCooldown(resource.ID, until, message)
}

func (p *Provider) recordHealthFailure(resource *models.UpstreamResource) error {
	if resource == nil || resource.ID == 0 {
		return nil
	}
	if err := p.db.Model(&models.UpstreamResource{}).Where("id = ?", resource.ID).
		UpdateColumn("failure_count", gorm.Expr("failure_count + ?", 1)).Error; err != nil {
		return fmt.Errorf("increment resource health failure count: %w", err)
	}
	_, err := p.store.HIncrBy(resourceKey(resource.ID), "failure_count", 1)
	return err
}

func (p *Provider) SetGlobalCooldown(resourceID uint, until time.Time, reason string) error {
	updates := map[string]any{"global_cooldown_until": until, "disabled_reason": reason}
	if err := p.db.Model(&models.UpstreamResource{}).Where("id = ?", resourceID).Updates(updates).Error; err != nil {
		return fmt.Errorf("persist global cooldown for resource %d: %w", resourceID, err)
	}
	return p.store.HSet(resourceKey(resourceID), map[string]any{
		"global_cooldown_until": until.Unix(),
		"disabled_reason":       reason,
	})
}

func (p *Provider) MarkInvalid(resource *models.UpstreamResource, reason string) error {
	if resource == nil || resource.ID == 0 {
		return errors.New("resource is required")
	}
	updates := map[string]any{
		"status":                models.ResourceStatusInvalid,
		"global_cooldown_until": nil,
		"disabled_reason":       reason,
	}
	if err := p.db.Model(&models.UpstreamResource{}).Where("id = ?", resource.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("mark resource %d invalid: %w", resource.ID, err)
	}
	cacheUpdates := map[string]any{
		"status":                models.ResourceStatusInvalid,
		"global_cooldown_until": 0,
		"disabled_reason":       reason,
	}
	if err := p.store.HSet(resourceKey(resource.ID), cacheUpdates); err != nil {
		return fmt.Errorf("cache invalid resource %d: %w", resource.ID, err)
	}
	if err := p.store.LRem(activeResourcesKey(resource.ResourcePoolID), 0, resource.ID); err != nil {
		return err
	}
	resource.Status = models.ResourceStatusInvalid
	resource.GlobalCooldownUntil = nil
	resource.DisabledReason = reason
	return p.syncSchedulerSnapshot(resource.ResourcePoolID)
}

// MarkHealthy clears runtime health failures after an explicit validation
// succeeds. Manual enablement remains untouched, so a disabled resource is not
// silently returned to scheduling.
func (p *Provider) MarkHealthy(resource *models.UpstreamResource) error {
	if resource == nil || resource.ID == 0 {
		return errors.New("resource is required")
	}
	updates := map[string]any{
		"status":                models.ResourceStatusActive,
		"failure_count":         0,
		"global_cooldown_until": nil,
		"disabled_reason":       "",
	}
	if err := p.db.Model(&models.UpstreamResource{}).Where("id = ?", resource.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("mark resource %d healthy: %w", resource.ID, err)
	}
	resource.Status = models.ResourceStatusActive
	resource.FailureCount = 0
	resource.GlobalCooldownUntil = nil
	resource.DisabledReason = ""
	return p.SyncResourceToStore(resource)
}

func (p *Provider) isSelectable(resource *models.UpstreamResource, _ string) bool {
	if resource == nil || !models.CredentialEnabled(resource.Enabled) || resource.Status != models.ResourceStatusActive {
		return false
	}
	if resource.GlobalCooldownUntil != nil && resource.GlobalCooldownUntil.After(time.Now()) {
		return false
	}
	return true
}

func (p *Provider) resourceFromStore(resourceID uint) (*models.UpstreamResource, error) {
	details, err := p.store.HGetAll(resourceKey(resourceID))
	if err != nil {
		return nil, err
	}
	poolID, _ := strconv.ParseUint(details["resource_pool_id"], 10, 64)
	failureCount, _ := strconv.ParseInt(details["failure_count"], 10, 64)
	priority, _ := strconv.Atoi(details["priority"])
	weight, _ := strconv.Atoi(details["weight"])
	enabled := details["enabled"] == "true" || details["enabled"] == "1"
	resource := &models.UpstreamResource{
		ID:             resourceID,
		ResourcePoolID: uint(poolID),
		Name:           details["name"],
		UpstreamURL:    details["upstream_url"],
		KeyValue:       details["key_string"],
		KeyHash:        details["key_hash"],
		Status:         details["status"],
		Enabled:        models.Bool(enabled),
		Priority:       priority,
		Weight:         weight,
		FailureCount:   failureCount,
		DisabledReason: details["disabled_reason"],
	}
	if raw := details["global_cooldown_until"]; raw != "" && raw != "0" {
		if unix, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil {
			value := time.Unix(unix, 0)
			resource.GlobalCooldownUntil = &value
		}
	}
	if decrypted, decryptErr := p.encryptionSvc.Decrypt(resource.KeyValue); decryptErr == nil {
		resource.KeyValue = decrypted
	}
	return resource, nil
}

func resourceToMap(resource *models.UpstreamResource) map[string]any {
	cooldownUntil := int64(0)
	if resource.GlobalCooldownUntil != nil {
		cooldownUntil = resource.GlobalCooldownUntil.Unix()
	}
	return map[string]any{
		"resource_pool_id":      resource.ResourcePoolID,
		"name":                  resource.Name,
		"upstream_url":          resource.UpstreamURL,
		"key_string":            resource.KeyValue,
		"key_hash":              resource.KeyHash,
		"identity_hash":         resource.IdentityHash,
		"status":                resource.Status,
		"enabled":               models.CredentialEnabled(resource.Enabled),
		"priority":              resource.Priority,
		"weight":                resource.Weight,
		"failure_count":         resource.FailureCount,
		"global_cooldown_until": cooldownUntil,
		"disabled_reason":       resource.DisabledReason,
	}
}

func activeResourcesKey(poolID uint) string {
	return fmt.Sprintf("resource_pool:%d:active_resources", poolID)
}

func poolKey(poolID uint) string {
	return fmt.Sprintf("resource_pool:%d", poolID)
}

func resourceKey(resourceID uint) string {
	return fmt.Sprintf("resource:%d", resourceID)
}

func endpointKey(endpointID uint) string {
	return fmt.Sprintf("resource_endpoint:%d", endpointID)
}

func affinityKey(poolID uint, affinity string) string {
	return fmt.Sprintf("resource_affinity:%d:%s", poolID, affinity)
}
