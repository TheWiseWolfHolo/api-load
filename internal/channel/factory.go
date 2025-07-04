package channel

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"net/http"
	"net/url"
	"time"

	"gorm.io/datatypes"
)

// Factory is responsible for creating channel proxies.
type Factory struct {
	settingsManager *config.SystemSettingsManager
}

// NewFactory creates a new channel factory.
func NewFactory(settingsManager *config.SystemSettingsManager) *Factory {
	return &Factory{
		settingsManager: settingsManager,
	}
}

// GetChannel returns a channel proxy based on the group's channel type.
func (f *Factory) GetChannel(group *models.Group) (ChannelProxy, error) {
	switch group.ChannelType {
	case "openai":
		return f.NewOpenAIChannel(group)
	case "gemini":
		return f.NewGeminiChannel(group)
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", group.ChannelType)
	}
}

// newBaseChannel is a helper function to create and configure a BaseChannel.
func (f *Factory) newBaseChannel(name string, upstreamsJSON datatypes.JSON, groupConfig datatypes.JSONMap) (*BaseChannel, error) {
	type upstreamDef struct {
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}

	var defs []upstreamDef
	if err := json.Unmarshal(upstreamsJSON, &defs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upstreams for %s channel: %w", name, err)
	}

	if len(defs) == 0 {
		return nil, fmt.Errorf("at least one upstream is required for %s channel", name)
	}

	var upstreamInfos []UpstreamInfo
	for _, def := range defs {
		u, err := url.Parse(def.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse upstream url '%s' for %s channel: %w", def.URL, name, err)
		}
		weight := def.Weight
		if weight <= 0 {
			weight = 1 // Default weight to 1 if not specified or invalid
		}
		upstreamInfos = append(upstreamInfos, UpstreamInfo{URL: u, Weight: weight})
	}

	// Get effective settings by merging system and group configs
	effectiveSettings := f.settingsManager.GetEffectiveConfig(groupConfig)

	// Configure the HTTP client with the effective timeouts
	httpClient := &http.Client{
		Transport: &http.Transport{
			IdleConnTimeout: time.Duration(effectiveSettings.IdleConnTimeout) * time.Second,
		},
		Timeout: time.Duration(effectiveSettings.RequestTimeout) * time.Second,
	}

	return &BaseChannel{
		Name:       name,
		Upstreams:  upstreamInfos,
		HTTPClient: httpClient,
	}, nil
}
