package services

import (
	"context"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	leaderLockKey = "cron:leader:key_validation"
	leaderLockTTL = 10 * time.Minute
)

// KeyCronService is responsible for periodically submitting keys for validation.
type KeyCronService struct {
	DB              *gorm.DB
	SettingsManager *config.SystemSettingsManager
	Pool            *KeyValidationPool
	Store           store.Store
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewKeyCronService creates a new KeyCronService.
func NewKeyCronService(db *gorm.DB, settingsManager *config.SystemSettingsManager, pool *KeyValidationPool, store store.Store) *KeyCronService {
	return &KeyCronService{
		DB:              db,
		SettingsManager: settingsManager,
		Pool:            pool,
		Store:           store,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the leader election and cron job execution.
func (s *KeyCronService) Start() {
	logrus.Info("Starting KeyCronService with leader election...")
	s.wg.Add(1)
	go s.leaderElectionLoop()

	// processResults still needs to run independently as it handles results from the Pool
	s.wg.Add(1)
	go s.processResults()
}

// Stop stops the cron job.
func (s *KeyCronService) Stop() {
	logrus.Info("Stopping KeyCronService...")
	close(s.stopChan)
	s.wg.Wait()
	logrus.Info("KeyCronService stopped.")
}

// leaderElectionLoop is the main loop that attempts to acquire leadership.
func (s *KeyCronService) leaderElectionLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			// Attempt to acquire the leader lock
			isLeader, err := s.tryAcquireLock()
			if err != nil {
				logrus.Errorf("KeyCronService: Error trying to acquire leader lock: %v. Retrying in 1 minute.", err)
				time.Sleep(1 * time.Minute) // Wait for a while before retrying on error
				continue
			}

			if isLeader {
				// Successfully became the leader, start executing the cron job
				s.runAsLeader()
			} else {
				// Failed to become the leader, enter standby mode
				logrus.Debug("KeyCronService: Not the leader. Standing by.")
				// Wait for a lock TTL duration before trying again to avoid frequent contention
				time.Sleep(leaderLockTTL)
			}
		}
	}
}

// tryAcquireLock attempts to set a key in the store, effectively acquiring a lock.
// This relies on an atomic operation if the underlying store supports it (like Redis SET NX).
func (s *KeyCronService) tryAcquireLock() (bool, error) {
	// A simple implementation for the generic store interface.
	// The RedisStore implementation should use SET NX for atomicity.
	exists, err := s.Store.Exists(leaderLockKey)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil // Lock is held by another node
	}

	// Attempt to set the lock. This is not atomic here but works in low-contention scenarios.
	// The robustness relies on the underlying store's implementation.
	lockValue := []byte(time.Now().String())
	err = s.Store.Set(leaderLockKey, lockValue, leaderLockTTL)
	if err != nil {
		// It's possible the lock was acquired by another node between the Exists and Set calls
		return false, err
	}

	logrus.Info("KeyCronService: Successfully acquired leader lock.")
	return true, nil
}

// runAsLeader contains the original logic that should only be run by the leader node.
func (s *KeyCronService) runAsLeader() {
	logrus.Info("KeyCronService: Running as leader.")
	// Defer releasing the lock
	defer func() {
		if err := s.Store.Delete(leaderLockKey); err != nil {
			logrus.Errorf("KeyCronService: Failed to release leader lock: %v", err)
		}
		logrus.Info("KeyCronService: Released leader lock.")
	}()

	// Run once on start
	s.submitValidationJobs()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	heartbeat := time.NewTicker(leaderLockTTL / 2)
	defer heartbeat.Stop()

	for {
		select {
		case <-ticker.C:
			s.submitValidationJobs()
		case <-heartbeat.C:
			// Renew the lock to prevent it from expiring during long-running tasks
			logrus.Debug("KeyCronService: Renewing leader lock.")
			err := s.Store.Set(leaderLockKey, []byte(time.Now().String()), leaderLockTTL)
			if err != nil {
				logrus.Errorf("KeyCronService: Failed to renew leader lock: %v. Relinquishing leadership.", err)
				return // Relinquish leadership on renewal failure
			}
		case <-s.stopChan:
			return // Service stopping
		}
	}
}

// processResults consumes results from the validation pool and updates the database.
func (s *KeyCronService) processResults() {
	defer s.wg.Done()
	keysToUpdate := make(map[uint]models.APIKey)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case result, ok := <-s.Pool.ResultsChannel():
			if !ok {
				s.batchUpdateKeyStatus(keysToUpdate)
				return
			}

			key := result.Job.Key
			var newStatus string
			var newErrorReason string

			if result.Error != nil {
				newStatus = "inactive"
				newErrorReason = result.Error.Error()
			} else {
				if result.IsValid {
					newStatus = "active"
					newErrorReason = ""
				} else {
					newStatus = "inactive"
					newErrorReason = "Validation returned false without a specific error."
				}
			}

			if key.Status != newStatus || key.ErrorReason != newErrorReason {
				key.Status = newStatus
				key.ErrorReason = newErrorReason
				keysToUpdate[key.ID] = key
			}

		case <-ticker.C:
			// Process batch on ticker interval
			if len(keysToUpdate) > 0 {
				s.batchUpdateKeyStatus(keysToUpdate)
				keysToUpdate = make(map[uint]models.APIKey)
			}
		case <-s.stopChan:
			// Process any remaining keys before stopping
			if len(keysToUpdate) > 0 {
				s.batchUpdateKeyStatus(keysToUpdate)
			}
			return
		}
	}
}

// submitValidationJobs finds groups and keys that need validation and submits them to the pool.
func (s *KeyCronService) submitValidationJobs() {
	logrus.Info("KeyCronService: Starting validation submission cycle.")
	var groups []models.Group
	if err := s.DB.Find(&groups).Error; err != nil {
		logrus.Errorf("KeyCronService: Failed to get groups: %v", err)
		return
	}

	validationStartTime := time.Now()
	groupsToUpdateTimestamp := make(map[uint]*models.Group)

	total := 0
	for i := range groups {
		group := &groups[i]
		effectiveSettings := s.SettingsManager.GetEffectiveConfig(group.Config)
		interval := time.Duration(effectiveSettings.KeyValidationIntervalMinutes) * time.Minute

		if group.LastValidatedAt == nil || validationStartTime.Sub(*group.LastValidatedAt) > interval {
			groupsToUpdateTimestamp[group.ID] = group
			var keys []models.APIKey
			if err := s.DB.Where("group_id = ?", group.ID).Find(&keys).Error; err != nil {
				logrus.Errorf("KeyCronService: Failed to get keys for group %s: %v", group.Name, err)
				continue
			}

			if len(keys) == 0 {
				continue
			}

			total += len(keys)

			logrus.Infof("KeyCronService: Submitting %d keys for group %s for validation.", len(keys), group.Name)

			for _, key := range keys {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

				job := ValidationJob{
					Key:        key,
					Group:      group,
					Ctx:        ctx,
					CancelFunc: cancel,
				}

				s.Pool.SubmitJob(job)
			}
		}
	}

	// Update timestamps for all groups that were due for validation
	if len(groupsToUpdateTimestamp) > 0 {
		s.updateGroupTimestamps(groupsToUpdateTimestamp, validationStartTime)
	}

	logrus.Infof("KeyCronService: Submitted %d keys for validation across %d groups.", total, len(groupsToUpdateTimestamp))
}

func (s *KeyCronService) updateGroupTimestamps(groups map[uint]*models.Group, validationStartTime time.Time) {
	var groupIDs []uint
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}
	if err := s.DB.Model(&models.Group{}).Where("id IN ?", groupIDs).Update("last_validated_at", validationStartTime).Error; err != nil {
		logrus.Errorf("KeyCronService: Failed to batch update last_validated_at for groups: %v", err)
	}
}

func (s *KeyCronService) batchUpdateKeyStatus(keysToUpdate map[uint]models.APIKey) {
	if len(keysToUpdate) == 0 {
		return
	}
	logrus.Infof("KeyCronService: Batch updating status for %d keys.", len(keysToUpdate))

	var keys []models.APIKey
	for _, key := range keysToUpdate {
		keys = append(keys, key)
	}

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		for _, key := range keys {
			updates := map[string]any{
				"status":       key.Status,
				"error_reason": key.ErrorReason,
			}
			if err := tx.Model(&models.APIKey{}).Where("id = ?", key.ID).Updates(updates).Error; err != nil {
				logrus.Errorf("KeyCronService: Failed to update key ID %d: %v", key.ID, err)
			}
		}
		return nil
	})

	if err != nil {
		logrus.Errorf("KeyCronService: Transaction failed during batch update of key statuses: %v", err)
	}
}
