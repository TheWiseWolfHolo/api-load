package handler

import (
	"gpt-load/internal/response"
	app_errors "gpt-load/internal/errors"

	"github.com/gin-gonic/gin"
)

// GetTaskStatus handles requests for the status of the global long-running task.
func (s *Server) GetTaskStatus(c *gin.Context) {
	taskStatus := s.TaskService.GetTaskStatus()
	response.Success(c, taskStatus)
}

// GetTaskResult handles requests for the result of a finished task.
func (s *Server) GetTaskResult(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Task ID is required"))
		return
	}

	result, found := s.TaskService.GetResult(taskID)
	if !found {
		response.Error(c, app_errors.ErrResourceNotFound)
		return
	}

	response.Success(c, result)
}
