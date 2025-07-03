// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"fmt"
	"gpt-load/internal/channel"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ProxyServer represents the proxy server
type ProxyServer struct {
	DB             *gorm.DB
	groupCounters  sync.Map // map[uint]*atomic.Uint64
	requestLogChan chan models.RequestLog
}

// NewProxyServer creates a new proxy server
func NewProxyServer(db *gorm.DB, requestLogChan chan models.RequestLog) (*ProxyServer, error) {
	return &ProxyServer{
		DB:             db,
		groupCounters:  sync.Map{},
		requestLogChan: requestLogChan,
	}, nil
}

// HandleProxy handles the main proxy logic
func (ps *ProxyServer) HandleProxy(c *gin.Context) {
	startTime := time.Now()
	groupName := c.Param("group_name")

	// 1. Find the group by name
	var group models.Group
	if err := ps.DB.Preload("APIKeys").Where("name = ?", groupName).First(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 2. Select an available API key from the group
	apiKey, err := ps.selectAPIKey(&group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error()))
		return
	}

	// 3. Get the appropriate channel handler from the factory
	channelHandler, err := channel.GetChannel(&group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get channel for group '%s': %v", groupName, err)))
		return
	}

	// 4. Forward the request using the channel handler
	err = channelHandler.Handle(c, apiKey, &group)

	// 5. Log the request asynchronously
	isSuccess := err == nil
	if !isSuccess {
		logrus.WithFields(logrus.Fields{
			"group": group.Name,
			"key_id": apiKey.ID,
			"error": err.Error(),
		}).Error("Channel handler failed")
	}
	go ps.logRequest(c, &group, apiKey, startTime, isSuccess)
}

// selectAPIKey selects an API key from a group using round-robin
func (ps *ProxyServer) selectAPIKey(group *models.Group) (*models.APIKey, error) {
	activeKeys := make([]models.APIKey, 0, len(group.APIKeys))
	for _, key := range group.APIKeys {
		if key.Status == "active" {
			activeKeys = append(activeKeys, key)
		}
	}

	if len(activeKeys) == 0 {
		return nil, fmt.Errorf("no active API keys available in group '%s'", group.Name)
	}

	// Get or create a counter for the group. The value is a pointer to a uint64.
	val, _ := ps.groupCounters.LoadOrStore(group.ID, new(atomic.Uint64))
	counter := val.(*atomic.Uint64)

	// Atomically increment the counter and get the index for this request.
	index := counter.Add(1) - 1
	selectedKey := activeKeys[int(index%uint64(len(activeKeys)))]

	return &selectedKey, nil
}

func (ps *ProxyServer) logRequest(c *gin.Context, group *models.Group, key *models.APIKey, startTime time.Time, isSuccess bool) {
	// Update key stats based on request success
	go ps.updateKeyStats(key.ID, isSuccess)

	logEntry := models.RequestLog{
		ID:                 fmt.Sprintf("req_%d", time.Now().UnixNano()),
		Timestamp:          startTime,
		GroupID:            group.ID,
		KeyID:              key.ID,
		SourceIP:           c.ClientIP(),
		StatusCode:         c.Writer.Status(),
		RequestPath:        c.Request.URL.Path,
		RequestBodySnippet: "", // Can be implemented later if needed
	}

	// Send to the logging channel without blocking
	select {
	case ps.requestLogChan <- logEntry:
	default:
		logrus.Warn("Request log channel is full. Dropping log entry.")
	}
}

// updateKeyStats atomically updates the request and failure counts for a key
func (ps *ProxyServer) updateKeyStats(keyID uint, success bool) {
	// Always increment the request count
	updates := map[string]interface{}{
		"request_count": gorm.Expr("request_count + 1"),
	}

	// Additionally, increment the failure count if the request was not successful
	if !success {
		updates["failure_count"] = gorm.Expr("failure_count + 1")
	}

	result := ps.DB.Model(&models.APIKey{}).Where("id = ?", keyID).Updates(updates)
	if result.Error != nil {
		logrus.WithFields(logrus.Fields{
			"keyID": keyID,
			"error": result.Error,
		}).Error("Failed to update key stats")
	}
}

// Close cleans up resources
func (ps *ProxyServer) Close() {
	// Nothing to close for now
}
