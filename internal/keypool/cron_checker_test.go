package keypool

import (
	"gpt-load/internal/encryption"
	"gpt-load/internal/models"
	"sync/atomic"
	"testing"
)

type countingEncryptionService struct {
	base         encryption.Service
	decryptCount atomic.Int64
}

func (s *countingEncryptionService) Encrypt(plaintext string) (string, error) {
	return s.base.Encrypt(plaintext)
}

func (s *countingEncryptionService) Decrypt(ciphertext string) (string, error) {
	s.decryptCount.Add(1)
	return s.base.Decrypt(ciphertext)
}

func (s *countingEncryptionService) Hash(plaintext string) string {
	return s.base.Hash(plaintext)
}

func TestKEY008CronCheckerDoesNotDecryptDisabledKeys(t *testing.T) {
	_, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	key := models.APIKey{
		GroupID:  group.ID,
		KeyValue: "sk-test-disabled",
		KeyHash:  "hash-disabled",
		Status:   models.KeyStatusDisabled,
	}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create disabled key: %v", err)
	}

	baseEncryption, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("create encryption service: %v", err)
	}
	countingEncryption := &countingEncryptionService{base: baseEncryption}
	checker := &CronChecker{
		DB:            db,
		EncryptionSvc: countingEncryption,
		stopChan:      make(chan struct{}),
	}

	checker.validateGroupKeys(&group)

	if countingEncryption.decryptCount.Load() != 0 {
		t.Fatalf("disabled key was decrypted %d times", countingEncryption.decryptCount.Load())
	}
	var storedGroup models.Group
	if err := db.First(&storedGroup, group.ID).Error; err != nil {
		t.Fatalf("reload group: %v", err)
	}
	if storedGroup.LastValidatedAt == nil {
		t.Fatal("expected last_validated_at to update when no invalid keys exist")
	}
}
