package channel

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/models"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)
type GeminiChannel struct {
	BaseChannel
}

type GeminiChannelConfig struct {
	BaseURL string `json:"base_url"`
}

func NewGeminiChannel(group *models.Group) (*GeminiChannel, error) {
	var config GeminiChannelConfig
	if err := json.Unmarshal([]byte(group.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal channel config: %w", err)
	}
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required for gemini channel")
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base_url: %w", err)
	}

	return &GeminiChannel{
		BaseChannel: BaseChannel{
			Name:       "gemini",
			BaseURL:    baseURL,
			HTTPClient: &http.Client{},
		},
	}, nil
}

func (ch *GeminiChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error {
	modifier := func(req *http.Request, key *models.APIKey) {
		q := req.URL.Query()
		q.Set("key", key.KeyValue)
		req.URL.RawQuery = q.Encode()
	}
	return ch.ProcessRequest(c, apiKey, modifier)
}