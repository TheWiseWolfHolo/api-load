package resourcepool

import (
	"api-load/internal/keypool"
	"api-load/internal/models"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GroupSelectionPolicy resolves group-owned scheduler settings while retaining
// the pool timing values as compatibility defaults.
type GroupSelectionPolicy struct {
	Strategy               string
	SchedulerScope         string
	AffinityTTL            time.Duration
	BusyWait               time.Duration
	MaxConsecutiveRequests int
}

func SelectionPolicyForGroup(group *models.Group, defaults PoolConfig) GroupSelectionPolicy {
	policy := GroupSelectionPolicy{
		Strategy:       keypool.KeySelectionStrategySticky,
		AffinityTTL:    defaults.AffinityTTL,
		BusyWait:       defaults.BusyWait,
		SchedulerScope: "group:0",
	}
	if policy.AffinityTTL <= 0 {
		policy.AffinityTTL = DefaultAffinityTTL
	}
	if group == nil {
		return policy
	}
	policy.SchedulerScope = fmt.Sprintf("group:%d", group.ID)
	if strategy := groupConfigString(group, "key_selection_strategy"); strategy != "" {
		policy.Strategy = strategy
	}
	if seconds, ok := groupConfigIntValue(group, "resource_affinity_ttl_seconds"); ok && seconds > 0 {
		policy.AffinityTTL = time.Duration(seconds) * time.Second
	}
	if milliseconds, ok := groupConfigIntValue(group, "resource_busy_wait_milliseconds"); ok && milliseconds >= 0 {
		policy.BusyWait = time.Duration(milliseconds) * time.Millisecond
	}
	if count, ok := groupConfigIntValue(group, "fill_max_consecutive_requests"); ok && count > 0 {
		policy.MaxConsecutiveRequests = count
	}
	return policy
}

func groupConfigString(group *models.Group, key string) string {
	if group == nil || group.Config == nil {
		return ""
	}
	value, _ := group.Config[key].(string)
	return strings.TrimSpace(value)
}

func groupConfigIntValue(group *models.Group, key string) (int, bool) {
	if group == nil || group.Config == nil {
		return 0, false
	}
	switch value := group.Config[key].(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		return parsed, err == nil
	default:
		return 0, false
	}
}
