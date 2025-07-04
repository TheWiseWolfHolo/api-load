package handler

import (
	"github.com/gin-gonic/gin"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
)

// GetDashboardStats godoc
// @Summary Get dashboard statistics
// @Description Get statistics for the dashboard, including key counts and request metrics.
// @Tags Dashboard
// @Accept  json
// @Produce  json
// @Success 200 {object} map[string]interface{}
// @Router /api/dashboard/stats [get]
func (s *Server) Stats(c *gin.Context) {
	var totalRequests, successRequests int64
	var groupStats []models.GroupRequestStat

	// 1. Get total and successful requests from the api_keys table
	s.DB.Model(&models.APIKey{}).Select("SUM(request_count)").Row().Scan(&totalRequests)
	s.DB.Model(&models.APIKey{}).Select("SUM(request_count) - SUM(failure_count)").Row().Scan(&successRequests)

	// 2. Get request counts per group
	s.DB.Table("api_keys").
		Select("groups.display_name as display_name, SUM(api_keys.request_count) as request_count").
		Joins("join groups on groups.id = api_keys.group_id").
		Group("groups.id, groups.display_name").
		Order("request_count DESC").
		Scan(&groupStats)

	// 3. Calculate success rate
	var successRate float64
	if totalRequests > 0 {
		successRate = float64(successRequests) / float64(totalRequests) * 100
	}

	stats := models.DashboardStats{
		TotalRequests:   totalRequests,
		SuccessRequests: successRequests,
		SuccessRate:     successRate,
		GroupStats:      groupStats,
	}

	response.Success(c, stats)
}
