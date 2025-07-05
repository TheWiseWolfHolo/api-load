package channel

import (
	"context"
	"gpt-load/internal/models"

	"github.com/gin-gonic/gin"
)

// ChannelProxy defines the interface for different API channel proxies.
type ChannelProxy interface {
	// Handle takes a context, an API key, and the original request,
	// then forwards the request to the upstream service.
	Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error

	// ValidateKey checks if the given API key is valid.
	ValidateKey(ctx context.Context, key string) (bool, error)

	// IsStreamingRequest checks if the request is for a streaming response.
	IsStreamingRequest(c *gin.Context) bool

	// IsConfigStale checks if the channel's configuration is stale compared to the provided group.
	IsConfigStale(group *models.Group) bool
}
