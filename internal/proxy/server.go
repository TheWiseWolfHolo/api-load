// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/services"
	"gpt-load/internal/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// A list of errors that are considered normal during streaming when a client disconnects.
var ignorableStreamErrors = []string{
	"context canceled",
	"connection reset by peer",
	"broken pipe",
	"use of closed network connection",
}

// isIgnorableStreamError checks if the error is a common, non-critical error that can occur
// when a client disconnects during a streaming response.
func isIgnorableStreamError(err error) bool {
	if err == nil {
		return false
	}
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
	keyProvider     *keypool.KeyProvider
	groupManager    *services.GroupManager
	settingsManager *config.SystemSettingsManager
	channelFactory  *channel.Factory
}

// NewProxyServer creates a new proxy server
func NewProxyServer(
	keyProvider *keypool.KeyProvider,
	groupManager *services.GroupManager,
	settingsManager *config.SystemSettingsManager,
	channelFactory *channel.Factory,
) (*ProxyServer, error) {
	return &ProxyServer{
		keyProvider:     keyProvider,
		groupManager:    groupManager,
		settingsManager: settingsManager,
		channelFactory:  channelFactory,
	}, nil
}

// HandleProxy is the main entry point for proxy requests, refactored based on the stable .bak logic.
func (ps *ProxyServer) HandleProxy(c *gin.Context) {
	startTime := time.Now()
	groupName := c.Param("group_name")

	group, err := ps.groupManager.GetGroupByName(groupName)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	channelHandler, err := ps.channelFactory.GetChannel(group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get channel for group '%s': %v", groupName, err)))
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logrus.Errorf("Failed to read request body: %v", err)
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Failed to read request body"))
		return
	}
	c.Request.Body.Close()

	// 4. Apply parameter overrides if any.
	finalBodyBytes, err := ps.applyParamOverrides(bodyBytes, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to apply parameter overrides: %v", err)))
		return
	}

	// 5. Determine if this is a streaming request.
	isStream := channelHandler.IsStreamRequest(c, bodyBytes)

	// 6. Execute the request using the recursive retry logic.
	ps.executeRequestWithRetry(c, channelHandler, group, finalBodyBytes, isStream, startTime, 0, nil)
}

// executeRequestWithRetry is the core recursive function for handling requests and retries.
func (ps *ProxyServer) executeRequestWithRetry(
	c *gin.Context,
	channelHandler channel.ChannelProxy,
	group *models.Group,
	bodyBytes []byte,
	isStream bool,
	startTime time.Time,
	retryCount int,
	retryErrors []types.RetryError,
) {
	cfg := group.EffectiveConfig
	if retryCount > cfg.MaxRetries {
		logrus.Errorf("Max retries exceeded for group %s after %d attempts.", group.Name, retryCount)
		if len(retryErrors) > 0 {
			lastError := retryErrors[len(retryErrors)-1]
			var errorJSON map[string]any
			if err := json.Unmarshal([]byte(lastError.ErrorMessage), &errorJSON); err == nil {
				c.JSON(lastError.StatusCode, errorJSON)
			} else {
				response.Error(c, app_errors.NewAPIErrorWithUpstream(lastError.StatusCode, "UPSTREAM_ERROR", lastError.ErrorMessage))
			}
		} else {
			response.Error(c, app_errors.ErrMaxRetriesExceeded)
		}
		return
	}

	apiKey, err := ps.keyProvider.SelectKey(group.ID)
	if err != nil {
		logrus.Errorf("Failed to select a key for group %s on attempt %d: %v", group.Name, retryCount+1, err)
		response.Error(c, app_errors.NewAPIError(app_errors.ErrNoKeysAvailable, err.Error()))
		return
	}

	upstreamURL, err := channelHandler.BuildUpstreamURL(c.Request.URL, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to build upstream URL: %v", err)))
		return
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if isStream {
		ctx, cancel = context.WithCancel(c.Request.Context())
	} else {
		timeout := time.Duration(cfg.RequestTimeout) * time.Second
		ctx, cancel = context.WithTimeout(c.Request.Context(), timeout)
	}
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, c.Request.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		logrus.Errorf("Failed to create upstream request: %v", err)
		response.Error(c, app_errors.ErrInternalServer)
		return
	}
	req.ContentLength = int64(len(bodyBytes))

	req.Header = c.Request.Header.Clone()
	channelHandler.ModifyRequest(req, apiKey, group)

	client := channelHandler.GetHTTPClient()
	if isStream {
		client = channelHandler.GetStreamClient()
		req.Header.Set("X-Accel-Buffering", "no")
	}

	resp, err := client.Do(req)
	if err != nil {
		ps.keyProvider.UpdateStatus(apiKey.ID, group.ID, false)
		logrus.Warnf("Request failed (attempt %d/%d) for key %s: %v", retryCount+1, cfg.MaxRetries, apiKey.KeyValue[:8], err)

		newRetryErrors := append(retryErrors, types.RetryError{
			StatusCode:   0,
			ErrorMessage: err.Error(),
			KeyID:        fmt.Sprintf("%d", apiKey.ID),
			Attempt:      retryCount + 1,
		})
		ps.executeRequestWithRetry(c, channelHandler, group, bodyBytes, isStream, startTime, retryCount+1, newRetryErrors)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		ps.keyProvider.UpdateStatus(apiKey.ID, group.ID, false)
		errorBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			logrus.Errorf("Failed to read error body: %v", readErr)
			// Even if reading fails, we should proceed with retry logic
			errorBody = []byte("Failed to read error body")
		}

		// Check for gzip encoding and decompress if necessary.
		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(errorBody))
			if err == nil {
				decompressedBody, err := io.ReadAll(reader)
				if err == nil {
					errorBody = decompressedBody
				} else {
					logrus.Warnf("Failed to decompress gzip error body: %v", err)
				}
				reader.Close()
			} else {
				logrus.Warnf("Failed to create gzip reader for error body: %v", err)
			}
		}

		logrus.Warnf("Request failed with status %d (attempt %d/%d) for key %s. Body: %s", resp.StatusCode, retryCount+1, cfg.MaxRetries, apiKey.KeyValue[:8], string(errorBody))

		newRetryErrors := append(retryErrors, types.RetryError{
			StatusCode:   resp.StatusCode,
			ErrorMessage: string(errorBody),
			KeyID:        fmt.Sprintf("%d", apiKey.ID),
			Attempt:      retryCount + 1,
		})
		ps.executeRequestWithRetry(c, channelHandler, group, bodyBytes, isStream, startTime, retryCount+1, newRetryErrors)
		return
	}

	ps.keyProvider.UpdateStatus(apiKey.ID, group.ID, true)
	logrus.Debugf("Request for group %s succeeded on attempt %d with key %s", group.Name, retryCount+1, apiKey.KeyValue[:8])

	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Status(resp.StatusCode)

	if isStream {
		ps.handleStreamingResponse(c, resp)
	} else {
		ps.handleNormalResponse(c, resp)
	}
}

func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported by the writer")
		ps.handleNormalResponse(c, resp)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if _, err := c.Writer.Write(scanner.Bytes()); err != nil {
			if !isIgnorableStreamError(err) {
				logrus.Errorf("Error writing to client: %v", err)
			}
			return
		}
		if _, err := c.Writer.Write([]byte("\n")); err != nil {
			if !isIgnorableStreamError(err) {
				logrus.Errorf("Error writing newline to client: %v", err)
			}
			return
		}
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil && !isIgnorableStreamError(err) {
		logrus.Errorf("Error reading from upstream: %v", err)
	}
}

func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response) {
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		if !isIgnorableStreamError(err) {
			logrus.Errorf("Failed to copy response body to client: %v", err)
		}
	}
}

func (ps *ProxyServer) applyParamOverrides(bodyBytes []byte, group *models.Group) ([]byte, error) {
	if len(group.ParamOverrides) == 0 || len(bodyBytes) == 0 {
		return bodyBytes, nil
	}

	var requestData map[string]any
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		logrus.Warnf("failed to unmarshal request body for param override, passing through: %v", err)
		return bodyBytes, nil
	}

	for key, value := range group.ParamOverrides {
		requestData[key] = value
	}

	return json.Marshal(requestData)
}

func (ps *ProxyServer) Close() {
	// The HTTP clients are now managed by the channel factory and httpclient manager,
	// so the proxy server itself doesn't need to close them.
	// The httpclient manager will handle closing idle connections for all its clients.
}
