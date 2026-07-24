package db

import (
	"testing"

	"api-load/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestV130SeparatesEndpointsAndDeduplicatesSharedKeys(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:v130-migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(
		&models.ResourcePool{},
		&models.ResourcePoolEndpoint{},
		&models.Group{},
		&models.UpstreamResource{},
		&models.UpstreamObjectBinding{},
		&models.RequestLog{},
	); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	pool := models.ResourcePool{Name: "shared", Strategy: "round_robin", AffinityTTLSeconds: 3600}
	if err := database.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	group := models.Group{
		Name: "chat", GroupType: "standard", ResourcePoolID: &pool.ID,
		Upstreams: []byte("[]"), ChannelType: "openai", TestModel: "gpt-test",
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	resources := []models.UpstreamResource{
		{
			ResourcePoolID: pool.ID, Name: "a", UpstreamURL: "https://a.example.test",
			KeyValue: "encrypted-a", KeyHash: "shared-key-hash", IdentityHash: "old-a",
			Enabled: models.Bool(true), Status: models.ResourceStatusActive, Priority: 10, Weight: 1,
			RequestCount: 2,
		},
		{
			ResourcePoolID: pool.ID, Name: "b", UpstreamURL: "https://b.example.test/",
			KeyValue: "encrypted-b", KeyHash: "shared-key-hash", IdentityHash: "old-b",
			Enabled: models.Bool(false), Status: models.ResourceStatusInvalid, Priority: 3, Weight: 7,
			RequestCount: 5, TotalFailureCount: 2, FailureCount: 1,
		},
	}
	if err := database.Create(&resources).Error; err != nil {
		t.Fatalf("create legacy resources: %v", err)
	}
	binding := models.UpstreamObjectBinding{
		GroupID: group.ID, ResourcePoolID: pool.ID, ResourceID: resources[1].ID,
		ObjectType: models.UpstreamObjectTypeFile, ObjectID: "file-1",
	}
	if err := database.Create(&binding).Error; err != nil {
		t.Fatalf("create legacy binding: %v", err)
	}
	orphanPool := models.ResourcePool{Name: "unbound", Strategy: "round_robin", AffinityTTLSeconds: 3600}
	if err := database.Create(&orphanPool).Error; err != nil {
		t.Fatalf("create unbound pool: %v", err)
	}
	orphanResource := models.UpstreamResource{
		ResourcePoolID: orphanPool.ID, UpstreamURL: "https://unassigned.example.test",
		KeyValue: "encrypted-orphan", KeyHash: "orphan-key-hash", IdentityHash: "orphan-old",
		Enabled: models.Bool(true), Status: models.ResourceStatusActive, Priority: 10, Weight: 1,
	}
	if err := database.Create(&orphanResource).Error; err != nil {
		t.Fatalf("create unbound legacy resource: %v", err)
	}

	if err := V1_3_0_SharePoolCredentialsAcrossEndpoints(database); err != nil {
		t.Fatalf("run migration: %v", err)
	}
	if err := V1_3_0_SharePoolCredentialsAcrossEndpoints(database); err != nil {
		t.Fatalf("rerun migration: %v", err)
	}

	var endpoints []models.ResourcePoolEndpoint
	if err := database.Where("resource_pool_id = ?", pool.ID).Order("base_url asc").Find(&endpoints).Error; err != nil {
		t.Fatalf("load endpoints: %v", err)
	}
	if len(endpoints) != 2 || endpoints[0].ChannelType != "openai" {
		t.Fatalf("unexpected migrated endpoints: %#v", endpoints)
	}
	if err := database.First(&group, group.ID).Error; err != nil {
		t.Fatalf("reload group: %v", err)
	}
	if group.ResourceEndpointID == nil || *group.ResourceEndpointID != endpoints[0].ID {
		t.Fatalf("group did not bind preferred endpoint: %#v", group)
	}

	var sharedKeys []models.UpstreamResource
	if err := database.Where("resource_pool_id = ?", pool.ID).Find(&sharedKeys).Error; err != nil {
		t.Fatalf("load shared keys: %v", err)
	}
	if len(sharedKeys) != 1 {
		t.Fatalf("expected one deduplicated key, got %#v", sharedKeys)
	}
	key := sharedKeys[0]
	if key.IdentityHash != key.KeyHash || key.UpstreamURL != "" || models.CredentialEnabled(key.Enabled) ||
		key.Status != models.ResourceStatusInvalid || key.Priority != 3 || key.Weight != 7 ||
		key.RequestCount != 7 || key.TotalFailureCount != 2 {
		t.Fatalf("shared key state was not conservatively merged: %#v", key)
	}
	if err := database.First(&binding, binding.ID).Error; err != nil {
		t.Fatalf("reload binding: %v", err)
	}
	if binding.ResourceID != key.ID || binding.ResourceEndpointID != *group.ResourceEndpointID {
		t.Fatalf("object ownership did not migrate with key and endpoint: %#v", binding)
	}
	var unassigned models.ResourcePoolEndpoint
	if err := database.Where("resource_pool_id = ?", orphanPool.ID).First(&unassigned).Error; err != nil {
		t.Fatalf("load unassigned endpoint: %v", err)
	}
	if unassigned.ChannelType != "legacy" || unassigned.BaseURL != "https://unassigned.example.test" {
		t.Fatalf("unbound pool URL was not preserved for manual classification: %#v", unassigned)
	}
}
