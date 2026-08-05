package scheduler

// IntnSource is the minimal random source used by weighted selectors.
type IntnSource interface {
	Intn(n int) int
}

// PickWeightedRandom chooses one candidate proportionally to its normalized
// weight. Priority tiers are intentionally handled by the caller.
func PickWeightedRandom(candidates []Candidate, rng IntnSource) (uint, bool) {
	if len(candidates) == 0 {
		return 0, false
	}
	totalWeight := 0
	for _, candidate := range candidates {
		weight := candidate.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}
	if totalWeight <= 0 {
		return candidates[0].ID, true
	}
	slot := 0
	if rng != nil {
		slot = rng.Intn(totalWeight)
	}
	for _, candidate := range candidates {
		weight := candidate.Weight
		if weight <= 0 {
			weight = 1
		}
		if slot < weight {
			return candidate.ID, true
		}
		slot -= weight
	}
	return candidates[len(candidates)-1].ID, true
}
