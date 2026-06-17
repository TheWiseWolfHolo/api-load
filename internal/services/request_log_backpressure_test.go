package services

import (
	"testing"

	"gpt-load/internal/models"
)

func TestLOG001RequestLogsFlushInBatches(t *testing.T) {
	db := newRequestLogServiceTestDB(t)
	service := &RequestLogService{db: db}
	service.ConfigureBackpressure(LogBackpressureConfig{BatchSize: 2, EmergencyThreshold: 10, HardLimit: 10})

	if err := service.EnqueueBufferedLog(&models.RequestLog{ID: "log-1", GroupID: 1, RequestType: models.RequestTypeFinal}); err != nil {
		t.Fatalf("enqueue log 1: %v", err)
	}
	if err := service.EnqueueBufferedLog(&models.RequestLog{ID: "log-2", GroupID: 1, RequestType: models.RequestTypeFinal}); err != nil {
		t.Fatalf("enqueue log 2: %v", err)
	}

	var count int64
	if err := db.Model(&models.RequestLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if count != 2 || service.PendingLogCount() != 0 {
		t.Fatalf("unexpected flush result count=%d pending=%d", count, service.PendingLogCount())
	}
}

func TestLOG002EmergencyFlushTriggersWhenPendingExceedsThreshold(t *testing.T) {
	db := newRequestLogServiceTestDB(t)
	service := &RequestLogService{db: db}
	service.ConfigureBackpressure(LogBackpressureConfig{BatchSize: 10, EmergencyThreshold: 2, HardLimit: 10})

	for _, id := range []string{"emergency-1", "emergency-2", "emergency-3"} {
		if err := service.EnqueueBufferedLog(&models.RequestLog{ID: id, GroupID: 1, RequestType: models.RequestTypeFinal}); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}

	var count int64
	if err := db.Model(&models.RequestLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if count != 3 || service.PendingLogCount() != 0 {
		t.Fatalf("expected emergency flush, count=%d pending=%d", count, service.PendingLogCount())
	}
}

func TestLOG003DroppedLogCountRecordedWhenHardLimitExceeded(t *testing.T) {
	db := newRequestLogServiceTestDB(t)
	service := &RequestLogService{db: db}
	service.ConfigureBackpressure(LogBackpressureConfig{BatchSize: 10, EmergencyThreshold: 10, HardLimit: 2})

	for _, id := range []string{"drop-1", "drop-2", "drop-3"} {
		if err := service.EnqueueBufferedLog(&models.RequestLog{ID: id, GroupID: 1, RequestType: models.RequestTypeFinal}); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}
	if service.DroppedLogCount() != 1 || service.PendingLogCount() != 2 {
		t.Fatalf("unexpected drop accounting dropped=%d pending=%d", service.DroppedLogCount(), service.PendingLogCount())
	}
}

func TestLOG003SecurityWarningLogsAreNotDroppedAtHardLimit(t *testing.T) {
	db := newRequestLogServiceTestDB(t)
	service := &RequestLogService{db: db}
	service.ConfigureBackpressure(LogBackpressureConfig{BatchSize: 10, EmergencyThreshold: 10, HardLimit: 2})

	for _, id := range []string{"normal-1", "normal-2"} {
		if err := service.EnqueueBufferedLog(&models.RequestLog{ID: id, GroupID: 1, RequestType: models.RequestTypeFinal}); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}
	if err := service.EnqueueBufferedLog(&models.RequestLog{ID: "security-warning", GroupID: 1, RequestType: models.RequestTypeFinal, IsSecurityWarning: true}); err != nil {
		t.Fatalf("enqueue security warning: %v", err)
	}
	if err := service.FlushBufferedLogs(); err != nil {
		t.Fatalf("flush buffered logs: %v", err)
	}

	var warningCount int64
	if err := db.Model(&models.RequestLog{}).Where("id = ?", "security-warning").Count(&warningCount).Error; err != nil {
		t.Fatalf("count security warning logs: %v", err)
	}
	if warningCount != 1 || service.DroppedLogCount() != 1 {
		t.Fatalf("security warning was dropped or drop count was wrong warning=%d dropped=%d", warningCount, service.DroppedLogCount())
	}
}
