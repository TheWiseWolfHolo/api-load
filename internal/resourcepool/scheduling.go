package resourcepool

import (
	app_errors "api-load/internal/errors"
	"api-load/internal/models"
	"api-load/internal/scheduler"
	"encoding/json"
	"fmt"
)

func resourceSchedulerSnapshotKey(poolID uint) string {
	return fmt.Sprintf("resource_pool:%d:scheduler_candidates", poolID)
}

func (p *Provider) syncSchedulerSnapshot(poolID uint) error {
	if poolID == 0 {
		return nil
	}
	var resources []models.UpstreamResource
	if err := p.db.Select("id", "priority", "weight").
		Where("resource_pool_id = ? AND enabled = ? AND status = ?", poolID, true, models.ResourceStatusActive).
		Order("priority asc, id asc").Find(&resources).Error; err != nil {
		return err
	}
	candidates := make([]scheduler.Candidate, 0, len(resources))
	for _, resource := range resources {
		candidates = append(candidates, scheduler.Candidate{ID: resource.ID, Priority: resource.Priority, Weight: resource.Weight})
	}
	candidates = scheduler.Normalize(candidates, models.DefaultCredentialPriority, models.DefaultCredentialWeight)
	encoded, err := json.Marshal(candidates)
	if err != nil {
		return err
	}
	return p.store.Set(resourceSchedulerSnapshotKey(poolID), encoded, 0)
}

func (p *Provider) schedulerCandidates(poolID uint) ([]scheduler.Candidate, error) {
	raw, err := p.store.Get(resourceSchedulerSnapshotKey(poolID))
	if err != nil {
		if err := p.syncSchedulerSnapshot(poolID); err != nil {
			return nil, err
		}
		raw, err = p.store.Get(resourceSchedulerSnapshotKey(poolID))
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

func (p *Provider) selectWeightedResource(poolID uint, route string, excluded map[uint]struct{}) (*models.UpstreamResource, error) {
	candidates, err := p.schedulerCandidates(poolID)
	if err != nil {
		return nil, err
	}
	for _, tier := range scheduler.PriorityTiers(candidates) {
		eligible := make([]scheduler.Candidate, 0, len(tier))
		for _, candidate := range tier {
			if _, skip := excluded[candidate.ID]; !skip {
				eligible = append(eligible, candidate)
			}
		}
		for len(eligible) > 0 {
			resourceID, ok := p.weightedPicker.Pick(fmt.Sprintf("resource_pool:%d", poolID), eligible)
			if !ok {
				break
			}
			resource, loadErr := p.resourceFromStore(resourceID)
			if loadErr == nil && p.isSelectable(resource, route) {
				return resource, nil
			}
			eligible = removeResourceCandidate(eligible, resourceID)
		}
	}
	return nil, app_errors.ErrNoActiveKeys
}

func removeResourceCandidate(candidates []scheduler.Candidate, resourceID uint) []scheduler.Candidate {
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if candidate.ID != resourceID {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

// RefreshScheduler republishes the candidate snapshot after a DB mutation.
func (p *Provider) RefreshScheduler(poolID uint) error {
	return p.syncSchedulerSnapshot(poolID)
}
