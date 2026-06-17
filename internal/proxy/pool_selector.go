package proxy

import (
	"errors"
	"sync"
)

const (
	ProxyPoolStrategyRoundRobin = "round_robin"
	ProxyPoolStrategyRandom     = "random"
	ProxyPoolStrategySticky     = "sticky"
	ProxyPoolStrategyFailover   = "failover"
)

var ErrNoEnabledProxyItems = errors.New("no enabled proxy items")

type proxyPoolRNG interface {
	Intn(n int) int
}

type ProxyPoolSelector struct {
	strategy string
	rng      proxyPoolRNG
	mu       sync.Mutex
	next     map[string]int
	sticky   map[string]string
}

func NewProxyPoolSelector(strategy string, rng proxyPoolRNG) *ProxyPoolSelector {
	return &ProxyPoolSelector{
		strategy: strategy,
		rng:      rng,
		next:     make(map[string]int),
		sticky:   make(map[string]string),
	}
}

func (s *ProxyPoolSelector) Select(poolID string, items []ProxyPoolItem, affinity string) (ProxyPoolItem, error) {
	enabled := enabledProxyItems(items)
	if len(enabled) == 0 {
		return ProxyPoolItem{}, ErrNoEnabledProxyItems
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.strategy {
	case ProxyPoolStrategyRandom:
		index := 0
		if s.rng != nil {
			index = s.rng.Intn(len(enabled))
		}
		return enabled[index], nil
	case ProxyPoolStrategySticky:
		stickyKey := poolID + ":" + affinity
		if itemID, ok := s.sticky[stickyKey]; ok {
			for _, item := range enabled {
				if item.ID == itemID {
					return item, nil
				}
			}
		}
		item := enabled[s.nextIndex(poolID, len(enabled))]
		s.sticky[stickyKey] = item.ID
		return item, nil
	case ProxyPoolStrategyFailover:
		return enabled[0], nil
	default:
		return enabled[s.nextIndex(poolID, len(enabled))], nil
	}
}

func (s *ProxyPoolSelector) nextIndex(poolID string, length int) int {
	index := s.next[poolID] % length
	s.next[poolID] = index + 1
	return index
}

func enabledProxyItems(items []ProxyPoolItem) []ProxyPoolItem {
	enabled := make([]ProxyPoolItem, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			enabled = append(enabled, item)
		}
	}
	return enabled
}
