package resourcepool

import (
	"testing"

	"api-load/internal/models"
)

type fixedSelectionRNG struct{ value int }

func (r fixedSelectionRNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return r.value % n
}

func loadPoolResources(t *testing.T, provider *Provider, poolID uint) []models.UpstreamResource {
	t.Helper()
	var resources []models.UpstreamResource
	if err := provider.db.Where("resource_pool_id = ?", poolID).Order("id asc").Find(&resources).Error; err != nil {
		t.Fatalf("load resources: %v", err)
	}
	return resources
}

func TestRES015RoundRobinIgnoresExistingAffinity(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	resources := loadPoolResources(t, provider, pool.ID)

	resources[0].Priority = 20
	resources[1].Priority = 10
	for i := range resources {
		if err := db.Save(&resources[i]).Error; err != nil {
			t.Fatalf("save resource: %v", err)
		}
		if err := provider.SyncResourceToStore(&resources[i]); err != nil {
			t.Fatalf("sync resource: %v", err)
		}
	}

	if err := provider.BindAffinityForScope(pool.ID, "group:1", "session", resources[0].ID, 0); err != nil {
		t.Fatalf("bind affinity: %v", err)
	}
	selected, err := provider.SelectResource(pool.ID, SelectionRequest{
		Strategy:       "round_robin",
		SchedulerScope: "group:1",
		Affinity:       "session",
	})
	if err != nil {
		t.Fatalf("select round-robin resource: %v", err)
	}
	if selected.ID != resources[1].ID {
		t.Fatalf("round robin reused affinity resource %d; want priority resource %d", selected.ID, resources[1].ID)
	}
}

func TestRES016StickyAffinityIsScopedPerGroup(t *testing.T) {
	provider, _, pool := newTestProvider(t)
	resources := loadPoolResources(t, provider, pool.ID)

	if err := provider.BindAffinityForScope(pool.ID, "group:1", "same-session", resources[0].ID, 0); err != nil {
		t.Fatalf("bind group 1: %v", err)
	}
	if err := provider.BindAffinityForScope(pool.ID, "group:2", "same-session", resources[1].ID, 0); err != nil {
		t.Fatalf("bind group 2: %v", err)
	}

	for _, tc := range []struct {
		scope string
		want  uint
	}{{"group:1", resources[0].ID}, {"group:2", resources[1].ID}} {
		selected, err := provider.SelectResource(pool.ID, SelectionRequest{
			Strategy: "sticky", SchedulerScope: tc.scope, Affinity: "same-session",
		})
		if err != nil {
			t.Fatalf("select %s: %v", tc.scope, err)
		}
		if selected.ID != tc.want {
			t.Fatalf("scope %s selected %d, want %d", tc.scope, selected.ID, tc.want)
		}
	}
}

func TestRES017RandomRespectsPriorityAndWeight(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	resources := loadPoolResources(t, provider, pool.ID)
	resources[0].Priority, resources[0].Weight = 10, 1
	resources[1].Priority, resources[1].Weight = 10, 3
	for i := range resources {
		if err := db.Save(&resources[i]).Error; err != nil {
			t.Fatalf("save resource: %v", err)
		}
		if err := provider.SyncResourceToStore(&resources[i]); err != nil {
			t.Fatalf("sync resource: %v", err)
		}
	}
	provider.SetSelectionRNG(fixedSelectionRNG{value: 3})

	selected, err := provider.SelectResource(pool.ID, SelectionRequest{Strategy: "random"})
	if err != nil {
		t.Fatalf("select random resource: %v", err)
	}
	if selected.ID != resources[1].ID {
		t.Fatalf("weighted random selected %d, want %d", selected.ID, resources[1].ID)
	}

	resources[0].Priority = 1
	if err := db.Save(&resources[0]).Error; err != nil {
		t.Fatalf("save priority resource: %v", err)
	}
	if err := provider.SyncResourceToStore(&resources[0]); err != nil {
		t.Fatalf("sync priority resource: %v", err)
	}
	selected, err = provider.SelectResource(pool.ID, SelectionRequest{Strategy: "random"})
	if err != nil {
		t.Fatalf("select priority random resource: %v", err)
	}
	if selected.ID != resources[0].ID {
		t.Fatalf("random crossed priority tier: got %d want %d", selected.ID, resources[0].ID)
	}
}

func TestRES018FillFirstIsScopedAndRotatesAfterSuccessLimit(t *testing.T) {
	provider, _, pool := newTestProvider(t)
	req := SelectionRequest{
		Strategy: "fill_first", SchedulerScope: "group:7", MaxConsecutiveRequests: 2,
	}

	first, err := provider.SelectResource(pool.ID, req)
	if err != nil {
		t.Fatalf("select first resource: %v", err)
	}
	if err := provider.RecordSelectionSuccess(pool.ID, first.ID, req); err != nil {
		t.Fatalf("record first success: %v", err)
	}
	second, err := provider.SelectResource(pool.ID, req)
	if err != nil {
		t.Fatalf("select sticky fill resource: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("fill-first moved before limit: first=%d second=%d", first.ID, second.ID)
	}
	if err := provider.RecordSelectionSuccess(pool.ID, second.ID, req); err != nil {
		t.Fatalf("record second success: %v", err)
	}

	rotated, err := provider.SelectResource(pool.ID, req)
	if err != nil {
		t.Fatalf("select rotated resource: %v", err)
	}
	if rotated.ID == first.ID {
		t.Fatalf("fill-first did not rotate after %d successes", req.MaxConsecutiveRequests)
	}
	if err := provider.RecordSelectionSuccess(pool.ID, rotated.ID, req); err != nil {
		t.Fatalf("record rotated success: %v", err)
	}

	otherScope, err := provider.SelectResource(pool.ID, SelectionRequest{
		Strategy: "fill_first", SchedulerScope: "group:8", MaxConsecutiveRequests: 2,
	})
	if err != nil {
		t.Fatalf("select other scope: %v", err)
	}
	if err := provider.RecordSelectionSuccess(pool.ID, otherScope.ID, SelectionRequest{
		Strategy: "fill_first", SchedulerScope: "group:8", MaxConsecutiveRequests: 2,
	}); err != nil {
		t.Fatalf("record other scope: %v", err)
	}
	selectedAgain, err := provider.SelectResource(pool.ID, req)
	if err != nil {
		t.Fatalf("select original scope again: %v", err)
	}
	if selectedAgain.ID != rotated.ID {
		t.Fatalf("another group changed fill-first state: got %d want %d", selectedAgain.ID, rotated.ID)
	}
}

func TestRES019FillFirstReusesOnlyResourceAfterSuccessLimit(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	resources := loadPoolResources(t, provider, pool.ID)
	resources[1].Enabled = models.Bool(false)
	if err := db.Save(&resources[1]).Error; err != nil {
		t.Fatalf("disable second resource: %v", err)
	}
	if err := provider.SyncResourceToStore(&resources[1]); err != nil {
		t.Fatalf("sync disabled resource: %v", err)
	}

	req := SelectionRequest{
		Strategy: "fill_first", SchedulerScope: "group:9", MaxConsecutiveRequests: 1,
	}
	first, err := provider.SelectResource(pool.ID, req)
	if err != nil {
		t.Fatalf("select only resource: %v", err)
	}
	if err := provider.RecordSelectionSuccess(pool.ID, first.ID, req); err != nil {
		t.Fatalf("record success: %v", err)
	}
	selectedAgain, err := provider.SelectResource(pool.ID, req)
	if err != nil {
		t.Fatalf("reuse only resource: %v", err)
	}
	if selectedAgain.ID != first.ID {
		t.Fatalf("selected %d after limit, want only resource %d", selectedAgain.ID, first.ID)
	}
}
