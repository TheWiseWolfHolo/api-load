package channel

import (
	"bytes"
	"encoding/json"
	"gpt-load/internal/models"
	"net/http"
	"net/url"
	"sync"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

// UpstreamInfo holds the information for a single upstream server, including its weight.
type UpstreamInfo struct {
	URL           *url.URL
	Weight        int
	CurrentWeight int
}

// BaseChannel provides common functionality for channel proxies.
type BaseChannel struct {
	Name            string
	Upstreams       []UpstreamInfo
	HTTPClient      *http.Client
	StreamClient    *http.Client
	TestModel       string
	upstreamLock    sync.Mutex
	groupUpstreams  datatypes.JSON
	groupConfig     datatypes.JSONMap
}

// getUpstreamURL selects an upstream URL using a smooth weighted round-robin algorithm.
func (b *BaseChannel) getUpstreamURL() *url.URL {
	b.upstreamLock.Lock()
	defer b.upstreamLock.Unlock()

	if len(b.Upstreams) == 0 {
		return nil
	}
	if len(b.Upstreams) == 1 {
		return b.Upstreams[0].URL
	}

	totalWeight := 0
	var best *UpstreamInfo

	for i := range b.Upstreams {
		up := &b.Upstreams[i]
		totalWeight += up.Weight
		up.CurrentWeight += up.Weight

		if best == nil || up.CurrentWeight > best.CurrentWeight {
			best = up
		}
	}

	if best == nil {
		return b.Upstreams[0].URL // 降级到第一个可用的
	}

	best.CurrentWeight -= totalWeight
	return best.URL
}

// IsConfigStale checks if the channel's configuration is stale compared to the provided group.
func (b *BaseChannel) IsConfigStale(group *models.Group) bool {
	// It's important to compare the raw JSON here to detect any changes.
	if !bytes.Equal(b.groupUpstreams, group.Upstreams) {
		return true
	}

	// For JSONMap, we need to marshal it to compare.
	currentConfigBytes, err := json.Marshal(b.groupConfig)
	if err != nil {
		// Log the error and assume it's stale to be safe
		logrus.Errorf("failed to marshal current group config: %v", err)
		return true
	}
	newConfigBytes, err := json.Marshal(group.Config)
	if err != nil {
		// Log the error and assume it's stale
		logrus.Errorf("failed to marshal new group config: %v", err)
		return true
	}

	if !bytes.Equal(currentConfigBytes, newConfigBytes) {
		return true
	}

	return false
}

// GetHTTPClient returns the client for standard requests.
func (b *BaseChannel) GetHTTPClient() *http.Client {
	return b.HTTPClient
}

// GetStreamClient returns the client for streaming requests.
func (b *BaseChannel) GetStreamClient() *http.Client {
	return b.StreamClient
}
