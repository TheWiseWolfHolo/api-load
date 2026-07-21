package scheduler

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Candidate is the common scheduling metadata for an API key or a physical resource.
type Candidate struct {
	ID       uint `json:"id"`
	Priority int  `json:"priority"`
	Weight   int  `json:"weight"`
}

func Normalize(candidates []Candidate, defaultPriority, defaultWeight int) []Candidate {
	normalized := append([]Candidate(nil), candidates...)
	for i := range normalized {
		if normalized[i].Priority <= 0 {
			normalized[i].Priority = defaultPriority
		}
		if normalized[i].Weight <= 0 {
			normalized[i].Weight = defaultWeight
		}
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Priority != normalized[j].Priority {
			return normalized[i].Priority < normalized[j].Priority
		}
		return normalized[i].ID < normalized[j].ID
	})
	return normalized
}

// PriorityTiers returns candidates split into hard priority layers.
func PriorityTiers(candidates []Candidate) [][]Candidate {
	if len(candidates) == 0 {
		return nil
	}
	tiers := make([][]Candidate, 0)
	for _, candidate := range candidates {
		if len(tiers) == 0 || tiers[len(tiers)-1][0].Priority != candidate.Priority {
			tiers = append(tiers, []Candidate{candidate})
			continue
		}
		tiers[len(tiers)-1] = append(tiers[len(tiers)-1], candidate)
	}
	return tiers
}

type smoothState struct {
	signature string
	current   map[uint]int64
}

// SmoothPicker implements smooth weighted round robin. State is local to one
// api-load process; every process independently converges to the same ratio.
type SmoothPicker struct {
	mu     sync.Mutex
	states map[string]*smoothState
}

func NewSmoothPicker() *SmoothPicker {
	return &SmoothPicker{states: make(map[string]*smoothState)}
}

func (p *SmoothPicker) Pick(scope string, candidates []Candidate) (uint, bool) {
	if len(candidates) == 0 {
		return 0, false
	}
	signature := candidateSignature(candidates)

	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.states[scope]
	if state == nil || state.signature != signature {
		state = &smoothState{signature: signature, current: make(map[uint]int64, len(candidates))}
		p.states[scope] = state
	}

	var selected uint
	var selectedCurrent int64
	var total int64
	for i, candidate := range candidates {
		weight := int64(candidate.Weight)
		if weight <= 0 {
			weight = 1
		}
		state.current[candidate.ID] += weight
		total += weight
		if i == 0 || state.current[candidate.ID] > selectedCurrent {
			selected = candidate.ID
			selectedCurrent = state.current[candidate.ID]
		}
	}
	state.current[selected] -= total
	return selected, true
}

func candidateSignature(candidates []Candidate) string {
	var builder strings.Builder
	for _, candidate := range candidates {
		fmt.Fprintf(&builder, "%d:%d;", candidate.ID, candidate.Weight)
	}
	return builder.String()
}
