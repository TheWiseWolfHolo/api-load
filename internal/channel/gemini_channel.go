package channel

import (
	"fmt"
	"gpt-load/internal/models"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const GeminiBaseURL = "https://generativelanguage.googleapis.com"

type GeminiChannel struct {
	BaseURL *url.URL
}

func NewGeminiChannel(group *models.Group) (*GeminiChannel, error) {
	baseURL, err := url.Parse(GeminiBaseURL)
	if err != nil {
		return nil, err // Should not happen with a constant
	}
	return &GeminiChannel{BaseURL: baseURL}, nil
}

func (ch *GeminiChannel) Handle(c *gin.Context, apiKey *models.APIKey, group *models.Group) {
	proxy := httputil.NewSingleHostReverseProxy(ch.BaseURL)

	proxy.Director = func(req *http.Request) {
		// Gemini API key is passed as a query parameter
		originalPath := c.Param("path")
		newPath := fmt.Sprintf("%s?key=%s", originalPath, apiKey.KeyValue)

		req.URL.Scheme = ch.BaseURL.Scheme
		req.URL.Host = ch.BaseURL.Host
		req.URL.Path = newPath
		req.Host = ch.BaseURL.Host
		// Remove the Authorization header if it was passed by the client
		req.Header.Del("Authorization")
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		// Log the response, etc.
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logrus.Errorf("Proxy error to Gemini: %v", err)
		// Handle error, maybe update key status
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}