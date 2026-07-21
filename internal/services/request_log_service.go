package services

import (
	"api-load/internal/config"
	"api-load/internal/models"
	"api-load/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RequestLogCachePrefix    = "request_log:"
	PendingLogKeysSet        = "pending_log_keys"
	DefaultLogFlushBatchSize = 200
)

type credentialUsageStat struct {
	SuccessCount int64
	FailureCount int64
	LastSuccess  time.Time
	LastFailure  time.Time
}

type credentialStatCaseItem struct {
	Key  any
	Stat credentialUsageStat
}

// RequestLogService is responsible for managing request logs.
type RequestLogService struct {
	db              *gorm.DB
	store           store.Store
	settingsManager *config.SystemSettingsManager
	stopChan        chan struct{}
	wg              sync.WaitGroup
	ticker          *time.Ticker
	bufferMu        sync.Mutex
	bufferedLogs    []*models.RequestLog
	backpressure    LogBackpressureConfig
	droppedLogs     int64
}

type LogBackpressureConfig struct {
	BatchSize          int
	EmergencyThreshold int
	HardLimit          int
}

// NewRequestLogService creates a new RequestLogService instance
func NewRequestLogService(db *gorm.DB, store store.Store, sm *config.SystemSettingsManager) *RequestLogService {
	return &RequestLogService{
		db:              db,
		store:           store,
		settingsManager: sm,
		stopChan:        make(chan struct{}),
	}
}

func (s *RequestLogService) ConfigureBackpressure(config LogBackpressureConfig) {
	if config.BatchSize <= 0 {
		config.BatchSize = DefaultLogFlushBatchSize
	}
	s.bufferMu.Lock()
	s.backpressure = config
	s.bufferMu.Unlock()
}

func (s *RequestLogService) EnqueueBufferedLog(log *models.RequestLog) error {
	s.bufferMu.Lock()
	if s.backpressure.HardLimit > 0 && len(s.bufferedLogs) >= s.backpressure.HardLimit {
		if !log.IsSecurityWarning {
			s.droppedLogs++
			s.bufferMu.Unlock()
			return nil
		}
		if s.dropOldestNonSecurityWarningLocked() {
			s.droppedLogs++
		}
	}
	s.bufferedLogs = append(s.bufferedLogs, log)
	shouldEmergencyFlush := s.backpressure.EmergencyThreshold > 0 && len(s.bufferedLogs) > s.backpressure.EmergencyThreshold
	shouldBatchFlush := !shouldEmergencyFlush && s.backpressure.BatchSize > 0 && len(s.bufferedLogs) >= s.backpressure.BatchSize
	s.bufferMu.Unlock()

	if shouldEmergencyFlush {
		return s.FlushBufferedLogs()
	}
	if shouldBatchFlush {
		return s.FlushBufferedLogs()
	}
	return nil
}

func (s *RequestLogService) dropOldestNonSecurityWarningLocked() bool {
	for i, log := range s.bufferedLogs {
		if log == nil || !log.IsSecurityWarning {
			s.bufferedLogs = append(s.bufferedLogs[:i], s.bufferedLogs[i+1:]...)
			return true
		}
	}
	return false
}

func (s *RequestLogService) FlushBufferedLogs() error {
	for {
		batch := s.nextBufferedBatch()
		if len(batch) == 0 {
			return nil
		}
		if err := s.writeLogsToDB(batch); err != nil {
			s.requeueBufferedLogs(batch)
			return err
		}
	}
}

func (s *RequestLogService) nextBufferedBatch() []*models.RequestLog {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()
	if len(s.bufferedLogs) == 0 {
		return nil
	}
	batchSize := s.backpressure.BatchSize
	if batchSize <= 0 || batchSize > len(s.bufferedLogs) {
		batchSize = len(s.bufferedLogs)
	}
	batch := append([]*models.RequestLog(nil), s.bufferedLogs[:batchSize]...)
	s.bufferedLogs = append([]*models.RequestLog(nil), s.bufferedLogs[batchSize:]...)
	return batch
}

func (s *RequestLogService) requeueBufferedLogs(logs []*models.RequestLog) {
	s.bufferMu.Lock()
	s.bufferedLogs = append(logs, s.bufferedLogs...)
	s.bufferMu.Unlock()
}

func (s *RequestLogService) PendingLogCount() int {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()
	return len(s.bufferedLogs)
}

func (s *RequestLogService) DroppedLogCount() int64 {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()
	return s.droppedLogs
}

// Start initializes the service and starts the periodic flush routine
func (s *RequestLogService) Start() {
	s.wg.Add(1)
	go s.runLoop()
}

func (s *RequestLogService) runLoop() {
	defer s.wg.Done()

	// Initial flush on start
	s.flush()

	interval := time.Duration(s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = time.Minute
	}
	s.ticker = time.NewTicker(interval)
	defer s.ticker.Stop()

	for {
		select {
		case <-s.ticker.C:
			newInterval := time.Duration(s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes) * time.Minute
			if newInterval <= 0 {
				newInterval = time.Minute
			}
			if newInterval != interval {
				s.ticker.Reset(newInterval)
				interval = newInterval
				logrus.Debugf("Request log write interval updated to: %v", interval)
			}
			s.flush()
		case <-s.stopChan:
			return
		}
	}
}

// Stop gracefully stops the RequestLogService
func (s *RequestLogService) Stop(ctx context.Context) {
	close(s.stopChan)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.flush()
		logrus.Info("RequestLogService stopped gracefully.")
	case <-ctx.Done():
		logrus.Warn("RequestLogService stop timed out.")
	}
}

// Record logs a request to the database and cache
func (s *RequestLogService) Record(log *models.RequestLog) error {
	log.ID = uuid.NewString()
	log.Timestamp = time.Now()

	if s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes == 0 {
		return s.writeLogsToDB([]*models.RequestLog{log})
	}

	cacheKey := RequestLogCachePrefix + log.ID

	logBytes, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal request log: %w", err)
	}

	ttl := time.Duration(s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes*5) * time.Minute
	if err := s.store.Set(cacheKey, logBytes, ttl); err != nil {
		return err
	}

	return s.store.SAdd(PendingLogKeysSet, cacheKey)
}

// flush data from cache to database
func (s *RequestLogService) flush() {
	if s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes == 0 {
		logrus.Debug("Sync mode enabled, skipping scheduled log flush.")
		return
	}

	logrus.Debug("Master starting to flush request logs...")

	for {
		keys, err := s.store.SPopN(PendingLogKeysSet, DefaultLogFlushBatchSize)
		if err != nil {
			logrus.Errorf("Failed to pop pending log keys from store: %v", err)
			return
		}

		if len(keys) == 0 {
			return
		}

		logrus.Debugf("Popped %d request logs to flush.", len(keys))

		var logs []*models.RequestLog
		var processedKeys []string
		for _, key := range keys {
			logBytes, err := s.store.Get(key)
			if err != nil {
				if err == store.ErrNotFound {
					logrus.Warnf("Log key %s found in set but not in store, skipping.", key)
				} else {
					logrus.Warnf("Failed to get log for key %s: %v", key, err)
				}
				continue
			}
			var log models.RequestLog
			if err := json.Unmarshal(logBytes, &log); err != nil {
				logrus.Warnf("Failed to unmarshal log for key %s: %v", key, err)
				continue
			}
			logs = append(logs, &log)
			processedKeys = append(processedKeys, key)
		}

		if len(logs) == 0 {
			continue
		}

		err = s.writeLogsToDB(logs)

		if err != nil {
			logrus.Errorf("Failed to flush request logs batch, will retry next time. Error: %v", err)
			if len(keys) > 0 {
				keysToRetry := make([]any, len(keys))
				for i, k := range keys {
					keysToRetry[i] = k
				}
				if saddErr := s.store.SAdd(PendingLogKeysSet, keysToRetry...); saddErr != nil {
					logrus.Errorf("CRITICAL: Failed to re-add failed log keys to set: %v", saddErr)
				}
			}
			return
		}

		if len(processedKeys) > 0 {
			if err := s.store.Del(processedKeys...); err != nil {
				logrus.Errorf("Failed to delete flushed log bodies from store: %v", err)
			}
		}
		logrus.Infof("Successfully flushed %d request logs.", len(logs))
	}
}

// writeLogsToDB writes a batch of request logs to the database
func (s *RequestLogService) writeLogsToDB(logs []*models.RequestLog) error {
	if len(logs) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(logs, len(logs)).Error; err != nil {
			return fmt.Errorf("failed to batch insert request logs: %w", err)
		}

		keyStats := make(map[uint]map[string]credentialUsageStat)
		resourceStats := make(map[uint]credentialUsageStat)
		for _, log := range logs {
			if log == nil {
				continue
			}
			apply := func(stats credentialUsageStat) credentialUsageStat {
				if log.IsSuccess {
					stats.SuccessCount++
					if stats.LastSuccess.IsZero() || log.Timestamp.After(stats.LastSuccess) {
						stats.LastSuccess = log.Timestamp
					}
				} else if log.StatusCode != 404 {
					stats.FailureCount++
					if stats.LastFailure.IsZero() || log.Timestamp.After(stats.LastFailure) {
						stats.LastFailure = log.Timestamp
					}
				}
				return stats
			}
			if log.KeyHash != "" {
				if keyStats[log.GroupID] == nil {
					keyStats[log.GroupID] = make(map[string]credentialUsageStat)
				}
				keyStats[log.GroupID][log.KeyHash] = apply(keyStats[log.GroupID][log.KeyHash])
			}
			if log.ResourceID > 0 {
				resourceStats[log.ResourceID] = apply(resourceStats[log.ResourceID])
			}
		}

		for groupID, statsByHash := range keyStats {
			items := make([]credentialStatCaseItem, 0, len(statsByHash))
			hashes := make([]string, 0, len(statsByHash))
			for hash, stats := range statsByHash {
				items = append(items, credentialStatCaseItem{Key: hash, Stat: stats})
				hashes = append(hashes, hash)
			}
			updates := credentialStatCaseUpdates("key_hash", items)
			if len(updates) == 0 {
				continue
			}
			if err := tx.Model(&models.APIKey{}).Where("group_id = ? AND key_hash IN ?", groupID, hashes).Updates(updates).Error; err != nil {
				return fmt.Errorf("update api key stats: %w", err)
			}
		}
		if len(resourceStats) > 0 {
			items := make([]credentialStatCaseItem, 0, len(resourceStats))
			ids := make([]uint, 0, len(resourceStats))
			for resourceID, stats := range resourceStats {
				items = append(items, credentialStatCaseItem{Key: resourceID, Stat: stats})
				ids = append(ids, resourceID)
			}
			updates := credentialStatCaseUpdates("id", items)
			if len(updates) > 0 {
				if err := tx.Model(&models.UpstreamResource{}).Where("id IN ?", ids).Updates(updates).Error; err != nil {
					return fmt.Errorf("update upstream resource stats: %w", err)
				}
			}
		}

		// 更新统计表
		hourlyStats := make(map[struct {
			Time    time.Time
			GroupID uint
		}]struct{ Success, Failure int64 })
		for _, log := range logs {
			if log == nil {
				continue
			}
			if log.RequestType == models.RequestTypeRetry {
				continue
			}
			if !log.IsSuccess && log.StatusCode == 404 {
				continue
			}
			hourlyTime := log.Timestamp.Truncate(time.Hour)
			key := struct {
				Time    time.Time
				GroupID uint
			}{Time: hourlyTime, GroupID: log.GroupID}

			counts := hourlyStats[key]
			if log.IsSuccess {
				counts.Success++
			} else {
				counts.Failure++
			}
			hourlyStats[key] = counts

			if log.ParentGroupID > 0 {
				parentKey := struct {
					Time    time.Time
					GroupID uint
				}{Time: hourlyTime, GroupID: log.ParentGroupID}

				parentCounts := hourlyStats[parentKey]
				if log.IsSuccess {
					parentCounts.Success++
				} else {
					parentCounts.Failure++
				}
				hourlyStats[parentKey] = parentCounts
			}
		}

		if len(hourlyStats) > 0 {
			for key, counts := range hourlyStats {
				err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "time"}, {Name: "group_id"}},
					DoUpdates: clause.Assignments(map[string]any{
						"success_count": gorm.Expr("group_hourly_stats.success_count + ?", counts.Success),
						"failure_count": gorm.Expr("group_hourly_stats.failure_count + ?", counts.Failure),
						"updated_at":    time.Now(),
					}),
				}).Create(&models.GroupHourlyStat{
					Time:         key.Time,
					GroupID:      key.GroupID,
					SuccessCount: counts.Success,
					FailureCount: counts.Failure,
				}).Error

				if err != nil {
					return fmt.Errorf("failed to upsert group hourly stat: %w", err)
				}
			}
		}

		return nil
	})
}

func credentialStatCaseUpdates(keyColumn string, items []credentialStatCaseItem) map[string]any {
	updates := make(map[string]any, 5)
	if expr, ok := credentialIncrementCase(keyColumn, "request_count", items, func(item credentialStatCaseItem) int64 { return item.Stat.SuccessCount }); ok {
		updates["request_count"] = expr
	}
	if expr, ok := credentialIncrementCase(keyColumn, "total_failure_count", items, func(item credentialStatCaseItem) int64 { return item.Stat.FailureCount }); ok {
		updates["total_failure_count"] = expr
	}
	if expr, ok := credentialTimeCase(keyColumn, "last_used_at", items, func(item credentialStatCaseItem) time.Time { return item.Stat.LastSuccess }); ok {
		updates["last_used_at"] = expr
		updates["last_success_at"] = expr
	}
	if expr, ok := credentialTimeCase(keyColumn, "last_failure_at", items, func(item credentialStatCaseItem) time.Time { return item.Stat.LastFailure }); ok {
		updates["last_failure_at"] = expr
	}
	return updates
}

func credentialIncrementCase(keyColumn, targetColumn string, items []credentialStatCaseItem, value func(credentialStatCaseItem) int64) (clause.Expr, bool) {
	var builder strings.Builder
	fmt.Fprintf(&builder, "CASE %s ", keyColumn)
	args := make([]any, 0, len(items)*2)
	matched := false
	for _, item := range items {
		increment := value(item)
		if increment <= 0 {
			continue
		}
		builder.WriteString("WHEN ? THEN " + targetColumn + " + ? ")
		args = append(args, item.Key, increment)
		matched = true
	}
	builder.WriteString("ELSE " + targetColumn + " END")
	return gorm.Expr(builder.String(), args...), matched
}

func credentialTimeCase(keyColumn, targetColumn string, items []credentialStatCaseItem, value func(credentialStatCaseItem) time.Time) (clause.Expr, bool) {
	var builder strings.Builder
	fmt.Fprintf(&builder, "CASE %s ", keyColumn)
	args := make([]any, 0, len(items)*2)
	matched := false
	for _, item := range items {
		timestamp := value(item)
		if timestamp.IsZero() {
			continue
		}
		builder.WriteString("WHEN ? THEN ? ")
		args = append(args, item.Key, timestamp)
		matched = true
	}
	builder.WriteString("ELSE " + targetColumn + " END")
	return gorm.Expr(builder.String(), args...), matched
}
