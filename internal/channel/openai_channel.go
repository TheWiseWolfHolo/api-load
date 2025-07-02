package channel

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/models"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)
type OpenAIChannel struct {
	BaseChannel
}

type OpenAIChannelConfig struct {
	BaseURL string `json:"base_url"`
}

func NewOpenAIChannel(group *models.Group) (*OpenAIChannel, error) {
	var config OpenAIChannelConfig
	if err := json.Unmarshal([]byte(group.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal channel config: %w", err)
	}
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required for openai channel")
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base_url: %w", err)
	}

	return &OpenAIChannel{
		BaseChannel: BaseChannel{
			Name:       "openai",
			BaseURL:    baseURL,
			HTTPClient: &http.Client{},
		},
	}, nil
}

func (ch *OpenAIChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) error {
	modifier := func(req *http.Request, key *models.APIKey) {
		req.Header.Set("Authorization", "Bearer "+key.KeyValue)
	}
	return ch.ProcessRequest(c, apiKey, modifier)
}