package channel

import (
	"gpt-load/internal/models"

	"github.com/gin-gonic/gin"
)

// ChannelProxy defines the interface for different API channel proxies.
type ChannelProxy interface {
	// Handle takes a context, an API key, and the original request,
	// then forwards the request to the upstream service.
	Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error
}