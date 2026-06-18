package keypool

import (
	"api-load/internal/encryption"
	app_errors "api-load/internal/errors"
	"api-load/internal/models"
	"api-load/internal/store"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestProvider(t *testing.T) (*KeyProvider, *gorm.DB, store.Store) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Group{}, &models.APIKey{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	encryptionSvc, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("create encryption service: %v", err)
	}
	memStore := store.NewMemoryStore()
	return NewProvider(db, memStore, nil, encryptionSvc), db, memStore
}

func createTestGroup(t *testing.T, db *gorm.DB) models.Group {
	t.Helper()

	group := models.Group{
		Name:               fmt.Sprintf("test-group-%d", time.Now().UnixNano()),
		GroupType:          "standard",
		Upstreams:          []byte(`[{"url":"https://example.invalid","weight":1}]`),
		ChannelType:        "openai",
		TestModel:          "gpt-test",
		ValidationEndpoint: "/v1/models",
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	return group
}

func TestKEY002AddKeysCachesOnlyActiveKeysInActiveList(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-active", KeyHash: "hash-active", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-invalid", KeyHash: "hash-invalid", Status: models.KeyStatusInvalid},
		{GroupID: group.ID, KeyValue: "sk-test-disabled", KeyHash: "hash-disabled", Status: "disabled"},
	}

	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	length, err := memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list length: %v", err)
	}
	if length != 1 {
		t.Fatalf("expected only one active key in active list, got %d", length)
	}

	selected, err := provider.SelectKey(group.ID)
	if err != nil {
		t.Fatalf("select key: %v", err)
	}
	if selected.KeyValue != "sk-test-active" {
		t.Fatalf("expected active key to be selected, got %q with status %q", selected.KeyValue, selected.Status)
	}
}

func TestKEY003SelectKeyReturnsNoActiveKeysWhenOnlyDisabledOrInvalidExist(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-invalid", KeyHash: "hash-invalid", Status: models.KeyStatusInvalid},
		{GroupID: group.ID, KeyValue: "sk-test-disabled", KeyHash: "hash-disabled", Status: "disabled"},
	}

	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	_, err := provider.SelectKey(group.ID)
	if !errors.Is(err, app_errors.ErrNoActiveKeys) {
		t.Fatalf("expected ErrNoActiveKeys, got %v", err)
	}
}

func TestKEY003SelectKeySkipsStaleDisabledEntryInActiveList(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)

	disabled := models.APIKey{
		GroupID:  group.ID,
		KeyValue: "sk-test-disabled",
		KeyHash:  "hash-disabled",
		Status:   "disabled",
	}
	active := models.APIKey{
		GroupID:  group.ID,
		KeyValue: "sk-test-active",
		KeyHash:  "hash-active",
		Status:   models.KeyStatusActive,
	}
	if err := db.Create(&[]models.APIKey{disabled, active}).Error; err != nil {
		t.Fatalf("create keys: %v", err)
	}

	var keys []models.APIKey
	if err := db.Order("id asc").Find(&keys).Error; err != nil {
		t.Fatalf("reload keys: %v", err)
	}
	for i := range keys {
		if err := memStore.HSet(fmt.Sprintf("key:%d", keys[i].ID), provider.apiKeyToMap(&keys[i])); err != nil {
			t.Fatalf("cache key %d: %v", keys[i].ID, err)
		}
	}

	activeListKey := fmt.Sprintf("group:%d:active_keys", group.ID)
	if err := memStore.LPush(activeListKey, keys[1].ID, keys[0].ID); err != nil {
		t.Fatalf("seed active list: %v", err)
	}

	selected, err := provider.SelectKey(group.ID)
	if err != nil {
		t.Fatalf("select key: %v", err)
	}
	if selected.Status != models.KeyStatusActive || selected.KeyValue != "sk-test-active" {
		t.Fatalf("expected active key, got %q status %q", selected.KeyValue, selected.Status)
	}

	length, err := memStore.LLen(activeListKey)
	if err != nil {
		t.Fatalf("read active list length: %v", err)
	}
	if length != 1 {
		t.Fatalf("expected stale disabled entry removed from active list, got length %d", length)
	}
}

func TestKEY004HandleSuccessDoesNotEnableDisabledKey(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)

	key := models.APIKey{
		GroupID:      group.ID,
		KeyValue:     "sk-test-disabled",
		KeyHash:      "hash-disabled",
		Status:       "disabled",
		FailureCount: 3,
	}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}
	if err := memStore.HSet(fmt.Sprintf("key:%d", key.ID), provider.apiKeyToMap(&key)); err != nil {
		t.Fatalf("cache key: %v", err)
	}

	if err := provider.handleSuccess(key.ID, fmt.Sprintf("key:%d", key.ID), fmt.Sprintf("group:%d:active_keys", group.ID)); err != nil {
		t.Fatalf("handle success: %v", err)
	}

	var stored models.APIKey
	if err := db.First(&stored, key.ID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if stored.Status != "disabled" {
		t.Fatalf("expected disabled status to remain unchanged, got %q", stored.Status)
	}
	if stored.FailureCount != 3 {
		t.Fatalf("expected disabled key failure count to remain 3, got %d", stored.FailureCount)
	}

	length, err := memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list length: %v", err)
	}
	if length != 0 {
		t.Fatalf("expected disabled key not to be re-added to active list, got list length %d", length)
	}
}

func TestKEY005RestoreKeysOnlyRestoresInvalidKeys(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-invalid", KeyHash: "hash-invalid", Status: models.KeyStatusInvalid, FailureCount: 2},
		{GroupID: group.ID, KeyValue: "sk-test-disabled", KeyHash: "hash-disabled", Status: "disabled", FailureCount: 4},
	}
	if err := db.Create(&keys).Error; err != nil {
		t.Fatalf("create keys: %v", err)
	}
	for i := range keys {
		if err := memStore.HSet(fmt.Sprintf("key:%d", keys[i].ID), provider.apiKeyToMap(&keys[i])); err != nil {
			t.Fatalf("cache key %d: %v", keys[i].ID, err)
		}
	}

	restored, err := provider.RestoreKeys(group.ID)
	if err != nil {
		t.Fatalf("restore keys: %v", err)
	}
	if restored != 1 {
		t.Fatalf("expected one invalid key restored, got %d", restored)
	}

	var disabled models.APIKey
	if err := db.Where("key_hash = ?", "hash-disabled").First(&disabled).Error; err != nil {
		t.Fatalf("load disabled key: %v", err)
	}
	if disabled.Status != "disabled" {
		t.Fatalf("expected disabled key to remain disabled, got %q", disabled.Status)
	}

	length, err := memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list length: %v", err)
	}
	if length != 1 {
		t.Fatalf("expected only restored invalid key in active list, got %d", length)
	}
}
