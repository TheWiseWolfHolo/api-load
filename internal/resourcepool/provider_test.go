package resourcepool

import (
	"api-load/internal/encryption"
	"api-load/internal/models"
	"api-load/internal/store"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestProvider(t *testing.T) (*Provider, *gorm.DB, *models.ResourcePool) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:resource-pool-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ResourcePool{}, &models.UpstreamResource{}, &models.UpstreamObjectBinding{}, &models.Group{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool := &models.ResourcePool{Name: "shared", Strategy: "round_robin", AffinityTTLSeconds: 3600, BusyWaitMilliseconds: 2000}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	crypto, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("create encryption service: %v", err)
	}
	provider := NewProvider(db, store.NewMemoryStore(), crypto)
	resources := []models.UpstreamResource{
		{Name: "a", UpstreamURL: "https://a.example.invalid", KeyValue: "key-a"},
		{Name: "b", UpstreamURL: "https://b.example.invalid", KeyValue: "key-b"},
	}
	if _, err := provider.AddResources(pool.ID, resources); err != nil {
		t.Fatalf("create resources: %v", err)
	}
	if err := provider.LoadResourcesFromDB(); err != nil {
		t.Fatalf("load resources: %v", err)
	}
	return provider, db, pool
}

func TestBAT000UpstreamObjectOwnerCannotMoveBetweenResources(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	var resources []models.UpstreamResource
	if err := db.Where("resource_pool_id = ?", pool.ID).Order("id asc").Find(&resources).Error; err != nil {
		t.Fatalf("load resources: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("unexpected resource count: %d", len(resources))
	}
	binding := models.UpstreamObjectBinding{
		GroupID: 1, ResourcePoolID: pool.ID, ResourceID: resources[0].ID,
		ObjectType: models.UpstreamObjectTypeBatch, ObjectID: "batch-owned",
	}
	if err := provider.BindObject(context.Background(), binding); err != nil {
		t.Fatalf("bind batch owner: %v", err)
	}
	binding.ResourceID = resources[1].ID
	if err := provider.BindObject(context.Background(), binding); err == nil {
		t.Fatal("batch ownership silently moved to another physical resource")
	}
	owner, err := provider.FindObjectBinding(context.Background(), 1, models.UpstreamObjectTypeBatch, "batch-owned")
	if err != nil || owner.ResourceID != resources[0].ID {
		t.Fatalf("batch owner changed: %#v %v", owner, err)
	}
}

func TestRES000ResourceIdentityIsURLAndKeyPair(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	created, err := provider.AddResources(pool.ID, []models.UpstreamResource{
		{UpstreamURL: "https://a.example.invalid", KeyValue: "key-c"},
		{UpstreamURL: "https://other.example.invalid", KeyValue: "key-a"},
	})
	if err != nil {
		t.Fatalf("same URL with a new key and same key at a new URL should be accepted: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("unexpected newly created resource count: got %d want 2", len(created))
	}
	created, err = provider.AddResources(pool.ID, []models.UpstreamResource{
		{UpstreamURL: "https://a.example.invalid", KeyValue: "key-a"},
		{UpstreamURL: "https://a.example.invalid", KeyValue: "key-c"},
		{UpstreamURL: "https://a.example.invalid", KeyValue: "key-d"},
		{UpstreamURL: "https://a.example.invalid", KeyValue: "key-d"},
	})
	if err != nil {
		t.Fatalf("idempotent resource append failed: %v", err)
	}
	if len(created) != 1 || created[0].UpstreamURL != "https://a.example.invalid" {
		t.Fatalf("duplicate resources were not skipped: %#v", created)
	}
	var count int64
	if err := db.Model(&models.UpstreamResource{}).Where("resource_pool_id = ?", pool.ID).Count(&count).Error; err != nil {
		t.Fatalf("count resources: %v", err)
	}
	if count != 5 {
		t.Fatalf("unexpected resource count: got %d want 5", count)
	}
}

func TestRES000LargeResourceAppendIsBatchedAndIdempotent(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	const initialCount = 1200
	resources := make([]models.UpstreamResource, 0, initialCount)
	for i := range initialCount {
		resources = append(resources, models.UpstreamResource{
			UpstreamURL: "https://bulk.example.invalid",
			KeyValue:    fmt.Sprintf("bulk-key-%04d", i),
		})
	}
	created, err := provider.AddResources(pool.ID, resources)
	if err != nil {
		t.Fatalf("add large resource batch: %v", err)
	}
	if len(created) != initialCount {
		t.Fatalf("unexpected large batch count: got %d want %d", len(created), initialCount)
	}

	repeatedAndNew := append([]models.UpstreamResource(nil), resources[:600]...)
	for i := initialCount; i < initialCount+25; i++ {
		repeatedAndNew = append(repeatedAndNew, models.UpstreamResource{
			UpstreamURL: "https://bulk.example.invalid",
			KeyValue:    fmt.Sprintf("bulk-key-%04d", i),
		})
	}
	created, err = provider.AddResources(pool.ID, repeatedAndNew)
	if err != nil {
		t.Fatalf("append mixed existing and new resources: %v", err)
	}
	if len(created) != 25 {
		t.Fatalf("unexpected appended resource count: got %d want 25", len(created))
	}
	var count int64
	if err := db.Model(&models.UpstreamResource{}).Where("resource_pool_id = ?", pool.ID).Count(&count).Error; err != nil {
		t.Fatalf("count appended resources: %v", err)
	}
	if count != initialCount+27 {
		t.Fatalf("unexpected total resource count: got %d want %d", count, initialCount+27)
	}
}

func TestRES001AffinityUsesAtomicURLKeyResource(t *testing.T) {
	provider, _, pool := newTestProvider(t)
	first, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic", Affinity: "project-a"})
	if err != nil {
		t.Fatalf("first select: %v", err)
	}
	if err := provider.BindAffinity(pool.ID, "project-a", first.ID, time.Hour); err != nil {
		t.Fatalf("bind successful selection: %v", err)
	}
	second, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic", Affinity: "project-a"})
	if err != nil {
		t.Fatalf("second select: %v", err)
	}
	if first.ID != second.ID || first.UpstreamURL != second.UpstreamURL || first.KeyValue != second.KeyValue {
		t.Fatalf("affinity split atomic resource: first=%#v second=%#v", first, second)
	}
}

func TestRES002NotFoundDoesNotCountOrDisableResource(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	resource, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic"})
	if err != nil {
		t.Fatalf("select resource: %v", err)
	}
	if err := provider.HandleFailure(resource, "anthropic", http.StatusNotFound, "model not found", nil); err != nil {
		t.Fatalf("ignore 404: %v", err)
	}
	var stored models.UpstreamResource
	if err := db.First(&stored, resource.ID).Error; err != nil {
		t.Fatalf("reload resource: %v", err)
	}
	if stored.Status != models.ResourceStatusActive || stored.FailureCount != 0 {
		t.Fatalf("404 changed resource health: %#v", stored)
	}
	selected, err := provider.SelectBoundResource(pool.ID, resource.ID, "openai")
	if err != nil || selected.ID != resource.ID {
		t.Fatalf("404 removed resource from another route: %#v %v", selected, err)
	}
}

func TestRES003InvalidResourceStopsAllRoutes(t *testing.T) {
	provider, _, pool := newTestProvider(t)
	resource, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic"})
	if err != nil {
		t.Fatalf("select resource: %v", err)
	}
	if err := provider.MarkInvalid(resource, "credential rejected"); err != nil {
		t.Fatalf("mark invalid: %v", err)
	}
	for _, route := range []string{"anthropic", "openai"} {
		selected, selectErr := provider.SelectResource(pool.ID, SelectionRequest{Route: route, ExcludeResourceIDs: []uint{otherResourceID(t, provider, pool.ID, resource.ID)}})
		if selectErr == nil || selected != nil {
			t.Fatalf("invalid resource remained selectable for %s: %#v, %v", route, selected, selectErr)
		}
	}
}

func TestRES004EveryNon404FailureAutoDisablesAllRoutes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{name: "bad request", statusCode: http.StatusBadRequest, message: "bad request"},
		{name: "unauthorized", statusCode: http.StatusUnauthorized, message: "credential rejected"},
		{name: "rate limit", statusCode: http.StatusTooManyRequests, message: "rate limited"},
		{name: "upstream failure", statusCode: http.StatusBadGateway, message: "upstream failed"},
		{name: "network failure", statusCode: 0, message: "connection reset"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider, db, pool := newTestProvider(t)
			resource, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic"})
			if err != nil {
				t.Fatalf("select resource: %v", err)
			}
			otherID := otherResourceID(t, provider, pool.ID, resource.ID)
			if err := provider.HandleFailure(resource, "anthropic", tc.statusCode, tc.message, nil); err != nil {
				t.Fatalf("handle resource failure: %v", err)
			}
			var stored models.UpstreamResource
			if err := db.First(&stored, resource.ID).Error; err != nil {
				t.Fatalf("reload resource: %v", err)
			}
			if stored.Status != models.ResourceStatusInvalid || stored.FailureCount != 1 || stored.GlobalCooldownUntil != nil {
				t.Fatalf("failure did not immediately disable resource: %#v", stored)
			}
			for _, route := range []string{"anthropic", "openai"} {
				selected, selectErr := provider.SelectResource(pool.ID, SelectionRequest{
					Route: route, ExcludeResourceIDs: []uint{otherID},
				})
				if selectErr == nil || selected != nil {
					t.Fatalf("auto-disabled resource remained selectable for %s: %#v %v", route, selected, selectErr)
				}
			}
		})
	}
}

func TestRES005AffinityRebindsOnlyAfterSuccessfulFallback(t *testing.T) {
	provider, _, pool := newTestProvider(t)
	const affinity = "project-success-only"
	first, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic", Affinity: affinity})
	if err != nil {
		t.Fatalf("select initial resource: %v", err)
	}
	if err := provider.BindAffinity(pool.ID, affinity, first.ID, time.Hour); err != nil {
		t.Fatalf("bind initial resource: %v", err)
	}

	fallback, err := provider.SelectResource(pool.ID, SelectionRequest{
		Route: "anthropic", Affinity: affinity, ExcludeResourceIDs: []uint{first.ID},
	})
	if err != nil {
		t.Fatalf("select fallback: %v", err)
	}
	if fallback.ID == first.ID {
		t.Fatal("fallback did not leave the failed resource")
	}
	stillBound, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic", Affinity: affinity})
	if err != nil {
		t.Fatalf("read affinity before fallback success: %v", err)
	}
	if stillBound.ID != first.ID {
		t.Fatalf("failed fallback rewrote affinity: got %d want %d", stillBound.ID, first.ID)
	}

	if err := provider.BindAffinity(pool.ID, affinity, fallback.ID, time.Hour); err != nil {
		t.Fatalf("bind successful fallback: %v", err)
	}
	rebound, err := provider.SelectResource(pool.ID, SelectionRequest{Route: "anthropic", Affinity: affinity})
	if err != nil {
		t.Fatalf("read rebound affinity: %v", err)
	}
	if rebound.ID != fallback.ID {
		t.Fatalf("successful fallback did not rebind affinity: got %d want %d", rebound.ID, fallback.ID)
	}
}

func otherResourceID(t *testing.T, provider *Provider, poolID, excluded uint) uint {
	t.Helper()
	resource, err := provider.SelectResource(poolID, SelectionRequest{ExcludeResourceIDs: []uint{excluded}})
	if err != nil {
		t.Fatalf("select other resource: %v", err)
	}
	return resource.ID
}
