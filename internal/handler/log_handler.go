package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gpt-load/internal/db"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
)

// GetLogs godoc
func GetLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	offset := (page - 1) * size

	query := db.DB.Model(&models.RequestLog{})

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		groupID, err := strconv.Atoi(groupIDStr)
		if err == nil {
			query = query.Where("group_id = ?", groupID)
		}
	}

	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err == nil {
			query = query.Where("timestamp >= ?", startTime)
		}
	}

	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err == nil {
			query = query.Where("timestamp <= ?", endTime)
		}
	}

	if statusCodeStr := c.Query("status_code"); statusCodeStr != "" {
		statusCode, err := strconv.Atoi(statusCodeStr)
		if err == nil {
			query = query.Where("status_code = ?", statusCode)
		}
	}

	var logs []models.RequestLog
	var total int64

	query.Count(&total)
	err := query.Order("timestamp desc").Offset(offset).Limit(size).Find(&logs).Error
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"size":  size,
		"data":  logs,
	})
}
