package handler

import (
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ReloadConfig handles the POST /api/reload request.
// It triggers a configuration reload.
func (s *Server) ReloadConfig(c *gin.Context) {
	if s.config == nil {
		response.InternalError(c, "Configuration manager is not initialized")
		return
	}

	err := s.config.ReloadConfig()
	if err != nil {
		logrus.Errorf("Failed to reload config: %v", err)
		response.InternalError(c, "Failed to reload config")
		return
	}

	response.Success(c, gin.H{"message": "Configuration reloaded successfully"})
}
