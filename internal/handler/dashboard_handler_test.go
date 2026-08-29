package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-load/internal/models"

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

func TestDASH001ProviderCapacityIncludesLegacyAndPooledCredentials(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:dashboard-capacity?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.APIKey{}, &models.UpstreamResource{}, &models.ResourcePool{},
		&models.ResourcePoolEndpoint{}, &models.Group{}, &models.RequestLog{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	future := now.Add(time.Hour)
	pool := models.ResourcePool{Name: "capacity", Strategy: "round_robin", AffinityTTLSeconds: 3600, BusyWaitMilliseconds: 2000}
	if err := db.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	endpoint := models.ResourcePoolEndpoint{ResourcePoolID: pool.ID, Name: "openai", ChannelType: "openai", BaseURL: "https://api.example.invalid", Enabled: models.Bool(true)}
	if err := db.Create(&endpoint).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	poolID, endpointID := pool.ID, endpoint.ID
	group := models.Group{Name: "pooled", GroupType: "standard", ResourcePoolID: &poolID, ResourceEndpointID: &endpointID, ChannelType: "openai", Upstreams: []byte("[]")}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	legacy := []models.APIKey{
		{GroupID: group.ID, KeyValue: "a", Enabled: models.Bool(true), Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "b", Enabled: models.Bool(false), Status: models.KeyStatusActive},
	}
	resources := []models.UpstreamResource{
		{ResourcePoolID: pool.ID, KeyValue: "c", KeyHash: "c", IdentityHash: "c", Enabled: models.Bool(true), Status: models.ResourceStatusInvalid},
		{ResourcePoolID: pool.ID, KeyValue: "d", KeyHash: "d", IdentityHash: "d", Enabled: models.Bool(true), Status: models.ResourceStatusActive, GlobalCooldownUntil: &future},
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy keys: %v", err)
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatalf("create pooled resources: %v", err)
	}
	logs := []models.RequestLog{
		{ID: "retry", TraceID: "trace", Attempt: 1, Timestamp: now.Add(-time.Hour), GroupID: group.ID, RequestType: models.RequestTypeRetry},
		{ID: "final", TraceID: "trace", Attempt: 2, Timestamp: now.Add(-time.Hour), GroupID: group.ID, RequestType: models.RequestTypeFinal},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create request logs: %v", err)
	}

	server := &Server{DB: db}
	capacity, err := server.getProviderCapacity(now, 1)
	if err != nil {
		t.Fatalf("get provider capacity: %v", err)
	}
	if capacity.TotalCredentials != 4 || capacity.ReadyCredentials != 1 ||
		capacity.AutoDisabledCredentials != 1 || capacity.PausedCredentials != 1 ||
		capacity.CoolingCredentials != 1 || capacity.ResourcePools != 1 ||
		capacity.ProtocolEndpoints != 1 || capacity.PoolBoundRoutes != 1 ||
		capacity.RetryAttempts24H != 1 || capacity.RetryRate24H != 50 {
		t.Fatalf("unexpected provider capacity: %#v", capacity)
	}
}
