package services

import (
	"api-load/internal/encryption"
	"api-load/internal/keypool"
	"api-load/internal/models"
	"api-load/internal/store"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var serviceTestGroupCounter uint64

func newTestKeyService(t *testing.T) (*KeyService, *gorm.DB, store.Store, encryption.Service) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:svc-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
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
	provider := keypool.NewProvider(db, memStore, nil, encryptionSvc)
	return NewKeyService(db, provider, nil, encryptionSvc), db, memStore, encryptionSvc
}

func createServiceTestGroup(t *testing.T, db *gorm.DB) models.Group {
	t.Helper()

	group := models.Group{
		Name:               fmt.Sprintf("svc-group-%d", atomic.AddUint64(&serviceTestGroupCounter, 1)),
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

func seedKey(t *testing.T, svc *KeyService, groupID uint, rawKey, notes, status string, failureCount, requestCount int64) models.APIKey {
	t.Helper()
	enabled := true
	if status == models.KeyStatusDisabled {
		status = models.KeyStatusActive
		enabled = false
	}

	key := models.APIKey{
		GroupID:      groupID,
		KeyValue:     rawKey,
		KeyHash:      svc.EncryptionSvc.Hash(rawKey),
		Notes:        notes,
		Status:       status,
		Enabled:      models.Bool(enabled),
		Priority:     models.DefaultCredentialPriority,
		Weight:       models.DefaultCredentialWeight,
		FailureCount: failureCount,
		RequestCount: requestCount,
	}
	if err := svc.KeyProvider.AddKeys(groupID, []models.APIKey{key}); err != nil {
		t.Fatalf("seed key: %v", err)
	}
	var stored models.APIKey
	if err := svc.DB.Where("key_hash = ?", key.KeyHash).First(&stored).Error; err != nil {
		t.Fatalf("reload seed key: %v", err)
	}
	return stored
}

func TestIMP005DuplicatePolicyKeepPreservesExistingRecord(t *testing.T) {
	svc, db, memStore, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-existing", "old notes", models.KeyStatusInvalid, 7, 11)

	result, err := svc.ImportKeyRecords(group.ID, []KeyImportRecord{
		{Key: "sk-test-existing", Notes: "new notes", Status: models.KeyStatusActive},
	}, KeyImportOptions{DuplicatePolicy: DuplicatePolicyKeep})
	if err != nil {
		t.Fatalf("import records: %v", err)
	}
	if result.DuplicateCount != 1 || result.AddedCount != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.IgnoredCount != 1 {
		t.Fatalf("expected kept duplicate to be counted as ignored, got %#v", result)
	}

	var stored models.APIKey
	if err := db.Where("key_hash = ?", svc.EncryptionSvc.Hash("sk-test-existing")).First(&stored).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if stored.Notes != "old notes" || stored.Status != models.KeyStatusInvalid || stored.FailureCount != 7 {
		t.Fatalf("keep changed existing record: %#v", stored)
	}
	length, err := memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list: %v", err)
	}
	if length != 0 {
		t.Fatalf("keep should not churn active list, got length %d", length)
	}
}

func TestIMP006DuplicatePolicyUpdateNotesUpdatesOnlyNotes(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-existing", "old notes", models.KeyStatusInvalid, 3, 0)

	result, err := svc.ImportKeyRecords(group.ID, []KeyImportRecord{
		{Key: "sk-test-existing", Notes: "new notes", Status: models.KeyStatusActive},
	}, KeyImportOptions{DuplicatePolicy: DuplicatePolicyUpdateNotes})
	if err != nil {
		t.Fatalf("import records: %v", err)
	}
	if result.UpdatedCount != 1 {
		t.Fatalf("expected one updated record, got %#v", result)
	}

	var stored models.APIKey
	if err := db.Where("key_hash = ?", svc.EncryptionSvc.Hash("sk-test-existing")).First(&stored).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if stored.Notes != "new notes" || stored.Status != models.KeyStatusInvalid || stored.FailureCount != 3 {
		t.Fatalf("update_notes changed more than notes: %#v", stored)
	}
}

func TestIMP007DuplicatePolicyUpdateStatusMaintainsActiveList(t *testing.T) {
	svc, db, memStore, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-existing", "notes", models.KeyStatusActive, 2, 0)

	if _, err := svc.ImportKeyRecords(group.ID, []KeyImportRecord{
		{Key: "sk-test-existing", Notes: "ignored", Status: models.KeyStatusDisabled},
	}, KeyImportOptions{DuplicatePolicy: DuplicatePolicyUpdateStatus}); err != nil {
		t.Fatalf("disable through import: %v", err)
	}
	length, err := memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list after disable: %v", err)
	}
	if length != 0 {
		t.Fatalf("expected disabled duplicate removed from active list, got %d", length)
	}

	if _, err := svc.ImportKeyRecords(group.ID, []KeyImportRecord{
		{Key: "sk-test-existing", Status: models.KeyStatusActive},
	}, KeyImportOptions{DuplicatePolicy: DuplicatePolicyUpdateStatus}); err != nil {
		t.Fatalf("enable through import: %v", err)
	}
	length, err = memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list after enable: %v", err)
	}
	if length != 1 {
		t.Fatalf("expected active duplicate added to active list, got %d", length)
	}

	var stored models.APIKey
	if err := db.Where("key_hash = ?", svc.EncryptionSvc.Hash("sk-test-existing")).First(&stored).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if stored.Notes != "notes" || stored.Status != models.KeyStatusActive {
		t.Fatalf("update_status changed unexpected fields: %#v", stored)
	}
}

func TestIMP008DuplicatePolicyOverwriteUpdatesEditableFields(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	created := seedKey(t, svc, group.ID, "sk-test-existing", "old notes", models.KeyStatusInvalid, 5, 42)

	result, err := svc.ImportKeyRecords(group.ID, []KeyImportRecord{
		{Key: "sk-test-existing", Notes: "new notes", Status: models.KeyStatusDisabled},
	}, KeyImportOptions{DuplicatePolicy: DuplicatePolicyOverwrite})
	if err != nil {
		t.Fatalf("import records: %v", err)
	}
	if result.UpdatedCount != 1 {
		t.Fatalf("expected one overwritten record, got %#v", result)
	}

	var stored models.APIKey
	if err := db.Where("key_hash = ?", svc.EncryptionSvc.Hash("sk-test-existing")).First(&stored).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if stored.Notes != "new notes" || stored.Status != models.KeyStatusActive || models.CredentialEnabled(stored.Enabled) {
		t.Fatalf("overwrite did not update editable fields: %#v", stored)
	}
	if stored.RequestCount != 42 || !stored.CreatedAt.Equal(created.CreatedAt) || stored.KeyHash != created.KeyHash {
		t.Fatalf("overwrite changed preserved fields: %#v original=%#v", stored, created)
	}
}

func TestIMP003AsyncImportPreservesJSONLNotesAndStatus(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	taskStore := store.NewMemoryStore()
	importSvc := NewKeyImportService(NewTaskService(taskStore), svc)

	_, err := importSvc.StartImportTask(&group, `{"key":"sk-test-jsonl","notes":"paused","status":"disabled"}`)
	if err != nil {
		t.Fatalf("start import task: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		status, err := importSvc.TaskService.GetTaskStatus()
		if err != nil {
			t.Fatalf("get task status: %v", err)
		}
		if !status.IsRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for import task")
		}
		time.Sleep(10 * time.Millisecond)
	}

	var stored models.APIKey
	if err := db.Where("key_hash = ?", svc.EncryptionSvc.Hash("sk-test-jsonl")).First(&stored).Error; err != nil {
		t.Fatalf("load imported key: %v", err)
	}
	if stored.Notes != "paused" || stored.Status != models.KeyStatusActive || models.CredentialEnabled(stored.Enabled) {
		t.Fatalf("async import lost JSONL fields: %#v", stored)
	}
}
