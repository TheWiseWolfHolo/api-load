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

// channelConstructor defines the function signature for creating a new channel proxy.
type channelConstructor func(f *Factory, group *models.Group) (ChannelProxy, error)

var (
	// channelRegistry holds the mapping from channel type string to its constructor.
	channelRegistry = make(map[string]channelConstructor)
)

// Register adds a new channel constructor to the registry.
// This function is intended to be called from the init() function of each channel implementation.
func Register(channelType string, constructor channelConstructor) {
	if _, exists := channelRegistry[channelType]; exists {
		panic(fmt.Sprintf("channel type '%s' is already registered", channelType))
	}
	channelRegistry[channelType] = constructor
}

// GetChannels returns a slice of all registered channel type names.
func GetChannels() []string {
	supportedTypes := make([]string, 0, len(channelRegistry))
	for t := range channelRegistry {
		supportedTypes = append(supportedTypes, t)
	}
	return supportedTypes
}

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
	constructor, ok := channelRegistry[group.ChannelType]
	if !ok {
		return nil, fmt.Errorf("unsupported channel type: %s", group.ChannelType)
	}
	return constructor(f, group)
}

// newBaseChannel is a helper function to create and configure a BaseChannel.
func (f *Factory) newBaseChannel(name string, upstreamsJSON datatypes.JSON, groupConfig datatypes.JSONMap, testModel string) (*BaseChannel, error) {
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
			weight = 1
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
		TestModel:  testModel,
	}, nil
}
