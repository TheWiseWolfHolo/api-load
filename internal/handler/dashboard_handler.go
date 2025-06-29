package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gpt-load/internal/db"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
)

// GetDashboardStats godoc
// @Summary Get dashboard statistics
// @Description Get statistics for the dashboard, including total requests, success rate, and group distribution.
// @Tags Dashboard
// @Accept  json
// @Produce  json
// @Success 200 {object} models.DashboardStats
// @Router /api/dashboard/stats [get]
func GetDashboardStats(c *gin.Context) {
	var totalRequests, successRequests int64
	var groupStats []models.GroupRequestStat

	// Get total requests
	if err := db.DB.Model(&models.RequestLog{}).Count(&totalRequests).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get total requests")
		return
	}

	// Get success requests (status code 2xx)
	if err := db.DB.Model(&models.RequestLog{}).Where("status_code >= ? AND status_code < ?", 200, 300).Count(&successRequests).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get success requests")
		return
	}

	// Calculate success rate
	var successRate float64
	if totalRequests > 0 {
		successRate = float64(successRequests) / float64(totalRequests)
	}

	// Get group stats
	err := db.DB.Table("request_logs").
		Select("groups.name as group_name, count(request_logs.id) as request_count").
		Joins("join groups on groups.id = request_logs.group_id").
		Group("groups.name").
		Order("request_count desc").
		Scan(&groupStats).Error
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get group stats")
		return
	}

	stats := models.DashboardStats{
		TotalRequests:   totalRequests,
		SuccessRequests: successRequests,
		SuccessRate:     successRate,
		GroupStats:      groupStats,
	}

	response.Success(c, stats)
}