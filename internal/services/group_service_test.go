package services

import (
	"context"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"strings"
	"testing"
)

func TestKEY010GroupStatsDistinguishInvalidAndDisabled(t *testing.T) {
	svc, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	seedKey(t, svc, group.ID, "sk-test-active", "", models.KeyStatusActive, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-invalid", "", models.KeyStatusInvalid, 0, 0)
	seedKey(t, svc, group.ID, "sk-test-disabled", "", models.KeyStatusDisabled, 0, 0)

	groupSvc := &GroupService{db: db}
	stats, err := groupSvc.fetchKeyStats(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("fetch key stats: %v", err)
	}

	if stats.TotalKeys != 3 || stats.ActiveKeys != 1 || stats.InvalidKeys != 1 || stats.DisabledKeys != 1 {
		t.Fatalf("unexpected key stats: %#v", stats)
	}
}

func TestMOD005GroupModelSelectionPersistsEnabledModels(t *testing.T) {
	_, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	groupSvc := &GroupService{db: db}

	if err := groupSvc.SaveGroupModels(context.Background(), group.ID, []string{" gpt-b ", "gpt-a", "gpt-b", ""}); err != nil {
		t.Fatalf("save group models: %v", err)
	}

	models, err := groupSvc.GetGroupModels(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("get group models: %v", err)
	}
	if strings.Join(models, ",") != "gpt-a,gpt-b" {
		t.Fatalf("expected deterministic deduplicated models, got %#v", models)
	}
}

func TestSCH009SchedulerConfigRoundTripsOnGroupConfig(t *testing.T) {
	_, db, _, _ := newTestKeyService(t)
	group := createServiceTestGroup(t, db)
	groupSvc := &GroupService{
		db:              db,
		settingsManager: config.NewSystemSettingsManager(),
		channelRegistry: []string{"openai"},
	}

	updated, err := groupSvc.UpdateGroup(context.Background(), group.ID, GroupUpdateParams{Config: map[string]any{
		"key_selection_strategy":        "fill_first",
		"key_affinity_scope":            "model+proxy_key",
		"fill_cooldown_minutes":         float64(3),
		"fill_switch_status_codes":      "429,500",
		"fill_quota_patterns":           "balance exhausted",
		"fill_max_consecutive_requests": float64(7),
		"fill_max_consecutive_tokens":   float64(1000),
		"fill_sticky_ttl_seconds":       float64(60),
	}})
	if err != nil {
		t.Fatalf("update scheduler config: %v", err)
	}

	if updated.Config["key_selection_strategy"] != "fill_first" || updated.Config["key_affinity_scope"] != "model+proxy_key" {
		t.Fatalf("scheduler config did not round-trip: %#v", updated.Config)
	}
	if updated.Config["fill_max_consecutive_requests"] != float64(7) {
		t.Fatalf("scheduler numeric config did not round-trip: %#v", updated.Config)
	}

	_, err = groupSvc.UpdateGroup(context.Background(), group.ID, GroupUpdateParams{Config: map[string]any{
		"key_selection_strategy": "teleport",
	}})
	if err == nil {
		t.Fatal("expected validation error for unknown scheduler strategy")
	}

	_, err = groupSvc.UpdateGroup(context.Background(), group.ID, GroupUpdateParams{Config: map[string]any{
		"key_selection_strategy": "sticky",
		"key_affinity_scope":     "raw_api_key",
	}})
	if err == nil {
		t.Fatal("expected validation error for unknown affinity scope")
	}
}
