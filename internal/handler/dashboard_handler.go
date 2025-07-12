package handler

import (
	"github.com/gin-gonic/gin"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"time"
)

// Stats godoc
// @Summary Get dashboard statistics
// @Description Get statistics for the dashboard cards
// @Tags Dashboard
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response{data=models.DashboardStatsResponse}
// @Router /dashboard/stats [get]
func (s *Server) Stats(c *gin.Context) {
	var activeKeys, invalidKeys, groupCount int64
	s.DB.Model(&models.APIKey{}).Where("status = ?", models.KeyStatusActive).Count(&activeKeys)
	s.DB.Model(&models.APIKey{}).Where("status = ?", models.KeyStatusInvalid).Count(&invalidKeys)
	s.DB.Model(&models.Group{}).Count(&groupCount)

	now := time.Now()
	twentyFourHoursAgo := now.Add(-24 * time.Hour)
	fortyEightHoursAgo := now.Add(-48 * time.Hour)

	currentPeriod, err := s.getHourlyStats(twentyFourHoursAgo, now)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, "failed to get current period stats"))
		return
	}
	previousPeriod, err := s.getHourlyStats(fortyEightHoursAgo, twentyFourHoursAgo)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, "failed to get previous period stats"))
		return
	}

	reqTrend := 0.0
	if previousPeriod.TotalRequests > 0 {
		reqTrend = (float64(currentPeriod.TotalRequests-previousPeriod.TotalRequests) / float64(previousPeriod.TotalRequests)) * 100
	}

	currentErrorRate := 0.0
	if currentPeriod.TotalRequests > 0 {
		currentErrorRate = (float64(currentPeriod.TotalFailures) / float64(currentPeriod.TotalRequests)) * 100
	}

	previousErrorRate := 0.0
	if previousPeriod.TotalRequests > 0 {
		previousErrorRate = (float64(previousPeriod.TotalFailures) / float64(previousPeriod.TotalRequests)) * 100
	}

	errorRateTrend := currentErrorRate - previousErrorRate

	stats := models.DashboardStatsResponse{
		KeyCount: models.StatCard{
			Value:       float64(activeKeys),
			SubValue:    invalidKeys,
			SubValueTip: "无效秘钥数量",
		},
		GroupCount: models.StatCard{
			Value: float64(groupCount),
		},
		RequestCount: models.StatCard{
			Value:         float64(currentPeriod.TotalRequests),
			Trend:         reqTrend,
			TrendIsGrowth: reqTrend >= 0,
		},
		ErrorRate: models.StatCard{
			Value:         currentErrorRate,
			Trend:         errorRateTrend,
			TrendIsGrowth: errorRateTrend < 0, // 错误率下降是好事
		},
	}

	response.Success(c, stats)
}


// Chart godoc
// @Summary Get dashboard chart data
// @Description Get chart data for the last 24 hours
// @Tags Dashboard
// @Accept  json
// @Produce  json
// @Param groupId query int false "Group ID"
// @Success 200 {object} response.Response{data=models.ChartData}
// @Router /dashboard/chart [get]
func (s *Server) Chart(c *gin.Context) {
	groupID := c.Query("groupId")

	now := time.Now()
	twentyFourHoursAgo := now.Add(-24 * time.Hour)

	var hourlyStats []models.GroupHourlyStat
	query := s.DB.Where("time >= ?", twentyFourHoursAgo)
	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	if err := query.Order("time asc").Find(&hourlyStats).Error; err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, "failed to get chart data"))
		return
	}

	statsByHour := make(map[time.Time]map[string]int64)
	for _, stat := range hourlyStats {
		hour := stat.Time.Truncate(time.Hour)
		if _, ok := statsByHour[hour]; !ok {
			statsByHour[hour] = make(map[string]int64)
		}
		statsByHour[hour]["success"] += stat.SuccessCount
		statsByHour[hour]["failure"] += stat.FailureCount
	}

	var labels []string
	var successData, failureData []int64

	for i := 0; i < 24; i++ {
		hour := twentyFourHoursAgo.Add(time.Duration(i) * time.Hour).Truncate(time.Hour)
		labels = append(labels, hour.Format("15:04"))

		if data, ok := statsByHour[hour]; ok {
			successData = append(successData, data["success"])
			failureData = append(failureData, data["failure"])
		} else {
			successData = append(successData, 0)
			failureData = append(failureData, 0)
		}
	}

	chartData := models.ChartData{
		Labels: labels,
		Datasets: []models.ChartDataset{
			{
				Label: "成功请求",
				Data:  successData,
				Color: "rgba(10, 200, 110, 1)",
			},
			{
				Label: "失败请求",
				Data:  failureData,
				Color: "rgba(255, 70, 70, 1)",
			},
		},
	}

	response.Success(c, chartData)
}


type hourlyStatResult struct {
	TotalRequests int64
	TotalFailures int64
}

func (s *Server) getHourlyStats(startTime, endTime time.Time) (hourlyStatResult, error) {
	var result hourlyStatResult
	err := s.DB.Model(&models.GroupHourlyStat{}).
		Select("sum(success_count) + sum(failure_count) as total_requests, sum(failure_count) as total_failures").
		Where("time >= ? AND time < ?", startTime, endTime).
		Scan(&result).Error
	return result, err
}
