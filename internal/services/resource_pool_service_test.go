package services

import (
	"api-load/internal/config"
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"context"
	"encoding/json"
	"fmt"
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

func TestRES011ResourcePoolResourcesArePaginatedSearchableAndCounted(t *testing.T) {
	svc, _, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "large-company-pool"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	params := make([]ResourceCreateParams, 25)
	for i := range params {
		params[i] = ResourceCreateParams{
			Name:        fmt.Sprintf("seat-%02d", i+1),
			UpstreamURL: fmt.Sprintf("https://api-%02d.example.invalid", i+1),
			Key:         fmt.Sprintf("sk-paged-%02d", i+1),
		}
	}
	created, err := svc.AddResources(ctx, pool.ID, params)
	if err != nil {
		t.Fatalf("add resources: %v", err)
	}
	if len(created) != 25 {
		t.Fatalf("add response should contain only newly created resources: %d", len(created))
	}

	page, err := svc.ListResources(ctx, pool.ID, ResourceListParams{Page: 2, PageSize: 10})
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if len(page.Items) != 10 || page.Pagination.TotalItems != 25 || page.Pagination.TotalPages != 3 || page.Items[0].Name != "seat-11" {
		t.Fatalf("unexpected resource page: %#v", page)
	}
	search, err := svc.ListResources(ctx, pool.ID, ResourceListParams{Search: "sk-paged-17", PageSize: 20})
	if err != nil {
		t.Fatalf("search exact key: %v", err)
	}
	if len(search.Items) != 1 || search.Items[0].Name != "seat-17" || search.Items[0].MaskedKey != "****d-17" {
		t.Fatalf("exact key search did not remain masked: %#v", search.Items)
	}

	pools, err := svc.ListPools(ctx)
	if err != nil {
		t.Fatalf("list pools: %v", err)
	}
	if len(pools) != 1 || pools[0].ResourceCount != 25 {
		t.Fatalf("pool metadata count is incorrect: %#v", pools)
	}
	encoded, err := json.Marshal(pools[0])
	if err != nil {
		t.Fatalf("marshal pool metadata: %v", err)
	}
	if strings.Contains(string(encoded), "resources") || strings.Contains(string(encoded), "sk-paged") {
		t.Fatalf("pool metadata eagerly embedded resources: %s", encoded)
	}
}

func TestRES012ResourcePoolResourcesSupportEditAndSafeBulkOperations(t *testing.T) {
	svc, provider, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "managed-company-pool"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	created, err := svc.AddResources(ctx, pool.ID, []ResourceCreateParams{
		{Name: "seat-a", UpstreamURL: "https://old-a.example.invalid", Key: "key-a"},
		{Name: "seat-b", UpstreamURL: "https://old-b.example.invalid", Key: "key-b"},
	})
	if err != nil {
		t.Fatalf("add resources: %v", err)
	}
	replacement := "key-a-replaced"
	updated, err := svc.UpdateResource(ctx, pool.ID, created[0].ID, ResourceUpdateParams{
		Name: "seat-a-renamed", UpstreamURL: "https://new-a.example.invalid/", Key: &replacement,
	})
	if err != nil {
		t.Fatalf("update resource: %v", err)
	}
	if updated.Name != "seat-a-renamed" || updated.UpstreamURL != "https://new-a.example.invalid" || updated.MaskedKey != "****aced" {
		t.Fatalf("unexpected edited resource: %#v", updated)
	}
	selected, err := provider.SelectBoundResource(pool.ID, created[0].ID, "anthropic")
	if err != nil || selected.KeyValue != replacement || selected.UpstreamURL != "https://new-a.example.invalid" {
		t.Fatalf("edited resource was not synchronized to scheduler: %#v %v", selected, err)
	}

	statusResult, err := svc.BulkUpdateResourceStatus(ctx, pool.ID, []uint{created[0].ID, created[1].ID, created[1].ID}, models.ResourceStatusDisabled)
	if err != nil {
		t.Fatalf("bulk disable: %v", err)
	}
	if statusResult.RequestedCount != 2 || statusResult.UpdatedCount != 2 {
		t.Fatalf("unexpected bulk status result: %#v", statusResult)
	}
	if selected, err := provider.SelectResource(pool.ID, resourcepool.SelectionRequest{Route: "openai"}); err == nil || selected != nil {
		t.Fatalf("bulk-disabled resources remained selectable: %#v %v", selected, err)
	}
	if _, err := svc.BulkUpdateResourceStatus(ctx, pool.ID, []uint{created[0].ID, created[1].ID}, models.ResourceStatusActive); err != nil {
		t.Fatalf("bulk enable: %v", err)
	}
	if err := provider.BindObject(ctx, models.UpstreamObjectBinding{
		GroupID: 1, ResourcePoolID: pool.ID, ResourceID: created[0].ID,
		ObjectType: models.UpstreamObjectTypeBatch, ObjectID: "batch-protected",
	}); err != nil {
		t.Fatalf("bind protected object: %v", err)
	}
	deleteResult, err := svc.BulkDeleteResources(ctx, pool.ID, []uint{created[0].ID}, []string{"key-b", "missing-key", "key-b"})
	if err != nil {
		t.Fatalf("bulk delete: %v", err)
	}
	if deleteResult.RequestedIDCount != 1 || deleteResult.RequestedKeyCount != 2 || deleteResult.MatchedCount != 2 || deleteResult.DeletedCount != 1 || deleteResult.BlockedCount != 1 || deleteResult.MissingKeyCount != 1 {
		t.Fatalf("unexpected bulk delete result: %#v", deleteResult)
	}
	page, err := svc.ListResources(ctx, pool.ID, ResourceListParams{})
	if err != nil || page.Pagination.TotalItems != 1 || page.Items[0].ID != created[0].ID {
		t.Fatalf("bulk delete removed the protected resource or kept the deletable one: %#v %v", page, err)
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
		Upstreams:          json.RawMessage(`[{"url":"https://legacy.example.invalid","weight":1}]`),
		ChannelType:        "anthropic",
		TestModel:          "claude-test",
		ValidationEndpoint: "/v1/models",
	})
	if err != nil {
		t.Fatalf("create legacy group: %v", err)
	}

	bound, err := groupSvc.UpdateGroup(ctx, group.ID, GroupUpdateParams{
		HasResourcePoolID: true,
		ResourcePoolID:    &pool.ID,
	})
	if err != nil {
		t.Fatalf("bind shared pool: %v", err)
	}
	if bound.ResourcePoolID == nil || *bound.ResourcePoolID != pool.ID || !strings.Contains(string(bound.Upstreams), "legacy.example.invalid") {
		t.Fatalf("binding the pool deleted the dormant legacy upstream: %#v", bound)
	}

	updated, err := groupSvc.UpdateGroup(ctx, group.ID, GroupUpdateParams{
		HasResourcePoolID: true,
	})
	if err != nil {
		t.Fatalf("unbind and restore dormant upstream: %v", err)
	}
	if updated.ResourcePoolID != nil || !strings.Contains(string(updated.Upstreams), "legacy.example.invalid") {
		t.Fatalf("group did not return to legacy routing: %#v", updated)
	}
}

func TestRES013PoolBoundGroupCanStoreDormantLegacyUpstreams(t *testing.T) {
	poolSvc, _, groupSvc := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := poolSvc.CreatePool(ctx, ResourcePoolCreateParams{Name: "shared-with-fallback"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	group, err := groupSvc.CreateGroup(ctx, GroupCreateParams{
		Name: "chat-route-with-fallback", GroupType: "standard", ResourcePoolID: &pool.ID,
		Upstreams:   json.RawMessage(`[{"url":"https://dormant.example.invalid","weight":1}]`),
		ChannelType: "openai", TestModel: "gpt-test", ValidationEndpoint: "/v1/models",
	})
	if err != nil {
		t.Fatalf("create pool-bound group with dormant upstream: %v", err)
	}
	if group.ResourcePoolID == nil || !strings.Contains(string(group.Upstreams), "dormant.example.invalid") {
		t.Fatalf("pool-bound create discarded dormant upstream: %#v", group)
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
