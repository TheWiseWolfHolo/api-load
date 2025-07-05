package services

import (
	"context"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// KeyCronService is responsible for periodically submitting keys for validation.
type KeyCronService struct {
	DB              *gorm.DB
	SettingsManager *config.SystemSettingsManager
	Pool            *KeyValidationPool
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewKeyCronService creates a new KeyCronService.
func NewKeyCronService(db *gorm.DB, settingsManager *config.SystemSettingsManager, pool *KeyValidationPool) *KeyCronService {
	return &KeyCronService{
		DB:              db,
		SettingsManager: settingsManager,
		Pool:            pool,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the cron job and the results processor.
func (s *KeyCronService) Start() {
	logrus.Info("Starting KeyCronService...")
	s.wg.Add(2)
	go s.run()
	go s.processResults()
}

// Stop stops the cron job.
func (s *KeyCronService) Stop() {
	logrus.Info("Stopping KeyCronService...")
	close(s.stopChan)
	s.wg.Wait()
	logrus.Info("KeyCronService stopped.")
}

// run is the main ticker loop that triggers validation cycles.
func (s *KeyCronService) run() {
	defer s.wg.Done()
	// Run once on start, then start the ticker.
	s.submitValidationJobs()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.submitValidationJobs()
		case <-s.stopChan:
			return
		}
	}
}

// processResults consumes results from the validation pool and updates the database.
func (s *KeyCronService) processResults() {
	defer s.wg.Done()
	keysToUpdate := make(map[uint]models.APIKey)

	// Process results in batches to avoid constant DB writes.
	// This ticker defines the maximum delay for a batch update.
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

			logrus.Infof("KeyCronService: Submitting %d keys for group %s for validation.", len(keys), group.Name)

			for _, key := range keys {
				// Create a new context with timeout for each job
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
	logrus.Info("KeyCronService: Validation submission cycle finished.")
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
