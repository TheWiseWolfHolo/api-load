package handler

import (
	"strconv"
	"time"

	"gpt-load/internal/db"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
)

// LogResponse defines the structure for log entries in the API response
type LogResponse struct {
	models.RequestLog
}

// GetLogs Get request logs
func GetLogs(c *gin.Context) {
	query := db.DB.Model(&models.RequestLog{})

	if groupName := c.Query("group_name"); groupName != "" {
		query = query.Where("group_name LIKE ?", "%"+groupName+"%")
	}
	if keyValue := c.Query("key_value"); keyValue != "" {
		likePattern := "%" + keyValue[1:len(keyValue)-1] + "%"
		query = query.Where("key_value LIKE ?", likePattern)
	}
	if isSuccessStr := c.Query("is_success"); isSuccessStr != "" {
		if isSuccess, err := strconv.ParseBool(isSuccessStr); err == nil {
			query = query.Where("is_success = ?", isSuccess)
		}
	}
	if statusCodeStr := c.Query("status_code"); statusCodeStr != "" {
		if statusCode, err := strconv.Atoi(statusCodeStr); err == nil {
			query = query.Where("status_code = ?", statusCode)
		}
	}
	if sourceIP := c.Query("source_ip"); sourceIP != "" {
		query = query.Where("source_ip = ?", sourceIP)
	}
	if errorContains := c.Query("error_contains"); errorContains != "" {
		query = query.Where("error_message LIKE ?", "%"+errorContains+"%")
	}
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			query = query.Where("timestamp >= ?", startTime)
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			query = query.Where("timestamp <= ?", endTime)
		}
	}

	var logs []models.RequestLog
	query = query.Order("timestamp desc")
	pagination, err := response.Paginate(c, query, &logs)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	pagination.Items = logs
	response.Success(c, pagination)
}
