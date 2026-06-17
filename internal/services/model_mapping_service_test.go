package services

import "testing"

type mappingSequenceRNG struct {
	values []int
	next   int
}

func (r *mappingSequenceRNG) Intn(n int) int {
	value := r.values[r.next%len(r.values)]
	r.next++
	if n == 0 {
		return 0
	}
	return value % n
}

func TestMAP001ExactModelAliasMapsToWeightedTargets(t *testing.T) {
	service := NewModelMappingService(&mappingSequenceRNG{values: []int{12}})
	decision, err := service.Resolve("gpt-4.1", []ModelMappingRule{
		{
			Alias: "gpt-4.1",
			Targets: []ModelMappingTarget{
				{SubGroupID: 10, Model: "gpt-4.1-main", Weight: 10},
				{SubGroupID: 20, Model: "gpt-4.1-backup", Weight: 5},
				{SubGroupID: 30, Model: "gpt-4.1-disabled", Weight: 0},
			},
		},
	}, true)
	if err != nil {
		t.Fatalf("resolve exact alias: %v", err)
	}
	if decision.SubGroupID != 20 || decision.Model != "gpt-4.1-backup" {
		t.Fatalf("unexpected weighted target: %#v", decision)
	}
}

func TestMAP002WildcardAliasesMatchWithExactPrecedence(t *testing.T) {
	service := NewModelMappingService(&mappingSequenceRNG{values: []int{0}})
	rules := []ModelMappingRule{
		{Alias: "gpt-4*", Targets: []ModelMappingTarget{{SubGroupID: 1, Model: "wildcard", Weight: 1}}},
		{Alias: "gpt-4.1", Targets: []ModelMappingTarget{{SubGroupID: 2, Model: "exact", Weight: 1}}},
		{Alias: "claude-*", Targets: []ModelMappingTarget{{SubGroupID: 3, Model: "claude-real", Weight: 1}}},
	}

	exact, err := service.Resolve("gpt-4.1", rules, true)
	if err != nil {
		t.Fatalf("resolve exact: %v", err)
	}
	if exact.SubGroupID != 2 || exact.Model != "exact" {
		t.Fatalf("expected exact alias precedence, got %#v", exact)
	}

	wildcard, err := service.Resolve("claude-3-5", rules, true)
	if err != nil {
		t.Fatalf("resolve wildcard: %v", err)
	}
	if wildcard.SubGroupID != 3 || wildcard.Model != "claude-real" {
		t.Fatalf("expected wildcard alias, got %#v", wildcard)
	}
}

func TestMAP003StrictModeRejectsUnmatchedModels(t *testing.T) {
	service := NewModelMappingService(&mappingSequenceRNG{values: []int{0}})
	_, err := service.Resolve("unknown-model", nil, true)
	if err == nil {
		t.Fatal("expected strict mode to reject unmatched model")
	}
	if !IsModelNotSupportedError(err) {
		t.Fatalf("expected model not supported error, got %v", err)
	}
}

func TestMAP004NonStrictModeFallsBackToDefaultRouting(t *testing.T) {
	service := NewModelMappingService(&mappingSequenceRNG{values: []int{0}})
	decision, err := service.Resolve("unknown-model", nil, false)
	if err != nil {
		t.Fatalf("resolve non-strict fallback: %v", err)
	}
	if !decision.Fallback || decision.Model != "unknown-model" || decision.SubGroupID != 0 {
		t.Fatalf("unexpected fallback decision: %#v", decision)
	}
}
