package services

import (
	"api-load/internal/config"
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func newTestResourcePoolService(t *testing.T) (*ResourcePoolService, *resourcepool.Provider, *GroupService) {
	t.Helper()
	_, db, memStore, encryptionSvc := newTestKeyService(t)
	if err := db.AutoMigrate(&models.ResourcePool{}, &models.UpstreamResource{}, &models.UpstreamObjectBinding{}); err != nil {
		t.Fatalf("migrate resource pool models: %v", err)
	}
	provider := resourcepool.NewProvider(db, memStore, encryptionSvc)
	poolSvc := NewResourcePoolService(db, provider, encryptionSvc)
	groupSvc := NewGroupService(db, config.NewSystemSettingsManager(), nil, nil, nil, encryptionSvc, nil)
	return poolSvc, provider, groupSvc
}

func TestRES006ResourcePoolManagementMasksAndPublishesResources(t *testing.T) {
	svc, provider, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "company-official"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if pool.AffinityTTLSeconds != 3600 || pool.BusyWaitMilliseconds != 2000 {
		t.Fatalf("unexpected defaults: %#v", pool)
	}

	resources, err := svc.AddResources(ctx, pool.ID, []ResourceCreateParams{{
		Name:        "seat-a",
		UpstreamURL: "https://api.example.invalid",
		Key:         "sk-company-1234",
	}})
	if err != nil {
		t.Fatalf("add resource: %v", err)
	}
	if len(resources) != 1 || resources[0].MaskedKey != "****1234" {
		t.Fatalf("resource key was not safely masked: %#v", resources)
	}
	encoded, err := json.Marshal(resources)
	if err != nil {
		t.Fatalf("marshal resource response: %v", err)
	}
	if strings.Contains(string(encoded), "sk-company") || strings.Contains(string(encoded), "key_value") {
		t.Fatalf("resource response leaked credential material: %s", encoded)
	}

	selected, err := provider.SelectResource(pool.ID, resourcepool.SelectionRequest{Route: "anthropic"})
	if err != nil || selected.ID != resources[0].ID || selected.KeyValue != "sk-company-1234" {
		t.Fatalf("new resource was not published atomically: %#v %v", selected, err)
	}
	if _, err := svc.UpdateResourceStatus(ctx, pool.ID, resources[0].ID, models.ResourceStatusDisabled); err != nil {
		t.Fatalf("disable resource: %v", err)
	}
	if selected, err := provider.SelectResource(pool.ID, resourcepool.SelectionRequest{Route: "openai"}); err == nil || selected != nil {
		t.Fatalf("disabled resource remained selectable: %#v %v", selected, err)
	}
	if _, err := svc.UpdateResourceStatus(ctx, pool.ID, resources[0].ID, models.ResourceStatusActive); err != nil {
		t.Fatalf("restore resource: %v", err)
	}
	if selected, err := provider.SelectResource(pool.ID, resourcepool.SelectionRequest{Route: "openai"}); err != nil || selected == nil {
		t.Fatalf("restored resource was not selectable: %#v %v", selected, err)
	}
}

func TestRES007GroupCanBindAndUnbindSharedResourcePool(t *testing.T) {
	poolSvc, _, groupSvc := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := poolSvc.CreatePool(ctx, ResourcePoolCreateParams{Name: "shared-routes"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	group, err := groupSvc.CreateGroup(ctx, GroupCreateParams{
		Name:               "coding-route",
		GroupType:          "standard",
		ResourcePoolID:     &pool.ID,
		ChannelType:        "anthropic",
		TestModel:          "claude-test",
		ValidationEndpoint: "/v1/models",
	})
	if err != nil {
		t.Fatalf("create pool-bound group without legacy upstreams: %v", err)
	}
	if group.ResourcePoolID == nil || *group.ResourcePoolID != pool.ID || string(group.Upstreams) != "[]" {
		t.Fatalf("group binding was not persisted: %#v", group)
	}

	_, err = groupSvc.UpdateGroup(ctx, group.ID, GroupUpdateParams{HasResourcePoolID: true})
	if err == nil {
		t.Fatal("unbinding without a replacement legacy upstream should fail")
	}
	updated, err := groupSvc.UpdateGroup(ctx, group.ID, GroupUpdateParams{
		HasResourcePoolID: true,
		HasUpstreams:      true,
		Upstreams:         json.RawMessage(`[{"url":"https://legacy.example.invalid","weight":1}]`),
	})
	if err != nil {
		t.Fatalf("unbind with replacement upstream: %v", err)
	}
	if updated.ResourcePoolID != nil || !strings.Contains(string(updated.Upstreams), "legacy.example.invalid") {
		t.Fatalf("group did not return to legacy routing: %#v", updated)
	}
}

func TestRES008ReferencedResourcePoolCannotBeDeleted(t *testing.T) {
	poolSvc, _, groupSvc := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := poolSvc.CreatePool(ctx, ResourcePoolCreateParams{Name: "protected-pool"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if _, err := groupSvc.CreateGroup(ctx, GroupCreateParams{
		Name: "chat-route", GroupType: "standard", ResourcePoolID: &pool.ID,
		ChannelType: "openai", TestModel: "gpt-test", ValidationEndpoint: "/v1/models",
	}); err != nil {
		t.Fatalf("create referencing group: %v", err)
	}
	if err := poolSvc.DeletePool(ctx, pool.ID); err == nil {
		t.Fatal("expected referenced resource pool deletion to be rejected")
	}
}
