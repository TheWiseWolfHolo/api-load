package resourcepool

import (
	"api-load/internal/models"
	"fmt"
	"strconv"
)

func (p *Provider) selectCurrentFillFirstResource(poolID uint, req SelectionRequest, excluded map[uint]struct{}) (*models.UpstreamResource, bool) {
	if req.SchedulerScope == "" {
		return nil, false
	}
	key := fillFirstCurrentKey(poolID, req.SchedulerScope)
	raw, err := p.store.Get(key)
	if err != nil {
		return nil, false
	}
	resourceID, err := strconv.ParseUint(string(raw), 10, 64)
	if err != nil {
		_ = p.clearFillFirst(poolID, req.SchedulerScope)
		return nil, false
	}
	if _, skip := excluded[uint(resourceID)]; skip {
		return nil, false
	}
	resource, err := p.resourceFromStore(uint(resourceID))
	if err != nil || !p.isSelectable(resource, req.Route) {
		_ = p.clearFillFirst(poolID, req.SchedulerScope)
		return nil, false
	}
	return resource, true
}

func (p *Provider) recordFillFirstSuccess(poolID, resourceID uint, req SelectionRequest) error {
	if req.SchedulerScope == "" || resourceID == 0 {
		return nil
	}
	currentKey := fillFirstCurrentKey(poolID, req.SchedulerScope)
	countKey := fillFirstCountKey(poolID, req.SchedulerScope)
	count := 0
	if current, err := p.store.Get(currentKey); err == nil && string(current) == strconv.FormatUint(uint64(resourceID), 10) {
		if rawCount, countErr := p.store.Get(countKey); countErr == nil {
			count, _ = strconv.Atoi(string(rawCount))
		}
	}
	count++
	if req.MaxConsecutiveRequests > 0 && count >= req.MaxConsecutiveRequests {
		if err := p.clearFillFirst(poolID, req.SchedulerScope); err != nil {
			return err
		}
		return p.store.Set(
			fillFirstRotateAwayKey(poolID, req.SchedulerScope),
			[]byte(strconv.FormatUint(uint64(resourceID), 10)),
			0,
		)
	}
	if err := p.store.Set(currentKey, []byte(strconv.FormatUint(uint64(resourceID), 10)), 0); err != nil {
		return err
	}
	return p.store.Set(countKey, []byte(strconv.Itoa(count)), 0)
}

func (p *Provider) clearFillFirst(poolID uint, scope string) error {
	if err := p.store.Delete(fillFirstCurrentKey(poolID, scope)); err != nil {
		return err
	}
	return p.store.Delete(fillFirstCountKey(poolID, scope))
}

func (p *Provider) consumeFillFirstRotateAway(poolID uint, scope string) uint {
	if scope == "" {
		return 0
	}
	key := fillFirstRotateAwayKey(poolID, scope)
	raw, err := p.store.Get(key)
	if err != nil {
		return 0
	}
	_ = p.store.Delete(key)
	resourceID, err := strconv.ParseUint(string(raw), 10, 64)
	if err != nil {
		return 0
	}
	return uint(resourceID)
}

func fillFirstCurrentKey(poolID uint, scope string) string {
	return fmt.Sprintf("resource_fill_first:%d:%s:current", poolID, scope)
}

func fillFirstCountKey(poolID uint, scope string) string {
	return fmt.Sprintf("resource_fill_first:%d:%s:count", poolID, scope)
}

func fillFirstRotateAwayKey(poolID uint, scope string) string {
	return fmt.Sprintf("resource_fill_first:%d:%s:rotate_away", poolID, scope)
}
