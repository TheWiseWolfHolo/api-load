package services

import (
	"api-load/internal/models"
	"fmt"
	"testing"
)

func TestKEY006SetKeyStatusDisableAndEnableAreExplicit(t *testing.T) {
	svc, db, memStore, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	key := seedKey(t, svc, group.ID, "sk-test-toggle", "notes", models.KeyStatusActive, 5, 0)

	result, err := svc.SetKeyStatus(key.ID, models.KeyStatusDisabled)
	if err != nil {
		t.Fatalf("disable key: %v", err)
	}
	if result.ChangedCount != 1 || result.IgnoredCount != 0 {
		t.Fatalf("unexpected disable result: %#v", result)
	}

	var stored models.APIKey
	if err := db.First(&stored, key.ID).Error; err != nil {
		t.Fatalf("reload disabled key: %v", err)
	}
	if stored.Status != models.KeyStatusActive || models.CredentialEnabled(stored.Enabled) || stored.FailureCount != 5 {
		t.Fatalf("disable should preserve health state and failure count, got %#v", stored)
	}
	length, err := memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list after disable: %v", err)
	}
	if length != 0 {
		t.Fatalf("expected disabled key removed from active list, got %d", length)
	}

	result, err = svc.SetKeyStatus(key.ID, models.KeyStatusActive)
	if err != nil {
		t.Fatalf("enable key: %v", err)
	}
	if result.ChangedCount != 1 || result.IgnoredCount != 0 {
		t.Fatalf("unexpected enable result: %#v", result)
	}
	if err := db.First(&stored, key.ID).Error; err != nil {
		t.Fatalf("reload enabled key: %v", err)
	}
	if stored.Status != models.KeyStatusActive || !models.CredentialEnabled(stored.Enabled) || stored.FailureCount != 0 {
		t.Fatalf("enable should reset failure count and set active, got %#v", stored)
	}
	length, err = memStore.LLen(fmt.Sprintf("group:%d:active_keys", group.ID))
	if err != nil {
		t.Fatalf("read active list after enable: %v", err)
	}
	if length != 1 {
		t.Fatalf("expected enabled key added to active list, got %d", length)
	}
}

func TestKEY006SetKeyStatusValidatesStatus(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	key := seedKey(t, svc, group.ID, "sk-test-toggle", "", models.KeyStatusActive, 0, 0)

	_, err := svc.SetKeyStatus(key.ID, "paused")
	if err == nil {
		t.Fatal("expected validation error for unknown status")
	}
}

func TestKEY007ListKeysInGroupStatusFiltersIncludeDisabledAndAll(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-active", "", models.KeyStatusActive, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-invalid", "", models.KeyStatusInvalid, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-disabled", "", models.KeyStatusDisabled, 0, 0)

	cases := []struct {
		status string
		want   int
	}{
		{"", 3},
		{"all", 3},
		{models.KeyStatusActive, 1},
		{models.KeyStatusInvalid, 1},
		{models.KeyStatusDisabled, 1},
	}
	for _, tc := range cases {
		var keys []models.APIKey
		if err := svc.ListKeysInGroupQuery(group.ID, tc.status, nil, "", "", "").Find(&keys).Error; err != nil {
			t.Fatalf("list status %q: %v", tc.status, err)
		}
		if len(keys) != tc.want {
			t.Fatalf("status %q: expected %d keys, got %d", tc.status, tc.want, len(keys))
		}
	}

	if svc.IsValidKeyStatusFilter("paused") {
		t.Fatal("unknown status should not validate")
	}
}

func TestKEY011ListKeysSearchesNotesAndExactKeyHash(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-notes", "billing backup", models.KeyStatusActive, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-hash", "primary", models.KeyStatusActive, 0, 0)

	var noteMatches []models.APIKey
	if err := svc.ListKeysInGroupQuery(group.ID, "all", nil, "", "billing", "").Find(&noteMatches).Error; err != nil {
		t.Fatalf("notes search: %v", err)
	}
	if len(noteMatches) != 1 || noteMatches[0].Notes != "billing backup" {
		t.Fatalf("unexpected notes matches: %#v", noteMatches)
	}

	var hashMatches []models.APIKey
	searchHash := svc.EncryptionSvc.Hash("sk-test-hash")
	if err := svc.ListKeysInGroupQuery(group.ID, "all", nil, searchHash, "", "").Find(&hashMatches).Error; err != nil {
		t.Fatalf("exact key hash search: %v", err)
	}
	if len(hashMatches) != 1 || hashMatches[0].KeyHash != searchHash {
		t.Fatalf("unexpected hash matches: %#v", hashMatches)
	}

	var generalMatches []models.APIKey
	if err := svc.ListKeysInGroupQuery(group.ID, "all", nil, "", "", "primary").Find(&generalMatches).Error; err != nil {
		t.Fatalf("general search: %v", err)
	}
	if len(generalMatches) != 1 || generalMatches[0].Notes != "primary" {
		t.Fatalf("unexpected general search matches: %#v", generalMatches)
	}
}
