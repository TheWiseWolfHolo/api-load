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
	base, err := f.newBaseChannel("gemini", group)
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

// ValidateKey checks if the given API key is valid by making a generateContent request.
func (ch *GeminiChannel) ValidateKey(ctx context.Context, key string) (bool, error) {
	upstreamURL := ch.getUpstreamURL()
	if upstreamURL == nil {
		return false, fmt.Errorf("no upstream URL configured for channel %s", ch.Name)
	}

	// Use the test model specified in the group settings.
	// The path format for Gemini is /v1beta/models/{model}:generateContent
	reqURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", upstreamURL.String(), ch.TestModel, key)

	// Use a minimal, low-cost payload for validation
	payload := gin.H{
		"contents": []gin.H{
			{"parts": []gin.H{
				{"text": "Only output 'ok'"},
			}},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal validation payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return false, fmt.Errorf("failed to create validation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ch.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	// A 200 OK status code indicates the key is valid.
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
func (ch *GeminiChannel) IsStreamingRequest(c *gin.Context) bool {
	// For Gemini, streaming is indicated by the path containing streaming keywords
	path := c.Request.URL.Path
	return strings.Contains(path, ":streamGenerateContent") ||
		strings.Contains(path, "streamGenerateContent") ||
		strings.Contains(path, ":stream") ||
		strings.Contains(path, "/stream")
}
