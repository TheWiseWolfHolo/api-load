package db

import (
	"testing"

	"api-load/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestV120SplitCredentialEnablementMigratesLegacyStateAndDefaults(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:migration-v120?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&models.APIKey{}, &models.UpstreamResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	key := models.APIKey{GroupID: 1, KeyHash: "legacy-key", KeyValue: "encrypted", Status: models.KeyStatusDisabled, Enabled: models.Bool(true)}
	resource := models.UpstreamResource{ResourcePoolID: 1, Name: "legacy-resource", UpstreamURL: "https://api.example.invalid", KeyValue: "encrypted", KeyHash: "legacy-resource", IdentityHash: "legacy-identity", Status: models.ResourceStatusDisabled, Enabled: models.Bool(true)}
	if err := database.Create(&key).Error; err != nil {
		t.Fatalf("create legacy key: %v", err)
	}
	if err := database.Create(&resource).Error; err != nil {
		t.Fatalf("create legacy resource: %v", err)
	}
	if err := database.Model(&models.APIKey{}).Where("id = ?", key.ID).Updates(map[string]any{"priority": 0, "weight": 0}).Error; err != nil {
		t.Fatalf("zero legacy key scheduling fields: %v", err)
	}
	if err := database.Model(&models.UpstreamResource{}).Where("id = ?", resource.ID).Updates(map[string]any{"priority": 0, "weight": 0}).Error; err != nil {
		t.Fatalf("zero legacy resource scheduling fields: %v", err)
	}

	if err := V1_2_0_SplitCredentialEnablement(database); err != nil {
		t.Fatalf("run migration: %v", err)
	}
	if err := database.First(&key, key.ID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if err := database.First(&resource, resource.ID).Error; err != nil {
		t.Fatalf("reload resource: %v", err)
	}

	if models.CredentialEnabled(key.Enabled) || key.Status != models.KeyStatusActive || key.Priority != models.DefaultCredentialPriority || key.Weight != models.DefaultCredentialWeight {
		t.Fatalf("unexpected migrated key: %#v", key)
	}
	if models.CredentialEnabled(resource.Enabled) || resource.Status != models.ResourceStatusActive || resource.Priority != models.DefaultCredentialPriority || resource.Weight != models.DefaultCredentialWeight {
		t.Fatalf("unexpected migrated resource: %#v", resource)
	}
}
