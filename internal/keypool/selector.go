package keypool

import (
	app_errors "api-load/internal/errors"
	"api-load/internal/models"
	"api-load/internal/store"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	KeySelectionStrategyRoundRobin = "round_robin"
	KeySelectionStrategyRandom     = "random"
	KeySelectionStrategySticky     = "sticky"
	KeySelectionStrategyFillFirst  = "fill_first"

	KeyAffinityScopeGroup         = "group"
	KeyAffinityScopeModel         = "model"
	KeyAffinityScopeModelProxyKey = "model+proxy_key"
)

// SelectionRNG is the small random source needed by strategy selectors.
type SelectionRNG interface {
	Intn(n int) int
}

// SelectionRequest carries request attributes that strategy selectors may use.
type SelectionRequest struct {
	Model         string
	ProxyKey      string
	ExcludeKeyIDs []uint
}

// SelectionResult reports scheduler-relevant outcome data for the selected key.
type SelectionResult struct {
	StatusCode   int
	ErrorMessage string
	Tokens       int64
}

// SetSelectionRNG injects a deterministic random source for strategy tests.
func (p *KeyProvider) SetSelectionRNG(rng SelectionRNG) {
	if rng == nil {
		return
	}
	p.selectionRNG = rng
}

// SelectKeyForRequest selects a key using the group's configured scheduler strategy.
func (p *KeyProvider) SelectKeyForRequest(group *models.Group, req SelectionRequest) (*models.APIKey, error) {
	if group == nil {
		return nil, app_errors.ErrNoActiveKeys
	}
	excluded := excludedKeySet(req.ExcludeKeyIDs)
	switch groupSchedulerStrategy(group) {
	case KeySelectionStrategyRandom:
		return p.selectRandomKey(group.ID, excluded)
	case KeySelectionStrategySticky:
		return p.selectStickyKey(group, req, excluded)
	case KeySelectionStrategyFillFirst:
		return p.selectFillFirstKey(group, excluded)
	default:
		return p.selectKeyExcluding(group.ID, excluded)
	}
}

func groupSchedulerStrategy(group *models.Group) string {
	if group == nil || group.Config == nil {
		return KeySelectionStrategyRoundRobin
	}
	if raw, ok := group.Config["key_selection_strategy"].(string); ok && raw != "" {
		return raw
	}
	return KeySelectionStrategyRoundRobin
}

func groupAffinityScope(group *models.Group) string {
	if group == nil || group.Config == nil {
		return KeyAffinityScopeGroup
	}
	if raw, ok := group.Config["key_affinity_scope"].(string); ok && raw != "" {
		return raw
	}
	return KeyAffinityScopeGroup
}

func (p *KeyProvider) selectRandomKey(groupID uint, excluded map[uint]struct{}) (*models.APIKey, error) {
	var keyIDs []uint
	if err := p.db.Model(&models.APIKey{}).
		Where("group_id = ? AND status = ?", groupID, models.KeyStatusActive).
		Order("id asc").
		Pluck("id", &keyIDs).Error; err != nil {
		return nil, err
	}
	candidates := make([]uint, 0, len(keyIDs))
	for _, keyID := range keyIDs {
		if !isExcludedKey(keyID, excluded) {
			candidates = append(candidates, keyID)
		}
	}
	if len(candidates) == 0 {
		return nil, app_errors.ErrNoActiveKeys
	}
	index := 0
	if p.selectionRNG != nil {
		index = p.selectionRNG.Intn(len(candidates))
	}
	return p.keyFromStore(groupID, candidates[index])
}

func (p *KeyProvider) selectStickyKey(group *models.Group, req SelectionRequest, excluded map[uint]struct{}) (*models.APIKey, error) {
	affinityKey := stickyAffinityKey(group.ID, groupAffinityScope(group), req)
	if rawID, err := p.store.Get(affinityKey); err == nil {
		if keyID, parseErr := strconv.ParseUint(string(rawID), 10, 64); parseErr == nil {
			if isExcludedKey(uint(keyID), excluded) {
				return p.selectKeyExcluding(group.ID, excluded)
			}
			apiKey, keyErr := p.keyFromStore(group.ID, uint(keyID))
			if keyErr == nil {
				return apiKey, nil
			}
		}
		_ = p.store.Delete(affinityKey)
	}

	apiKey, err := p.selectKeyExcluding(group.ID, excluded)
	if err != nil {
		return nil, err
	}
	if len(excluded) == 0 {
		if err := p.store.Set(affinityKey, []byte(strconv.FormatUint(uint64(apiKey.ID), 10)), 0); err != nil {
			return nil, err
		}
	}
	return apiKey, nil
}

func (p *KeyProvider) selectKeyExcluding(groupID uint, excluded map[uint]struct{}) (*models.APIKey, error) {
	if len(excluded) == 0 {
		return p.SelectKey(groupID)
	}

	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)
	listLen, err := p.store.LLen(activeKeysListKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read active key list length: %w", err)
	}
	if listLen == 0 {
		return nil, app_errors.ErrNoActiveKeys
	}

	for range listLen {
		keyIDStr, err := p.store.Rotate(activeKeysListKey)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, app_errors.ErrNoActiveKeys
			}
			return nil, fmt.Errorf("failed to rotate key from store: %w", err)
		}

		keyID, err := strconv.ParseUint(keyIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse key ID '%s': %w", keyIDStr, err)
		}
		if isExcludedKey(uint(keyID), excluded) {
			continue
		}

		apiKey, err := p.keyFromStore(groupID, uint(keyID))
		if err == nil {
			return apiKey, nil
		}
	}

	return nil, app_errors.ErrNoActiveKeys
}

func excludedKeySet(keyIDs []uint) map[uint]struct{} {
	if len(keyIDs) == 0 {
		return nil
	}
	excluded := make(map[uint]struct{}, len(keyIDs))
	for _, keyID := range keyIDs {
		excluded[keyID] = struct{}{}
	}
	return excluded
}

func isExcludedKey(keyID uint, excluded map[uint]struct{}) bool {
	if len(excluded) == 0 {
		return false
	}
	_, ok := excluded[keyID]
	return ok
}

func (p *KeyProvider) selectFillFirstKey(group *models.Group, excluded map[uint]struct{}) (*models.APIKey, error) {
	currentKey := fillFirstCurrentKey(group.ID)
	if rawID, err := p.store.Get(currentKey); err == nil {
		if keyID, parseErr := strconv.ParseUint(string(rawID), 10, 64); parseErr == nil {
			if isExcludedKey(uint(keyID), excluded) {
				return p.selectNextNonCooldownKey(group.ID, excluded)
			}
			if onCooldown, _ := p.store.Exists(fillFirstCooldownKey(uint(keyID))); !onCooldown {
				apiKey, keyErr := p.keyFromStore(group.ID, uint(keyID))
				if keyErr == nil {
					return apiKey, nil
				}
			}
		}
		_ = p.clearFillFirstCurrent(group.ID)
	}

	apiKey, err := p.selectNextNonCooldownKey(group.ID, excluded)
	if err != nil {
		return nil, err
	}
	if len(excluded) > 0 {
		return apiKey, nil
	}
	if err := p.store.Set(currentKey, []byte(strconv.FormatUint(uint64(apiKey.ID), 10)), 0); err != nil {
		return nil, err
	}
	if err := p.store.Set(fillFirstRequestCountKey(group.ID), []byte("0"), 0); err != nil {
		return nil, err
	}
	return apiKey, nil
}

func (p *KeyProvider) selectNextNonCooldownKey(groupID uint, excluded map[uint]struct{}) (*models.APIKey, error) {
	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)
	listLen, err := p.store.LLen(activeKeysListKey)
	if err != nil {
		return nil, err
	}
	if listLen == 0 {
		return nil, app_errors.ErrNoActiveKeys
	}
	for range listLen {
		apiKey, err := p.selectKeyExcluding(groupID, excluded)
		if err != nil {
			return nil, err
		}
		onCooldown, err := p.store.Exists(fillFirstCooldownKey(apiKey.ID))
		if err != nil {
			return nil, err
		}
		if !onCooldown {
			return apiKey, nil
		}
	}
	return nil, app_errors.ErrNoActiveKeys
}

// RecordSelectionResult updates strategy state after a request attempt.
func (p *KeyProvider) RecordSelectionResult(group *models.Group, apiKey *models.APIKey, result SelectionResult) error {
	if group == nil || apiKey == nil || groupSchedulerStrategy(group) != KeySelectionStrategyFillFirst {
		return nil
	}

	if result.StatusCode == 429 && !IsQuotaOrBillingFailure(result.ErrorMessage, nil) {
		cooldownMinutes := groupConfigInt(group, "fill_cooldown_minutes", 0)
		if cooldownMinutes > 0 {
			if err := p.store.Set(fillFirstCooldownKey(apiKey.ID), []byte("1"), time.Duration(cooldownMinutes)*time.Minute); err != nil {
				return err
			}
		}
		return p.clearFillFirstCurrent(group.ID)
	}

	if result.StatusCode >= 200 && result.StatusCode < 400 {
		countKey := fillFirstRequestCountKey(group.ID)
		current := 0
		if raw, err := p.store.Get(countKey); err == nil {
			current, _ = strconv.Atoi(string(raw))
		}
		current++
		if err := p.store.Set(countKey, []byte(strconv.Itoa(current)), 0); err != nil {
			return err
		}
		maxRequests := groupConfigInt(group, "fill_max_consecutive_requests", 0)
		if maxRequests > 0 && current >= maxRequests {
			return p.clearFillFirstCurrent(group.ID)
		}
	}

	return nil
}

func (p *KeyProvider) clearFillFirstCurrent(groupID uint) error {
	if err := p.store.Delete(fillFirstCurrentKey(groupID)); err != nil {
		return err
	}
	return p.store.Delete(fillFirstRequestCountKey(groupID))
}

func fillFirstCurrentKey(groupID uint) string {
	return fmt.Sprintf("fill_first:%d:current", groupID)
}

func fillFirstRequestCountKey(groupID uint) string {
	return fmt.Sprintf("fill_first:%d:requests", groupID)
}

func fillFirstCooldownKey(keyID uint) string {
	return fmt.Sprintf("fill_first:key:%d:cooldown", keyID)
}

func stickyAffinityKey(groupID uint, scope string, req SelectionRequest) string {
	switch scope {
	case KeyAffinityScopeModel:
		return fmt.Sprintf("sticky:%d:model:%s", groupID, normalizeAffinityPart(req.Model))
	case KeyAffinityScopeModelProxyKey:
		return fmt.Sprintf("sticky:%d:model_proxy:%s:%s", groupID, normalizeAffinityPart(req.Model), hashAffinitySecret(req.ProxyKey))
	default:
		return fmt.Sprintf("sticky:%d:group", groupID)
	}
}

func normalizeAffinityPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	return value
}

func hashAffinitySecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func groupConfigInt(group *models.Group, key string, fallback int) int {
	if group == nil || group.Config == nil {
		return fallback
	}
	switch raw := group.Config[key].(type) {
	case int:
		return raw
	case int64:
		return int(raw)
	case float64:
		return int(raw)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			return parsed
		}
	}
	return fallback
}

// IsQuotaOrBillingFailure reports whether an upstream error is an explicit quota or billing exhaustion signal.
func IsQuotaOrBillingFailure(message string, patterns []string) bool {
	normalized := strings.ToLower(message)
	defaultPatterns := []string{
		"insufficient_quota",
		"quota_exceeded",
		"billing_hard_limit",
		"quota exhausted",
		"exceeded your current quota",
		"credit balance is too low",
		"insufficient credits",
		"usage limit exceeded",
		"spending limit",
		"payment required",
	}
	for _, pattern := range append(defaultPatterns, patterns...) {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.Contains(normalized, pattern) {
			return true
		}
	}
	return false
}

func (p *KeyProvider) keyFromStore(groupID, keyID uint) (*models.APIKey, error) {
	keyHashKey := fmt.Sprintf("key:%d", keyID)
	keyDetails, err := p.store.HGetAll(keyHashKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get key details for key ID %d: %w", keyID, err)
	}
	if keyDetails["status"] != models.KeyStatusActive {
		return nil, app_errors.ErrNoActiveKeys
	}

	failureCount, _ := strconv.ParseInt(keyDetails["failure_count"], 10, 64)
	createdAt, _ := strconv.ParseInt(keyDetails["created_at"], 10, 64)
	encryptedKeyValue := keyDetails["key_string"]
	decryptedKeyValue, err := p.encryptionSvc.Decrypt(encryptedKeyValue)
	if err != nil {
		decryptedKeyValue = encryptedKeyValue
	}

	return &models.APIKey{
		ID:           keyID,
		KeyValue:     decryptedKeyValue,
		Status:       keyDetails["status"],
		FailureCount: failureCount,
		GroupID:      groupID,
		CreatedAt:    time.Unix(createdAt, 0),
	}, nil
}
