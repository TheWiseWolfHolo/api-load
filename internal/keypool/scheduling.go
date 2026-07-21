package keypool

import (
	app_errors "api-load/internal/errors"
	"api-load/internal/models"
	"api-load/internal/scheduler"
	"encoding/json"
	"fmt"
)

func keySchedulerSnapshotKey(groupID uint) string {
	return fmt.Sprintf("group:%d:scheduler_candidates", groupID)
}

func (p *KeyProvider) syncSchedulerSnapshot(groupID uint) error {
	if groupID == 0 {
		return nil
	}
	var keys []models.APIKey
	if err := p.db.Select("id", "priority", "weight").
		Where("group_id = ? AND enabled = ? AND status = ?", groupID, true, models.KeyStatusActive).
		Order("priority asc, id asc").Find(&keys).Error; err != nil {
		return err
	}
	candidates := make([]scheduler.Candidate, 0, len(keys))
	for _, key := range keys {
		candidates = append(candidates, scheduler.Candidate{ID: key.ID, Priority: key.Priority, Weight: key.Weight})
	}
	candidates = scheduler.Normalize(candidates, models.DefaultCredentialPriority, models.DefaultCredentialWeight)
	encoded, err := json.Marshal(candidates)
	if err != nil {
		return err
	}
	return p.store.Set(keySchedulerSnapshotKey(groupID), encoded, 0)
}

func (p *KeyProvider) schedulerCandidates(groupID uint) ([]scheduler.Candidate, error) {
	raw, err := p.store.Get(keySchedulerSnapshotKey(groupID))
	if err != nil {
		if err := p.syncSchedulerSnapshot(groupID); err != nil {
			return nil, err
		}
		raw, err = p.store.Get(keySchedulerSnapshotKey(groupID))
		if err != nil {
			return nil, err
		}
	}
	var candidates []scheduler.Candidate
	if err := json.Unmarshal(raw, &candidates); err != nil {
		return nil, err
	}
	return scheduler.Normalize(candidates, models.DefaultCredentialPriority, models.DefaultCredentialWeight), nil
}

func (p *KeyProvider) selectWeightedKey(groupID uint, excluded map[uint]struct{}, scope string) (*models.APIKey, error) {
	candidates, err := p.schedulerCandidates(groupID)
	if err != nil {
		return nil, err
	}
	for _, tier := range scheduler.PriorityTiers(candidates) {
		eligible := filterKeyCandidates(tier, excluded)
		for len(eligible) > 0 {
			keyID, ok := p.weightedPicker.Pick(fmt.Sprintf("key:%d:%s", groupID, scope), eligible)
			if !ok {
				break
			}
			apiKey, loadErr := p.keyFromStore(groupID, keyID)
			if loadErr == nil {
				return apiKey, nil
			}
			eligible = removeKeyCandidate(eligible, keyID)
		}
	}
	return nil, app_errors.ErrNoActiveKeys
}

func filterKeyCandidates(candidates []scheduler.Candidate, excluded map[uint]struct{}) []scheduler.Candidate {
	filtered := make([]scheduler.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if _, skip := excluded[candidate.ID]; !skip {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func removeKeyCandidate(candidates []scheduler.Candidate, keyID uint) []scheduler.Candidate {
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if candidate.ID != keyID {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}
