// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"api-load/internal/channel"
	"api-load/internal/config"
	"api-load/internal/encryption"
	app_errors "api-load/internal/errors"
	"api-load/internal/keypool"
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"api-load/internal/response"
	"api-load/internal/services"
	"api-load/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ProxyServer represents the proxy server
type ProxyServer struct {
	keyProvider       *keypool.KeyProvider
	resourceProvider  *resourcepool.Provider
	groupManager      *services.GroupManager
	subGroupManager   *services.SubGroupManager
	settingsManager   *config.SystemSettingsManager
	channelFactory    *channel.Factory
	requestLogService *services.RequestLogService
	encryptionSvc     encryption.Service
}

// NewProxyServer creates a new proxy server
func NewProxyServer(
	keyProvider *keypool.KeyProvider,
	resourceProvider *resourcepool.Provider,
	groupManager *services.GroupManager,
	subGroupManager *services.SubGroupManager,
	settingsManager *config.SystemSettingsManager,
	channelFactory *channel.Factory,
	requestLogService *services.RequestLogService,
	encryptionSvc encryption.Service,
) (*ProxyServer, error) {
	return &ProxyServer{
		keyProvider:       keyProvider,
		resourceProvider:  resourceProvider,
		groupManager:      groupManager,
		subGroupManager:   subGroupManager,
		settingsManager:   settingsManager,
		channelFactory:    channelFactory,
		requestLogService: requestLogService,
		encryptionSvc:     encryptionSvc,
	}, nil
}

// HandleProxy is the main entry point for proxy requests, refactored based on the stable .bak logic.
func (ps *ProxyServer) HandleProxy(c *gin.Context) {
	startTime := time.Now()
	groupName := c.Param("group_name")

	originalGroup, err := ps.groupManager.GetGroupByName(groupName)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Select sub-group if this is an aggregate group
	subGroupName, err := ps.subGroupManager.SelectSubGroup(originalGroup)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"aggregate_group": originalGroup.Name,
			"error":           err,
		}).Error("Failed to select sub-group from aggregate")
		response.Error(c, app_errors.NewAPIError(app_errors.ErrNoKeysAvailable, "No available sub-groups"))
		return
	}

	group := originalGroup
	if subGroupName != "" {
		group, err = ps.groupManager.GetGroupByName(subGroupName)
		if err != nil {
			response.Error(c, app_errors.ParseDBError(err))
			return
		}
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

	finalBodyBytes, err := ps.applyParamOverrides(bodyBytes, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to apply parameter overrides: %v", err)))
		return
	}

	isStream := channelHandler.IsStreamRequest(c, bodyBytes)
	objectRouting, objectRoutingErr := ps.resolveUpstreamObjectRouting(c, originalGroup, group, finalBodyBytes)
	if objectRoutingErr != nil {
		response.Error(c, objectRoutingErr)
		return
	}
	c.Set(upstreamObjectRoutingContextKey, objectRouting)
	affinity := deriveRequestAffinity(c, finalBodyBytes, extractProxyKeyForAffinity(c), ps.encryptionSvc)
	c.Set(requestAffinityContextKey, affinity)
	if affinity.Source != "" {
		c.Header("X-Api-Load-Affinity-Status", affinity.Source)
	} else {
		c.Header("X-Api-Load-Affinity-Status", "missing")
	}

	ps.executeRequestWithRetry(c, channelHandler, originalGroup, group, finalBodyBytes, isStream, startTime, 0, nil)
}

// executeRequestWithRetry is the core recursive function for handling requests and retries.
func (ps *ProxyServer) executeRequestWithRetry(
	c *gin.Context,
	channelHandler channel.ChannelProxy,
	originalGroup *models.Group,
	group *models.Group,
	bodyBytes []byte,
	isStream bool,
	startTime time.Time,
	retryCount int,
	excludedKeyIDs []uint,
) {
	cfg := group.EffectiveConfig

	selectionReq := keypool.SelectionRequest{
		Model:         channelHandler.ExtractModel(c, bodyBytes),
		ProxyKey:      extractProxyKeyForAffinity(c),
		ExcludeKeyIDs: excludedKeyIDs,
	}
	var selectedResource *models.UpstreamResource
	var selectedEndpoint *models.ResourcePoolEndpoint
	var selectedPoolConfig resourcepool.PoolConfig
	var apiKey *models.APIKey
	var err error
	if group.ResourcePoolID != nil && *group.ResourcePoolID > 0 {
		c.Set(upstreamResourceIDContextKey, uint(0))
		affinity, _ := c.Get(requestAffinityContextKey)
		affinityInfo, _ := affinity.(requestAffinity)
		selectedPoolConfig, err = ps.resourceProvider.GetPoolConfig(*group.ResourcePoolID)
		objectRouting := objectRoutingFromContext(c)
		endpointID := uint(0)
		if group.ResourceEndpointID != nil {
			endpointID = *group.ResourceEndpointID
		}
		if objectRouting.ForcedEndpointID > 0 {
			endpointID = objectRouting.ForcedEndpointID
		}
		if err == nil {
			selectedEndpoint, err = ps.resourceProvider.ResolveEndpoint(*group.ResourcePoolID, endpointID, group.ChannelType)
		}
		if err == nil && objectRouting.ForcedResourceID > 0 {
			selectedResource, err = ps.resourceProvider.SelectBoundResource(
				objectRouting.ForcedPoolID,
				objectRouting.ForcedResourceID,
				group.ChannelType,
			)
		} else if err == nil {
			selectedResource, err = ps.resourceProvider.SelectResource(*group.ResourcePoolID, resourcepool.SelectionRequest{
				Route:              group.ChannelType,
				Affinity:           affinityInfo.Hash,
				ExcludeResourceIDs: excludedKeyIDs,
				AffinityTTL:        selectedPoolConfig.AffinityTTL,
			})
		}
		if err == nil {
			apiKey = &models.APIKey{
				ID:       selectedResource.ID,
				GroupID:  group.ID,
				KeyValue: selectedResource.KeyValue,
				KeyHash:  selectedResource.KeyHash,
				Status:   selectedResource.Status,
			}
			c.Set(upstreamResourceIDContextKey, selectedResource.ID)
		}
	} else {
		apiKey, err = ps.keyProvider.SelectKeyForRequest(group, selectionReq)
	}
	if err != nil {
		logrus.Errorf("Failed to select a key for group %s on attempt %d: %v", group.Name, retryCount+1, err)
		message := err.Error()
		if objectRoutingFromContext(c).ForcedResourceID > 0 {
			message = "the physical resource that owns this upstream object is unavailable; cross-key migration was refused"
		}
		response.Error(c, app_errors.NewAPIError(app_errors.ErrNoKeysAvailable, message))
		ps.logRequest(c, originalGroup, group, nil, startTime, http.StatusServiceUnavailable, err, isStream, "", channelHandler, bodyBytes, models.RequestTypeFinal)
		return
	}

	var upstreamURL string
	if selectedResource != nil {
		upstreamURL, err = channelHandler.BuildUpstreamURLWithBase(c.Request.URL, originalGroup.Name, selectedEndpoint.BaseURL)
	} else {
		upstreamURL, err = channelHandler.BuildUpstreamURL(c.Request.URL, originalGroup.Name)
	}
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

	// Clean up client auth key
	req.Header.Del("Authorization")
	req.Header.Del("X-Api-Key")
	req.Header.Del("X-Goog-Api-Key")
	req.Header.Del(affinityHeader)

	// Apply model redirection
	finalBodyBytes, err := channelHandler.ApplyModelRedirect(req, bodyBytes, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, err.Error()))
		ps.logRequest(c, originalGroup, group, apiKey, startTime, http.StatusBadRequest, err, isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal)
		return
	}

	// Update request body if it was modified by redirection
	if !bytes.Equal(finalBodyBytes, bodyBytes) {
		req.Body = io.NopCloser(bytes.NewReader(finalBodyBytes))
		req.ContentLength = int64(len(finalBodyBytes))
	}

	channelHandler.ModifyRequest(req, apiKey, group)

	// Apply custom header rules
	if len(group.HeaderRuleList) > 0 {
		headerCtx := utils.NewHeaderVariableContextFromGin(c, group, apiKey)
		utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
	}

	var client *http.Client
	if isStream {
		client = channelHandler.GetStreamClient()
		req.Header.Set("X-Accel-Buffering", "no")
	} else {
		client = channelHandler.GetHTTPClient()
	}

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	// Unified error handling for retries.
	// Retry policy is fully defined by group.FailoverStatusCodeMatcher (derived from EffectiveConfig).
	shouldRetryByStatus := resp != nil && (shouldFailoverOnStatusCode(resp.StatusCode, group) ||
		(selectedResource != nil && shouldResourceFailoverOnStatusCode(resp.StatusCode)))
	if err != nil || shouldRetryByStatus {
		if err != nil && app_errors.IsIgnorableError(err) {
			logrus.Debugf("Client-side ignorable error for key %s, aborting retries: %v", utils.MaskAPIKey(apiKey.KeyValue), err)
			ps.logRequest(c, originalGroup, group, apiKey, startTime, 499, err, isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal)
			return
		}

		var statusCode int
		var errorMessage string
		var parsedError string

		if err != nil {
			statusCode = 500
			errorMessage = err.Error()
			parsedError = errorMessage
			logrus.Debugf("Request failed (attempt %d/%d) for key %s: %v", retryCount+1, cfg.MaxRetries, utils.MaskAPIKey(apiKey.KeyValue), err)
			if selectedResource != nil {
				if stateErr := ps.resourceProvider.HandleFailure(selectedResource, group.ChannelType, 0, parsedError, nil); stateErr != nil {
					logrus.WithError(stateErr).WithField("resourceID", selectedResource.ID).Warn("failed to update resource failure state")
				}
			}
		} else {
			// Retryable upstream response (HTTP status code matched failover policy)
			statusCode = resp.StatusCode
			errorBody, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				logrus.Errorf("Failed to read error body: %v", readErr)
				errorBody = []byte("Failed to read error body")
			}

			errorBody = handleGzipCompression(resp, errorBody)
			errorMessage = string(errorBody)
			parsedError = app_errors.ParseUpstreamError(errorBody)
			logrus.Debugf("Request failed with status %d (attempt %d/%d) for key %s. Parsed Error: %s", statusCode, retryCount+1, cfg.MaxRetries, utils.MaskAPIKey(apiKey.KeyValue), parsedError)
			if selectedResource != nil {
				if stateErr := ps.resourceProvider.HandleFailure(selectedResource, group.ChannelType, statusCode, parsedError, resp.Header); stateErr != nil {
					logrus.WithError(stateErr).WithField("resourceID", selectedResource.ID).Warn("failed to update resource failure state")
				}
			}
		}

		if selectedResource == nil {
			if err := ps.keyProvider.RecordSelectionResult(group, apiKey, keypool.SelectionResult{StatusCode: statusCode, ErrorMessage: parsedError, Model: selectionReq.Model, ProxyKey: selectionReq.ProxyKey}); err != nil {
				logrus.WithError(err).WithField("keyID", apiKey.ID).Warn("failed to update scheduler selection state")
			}

			// 使用解析后的错误信息更新密钥状态
			ps.keyProvider.UpdateStatus(apiKey, group, false, statusCode, parsedError)
		}

		// 判断是否为最后一次尝试
		objectRouting := objectRoutingFromContext(c)
		uncertainNonReplayableCreation := objectRouting.NonReplayableOnUncertain && (err != nil || statusCode >= http.StatusInternalServerError)
		isLastAttempt := retryCount >= cfg.MaxRetries || objectRouting.ForcedResourceID > 0 || uncertainNonReplayableCreation
		requestType := models.RequestTypeRetry
		if isLastAttempt {
			requestType = models.RequestTypeFinal
		}

		ps.logRequest(c, originalGroup, group, apiKey, startTime, statusCode, errors.New(parsedError), isStream, upstreamURL, channelHandler, bodyBytes, requestType)

		// 如果是最后一次尝试，直接返回错误，不再递归
		if isLastAttempt {
			var errorJSON map[string]any
			if err := json.Unmarshal([]byte(errorMessage), &errorJSON); err == nil {
				c.JSON(statusCode, errorJSON)
			} else {
				response.Error(c, app_errors.NewAPIErrorWithUpstream(statusCode, "UPSTREAM_ERROR", errorMessage))
			}
			return
		}
		if selectedResource != nil && statusCode == http.StatusTooManyRequests &&
			!keypool.IsQuotaOrBillingFailure(parsedError, nil) && requestHasAffinity(c) {
			if waitErr := waitBeforeResourceMigration(c.Request.Context(), resp, selectedPoolConfig.BusyWait); waitErr != nil {
				return
			}
		}

		nextExcludedKeyIDs := append(append([]uint(nil), excludedKeyIDs...), apiKey.ID)
		ps.executeRequestWithRetry(c, channelHandler, originalGroup, group, bodyBytes, isStream, startTime, retryCount+1, nextExcludedKeyIDs)
		return
	}

	if selectedResource != nil && resp.StatusCode >= http.StatusBadRequest {
		errorBody, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			errorBody = handleGzipCompression(resp, errorBody)
			resp.Body = io.NopCloser(bytes.NewReader(errorBody))
			parsedError := app_errors.ParseUpstreamError(errorBody)
			if stateErr := ps.resourceProvider.HandleFailure(selectedResource, group.ChannelType, resp.StatusCode, parsedError, resp.Header); stateErr != nil {
				logrus.WithError(stateErr).WithField("resourceID", selectedResource.ID).Warn("failed to update resource failure state")
			}
		}
	}

	// Successful request counters are persisted from request logs; health recovery remains validation-driven.
	if selectedResource == nil {
		if err := ps.keyProvider.RecordSelectionResult(group, apiKey, keypool.SelectionResult{StatusCode: resp.StatusCode, Model: selectionReq.Model, ProxyKey: selectionReq.ProxyKey}); err != nil {
			logrus.WithError(err).WithField("keyID", apiKey.ID).Warn("failed to update scheduler selection state")
		}
	} else if resp.StatusCode < http.StatusBadRequest {
		if affinityValue, ok := c.Get(requestAffinityContextKey); ok {
			if affinityInfo, ok := affinityValue.(requestAffinity); ok && affinityInfo.Hash != "" {
				objectRouting := objectRoutingFromContext(c)
				if objectRouting.ForcedResourceID == 0 {
					if bindErr := ps.resourceProvider.BindAffinity(*group.ResourcePoolID, affinityInfo.Hash, selectedResource.ID, selectedPoolConfig.AffinityTTL); bindErr != nil {
						logrus.WithError(bindErr).WithField("resourceID", selectedResource.ID).Warn("failed to bind successful resource affinity")
					}
				} else if refreshErr := ps.resourceProvider.RefreshAffinity(*group.ResourcePoolID, affinityInfo.Hash, selectedPoolConfig.AffinityTTL); refreshErr != nil {
					logrus.WithError(refreshErr).WithField("resourceID", selectedResource.ID).Debug("resource affinity did not need refreshing")
				}
			}
		}
	}
	objectRouting := objectRoutingFromContext(c)
	if !isStream && selectedResource != nil && objectRouting.ResponseObjectType != "" &&
		resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			logrus.WithError(readErr).Error("failed to read account-scoped object response")
			response.Error(c, app_errors.NewAPIError(app_errors.ErrBadGateway, "failed to read upstream object response"))
			ps.logRequest(c, originalGroup, group, apiKey, startTime, http.StatusBadGateway, readErr, isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal)
			return
		}
		bindingBody := handleGzipCompression(resp, responseBody)
		if bindErr := ps.persistUpstreamObjectBindings(c.Request.Context(), originalGroup.ID, objectRouting, selectedEndpoint.ID, selectedResource, bindingBody); bindErr != nil {
			logrus.WithError(bindErr).WithField("resourceID", selectedResource.ID).Error("failed to persist upstream object ownership")
			response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, "failed to persist upstream object ownership; response was withheld to prevent unsafe cross-key routing"))
			ps.logRequest(c, originalGroup, group, apiKey, startTime, http.StatusInternalServerError, bindErr, isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal)
			return
		}
		resp.Body = io.NopCloser(bytes.NewReader(responseBody))
	}
	logrus.Debugf("Request for group %s succeeded on attempt %d with key %s", group.Name, retryCount+1, utils.MaskAPIKey(apiKey.KeyValue))

	// Check if this is a model list request (needs special handling)
	if shouldInterceptModelList(c.Request.URL.Path, c.Request.Method) {
		ps.handleModelListResponse(c, resp, group, channelHandler)
	} else {
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}
		c.Status(resp.StatusCode)

		if isStream {
			ps.handleStreamingResponse(c, resp)
		} else {
			responseBody := ps.handleNormalResponse(c, resp)
			if usage, ok := extractUpstreamTokenUsage(responseBody); ok {
				c.Set(upstreamTokenUsageContextKey, usage)
			}
		}
	}

	ps.logRequest(c, originalGroup, group, apiKey, startTime, resp.StatusCode, nil, isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal)
}

func shouldFailoverOnStatusCode(statusCode int, group *models.Group) bool {
	if group == nil {
		return false
	}
	return group.FailoverStatusCodeMatcher.Match(statusCode)
}

func shouldResourceFailoverOnStatusCode(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden ||
		statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func requestHasAffinity(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(requestAffinityContextKey)
	if !ok {
		return false
	}
	affinity, ok := value.(requestAffinity)
	return ok && affinity.Hash != ""
}

func waitBeforeResourceMigration(ctx context.Context, resp *http.Response, maximum time.Duration) error {
	wait := maximum
	if resp != nil {
		if raw := resp.Header.Get("Retry-After"); raw != "" {
			if seconds, parseErr := time.ParseDuration(raw + "s"); parseErr == nil && seconds < wait {
				wait = seconds
			}
		}
	}
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func extractProxyKeyForAffinity(c *gin.Context) string {
	if value := c.GetHeader("Authorization"); value != "" {
		return value
	}
	if value := c.GetHeader("X-Api-Key"); value != "" {
		return value
	}
	return c.GetHeader("X-Goog-Api-Key")
}

const (
	requestAffinityContextKey       = "api_load_request_affinity"
	upstreamResourceIDContextKey    = "api_load_upstream_resource_id"
	upstreamObjectRoutingContextKey = "api_load_upstream_object_routing"
)

// logRequest is a helper function to create and record a request log.
func (ps *ProxyServer) logRequest(
	c *gin.Context,
	originalGroup *models.Group,
	group *models.Group,
	apiKey *models.APIKey,
	startTime time.Time,
	statusCode int,
	finalError error,
	isStream bool,
	upstreamAddr string,
	channelHandler channel.ChannelProxy,
	bodyBytes []byte,
	requestType string,
) {
	if ps.requestLogService == nil {
		return
	}

	var requestBodyToLog, userAgent string

	if group.EffectiveConfig.EnableRequestBodyLogging {
		requestBodyToLog = utils.TruncateString(string(bodyBytes), 65000)
		userAgent = c.Request.UserAgent()
	}

	duration := time.Since(startTime).Milliseconds()

	logEntry := &models.RequestLog{
		GroupID:      group.ID,
		GroupName:    group.Name,
		IsSuccess:    finalError == nil && statusCode < 400,
		SourceIP:     c.ClientIP(),
		StatusCode:   statusCode,
		RequestPath:  utils.TruncateString(c.Request.URL.String(), 500),
		Duration:     duration,
		UserAgent:    userAgent,
		RequestType:  requestType,
		IsStream:     isStream,
		UpstreamAddr: utils.TruncateString(upstreamAddr, 500),
		RequestBody:  requestBodyToLog,
	}
	if value, ok := c.Get(upstreamResourceIDContextKey); ok {
		if resourceID, ok := value.(uint); ok {
			logEntry.ResourceID = resourceID
		}
	}

	// Set parent group
	if originalGroup != nil && originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		logEntry.ParentGroupID = originalGroup.ID
		logEntry.ParentGroupName = originalGroup.Name
	}

	if channelHandler != nil && bodyBytes != nil {
		logEntry.Model = channelHandler.ExtractModel(c, bodyBytes)
	}

	if apiKey != nil {
		// 加密密钥值用于日志存储
		encryptedKeyValue, err := ps.encryptionSvc.Encrypt(apiKey.KeyValue)
		if err != nil {
			logrus.WithError(err).Error("Failed to encrypt key value for logging")
			logEntry.KeyValue = "failed-to-encryption"
		} else {
			logEntry.KeyValue = encryptedKeyValue
		}
		// 添加 KeyHash 用于反查
		logEntry.KeyHash = ps.encryptionSvc.Hash(apiKey.KeyValue)
	}

	if finalError != nil {
		logEntry.ErrorMessage = finalError.Error()
	}

	if rawUsage, ok := c.Get(upstreamTokenUsageContextKey); ok {
		if usage, ok := rawUsage.(services.TokenUsage); ok {
			services.ApplyUpstreamTokenUsage(logEntry, usage)
		}
	}

	if err := ps.requestLogService.Record(logEntry); err != nil {
		logrus.Errorf("Failed to record request log: %v", err)
	}
}
