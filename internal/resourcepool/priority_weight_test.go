package resourcepool

import (
	"testing"

	"api-load/internal/models"
)

func TestRES014ResourcePoolUsesHardPriorityAndSmoothWeight(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	var resources []models.UpstreamResource
	if err := db.Where("resource_pool_id = ?", pool.ID).Order("id asc").Find(&resources).Error; err != nil {
		t.Fatalf("load resources: %v", err)
	}
	resources[0].Enabled = models.Bool(true)
	resources[0].Priority = 1
	resources[0].Weight = 3
	resources[1].Enabled = models.Bool(true)
	resources[1].Priority = 1
	resources[1].Weight = 1
	for i := range resources {
		if err := db.Save(&resources[i]).Error; err != nil {
			t.Fatalf("save scheduling fields: %v", err)
		}
		if err := provider.SyncResourceToStore(&resources[i]); err != nil {
			t.Fatalf("sync resource: %v", err)
		}
	}

	counts := map[uint]int{}
	for range 8 {
		selected, err := provider.SelectResource(pool.ID, SelectionRequest{})
		if err != nil {
			t.Fatalf("select weighted resource: %v", err)
		}
		counts[selected.ID]++
	}
	if counts[resources[0].ID] != 6 || counts[resources[1].ID] != 2 {
		t.Fatalf("unexpected resource 3:1 distribution: %#v", counts)
	}

	resources[0].Priority = 10
	if err := db.Save(&resources[0]).Error; err != nil {
		t.Fatalf("lower resource priority: %v", err)
	}
	if err := provider.SyncResourceToStore(&resources[0]); err != nil {
		t.Fatalf("sync changed priority: %v", err)
	}
	for range 4 {
		selected, err := provider.SelectResource(pool.ID, SelectionRequest{})
		if err != nil {
			t.Fatalf("select priority resource: %v", err)
		}
		if selected.ID != resources[1].ID {
			t.Fatalf("lower priority resource %d selected over %d", selected.ID, resources[1].ID)
		}
	}
}
