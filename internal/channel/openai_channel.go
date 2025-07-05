package channel

import (
	"context"
	"fmt"
	"gpt-load/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func init() {
	Register("openai", newOpenAIChannel)
}

type OpenAIChannel struct {
	*BaseChannel
}

func newOpenAIChannel(f *Factory, group *models.Group) (ChannelProxy, error) {
	base, err := f.newBaseChannel("openai", group.Upstreams, group.Config)
	if err != nil {
		return nil, err
	}

	return &OpenAIChannel{
		BaseChannel: base,
	}, nil
}

func (ch *OpenAIChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error {
	modifier := func(req *http.Request, key *models.APIKey) {
		req.Header.Set("Authorization", "Bearer "+key.KeyValue)
	}
	return ch.ProcessRequest(c, apiKey, modifier, ch)
}

// ValidateKey checks if the given API key is valid by making a request to the models endpoint.
func (ch *OpenAIChannel) ValidateKey(ctx context.Context, key string) (bool, error) {
	upstreamURL := ch.getUpstreamURL()
	if upstreamURL == nil {
		return false, fmt.Errorf("no upstream URL configured for channel %s", ch.Name)
	}

	// Construct the request URL for listing models, a common endpoint for key validation.
	reqURL := upstreamURL.String() + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := ch.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	// A 200 OK status code indicates the key is valid.
	// Other status codes (e.g., 401 Unauthorized) indicate an invalid key.
	return resp.StatusCode == http.StatusOK, nil
}

// IsStreamingRequest checks if the request is for a streaming response.
func (ch *OpenAIChannel) IsStreamingRequest(c *gin.Context) bool {
	// For OpenAI, streaming is indicated by a "stream": true field in the JSON body.
	// We use ShouldBindBodyWith to check the body without consuming it, so it can be read again by the proxy.
	type streamPayload struct {
		Stream bool `json:"stream"`
	}
	var p streamPayload
	if err := c.ShouldBindBodyWith(&p, binding.JSON); err == nil {
		return p.Stream
	}
	return false
}
