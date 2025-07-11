package channel

import (
	"context"
	"gpt-load/internal/models"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// ChannelProxy defines the interface for different API channel proxies.
// It's responsible for channel-specific logic like URL building and request modification.
type ChannelProxy interface {
	// BuildUpstreamURL constructs the target URL for the upstream service.
	BuildUpstreamURL(originalURL *url.URL, group *models.Group) (string, error)

	// ModifyRequest allows the channel to add specific headers or modify the request
	// before it's sent to the upstream service.
	ModifyRequest(req *http.Request, apiKey *models.APIKey, group *models.Group)

	// IsStreamRequest checks if the request is for a streaming response,
	// now using the cached request body to avoid re-reading the stream.
	IsStreamRequest(c *gin.Context, bodyBytes []byte) bool

	// ValidateKey checks if the given API key is valid.
	ValidateKey(ctx context.Context, key string) (bool, error)

	// IsConfigStale checks if the channel's configuration is stale compared to the provided group.
	IsConfigStale(group *models.Group) bool

	// GetHTTPClient returns the client for standard requests.
	GetHTTPClient() *http.Client

	// GetStreamClient returns the client for streaming requests.
	GetStreamClient() *http.Client
}
