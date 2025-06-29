// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
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

// A list of errors that are considered normal during streaming when a client disconnects.
var ignorableStreamErrors = []string{
	"context canceled",
	"connection reset by peer",
}

// isIgnorableStreamError checks if the error is a common, non-critical error that can occur
// when a client disconnects during a streaming response.
func isIgnorableStreamError(err error) bool {
	errStr := err.Error()
	for _, ignorableError := range ignorableStreamErrors {
		if strings.Contains(errStr, ignorableError) {
			return true
		}
	}
	return false
}

// ProxyServer represents the proxy server
type ProxyServer struct {
	keyManager    types.KeyManager
	configManager types.ConfigManager
	httpClient    *http.Client
	streamClient  *http.Client // Dedicated client for streaming
	requestCount  int64
	startTime     time.Time
}

// NewProxyServer creates a new proxy server
func NewProxyServer(keyManager types.KeyManager, configManager types.ConfigManager) (*ProxyServer, error) {
	openaiConfig := configManager.GetOpenAIConfig()
	perfConfig := configManager.GetPerformanceConfig()

	// Create high-performance HTTP client
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       time.Duration(openaiConfig.IdleConnTimeout) * time.Second,
		TLSHandshakeTimeout:   time.Duration(openaiConfig.ResponseTimeout) * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    !perfConfig.EnableGzip,
		ForceAttemptHTTP2:     true,
		WriteBufferSize:       32 * 1024,
		ReadBufferSize:        32 * 1024,
	}

	// Create dedicated transport for streaming, optimize TCP parameters
	streamTransport := &http.Transport{
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   40,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       time.Duration(openaiConfig.IdleConnTimeout) * time.Second,
		TLSHandshakeTimeout:   time.Duration(openaiConfig.ResponseTimeout) * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true, // Always disable compression for streaming
		ForceAttemptHTTP2:     true,
		WriteBufferSize:       64 * 1024,
		ReadBufferSize:        64 * 1024,
		ResponseHeaderTimeout: time.Duration(openaiConfig.ResponseTimeout) * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(openaiConfig.RequestTimeout) * time.Second,
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

	if retryCount > keysConfig.MaxRetries {
		logrus.Debugf("Max retries exceeded (%d)", retryCount-1)

		errorResponse := gin.H{
			"error":        "Max retries exceeded",
			"code":         errors.ErrProxyRetryExhausted,
			"retry_count":  retryCount - 1,
			"retry_errors": retryErrors,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		}

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

	// Get a base URL from the config manager (handles round-robin)
	openaiConfig := ps.configManager.GetOpenAIConfig()
	upstreamURL, err := url.Parse(openaiConfig.BaseURL)
	if err != nil {
		logrus.Errorf("Failed to parse upstream URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid upstream URL configured",
			"code":  errors.ErrConfigInvalid,
		})
		return
	}

	// Build upstream request URL
	targetURL := *upstreamURL
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
		// Non-streaming requests use configured timeout from the already fetched config
		timeout := time.Duration(openaiConfig.RequestTimeout) * time.Second
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

	if c.GetHeader("Authorization") != "" {
		req.Header.Set("Authorization", "Bearer "+keyInfo.Key)
		req.Header.Del("X-Goog-Api-Key")
	} else if c.GetHeader("X-Goog-Api-Key") != "" || c.Query("key") != "" {
		req.Header.Set("X-Goog-Api-Key", keyInfo.Key)
		req.Header.Del("Authorization")
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "API key required. Please provide a key in 'Authorization' or 'X-Goog-Api-Key' header.",
			"code":  errors.ErrAuthMissing,
		})
		c.Abort()
		return
	}

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
			logrus.Warnf("Retry request failed (attempt %d): %v (response time: %v)", retryCount+1, err, responseTime)
		} else {
			logrus.Warnf("Initial request failed: %v (response time: %v)", err, responseTime)
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
	if resp.StatusCode >= 400 {
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

		var jsonError struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(errorMessage), &jsonError); err == nil && jsonError.Error.Message != "" {
			logrus.Warnf("Http Error: %s", jsonError.Error.Message)
		} else {
			logrus.Warnf("Http Error: %s", errorMessage)
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
		logrus.Debugf("Request succeeded after %d retries (response time: %v)", retryCount, responseTime)
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

	// Copy streaming data with optimized buffer size
	buffer := make([]byte, 32*1024) // 32KB buffer for better performance
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
				if isIgnorableStreamError(err) {
					logrus.Debugf("Stream closed by client or network: %v", err)
				} else {
					logrus.Errorf("Error reading streaming response: %v", err)
				}
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
