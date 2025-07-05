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

// KeyCronService is responsible for periodically validating all API keys.
type KeyCronService struct {
	DB              *gorm.DB
	Validator       *KeyValidatorService
	SettingsManager *config.SystemSettingsManager
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewKeyCronService creates a new KeyCronService.
func NewKeyCronService(db *gorm.DB, validator *KeyValidatorService, settingsManager *config.SystemSettingsManager) *KeyCronService {
	return &KeyCronService{
		DB:              db,
		Validator:       validator,
		SettingsManager: settingsManager,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the cron job.
func (s *KeyCronService) Start() {
	logrus.Info("Starting KeyCronService...")
	s.wg.Add(1)
	go s.run()
}

// Stop stops the cron job.
func (s *KeyCronService) Stop() {
	logrus.Info("Stopping KeyCronService...")
	close(s.stopChan)
	s.wg.Wait()
	logrus.Info("KeyCronService stopped.")
}

func (s *KeyCronService) run() {
	defer s.wg.Done()
	ctx := context.Background()

	// Run once on start
	s.validateAllGroups(ctx)

	for {
		// Dynamically get the interval for the next run
		intervalMinutes := s.SettingsManager.GetInt("key_validation_interval_minutes", 60)
		if intervalMinutes <= 0 {
			intervalMinutes = 60 // Fallback to a safe default
		}
		nextRunTimer := time.NewTimer(time.Duration(intervalMinutes) * time.Minute)

		select {
		case <-nextRunTimer.C:
			s.validateAllGroups(ctx)
		case <-s.stopChan:
			nextRunTimer.Stop()
			return
		}
	}
}

func (s *KeyCronService) validateAllGroups(ctx context.Context) {
	logrus.Info("KeyCronService: Starting validation cycle for all groups.")
	var groups []models.Group
	if err := s.DB.Find(&groups).Error; err != nil {
		logrus.Errorf("KeyCronService: Failed to get groups: %v", err)
		return
	}

	for _, group := range groups {
		groupCopy := group // Create a copy for the closure
		go func(g models.Group) {
			// Get effective settings for the group
			effectiveSettings := s.SettingsManager.GetEffectiveConfig(g.Config)
			interval := time.Duration(effectiveSettings.KeyValidationIntervalMinutes) * time.Minute

			// Check if it's time to validate this group
			if g.LastValidatedAt == nil || time.Since(*g.LastValidatedAt) > interval {
				s.validateGroup(ctx, &g)
			}
		}(groupCopy)
	}
	logrus.Info("KeyCronService: Validation cycle finished.")
}

func (s *KeyCronService) validateGroup(ctx context.Context, group *models.Group) {
	var keys []models.APIKey
	if err := s.DB.Where("group_id = ?", group.ID).Find(&keys).Error; err != nil {
		logrus.Errorf("KeyCronService: Failed to get keys for group %s: %v", group.Name, err)
		return
	}

	if len(keys) == 0 {
		return
	}

	logrus.Infof("KeyCronService: Validating %d keys for group %s", len(keys), group.Name)

	jobs := make(chan models.APIKey, len(keys))
	results := make(chan models.APIKey, len(keys))

	concurrency := s.SettingsManager.GetInt("key_validation_concurrency", 10)
	if concurrency <= 0 {
		concurrency = 10 // Fallback to a safe default
	}

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go s.worker(ctx, &wg, group, jobs, results)
	}

	for _, key := range keys {
		jobs <- key
	}
	close(jobs)

	wg.Wait()
	close(results)

	var keysToUpdate []models.APIKey
	for key := range results {
		keysToUpdate = append(keysToUpdate, key)
	}

	if len(keysToUpdate) > 0 {
		s.batchUpdateKeyStatus(keysToUpdate)
	}

	// Update the last validated timestamp for the group
	if err := s.DB.Model(group).Update("last_validated_at", time.Now()).Error; err != nil {
		logrus.Errorf("KeyCronService: Failed to update last_validated_at for group %s: %v", group.Name, err)
	}
}

func (s *KeyCronService) worker(ctx context.Context, wg *sync.WaitGroup, group *models.Group, jobs <-chan models.APIKey, results chan<- models.APIKey) {
	defer wg.Done()
	for key := range jobs {
		isValid, validationErr := s.Validator.ValidateSingleKey(ctx, &key, group)

		var newStatus string
		var newErrorReason string
		statusChanged := false

		if validationErr != nil {
			// Validation failed, mark as inactive and record the reason
			newStatus = "inactive"
			newErrorReason = validationErr.Error()
		} else {
			// Validation succeeded
			if isValid {
				newStatus = "active"
				newErrorReason = ""
			} else {
				newStatus = "inactive"
				newErrorReason = "Validation returned false without a specific error."
			}
		}

		// Check if status or error reason has changed
		if key.Status != newStatus || key.ErrorReason != newErrorReason {
			statusChanged = true
		}

		if statusChanged {
			key.Status = newStatus
			key.ErrorReason = newErrorReason
			results <- key
		}
	}
}

func (s *KeyCronService) batchUpdateKeyStatus(keys []models.APIKey) {
	if len(keys) == 0 {
		return
	}
	logrus.Infof("KeyCronService: Batch updating status/reason for %d keys.", len(keys))

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		for _, key := range keys {
			updates := map[string]interface{}{
				"status":       key.Status,
				"error_reason": key.ErrorReason,
			}
			if err := tx.Model(&models.APIKey{}).Where("id = ?", key.ID).Updates(updates).Error; err != nil {
				// Log the error for this specific key but continue the transaction
				logrus.Errorf("KeyCronService: Failed to update key ID %d: %v", key.ID, err)
			}
		}
		return nil // Commit the transaction even if some updates failed
	})

	if err != nil {
		// This error is for the transaction itself, not individual updates
		logrus.Errorf("KeyCronService: Transaction failed during batch update of key statuses: %v", err)
	}
}
