// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"gpt-load/internal/errors"
	"gpt-load/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ProxyServer represents the proxy server
type ProxyServer struct {
	keyManager    types.KeyManager
	configManager types.ConfigManager
	httpClient    *http.Client
	streamClient  *http.Client // Dedicated client for streaming
	upstreamURL   *url.URL
	requestCount  int64
	startTime     time.Time
}

// NewProxyServer creates a new proxy server
func NewProxyServer(keyManager types.KeyManager, configManager types.ConfigManager) (*ProxyServer, error) {
	openaiConfig := configManager.GetOpenAIConfig()
	perfConfig := configManager.GetPerformanceConfig()

	// Parse upstream URL
	upstreamURL, err := url.Parse(openaiConfig.BaseURL)
	if err != nil {
		return nil, errors.NewAppErrorWithCause(errors.ErrConfigInvalid, "Failed to parse upstream URL", err)
	}

	// Create high-performance HTTP client
	transport := &http.Transport{
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // No limit to avoid connection pool bottleneck
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    !perfConfig.EnableGzip,
		ForceAttemptHTTP2:     true,
		WriteBufferSize:       32 * 1024,
		ReadBufferSize:        32 * 1024,
	}

	// Create dedicated transport for streaming, optimize TCP parameters
	streamTransport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       300 * time.Second, // Keep streaming connections longer
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true, // Always disable compression for streaming
		ForceAttemptHTTP2:     true,
		WriteBufferSize:       64 * 1024,
		ReadBufferSize:        64 * 1024,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(openaiConfig.Timeout) * time.Millisecond,
	}

	// Streaming client without overall timeout
	streamClient := &http.Client{
		Transport: streamTransport,
	}

	return &ProxyServer{
		keyManager:    keyManager,
		configManager: configManager,
		httpClient:    httpClient,
		streamClient:  streamClient,
		upstreamURL:   upstreamURL,
		startTime:     time.Now(),
	}, nil
}

// HandleProxy handles proxy requests
func (ps *ProxyServer) HandleProxy(c *gin.Context) {
	startTime := time.Now()

	// Increment request count
	atomic.AddInt64(&ps.requestCount, 1)

	// Cache all request body upfront
	var bodyBytes []byte
	if c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to read request body",
				"code":  errors.ErrProxyRequest,
			})
			return
		}
	}

	// Determine if this is a streaming request using cached data
	isStreamRequest := ps.isStreamRequest(bodyBytes, c)

	// Execute request with retry
	ps.executeRequestWithRetry(c, startTime, bodyBytes, isStreamRequest, 0, nil)
}

// isStreamRequest determines if this is a streaming request
func (ps *ProxyServer) isStreamRequest(bodyBytes []byte, c *gin.Context) bool {
	// Check Accept header
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		return true
	}

	// Check URL query parameter
	if c.Query("stream") == "true" {
		return true
	}

	// Check stream parameter in request body
	if len(bodyBytes) > 0 {
		if strings.Contains(string(bodyBytes), `"stream":true`) ||
			strings.Contains(string(bodyBytes), `"stream": true`) {
			return true
		}
	}

	return false
}

// executeRequestWithRetry executes request with retry logic
func (ps *ProxyServer) executeRequestWithRetry(c *gin.Context, startTime time.Time, bodyBytes []byte, isStreamRequest bool, retryCount int, retryErrors []types.RetryError) {
	keysConfig := ps.configManager.GetKeysConfig()

	// Check retry limit
	if retryCount >= keysConfig.MaxRetries {
		logrus.Errorf("Max retries exceeded (%d)", retryCount)

		// Return detailed error information
		errorResponse := gin.H{
			"error":        "Max retries exceeded",
			"code":         errors.ErrProxyRetryExhausted,
			"retry_count":  retryCount,
			"retry_errors": retryErrors,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		}

		// Use the last error's status code if available
		statusCode := http.StatusBadGateway
		if len(retryErrors) > 0 && retryErrors[len(retryErrors)-1].StatusCode > 0 {
			statusCode = retryErrors[len(retryErrors)-1].StatusCode
		}

		c.JSON(statusCode, errorResponse)
		return
	}

	// Get key information
	keyInfo, err := ps.keyManager.GetNextKey()
	if err != nil {
		logrus.Errorf("Failed to get key: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "No API keys available",
			"code":  errors.ErrNoKeysAvailable,
		})
		return
	}

	// Set key information to context (for logging)
	c.Set("keyIndex", keyInfo.Index)
	c.Set("keyPreview", keyInfo.Preview)

	// Set retry information to context
	if retryCount > 0 {
		c.Set("retryCount", retryCount)
	}

	// Build upstream request URL
	targetURL := *ps.upstreamURL
	// Correctly append path instead of replacing it
	if strings.HasSuffix(targetURL.Path, "/") {
		targetURL.Path = targetURL.Path + strings.TrimPrefix(c.Request.URL.Path, "/")
	} else {
		targetURL.Path = targetURL.Path + c.Request.URL.Path
	}
	targetURL.RawQuery = c.Request.URL.RawQuery

	// Use different timeout strategies for streaming and non-streaming requests
	var ctx context.Context
	var cancel context.CancelFunc

	if isStreamRequest {
		// Streaming requests only set response header timeout, no overall timeout
		ctx, cancel = context.WithCancel(c.Request.Context())
	} else {
		// Non-streaming requests use configured timeout
		openaiConfig := ps.configManager.GetOpenAIConfig()
		timeout := time.Duration(openaiConfig.Timeout) * time.Millisecond
		ctx, cancel = context.WithTimeout(c.Request.Context(), timeout)
	}
	defer cancel()

	// Create request using cached bodyBytes
	req, err := http.NewRequestWithContext(
		ctx,
		c.Request.Method,
		targetURL.String(),
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		logrus.Errorf("Failed to create upstream request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create upstream request",
			"code":  errors.ErrProxyRequest,
		})
		return
	}
	req.ContentLength = int64(len(bodyBytes))

	// Copy request headers
	for key, values := range c.Request.Header {
		if key != "Host" {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+keyInfo.Key)

	// Choose appropriate client based on request type
	var client *http.Client
	if isStreamRequest {
		client = ps.streamClient
		// Add header to disable nginx buffering
		req.Header.Set("X-Accel-Buffering", "no")
	} else {
		client = ps.httpClient
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		responseTime := time.Since(startTime)

		// Log failure
		if retryCount > 0 {
			logrus.Debugf("Retry request failed (attempt %d): %v (response time: %v)", retryCount+1, err, responseTime)
		} else {
			logrus.Debugf("Initial request failed: %v (response time: %v)", err, responseTime)
		}

		// Record failure asynchronously
		go ps.keyManager.RecordFailure(keyInfo.Key, err)

		// Record retry error information
		if retryErrors == nil {
			retryErrors = make([]types.RetryError, 0)
		}
		retryErrors = append(retryErrors, types.RetryError{
			StatusCode:   0, // Network error, no HTTP status code
			ErrorMessage: err.Error(),
			KeyIndex:     keyInfo.Index,
			Attempt:      retryCount + 1,
		})

		// Retry
		ps.executeRequestWithRetry(c, startTime, bodyBytes, isStreamRequest, retryCount+1, retryErrors)
		return
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	// Check if HTTP status code requires retry
	// 429 (Too Many Requests) and 5xx server errors need retry
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		// Log failure
		if retryCount > 0 {
			logrus.Debugf("Retry request returned error %d (attempt %d) (response time: %v)", resp.StatusCode, retryCount+1, responseTime)
		} else {
			logrus.Debugf("Initial request returned error %d (response time: %v)", resp.StatusCode, responseTime)
		}

		// Read response body to get error information
		var errorMessage string
		if bodyBytes, err := io.ReadAll(resp.Body); err == nil {
			errorMessage = string(bodyBytes)
		} else {
			errorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}

		// Record failure asynchronously
		go ps.keyManager.RecordFailure(keyInfo.Key, fmt.Errorf("HTTP %d", resp.StatusCode))

		// Record retry error information
		if retryErrors == nil {
			retryErrors = make([]types.RetryError, 0)
		}
		retryErrors = append(retryErrors, types.RetryError{
			StatusCode:   resp.StatusCode,
			ErrorMessage: errorMessage,
			KeyIndex:     keyInfo.Index,
			Attempt:      retryCount + 1,
		})

		// Retry
		ps.executeRequestWithRetry(c, startTime, bodyBytes, isStreamRequest, retryCount+1, retryErrors)
		return
	}

	// Success - record success asynchronously
	go ps.keyManager.RecordSuccess(keyInfo.Key)

	// Log final success result
	if retryCount > 0 {
		logrus.Infof("Request succeeded after %d retries (response time: %v)", retryCount, responseTime)
	} else {
		logrus.Debugf("Request succeeded on first attempt (response time: %v)", responseTime)
	}

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Set status code
	c.Status(resp.StatusCode)

	// Handle streaming and non-streaming responses
	if isStreamRequest {
		ps.handleStreamingResponse(c, resp)
	} else {
		ps.handleNormalResponse(c, resp)
	}
}

// handleStreamingResponse handles streaming responses
func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response) {
	// Set headers for streaming
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Stream response directly
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Streaming unsupported",
			"code":  errors.ErrServerInternal,
		})
		return
	}

	// Copy streaming data
	buffer := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
				logrus.Errorf("Failed to write streaming data: %v", writeErr)
				break
			}
			flusher.Flush()
		}
		if err != nil {
			if err != io.EOF {
				logrus.Errorf("Error reading streaming response: %v", err)
			}
			break
		}
	}
}

// handleNormalResponse handles normal responses
func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response) {
	// Copy response body
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		logrus.Errorf("Failed to copy response body: %v", err)
	}
}

// Close closes the proxy server and cleans up resources
func (ps *ProxyServer) Close() {
	// Close HTTP clients if needed
	if ps.httpClient != nil {
		ps.httpClient.CloseIdleConnections()
	}
	if ps.streamClient != nil {
		ps.streamClient.CloseIdleConnections()
	}
}
