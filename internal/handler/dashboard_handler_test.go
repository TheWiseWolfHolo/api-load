package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gpt-load/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTOK003DashboardAggregatesTokenUsageByModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:dashboard-token?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	now := time.Now().UTC()
	logs := []models.RequestLog{
		{ID: "tok-1", Timestamp: now.Add(-time.Hour), GroupID: 1, GroupName: "g1", Model: "gpt-a", RequestType: models.RequestTypeFinal, TotalTokens: 10, CacheReadTokens: 2, CacheWriteTokens: 3, ThinkingTokens: 4, TokenUsageSource: models.TokenUsageSourceUpstream},
		{ID: "tok-2", Timestamp: now.Add(-30 * time.Minute), GroupID: 1, GroupName: "g1", Model: "gpt-a", RequestType: models.RequestTypeFinal, TotalTokens: 5, CacheReadTokens: 1, CacheWriteTokens: 1, ThinkingTokens: 2, TokenUsageSource: models.TokenUsageSourceEstimated},
		{ID: "tok-3", Timestamp: now.Add(-30 * time.Minute), GroupID: 2, GroupName: "g2", Model: "gpt-b", RequestType: models.RequestTypeRetry, TotalTokens: 99, TokenUsageSource: models.TokenUsageSourceUpstream},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	server := &Server{DB: db}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/dashboard/token-stats?group_by=model&start_time="+now.Add(-2*time.Hour).Format(time.RFC3339)+"&end_time="+now.Format(time.RFC3339), nil)

	server.TokenStats(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			Items []struct {
				Dimension        string `json:"dimension"`
				TotalTokens      int64  `json:"total_tokens"`
				CacheReadTokens  int64  `json:"cache_read_tokens"`
				CacheWriteTokens int64  `json:"cache_write_tokens"`
				ThinkingTokens   int64  `json:"thinking_tokens"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.Items) != 1 {
		t.Fatalf("expected one final-request model aggregate, got %#v", response.Data.Items)
	}
	item := response.Data.Items[0]
	if item.Dimension != "gpt-a" || item.TotalTokens != 15 || item.CacheReadTokens != 3 || item.CacheWriteTokens != 4 || item.ThinkingTokens != 6 {
		t.Fatalf("unexpected token aggregate: %#v", item)
	}
}
