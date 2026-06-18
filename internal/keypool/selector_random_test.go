package keypool

import (
	"testing"

	"api-load/internal/models"

	"gorm.io/datatypes"
)

type sequenceRNG struct {
	values []int
	next   int
}

func (r *sequenceRNG) Intn(n int) int {
	value := r.values[r.next%len(r.values)]
	r.next++
	if n == 0 {
		return 0
	}
	return value % n
}

func TestSCH002RandomStrategySelectsOnlyActiveKeys(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	provider.SetSelectionRNG(&sequenceRNG{values: []int{0, 1, 0, 1}})
	group := createTestGroup(t, db)
	group.Config = datatypes.JSONMap{"key_selection_strategy": "random"}

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-random-active-a", KeyHash: "hash-random-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-random-invalid", KeyHash: "hash-random-invalid", Status: models.KeyStatusInvalid},
		{GroupID: group.ID, KeyValue: "sk-test-random-disabled", KeyHash: "hash-random-disabled", Status: models.KeyStatusDisabled},
		{GroupID: group.ID, KeyValue: "sk-test-random-active-b", KeyHash: "hash-random-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	seen := map[string]bool{}
	for range 4 {
		selected, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
		if err != nil {
			t.Fatalf("select random key: %v", err)
		}
		if selected.Status != models.KeyStatusActive {
			t.Fatalf("random selected non-active key: %#v", selected)
		}
		seen[selected.KeyValue] = true
	}

	if !seen["sk-test-random-active-a"] || !seen["sk-test-random-active-b"] {
		t.Fatalf("expected deterministic rng to visit both active keys, saw %#v", seen)
	}
}
