package channel

import (
	"encoding/json"
	"gpt-load/internal/models"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type OpenAIChannel struct {
	BaseURL *url.URL
}

type OpenAIChannelConfig struct {
	BaseURL string `json:"base_url"`
}

func NewOpenAIChannel(group *models.Group) (*OpenAIChannel, error) {
	var config OpenAIChannelConfig
	if err := json.Unmarshal([]byte(group.Config), &config); err != nil {
		return nil, err
	}
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, err
	}
	return &OpenAIChannel{BaseURL: baseURL}, nil
}

func (ch *OpenAIChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) {
	proxy := httputil.NewSingleHostReverseProxy(ch.BaseURL)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = ch.BaseURL.Scheme
		req.URL.Host = ch.BaseURL.Host
		req.URL.Path = c.Param("path")
		req.Host = ch.BaseURL.Host
		req.Header.Set("Authorization", "Bearer "+apiKey.KeyValue)
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		// Log the response, etc.
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logrus.Errorf("Proxy error: %v", err)
		// Handle error, maybe update key status
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}