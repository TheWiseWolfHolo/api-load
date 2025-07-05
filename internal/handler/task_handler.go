package handler

import (
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
)

// GetTaskStatus handles requests for the status of the global long-running task.
func (s *Server) GetTaskStatus(c *gin.Context) {
	taskStatus, err := s.TaskService.GetTaskStatus()
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, "Failed to get task status"))
		return
	}
	response.Success(c, taskStatus)
}
