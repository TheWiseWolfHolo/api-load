package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gpt-load/internal/db"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
)

// LogResponse defines the structure for log entries in the API response,
// enriching the base log with related data.
type LogResponse struct {
	models.RequestLog
	GroupName string `json:"group_name"`
	KeyValue  string `json:"key_value"`
}

// GetLogs Get request logs
func GetLogs(c *gin.Context) {
	// --- 1. Build WHERE conditions ---
	query := db.DB.Model(&models.RequestLog{})

	if groupName := c.Query("group_name"); groupName != "" {
		var groupIDs []uint
		db.DB.Model(&models.Group{}).Where("name LIKE ? OR display_name LIKE ?", "%"+groupName+"%", "%"+groupName+"%").Pluck("id", &groupIDs)
		if len(groupIDs) == 0 {
			response.Success(c, &response.PaginatedResponse{
				Items:      []LogResponse{},
				Pagination: response.Pagination{TotalItems: 0, Page: 1, PageSize: response.DefaultPageSize},
			})
			return
		}
		query = query.Where("group_id IN ?", groupIDs)
	}
	if keyValue := c.Query("key_value"); keyValue != "" {
		var keyIDs []uint
		likePattern := "%" + keyValue[1:len(keyValue)-1] + "%"
		db.DB.Model(&models.APIKey{}).Where("key_value LIKE ?", likePattern).Pluck("id", &keyIDs)
		if len(keyIDs) == 0 {
			response.Success(c, &response.PaginatedResponse{
				Items:      []LogResponse{},
				Pagination: response.Pagination{TotalItems: 0, Page: 1, PageSize: response.DefaultPageSize},
			})
			return
		}
		query = query.Where("key_id IN ?", keyIDs)
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

	// --- 2. Get Paginated Logs ---
	var logs []models.RequestLog
	query = query.Order("timestamp desc") // Apply ordering before pagination
	pagination, err := response.Paginate(c, query, &logs)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// --- 3. Enrich Logs with GroupName and KeyValue ---
	if len(logs) == 0 {
		response.Success(c, pagination) // Return empty pagination response
		return
	}

	// Collect IDs for enrichment
	groupIds := make(map[uint]bool)
	keyIds := make(map[uint]bool)
	for _, log := range logs {
		if log.GroupID != 0 {
			groupIds[log.GroupID] = true
		}
		if log.KeyID != 0 {
			keyIds[log.KeyID] = true
		}
	}

	// Fetch enrichment data
	groupMap := make(map[uint]string)
	if len(groupIds) > 0 {
		var groups []models.Group
		var ids []uint
		for id := range groupIds {
			ids = append(ids, id)
		}
		db.DB.Where("id IN ?", ids).Find(&groups)
		for _, group := range groups {
			groupMap[group.ID] = group.Name
		}
	}

	keyMap := make(map[uint]string)
	if len(keyIds) > 0 {
		var keys []models.APIKey
		var ids []uint
		for id := range keyIds {
			ids = append(ids, id)
		}
		db.DB.Where("id IN ?", ids).Find(&keys)
		for _, key := range keys {
			keyMap[key.ID] = key.KeyValue
		}
	}

	// Build final response
	logResponses := make([]LogResponse, len(logs))
	for i, log := range logs {
		logResponses[i] = LogResponse{
			RequestLog: log,
			GroupName:  groupMap[log.GroupID],
			KeyValue:   keyMap[log.KeyID],
		}
	}

	// --- 4. Send Response ---
	pagination.Items = logResponses
	response.Success(c, pagination)
}
