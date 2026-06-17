package keypool

import (
	"testing"

	"gpt-load/internal/models"
)

func TestSCH001DefaultRoundRobinBehaviorIsUnchanged(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-round-robin-a", KeyHash: "hash-rr-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-round-robin-b", KeyHash: "hash-rr-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	first, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select first key: %v", err)
	}
	second, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select second key: %v", err)
	}
	third, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select third key: %v", err)
	}

	if first.ID == second.ID {
		t.Fatalf("expected round-robin to rotate between active keys, got same key %d twice", first.ID)
	}
	if third.ID != first.ID {
		t.Fatalf("expected round-robin cycle to return to first key %d, got %d", first.ID, third.ID)
	}
}
