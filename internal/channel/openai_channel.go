package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"io"
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
	base, err := f.newBaseChannel("openai", group.Upstreams, group.Config, group.TestModel)
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

// ValidateKey checks if the given API key is valid by making a chat completion request.
func (ch *OpenAIChannel) ValidateKey(ctx context.Context, key string) (bool, error) {
	upstreamURL := ch.getUpstreamURL()
	if upstreamURL == nil {
		return false, fmt.Errorf("no upstream URL configured for channel %s", ch.Name)
	}

	reqURL := upstreamURL.String() + "/v1/chat/completions"

	// Use a minimal, low-cost payload for validation
	payload := gin.H{
		"model": ch.TestModel,
		"messages": []gin.H{
			{"role": "user", "content": "Only output 'ok'"},
		},
		"max_tokens": 1,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal validation payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return false, fmt.Errorf("failed to create validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ch.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	// A 200 OK status code indicates the key is valid and can make requests.
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	// For non-200 responses, parse the body to provide a more specific error reason.
	errorBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("key is invalid (status %d), but failed to read error body: %w", resp.StatusCode, err)
	}

	// Use the new parser to extract a clean error message.
	parsedError := app_errors.ParseUpstreamError(errorBody)

	return false, fmt.Errorf("[status %d] %s", resp.StatusCode, parsedError)
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
