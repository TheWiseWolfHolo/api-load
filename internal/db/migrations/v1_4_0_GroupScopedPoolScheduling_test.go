package db

import (
	"fmt"
	"testing"

	"api-load/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestV140PreservesPoolBehaviorWithStickyGroupStrategy(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:v140-pool-scheduling?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&models.ResourcePool{}, &models.Group{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	pool := models.ResourcePool{Name: "shared", Strategy: "round_robin", AffinityTTLSeconds: 3600}
	if err := database.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	pooled := models.Group{
		Name: "pooled", GroupType: "standard", ResourcePoolID: &pool.ID,
		ChannelType: "anthropic", TestModel: "test", Upstreams: []byte("[]"),
		Config: datatypes.JSONMap{"key_selection_strategy": "round_robin", "max_retries": float64(2)},
	}
	ordinary := models.Group{
		Name: "ordinary", GroupType: "standard", ChannelType: "openai", TestModel: "test",
		Upstreams: []byte("[]"), Config: datatypes.JSONMap{"key_selection_strategy": "round_robin"},
	}
	if err := database.Create(&pooled).Error; err != nil {
		t.Fatalf("create pooled group: %v", err)
	}
	if err := database.Create(&ordinary).Error; err != nil {
		t.Fatalf("create ordinary group: %v", err)
	}

	if err := V1_4_0_GroupScopedPoolScheduling(database); err != nil {
		t.Fatalf("run migration: %v", err)
	}
	if err := V1_4_0_GroupScopedPoolScheduling(database); err != nil {
		t.Fatalf("rerun migration: %v", err)
	}
	if err := database.First(&pooled, pooled.ID).Error; err != nil {
		t.Fatalf("reload pooled group: %v", err)
	}
	if got := pooled.Config["key_selection_strategy"]; got != "sticky" {
		t.Fatalf("pooled strategy = %#v, want sticky", got)
	}
	if got := pooled.Config["max_retries"]; fmt.Sprint(got) != "2" {
		t.Fatalf("migration lost unrelated config: %#v", pooled.Config)
	}
	if err := database.First(&ordinary, ordinary.ID).Error; err != nil {
		t.Fatalf("reload ordinary group: %v", err)
	}
	if got := ordinary.Config["key_selection_strategy"]; got != "round_robin" {
		t.Fatalf("ordinary strategy changed: %#v", got)
	}
}
