package keypool

import (
	"testing"

	"api-load/internal/models"
	"gorm.io/gorm"
)

func TestSCH008RoundRobinUsesHardPriorityTiers(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-priority-high-a", KeyHash: "priority-high-a", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 1, Weight: 1},
		{GroupID: group.ID, KeyValue: "sk-priority-high-b", KeyHash: "priority-high-b", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 1, Weight: 1},
		{GroupID: group.ID, KeyValue: "sk-priority-low", KeyHash: "priority-low", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1000},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	highIDs := map[uint]bool{keys[0].ID: true, keys[1].ID: true}
	for range 8 {
		selected, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
		if err != nil {
			t.Fatalf("select priority key: %v", err)
		}
		if !highIDs[selected.ID] {
			t.Fatalf("lower priority key %d was selected while priority 1 remained available", selected.ID)
		}
	}

	selected, err := provider.SelectKeyForRequest(&group, SelectionRequest{ExcludeKeyIDs: []uint{keys[0].ID, keys[1].ID}})
	if err != nil {
		t.Fatalf("select fallback priority: %v", err)
	}
	if selected.ID != keys[2].ID {
		t.Fatalf("expected lower priority fallback %d, got %d", keys[2].ID, selected.ID)
	}
}

func TestSCH009RoundRobinUsesSmoothWeightsWithinPriority(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-weight-three", KeyHash: "weight-three", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 3},
		{GroupID: group.ID, KeyValue: "sk-weight-one", KeyHash: "weight-one", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	counts := map[uint]int{}
	sequence := make([]uint, 0, 8)
	for range 8 {
		selected, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
		if err != nil {
			t.Fatalf("select weighted key: %v", err)
		}
		counts[selected.ID]++
		sequence = append(sequence, selected.ID)
	}
	if counts[keys[0].ID] != 6 || counts[keys[1].ID] != 2 {
		t.Fatalf("unexpected 3:1 distribution: counts=%#v sequence=%v", counts, sequence)
	}
	if sequence[0] == sequence[1] && sequence[1] == sequence[2] {
		t.Fatalf("weighted round robin was clustered instead of smooth: %v", sequence)
	}
}

func TestSCH010ManualEnablementIsIndependentFromHealth(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-disabled-healthy", KeyHash: "disabled-healthy", Status: models.KeyStatusActive, Enabled: models.Bool(false), Priority: 1, Weight: 1},
		{GroupID: group.ID, KeyValue: "sk-enabled-invalid", KeyHash: "enabled-invalid", Status: models.KeyStatusInvalid, Enabled: models.Bool(true), Priority: 1, Weight: 1},
		{GroupID: group.ID, KeyValue: "sk-enabled-healthy", KeyHash: "enabled-healthy", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	selected, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select enabled healthy key: %v", err)
	}
	if selected.ID != keys[2].ID {
		t.Fatalf("scheduler ignored independent enablement/health state: got %d want %d", selected.ID, keys[2].ID)
	}
}

func TestKEY006HTTP404DoesNotAffectHealthFailureCount(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	key := models.APIKey{GroupID: group.ID, KeyValue: "sk-health", KeyHash: "health", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1}
	if err := provider.AddKeys(group.ID, []models.APIKey{key}); err != nil {
		t.Fatalf("add key: %v", err)
	}
	key = keyWithID(t, db, "health")

	if err := provider.updateStatus(&key, &group, false, 404, "model not found"); err != nil {
		t.Fatalf("record 404: %v", err)
	}
	stored := keyWithID(t, db, "health")
	if stored.FailureCount != 0 || stored.Status != models.KeyStatusActive {
		t.Fatalf("404 changed key health: %#v", stored)
	}

	if err := provider.updateStatus(&key, &group, false, 500, "upstream failed"); err != nil {
		t.Fatalf("record 500: %v", err)
	}
	stored = keyWithID(t, db, "health")
	if stored.FailureCount != 1 {
		t.Fatalf("non-404 failure was not counted: %#v", stored)
	}

	if err := provider.updateStatus(&key, &group, false, 400, "resource has been exhausted"); err != nil {
		t.Fatalf("record formerly uncounted failure: %v", err)
	}
	stored = keyWithID(t, db, "health")
	if stored.FailureCount != 2 {
		t.Fatalf("only 404 may bypass failure counting: %#v", stored)
	}
}

func TestSCH011StickyFailoverRebindsToReplacement(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = map[string]any{"key_selection_strategy": KeySelectionStrategySticky}
	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-sticky-a", KeyHash: "sticky-a", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
		{GroupID: group.ID, KeyValue: "sk-sticky-b", KeyHash: "sticky-b", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add sticky keys: %v", err)
	}

	first, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select initial sticky key: %v", err)
	}
	replacement, err := provider.SelectKeyForRequest(&group, SelectionRequest{ExcludeKeyIDs: []uint{first.ID}})
	if err != nil {
		t.Fatalf("select sticky failover: %v", err)
	}
	if replacement.ID == first.ID {
		t.Fatalf("sticky failover reused excluded key %d", first.ID)
	}
	if err := provider.RecordSelectionResult(&group, replacement, SelectionResult{StatusCode: 200}); err != nil {
		t.Fatalf("record successful sticky failover: %v", err)
	}
	rebound, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select rebound sticky key: %v", err)
	}
	if rebound.ID != replacement.ID {
		t.Fatalf("sticky route was not rebound after failover: got %d want %d", rebound.ID, replacement.ID)
	}
}

func keyWithID(t *testing.T, db *gorm.DB, hash string) models.APIKey {
	t.Helper()
	var key models.APIKey
	if err := db.Where("key_hash = ?", hash).First(&key).Error; err != nil {
		t.Fatalf("load key %s: %v", hash, err)
	}
	return key
}
