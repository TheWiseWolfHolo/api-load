package services

import (
	"fmt"
	"sync/atomic"
	"testing"

	"gpt-load/internal/models"

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
