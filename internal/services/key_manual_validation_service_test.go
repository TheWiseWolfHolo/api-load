package services

import (
	"gpt-load/internal/models"
	"testing"
)

func TestKEY009ManualValidationExcludesDisabledByDefault(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-active", "", models.KeyStatusActive, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-invalid", "", models.KeyStatusInvalid, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-disabled", "", models.KeyStatusDisabled, 0, 0)

	validationSvc := &KeyManualValidationService{DB: db}
	keys, err := validationSvc.queryKeysForValidation(&group, "")
	if err != nil {
		t.Fatalf("query keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected active and invalid keys only, got %#v", keys)
	}
	for _, key := range keys {
		if key.Status == models.KeyStatusDisabled {
			t.Fatalf("disabled key included in manual validation: %#v", key)
		}
	}
}

func TestKEY009ManualValidationRejectsDisabledStatusFilter(t *testing.T) {
	validationSvc := &KeyManualValidationService{}
	_, err := validationSvc.queryKeysForValidation(&models.Group{ID: 1, Name: "test"}, models.KeyStatusDisabled)
	if err == nil {
		t.Fatal("expected disabled status filter to be rejected")
	}
}
