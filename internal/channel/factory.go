package channel

import (
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"net/http"
	"net/url"
	"time"

	"gorm.io/datatypes"
)

// GetChannel returns a channel proxy based on the group's channel type.
func GetChannel(group *models.Group) (ChannelProxy, error) {
	switch group.ChannelType {
	case "openai":
		return NewOpenAIChannel(group.Upstreams, group.Config)
	case "gemini":
		return NewGeminiChannel(group.Upstreams, group.Config)
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", group.ChannelType)
	}
}

// newBaseChannelWithUpstreams is a helper function to create and configure a BaseChannel.
func newBaseChannelWithUpstreams(name string, upstreams []string, groupConfig datatypes.JSONMap) (BaseChannel, error) {
	if len(upstreams) == 0 {
		return BaseChannel{}, fmt.Errorf("at least one upstream is required for %s channel", name)
	}

	var upstreamURLs []*url.URL
	for _, us := range upstreams {
		u, err := url.Parse(us)
		if err != nil {
			return BaseChannel{}, fmt.Errorf("failed to parse upstream url '%s' for %s channel: %w", us, name, err)
		}
		upstreamURLs = append(upstreamURLs, u)
	}

	// Get effective settings by merging system and group configs
	settingsManager := config.GetSystemSettingsManager()
	effectiveSettings := settingsManager.GetEffectiveConfig(groupConfig)

	// Configure the HTTP client with the effective timeouts
	httpClient := &http.Client{
		Transport: &http.Transport{
			IdleConnTimeout: time.Duration(effectiveSettings.IdleConnTimeout) * time.Second,
		},
		Timeout: time.Duration(effectiveSettings.RequestTimeout) * time.Second,
	}

	return BaseChannel{
		Name:       name,
		Upstreams:  upstreamURLs,
		HTTPClient: httpClient,
	}, nil
}
