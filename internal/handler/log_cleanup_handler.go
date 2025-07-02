package handler

import (
	"gpt-load/internal/response"
	"gpt-load/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// LogCleanupHandler handles log cleanup related requests
type LogCleanupHandler struct {
	LogCleanupService *services.LogCleanupService
}

// NewLogCleanupHandler creates a new LogCleanupHandler
func NewLogCleanupHandler(s *services.LogCleanupService) *LogCleanupHandler {
	return &LogCleanupHandler{
		LogCleanupService: s,
	}
}

// CleanupLogsNow handles the POST /api/logs/cleanup request.
// It triggers an asynchronous cleanup of expired request logs.
func (h *LogCleanupHandler) CleanupLogsNow(c *gin.Context) {
	go func() {
		logrus.Info("Asynchronous log cleanup started from API request")
		h.LogCleanupService.CleanupNow()
	}()

	response.Success(c, gin.H{
		"message": "Log cleanup process started in the background",
	})
}
