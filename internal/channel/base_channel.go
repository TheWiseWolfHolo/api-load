package channel

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RequestModifier defines a function that can modify the upstream request,
// for example, by adding authentication headers.
type RequestModifier func(req *http.Request, apiKey *models.APIKey)

// BaseChannel provides a foundation for specific channel implementations.
type BaseChannel struct {
	Name       string
	BaseURL    *url.URL
	HTTPClient *http.Client
}

// ProcessRequest handles the generic logic of creating, sending, and handling an upstream request.
func (ch *BaseChannel) ProcessRequest(c *gin.Context, apiKey *models.APIKey, modifier RequestModifier) error {
	// 1. Create the upstream request
	req, err := ch.createUpstreamRequest(c, apiKey, modifier)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create upstream request")
		return fmt.Errorf("create upstream request failed: %w", err)
	}

	// 2. Send the request
	resp, err := ch.HTTPClient.Do(req)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Upstream service unavailable")
		return fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	// 3. Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		errorMsg := ch.getErrorMessage(resp)
		response.Error(c, resp.StatusCode, errorMsg)
		return fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// 4. Stream the successful response back to the client
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Status(http.StatusOK)
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		logrus.Errorf("Failed to copy response body to client: %v", err)
		return fmt.Errorf("copy response body failed: %w", err)
	}

	return nil
}

func (ch *BaseChannel) createUpstreamRequest(c *gin.Context, apiKey *models.APIKey, modifier RequestModifier) (*http.Request, error) {
	targetURL := *ch.BaseURL
	targetURL.Path = c.Param("path")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}

	req.Header = c.Request.Header.Clone()
	req.Host = ch.BaseURL.Host

	// Apply the channel-specific modifications
	if modifier != nil {
		modifier(req, apiKey)
	}

	return req, nil
}

func (ch *BaseChannel) getErrorMessage(resp *http.Response) string {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("HTTP %d (failed to read error body: %v)", resp.StatusCode, err)
	}

	var errorMessage string
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, gErr := gzip.NewReader(bytes.NewReader(bodyBytes))
		if gErr != nil {
			return string(bodyBytes)
		}
		defer reader.Close()
		uncompressedBytes, rErr := io.ReadAll(reader)
		if rErr != nil {
			return fmt.Sprintf("gzip read error: %v", rErr)
		}
		errorMessage = string(uncompressedBytes)
	} else {
		errorMessage = string(bodyBytes)
	}

	if strings.TrimSpace(errorMessage) == "" {
		return fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return errorMessage
}