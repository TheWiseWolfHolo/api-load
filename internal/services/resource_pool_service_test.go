package services

import (
	"api-load/internal/channel"
	"api-load/internal/config"
	"api-load/internal/httpclient"
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRES015ResourceExportRoundTripKeepsConfigAndResetsRuntimeState(t *testing.T) {
	svc, _, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	source, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "export-source"})
	if err != nil {
		t.Fatalf("create source pool: %v", err)
	}
	created, err := svc.AddResources(ctx, source.ID, []ResourceCreateParams{{
		Name: "seat-a", UpstreamURL: "https://api.example.invalid", Key: "sk-export-a",
		Enabled: models.Bool(false), Priority: 3, Weight: 7,
	}})
	if err != nil {
		t.Fatalf("add source resource: %v", err)
	}
	if err := svc.db.Model(&models.UpstreamResource{}).Where("id = ?", created[0].ID).Updates(map[string]any{
		"status": models.ResourceStatusInvalid, "request_count": 12, "total_failure_count": 4, "failure_count": 2,
	}).Error; err != nil {
		t.Fatalf("seed runtime state: %v", err)
	}

	var exported bytes.Buffer
	result, err := svc.ExportResourcesToWriter(ctx, source.ID, "all", nil, "full", "jsonl", &exported)
	if err != nil || result.ExportedCount != 1 {
		t.Fatalf("export resource: %#v %v", result, err)
	}
	if strings.Contains(exported.String(), `"status"`) || strings.Contains(exported.String(), "request_count") || strings.Contains(exported.String(), "failure_count") {
		t.Fatalf("full export leaked runtime state: %s", exported.String())
	}

	records, err := ParseResourceImportInput(exported.String())
	if err != nil {
		t.Fatalf("parse exported resource: %v", err)
	}
	target, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "export-target"})
	if err != nil {
		t.Fatalf("create target pool: %v", err)
	}
	imported, err := svc.AddResources(ctx, target.ID, records)
	if err != nil {
		t.Fatalf("import resource: %v", err)
	}
	if len(imported) != 1 || imported[0].Enabled || imported[0].Priority != 3 || imported[0].Weight != 7 || imported[0].Status != models.ResourceStatusActive || imported[0].RequestCount != 0 || imported[0].FailureCount != 0 {
		t.Fatalf("unexpected round-tripped resource: %#v", imported)
	}

	var keysOnly bytes.Buffer
	if _, err := svc.ExportResourcesToWriter(ctx, source.ID, "all", nil, "keys", "txt", &keysOnly); err != nil {
		t.Fatalf("export keys only: %v", err)
	}
	if strings.TrimSpace(keysOnly.String()) != "sk-export-a" {
		t.Fatalf("unexpected keys-only export: %q", keysOnly.String())
	}
}

func TestRES017KeysOnlyExportSeparatesHealthAndManualDisablement(t *testing.T) {
	svc, _, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "export-statuses"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	created, err := svc.AddResources(ctx, pool.ID, []ResourceCreateParams{
		{Name: "active-enabled", UpstreamURL: "https://a.example.invalid", Key: "key-active-enabled"},
		{Name: "invalid-enabled", UpstreamURL: "https://b.example.invalid", Key: "key-invalid-enabled"},
		{Name: "active-disabled", UpstreamURL: "https://c.example.invalid", Key: "key-active-disabled", Enabled: models.Bool(false)},
		{Name: "invalid-disabled", UpstreamURL: "https://d.example.invalid", Key: "key-invalid-disabled", Enabled: models.Bool(false)},
	})
	if err != nil {
		t.Fatalf("add resources: %v", err)
	}
	if err := svc.db.Model(&models.UpstreamResource{}).
		Where("id IN ?", []uint{created[1].ID, created[3].ID}).
		Update("status", models.ResourceStatusInvalid).Error; err != nil {
		t.Fatalf("mark invalid resources: %v", err)
	}

	tests := []struct {
		status string
		want   []string
	}{
		{"all", []string{"key-active-enabled", "key-invalid-enabled", "key-active-disabled", "key-invalid-disabled"}},
		{models.ResourceStatusActive, []string{"key-active-enabled"}},
		{models.ResourceStatusInvalid, []string{"key-invalid-enabled"}},
		{models.ResourceStatusDisabled, []string{"key-active-disabled", "key-invalid-disabled"}},
	}
	for _, tc := range tests {
		var output bytes.Buffer
		result, err := svc.ExportResourcesToWriter(ctx, pool.ID, tc.status, nil, "keys", "txt", &output)
		if err != nil {
			t.Fatalf("export status %q: %v", tc.status, err)
		}
		lines := strings.Fields(output.String())
		if result.ExportedCount != len(tc.want) || strings.Join(lines, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("status %q exported %v, want %v", tc.status, lines, tc.want)
		}
	}
}

func TestRES022ExportSeparatesCoolingFromActiveKeys(t *testing.T) {
	svc, _, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "export-cooling"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	created, err := svc.AddResources(ctx, pool.ID, []ResourceCreateParams{
		{Name: "ready", UpstreamURL: "https://a.example.invalid", Key: "key-ready"},
		{Name: "cooling", UpstreamURL: "https://b.example.invalid", Key: "key-cooling"},
		{Name: "expired", UpstreamURL: "https://c.example.invalid", Key: "key-expired"},
	})
	if err != nil {
		t.Fatalf("add resources: %v", err)
	}
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	if err := svc.db.Model(&models.UpstreamResource{}).Where("id = ?", created[1].ID).
		Update("global_cooldown_until", future).Error; err != nil {
		t.Fatalf("seed cooling resource: %v", err)
	}
	if err := svc.db.Model(&models.UpstreamResource{}).Where("id = ?", created[2].ID).
		Update("global_cooldown_until", past).Error; err != nil {
		t.Fatalf("seed expired cooldown resource: %v", err)
	}

	tests := []struct {
		status string
		want   []string
	}{
		// 冷却中的资源不算可用 key;冷却已过期的照常按可用导出。
		{models.ResourceStatusActive, []string{"key-ready", "key-expired"}},
		{ResourceExportStatusCooling, []string{"key-cooling"}},
		{"all", []string{"key-ready", "key-cooling", "key-expired"}},
	}
	for _, tc := range tests {
		var output bytes.Buffer
		result, err := svc.ExportResourcesToWriter(ctx, pool.ID, tc.status, nil, "keys", "txt", &output)
		if err != nil {
			t.Fatalf("export status %q: %v", tc.status, err)
		}
		lines := strings.Fields(output.String())
		if result.ExportedCount != len(tc.want) || strings.Join(lines, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("status %q exported %v, want %v", tc.status, lines, tc.want)
		}
	}
}

func TestRES023PoolAutoRestoreScheduleConfigValidation(t *testing.T) {
	svc, _, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	if _, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "bad-schedule", AutoRestoreSchedule: "not-a-schedule"}); err == nil {
		t.Fatal("invalid auto restore schedule must be rejected on create")
	}
	pool, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "with-schedule", AutoRestoreSchedule: "24h"})
	if err != nil || pool.AutoRestoreSchedule != "24h" {
		t.Fatalf("create pool with schedule: %#v %v", pool, err)
	}

	daily := "00:05 +08:00"
	updated, err := svc.UpdatePool(ctx, pool.ID, ResourcePoolUpdateParams{AutoRestoreSchedule: &daily})
	if err != nil || updated.AutoRestoreSchedule != daily {
		t.Fatalf("update pool schedule: %#v %v", updated, err)
	}
	empty := ""
	cleared, err := svc.UpdatePool(ctx, pool.ID, ResourcePoolUpdateParams{AutoRestoreSchedule: &empty})
	if err != nil || cleared.AutoRestoreSchedule != "" {
		t.Fatalf("clear pool schedule: %#v %v", cleared, err)
	}
	bad := "nope"
	if _, err := svc.UpdatePool(ctx, pool.ID, ResourcePoolUpdateParams{AutoRestoreSchedule: &bad}); err == nil {
		t.Fatal("invalid auto restore schedule must be rejected on update")
	}
}

func TestRES018SingleResourceValidationUsesBoundRouteWithoutCountingUsage(t *testing.T) {
	const rawKey = "sk-resource-validation"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer "+rawKey {
			http.Error(w, `{"error":{"message":"unexpected validation request"}}`, http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	validationSvc, poolSvc, provider, pool, resource, group := newTestResourceValidationFixture(t, upstream.URL, rawKey)
	future := time.Now().Add(time.Hour)
	if err := poolSvc.db.Model(&models.UpstreamResource{}).Where("id = ?", resource.ID).Updates(map[string]any{
		"enabled": false, "status": models.ResourceStatusInvalid, "failure_count": 4,
		"global_cooldown_until": future, "disabled_reason": "old quota state",
	}).Error; err != nil {
		t.Fatalf("seed unhealthy resource: %v", err)
	}
	var seeded models.UpstreamResource
	if err := poolSvc.db.First(&seeded, resource.ID).Error; err != nil {
		t.Fatalf("reload seeded resource: %v", err)
	}
	if err := provider.SyncResourceToStore(&seeded); err != nil {
		t.Fatalf("sync seeded resource: %v", err)
	}

	groups, err := validationSvc.ListValidationGroups(context.Background(), pool.ID)
	if err != nil || len(groups) != 1 || groups[0].ID != group.ID || groups[0].ChannelType != "openai" {
		t.Fatalf("unexpected validation groups: %#v %v", groups, err)
	}
	result, err := validationSvc.TestResource(context.Background(), pool.ID, resource.ID, group.ID)
	if err != nil || !result.IsValid || result.ResourceID != resource.ID || result.GroupID != group.ID {
		t.Fatalf("unexpected validation result: %#v %v", result, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal validation result: %v", err)
	}
	if strings.Contains(string(encoded), rawKey) {
		t.Fatalf("validation result leaked raw credential: %s", encoded)
	}

	var stored models.UpstreamResource
	if err := poolSvc.db.First(&stored, resource.ID).Error; err != nil {
		t.Fatalf("reload validated resource: %v", err)
	}
	if stored.Status != models.ResourceStatusActive || stored.FailureCount != 0 || stored.GlobalCooldownUntil != nil || stored.DisabledReason != "" {
		t.Fatalf("successful test did not restore health: %#v", stored)
	}
	if models.CredentialEnabled(stored.Enabled) {
		t.Fatal("successful test silently enabled a manually disabled resource")
	}
	if stored.RequestCount != 0 || stored.TotalFailureCount != 0 {
		t.Fatalf("validation probe changed usage counters: %#v", stored)
	}
}

func TestRES019SingleResourceValidationAppliesFailurePolicyExcept404(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		wantStatus  string
		wantFailure int64
	}{
		{name: "credential rejection", statusCode: http.StatusUnauthorized, wantStatus: models.ResourceStatusInvalid, wantFailure: 1},
		{name: "missing endpoint", statusCode: http.StatusNotFound, wantStatus: models.ResourceStatusActive, wantFailure: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(`{"error":{"message":"validation rejected"}}`))
			}))
			defer upstream.Close()

			validationSvc, poolSvc, _, pool, resource, group := newTestResourceValidationFixture(t, upstream.URL, "sk-rejected")
			result, err := validationSvc.TestResource(context.Background(), pool.ID, resource.ID, group.ID)
			if err != nil || result.IsValid || !strings.Contains(result.Error, fmt.Sprintf("[status %d]", tc.statusCode)) {
				t.Fatalf("unexpected failed validation: %#v %v", result, err)
			}
			var stored models.UpstreamResource
			if err := poolSvc.db.First(&stored, resource.ID).Error; err != nil {
				t.Fatalf("reload failed resource: %v", err)
			}
			if stored.Status != tc.wantStatus || stored.FailureCount != tc.wantFailure {
				t.Fatalf("status %d applied wrong health policy: %#v", tc.statusCode, stored)
			}
			if stored.RequestCount != 0 || stored.TotalFailureCount != 0 {
				t.Fatalf("failed validation changed usage counters: %#v", stored)
			}
		})
	}
}

func newTestResourceValidationFixture(
	t *testing.T,
	upstreamURL, rawKey string,
) (*ResourceValidationService, *ResourcePoolService, *resourcepool.Provider, *ResourcePoolView, ResourceView, *models.Group) {
	t.Helper()
	poolSvc, provider, groupSvc := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := poolSvc.CreatePool(ctx, ResourcePoolCreateParams{Name: "validation-pool-" + fmt.Sprint(time.Now().UnixNano())})
	if err != nil {
		t.Fatalf("create validation pool: %v", err)
	}
	resources, err := poolSvc.AddResources(ctx, pool.ID, []ResourceCreateParams{{
		Name: "validation-seat", UpstreamURL: upstreamURL, Key: rawKey,
	}})
	if err != nil {
		t.Fatalf("create validation resource: %v", err)
	}
	group, err := groupSvc.CreateGroup(ctx, GroupCreateParams{
		Name: "validation-route-" + fmt.Sprint(time.Now().UnixNano()), GroupType: "standard",
		ResourcePoolID: &pool.ID, ChannelType: "openai", TestModel: "gpt-test",
		ValidationEndpoint: "/v1/chat/completions",
	})
	if err != nil {
		t.Fatalf("create validation group: %v", err)
	}
	settings := config.NewSystemSettingsManager()
	validationSvc := NewResourceValidationService(
		poolSvc.db,
		channel.NewFactory(settings, httpclient.NewHTTPClientManager()),
		settings,
		provider,
		poolSvc.encryptionSvc,
	)
	return validationSvc, poolSvc, provider, pool, resources[0], group
}

func TestRES016ManualEnablementDoesNotOverwriteHealth(t *testing.T) {
	svc, _, _ := newTestResourcePoolService(t)
	ctx := context.Background()
	pool, err := svc.CreatePool(ctx, ResourcePoolCreateParams{Name: "independent-states"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	created, err := svc.AddResources(ctx, pool.ID, []ResourceCreateParams{{UpstreamURL: "https://api.example.invalid", Key: "sk-state"}})
	if err != nil {
		t.Fatalf("add resource: %v", err)
	}
	invalid := models.ResourceStatusInvalid
	if _, err := svc.BulkUpdateResources(ctx, pool.ID, []uint{created[0].ID}, ResourceBatchUpdateParams{Status: &invalid}); err != nil {
		t.Fatalf("mark invalid: %v", err)
	}
	if _, err := svc.BulkUpdateResources(ctx, pool.ID, []uint{created[0].ID}, ResourceBatchUpdateParams{Enabled: models.Bool(false)}); err != nil {
		t.Fatalf("disable resource: %v", err)
	}
	if _, err := svc.BulkUpdateResources(ctx, pool.ID, []uint{created[0].ID}, ResourceBatchUpdateParams{Enabled: models.Bool(true)}); err != nil {
		t.Fatalf("enable resource: %v", err)
	}
	page, err := svc.ListResources(ctx, pool.ID, ResourceListParams{Status: models.ResourceStatusInvalid})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != models.ResourceStatusInvalid || !page.Items[0].Enabled {
		t.Fatalf("manual enablement overwrote health: %#v %v", page, err)
	}
}

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
