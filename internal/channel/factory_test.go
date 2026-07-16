package channel

import (
	"api-load/internal/config"
	"api-load/internal/httpclient"
	"api-load/internal/models"
	"strings"
	"testing"

	"gorm.io/datatypes"
)

func TestFactoryAllowsEmptyUpstreamsOnlyForPoolBoundGroups(t *testing.T) {
	newFactory := func() *Factory {
		return NewFactory(config.NewSystemSettingsManager(), httpclient.NewHTTPClientManager())
	}

	poolID := uint(7)
	poolBound := &models.Group{
		ID:             1,
		Name:           "fireworks",
		ChannelType:    "openai",
		ResourcePoolID: &poolID,
		Upstreams:      datatypes.JSON(`[]`),
	}
	if _, err := newFactory().GetChannel(poolBound); err != nil {
		t.Fatalf("pool-bound channel should not require a dormant legacy upstream: %v", err)
	}

	unbound := &models.Group{
		ID:          2,
		Name:        "legacy-empty",
		ChannelType: "openai",
		Upstreams:   datatypes.JSON(`[]`),
	}
	if _, err := newFactory().GetChannel(unbound); err == nil || !strings.Contains(err.Error(), "at least one upstream is required") {
		t.Fatalf("unbound channel should still reject empty upstreams, got: %v", err)
	}
}
