package channel

import (
	"context"
	"fmt"
	"gpt-load/internal/models"
	"net/http"

	"strings"

	"github.com/gin-gonic/gin"
)

func init() {
	Register("gemini", newGeminiChannel)
}

type GeminiChannel struct {
	*BaseChannel
}

func newGeminiChannel(f *Factory, group *models.Group) (ChannelProxy, error) {
	base, err := f.newBaseChannel("gemini", group.Upstreams, group.Config)
	if err != nil {
		return nil, err
	}

	return &GeminiChannel{
		BaseChannel: base,
	}, nil
}

func (ch *GeminiChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error {
	modifier := func(req *http.Request, key *models.APIKey) {
		q := req.URL.Query()
		q.Set("key", key.KeyValue)
		req.URL.RawQuery = q.Encode()
	}
	return ch.ProcessRequest(c, apiKey, modifier, ch)
}

// ValidateKey checks if the given API key is valid by making a request to the models endpoint.
func (ch *GeminiChannel) ValidateKey(ctx context.Context, key string) (bool, error) {
	upstreamURL := ch.getUpstreamURL()
	if upstreamURL == nil {
		return false, fmt.Errorf("no upstream URL configured for channel %s", ch.Name)
	}

	// Construct the request URL for listing models.
	reqURL := fmt.Sprintf("%s/v1beta/models?key=%s", upstreamURL.String(), key)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create validation request: %w", err)
	}

	resp, err := ch.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	// A 200 OK status code indicates the key is valid.
	return resp.StatusCode == http.StatusOK, nil
}

// IsStreamingRequest checks if the request is for a streaming response.
func (ch *GeminiChannel) IsStreamingRequest(c *gin.Context) bool {
	// For Gemini, streaming is indicated by the path containing streaming keywords
	path := c.Request.URL.Path
	return strings.Contains(path, ":streamGenerateContent") ||
		strings.Contains(path, "streamGenerateContent") ||
		strings.Contains(path, ":stream") ||
		strings.Contains(path, "/stream")
}
