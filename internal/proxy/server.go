// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"fmt"
	"gpt-load/internal/channel"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ProxyServer represents the proxy server
type ProxyServer struct {
	DB             *gorm.DB
	groupCounters  sync.Map // For round-robin key selection
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

// RegisterProxyRoutes registers the main proxy route under a given router group
func (ps *ProxyServer) RegisterProxyRoutes(proxy *gin.RouterGroup) {
	proxy.Any("/:group_name/*path", ps.HandleProxy)
}

// HandleProxy handles the main proxy logic
func (ps *ProxyServer) HandleProxy(c *gin.Context) {
	startTime := time.Now()
	groupName := c.Param("group_name")

	// 1. Find the group by name
	var group models.Group
	if err := ps.DB.Preload("APIKeys").Where("name = ?", groupName).First(&group).Error; err != nil {
		response.Error(c, http.StatusNotFound, fmt.Sprintf("Group '%s' not found", groupName))
		return
	}

	// 2. Select an available API key from the group
	apiKey, err := ps.selectAPIKey(&group)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	// 3. Get the appropriate channel handler from the factory
	channelHandler, err := channel.GetChannel(&group)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get channel for group '%s': %v", groupName, err))
		return
	}

	// 4. Forward the request using the channel handler
	channelHandler.Handle(c, apiKey, &group)

	// 5. Log the request asynchronously
	go ps.logRequest(c, &group, apiKey, startTime)
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

	// Get the current counter for the group
	counter, _ := ps.groupCounters.LoadOrStore(group.ID, uint64(0))
	currentCounter := counter.(uint64)

	// Select the key and increment the counter
	selectedKey := activeKeys[int(currentCounter%uint64(len(activeKeys)))]
	ps.groupCounters.Store(group.ID, currentCounter+1)

	return &selectedKey, nil
}

func (ps *ProxyServer) logRequest(c *gin.Context, group *models.Group, key *models.APIKey, startTime time.Time) {
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

// Close cleans up resources
func (ps *ProxyServer) Close() {
	// Nothing to close for now
}
