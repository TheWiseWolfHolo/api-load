package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"gpt-load/internal/platform/encryption"
	"gpt-load/internal/storage"
	"gpt-load/internal/storage/models"
)

func TestSharedPoolImportPreservesDisabledAndDormantKeys(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "target.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := storage.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	crypt, err := encryption.NewService("new-32-character-migration-test-material")
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	disabled := false
	poolID, endpointID := uint(7), uint(8)
	old := legacyGroup{ID: 1, Name: "test", Kind: "standard", Channel: "openai", PoolID: &poolID, EndpointID: &endpointID, Models: []string{"test-model"}}
	source := snapshot{Groups: []legacyGroup{old}, Endpoints: []legacyEndpoint{{ID: 8, PoolID: 7, URL: "https://example.com", Enabled: &enabled}},
		Resources: []legacyCredential{{ID: 11, PoolID: 7, Value: "shared-active", Status: "active", Enabled: &enabled}, {ID: 12, PoolID: 7, Value: "shared-invalid", Status: "invalid", Enabled: &enabled}, {ID: 13, PoolID: 7, Value: "operator-disabled", Status: "active", Enabled: &disabled}},
		Keys:      []legacyCredential{{ID: 21, GroupID: 1, Value: "shared-active", Status: "active"}, {ID: 22, GroupID: 1, Value: "legacy-dormant", Status: "active"}}}
	report, err := importSnapshot(db, source, "", crypt, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Groups) != 1 || report.Groups[0].Credentials != 4 || report.Groups[0].Disabled != 3 || report.Groups[0].Deduplicated != 1 {
		t.Fatalf("unexpected report %#v", report.Groups)
	}
	var rows []models.Credential
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		raw, err := crypt.Decrypt(row.Data)
		if err != nil {
			t.Fatal(err)
		}
		var credential map[string]string
		if err := json.Unmarshal([]byte(raw), &credential); err != nil {
			t.Fatal(err)
		}
		want := models.CredentialStatusDisabled
		if credential["api_key"] == "shared-active" {
			want = models.CredentialStatusActive
		}
		if row.Status != want {
			t.Errorf("wrong enablement for credential %d", row.ID)
		}
	}
	if _, err := importSnapshot(db, source, "", crypt, time.Now()); err == nil {
		t.Fatal("second import must refuse populated target")
	}
}

func TestDecryptionFailureRollsBackAllImportedRows(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "target.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := storage.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	crypt, err := encryption.NewService("new-32-character-migration-test-material")
	if err != nil {
		t.Fatal(err)
	}
	source := snapshot{Groups: []legacyGroup{{ID: 1, Name: "test", Kind: "standard", Channel: "openai", Upstreams: []struct {
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}{{URL: "https://example.com", Weight: 1}}}}, Keys: []legacyCredential{{ID: 1, GroupID: 1, Value: "malformed-ciphertext", Status: "active"}}}
	if _, err := importSnapshot(db, source, "wrong-old-key", crypt, time.Now()); err == nil {
		t.Fatal("decryption must fail")
	}
	for _, model := range []any{&models.Group{}, &models.Credential{}, &models.AccessKey{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("partial import survived rollback")
		}
	}
}

func TestLegacyKeyUsesOriginalPBKDF2AndAESGCM(t *testing.T) {
	key := "legacy-32-character-encryption-material"
	derived := pbkdf2.Key([]byte(key), []byte("gpt-load-encryption-v1"), 100000, 32, sha256.New)
	block, err := aes.NewCipher(derived)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	encrypted := hex.EncodeToString(gcm.Seal(nonce, nonce, []byte("credential-test-value"), nil))
	got, err := legacyDecrypt(key, encrypted)
	if err != nil || got != "credential-test-value" {
		t.Fatal("legacy encryption was not preserved")
	}
	if _, err := legacyDecrypt("different-key", encrypted); err == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestCompatibleBaseURLKeepsLegacyVersionedPaths(t *testing.T) {
	for _, test := range []struct{ channel, source, target string }{
		{"openai_compatible", "https://example.com/api", "https://example.com/api/v1"},
		{"openai_compatible", "https://example.com/v1/", "https://example.com/v1"},
		{"mistral", "https://api.mistral.ai", "https://api.mistral.ai/v1"},
		{"cohere", "https://api.cohere.ai", "https://api.cohere.ai/v2"},
		{"anthropic", "https://example.com/coding", "https://example.com/coding"},
	} {
		if got := migratedBaseURL(test.channel, test.source); got != test.target {
			t.Errorf("wrong URL conversion for %s", test.channel)
		}
	}
}
