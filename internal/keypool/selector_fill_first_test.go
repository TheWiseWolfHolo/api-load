package keypool

import (
	"testing"

	"gpt-load/internal/models"

	"gorm.io/datatypes"
)

func TestSCH006FillFirstKeepsCurrentKeyUntilRequestLimit(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = datatypes.JSONMap{
		"key_selection_strategy":        "fill_first",
		"fill_max_consecutive_requests": float64(2),
	}

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-fill-first-a", KeyHash: "hash-fill-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-fill-first-b", KeyHash: "hash-fill-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	first, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select first fill-first key: %v", err)
	}
	if err := provider.RecordSelectionResult(&group, first, SelectionResult{StatusCode: 200}); err != nil {
		t.Fatalf("record first success: %v", err)
	}

	second, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select second fill-first key: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected fill-first to keep current key, got %d then %d", first.ID, second.ID)
	}
	if err := provider.RecordSelectionResult(&group, second, SelectionResult{StatusCode: 200}); err != nil {
		t.Fatalf("record second success: %v", err)
	}

	third, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select after request limit: %v", err)
	}
	if third.ID == first.ID {
		t.Fatalf("expected fill-first to switch after request limit, still got %d", first.ID)
	}
}

func TestSCH007FillFirstRateLimitUsesCooldownNotExhausted(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = datatypes.JSONMap{
		"key_selection_strategy": "fill_first",
		"fill_cooldown_minutes":  float64(1),
	}

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-fill-cooldown-a", KeyHash: "hash-fill-cooldown-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-fill-cooldown-b", KeyHash: "hash-fill-cooldown-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	first, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select first fill-first key: %v", err)
	}
	if err := provider.RecordSelectionResult(&group, first, SelectionResult{StatusCode: 429, ErrorMessage: "rate limited"}); err != nil {
		t.Fatalf("record plain 429: %v", err)
	}

	var stored models.APIKey
	if err := db.First(&stored, first.ID).Error; err != nil {
		t.Fatalf("load first key: %v", err)
	}
	if stored.Status != models.KeyStatusActive {
		t.Fatalf("plain 429 should not mark key exhausted or invalid, got status %q", stored.Status)
	}
	onCooldown, err := memStore.Exists(fillFirstCooldownKey(first.ID))
	if err != nil {
		t.Fatalf("check cooldown: %v", err)
	}
	if !onCooldown {
		t.Fatal("expected plain 429 key to enter cooldown")
	}

	next, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select after cooldown: %v", err)
	}
	if next.ID == first.ID {
		t.Fatalf("expected fill-first to skip cooldown key %d", first.ID)
	}
}
