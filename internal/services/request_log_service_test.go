package services

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"api-load/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var requestLogTestDBSeq uint64

func newRequestLogServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := atomic.AddUint64(&requestLogTestDBSeq, 1)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:request-log-%d?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}, &models.APIKey{}, &models.GroupHourlyStat{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestSTAT001KeyAndResourceCountersUseSuccessfulCallsAndExclude404Failures(t *testing.T) {
	db := newRequestLogServiceTestDB(t)
	if err := db.AutoMigrate(&models.UpstreamResource{}); err != nil {
		t.Fatalf("migrate resource: %v", err)
	}
	key := models.APIKey{GroupID: 1, KeyHash: "key-stat", KeyValue: "encrypted", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1}
	resource := models.UpstreamResource{ResourcePoolID: 1, Name: "seat", UpstreamURL: "https://api.example.invalid", KeyValue: "encrypted", KeyHash: "resource-stat", IdentityHash: "identity-stat", Status: models.ResourceStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatalf("create resource: %v", err)
	}

	service := &RequestLogService{db: db}
	now := time.Now().UTC().Truncate(time.Second)
	logs := []*models.RequestLog{
		{ID: "stat-success", Timestamp: now, GroupID: 1, ResourceID: resource.ID, KeyHash: key.KeyHash, IsSuccess: true, StatusCode: 200, RequestType: models.RequestTypeFinal},
		{ID: "stat-failure", Timestamp: now.Add(time.Second), GroupID: 1, ResourceID: resource.ID, KeyHash: key.KeyHash, IsSuccess: false, StatusCode: 500, RequestType: models.RequestTypeRetry},
		{ID: "stat-404", Timestamp: now.Add(2 * time.Second), GroupID: 1, ResourceID: resource.ID, KeyHash: key.KeyHash, IsSuccess: false, StatusCode: 404, RequestType: models.RequestTypeFinal},
	}
	if err := service.writeLogsToDB(logs); err != nil {
		t.Fatalf("write logs: %v", err)
	}

	if err := db.First(&key, key.ID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if key.RequestCount != 1 || key.TotalFailureCount != 1 || key.LastSuccessAt == nil || key.LastFailureAt == nil {
		t.Fatalf("unexpected key counters: %#v", key)
	}
	if err := db.First(&resource, resource.ID).Error; err != nil {
		t.Fatalf("reload resource: %v", err)
	}
	if resource.RequestCount != 1 || resource.TotalFailureCount != 1 || resource.LastSuccessAt == nil || resource.LastFailureAt == nil {
		t.Fatalf("unexpected resource counters: %#v", resource)
	}
}

func TestSTAT002CredentialCountersStayIsolatedWithinOneBatch(t *testing.T) {
	db := newRequestLogServiceTestDB(t)
	if err := db.AutoMigrate(&models.UpstreamResource{}); err != nil {
		t.Fatalf("migrate resource: %v", err)
	}
	keys := []models.APIKey{
		{GroupID: 1, KeyHash: "key-stat-a", KeyValue: "encrypted-a", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
		{GroupID: 1, KeyHash: "key-stat-b", KeyValue: "encrypted-b", Status: models.KeyStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
	}
	resources := []models.UpstreamResource{
		{ResourcePoolID: 1, Name: "seat-a", UpstreamURL: "https://a.example.invalid", KeyValue: "encrypted-a", KeyHash: "resource-stat-a", IdentityHash: "identity-stat-a", Status: models.ResourceStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
		{ResourcePoolID: 1, Name: "seat-b", UpstreamURL: "https://b.example.invalid", KeyValue: "encrypted-b", KeyHash: "resource-stat-b", IdentityHash: "identity-stat-b", Status: models.ResourceStatusActive, Enabled: models.Bool(true), Priority: 10, Weight: 1},
	}
	if err := db.Create(&keys).Error; err != nil {
		t.Fatalf("create keys: %v", err)
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatalf("create resources: %v", err)
	}

	service := &RequestLogService{db: db}
	now := time.Now().UTC().Truncate(time.Second)
	logs := []*models.RequestLog{
		{ID: "stat-a-success-1", Timestamp: now, GroupID: 1, ResourceID: resources[0].ID, KeyHash: keys[0].KeyHash, IsSuccess: true, StatusCode: 200, RequestType: models.RequestTypeFinal},
		{ID: "stat-a-success-2", Timestamp: now.Add(time.Second), GroupID: 1, ResourceID: resources[0].ID, KeyHash: keys[0].KeyHash, IsSuccess: true, StatusCode: 200, RequestType: models.RequestTypeFinal},
		{ID: "stat-a-failure", Timestamp: now.Add(2 * time.Second), GroupID: 1, ResourceID: resources[0].ID, KeyHash: keys[0].KeyHash, IsSuccess: false, StatusCode: 500, RequestType: models.RequestTypeFinal},
		{ID: "stat-b-success", Timestamp: now.Add(3 * time.Second), GroupID: 1, ResourceID: resources[1].ID, KeyHash: keys[1].KeyHash, IsSuccess: true, StatusCode: 200, RequestType: models.RequestTypeFinal},
		{ID: "stat-b-404", Timestamp: now.Add(4 * time.Second), GroupID: 1, ResourceID: resources[1].ID, KeyHash: keys[1].KeyHash, IsSuccess: false, StatusCode: 404, RequestType: models.RequestTypeFinal},
	}
	if err := service.writeLogsToDB(logs); err != nil {
		t.Fatalf("write logs: %v", err)
	}

	for i := range keys {
		if err := db.First(&keys[i], keys[i].ID).Error; err != nil {
			t.Fatalf("reload key %d: %v", i, err)
		}
		if err := db.First(&resources[i], resources[i].ID).Error; err != nil {
			t.Fatalf("reload resource %d: %v", i, err)
		}
	}
	if keys[0].RequestCount != 2 || keys[0].TotalFailureCount != 1 || resources[0].RequestCount != 2 || resources[0].TotalFailureCount != 1 {
		t.Fatalf("unexpected credential A counters: key=%#v resource=%#v", keys[0], resources[0])
	}
	if keys[1].RequestCount != 1 || keys[1].TotalFailureCount != 0 || resources[1].RequestCount != 1 || resources[1].TotalFailureCount != 0 {
		t.Fatalf("unexpected credential B counters: key=%#v resource=%#v", keys[1], resources[1])
	}
}

func TestTOK001RequestLogsStoreReportedTokenUsage(t *testing.T) {
	db := newRequestLogServiceTestDB(t)
	service := &RequestLogService{db: db}
	log := &models.RequestLog{ID: "tok-upstream", GroupID: 1, IsSuccess: true, RequestType: models.RequestTypeFinal}
	ApplyUpstreamTokenUsage(log, TokenUsage{
		InputTokens:      10,
		OutputTokens:     5,
		TotalTokens:      15,
		CacheReadTokens:  2,
		CacheWriteTokens: 3,
		ThinkingTokens:   4,
	})

	if err := service.writeLogsToDB([]*models.RequestLog{log}); err != nil {
		t.Fatalf("write log: %v", err)
	}

	var stored models.RequestLog
	if err := db.First(&stored, "id = ?", "tok-upstream").Error; err != nil {
		t.Fatalf("load stored log: %v", err)
	}
	if stored.InputTokens != 10 || stored.OutputTokens != 5 || stored.TotalTokens != 15 ||
		stored.CacheReadTokens != 2 || stored.CacheWriteTokens != 3 || stored.ThinkingTokens != 4 ||
		stored.TokenUsageSource != models.TokenUsageSourceUpstream {
		t.Fatalf("unexpected stored token usage: %#v", stored)
	}
}

func TestTOK002EstimatedTokenUsageDoesNotOverwriteUpstreamUsage(t *testing.T) {
	log := &models.RequestLog{}
	ApplyUpstreamTokenUsage(log, TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	ApplyEstimatedTokenUsage(log, TokenUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2})

	if log.InputTokens != 10 || log.OutputTokens != 5 || log.TotalTokens != 15 || log.TokenUsageSource != models.TokenUsageSourceUpstream {
		t.Fatalf("estimated usage overwrote upstream usage: %#v", log)
	}

	estimatedOnly := &models.RequestLog{}
	ApplyEstimatedTokenUsage(estimatedOnly, TokenUsage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7})
	if estimatedOnly.InputTokens != 3 || estimatedOnly.OutputTokens != 4 || estimatedOnly.TotalTokens != 7 ||
		estimatedOnly.TokenUsageSource != models.TokenUsageSourceEstimated {
		t.Fatalf("estimated usage not recorded separately: %#v", estimatedOnly)
	}
}
