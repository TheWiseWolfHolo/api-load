package keypool

import (
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// NewCronChecker is responsible for periodically validating invalid keys.
type CronChecker struct {
	DB              *gorm.DB
	SettingsManager *config.SystemSettingsManager
	Validator       *KeyValidator
	LeaderLock      *store.LeaderLock
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewCronChecker creates a new CronChecker.
func NewCronChecker(
	db *gorm.DB,
	settingsManager *config.SystemSettingsManager,
	validator *KeyValidator,
	leaderLock *store.LeaderLock,
) *CronChecker {
	return &CronChecker{
		DB:              db,
		SettingsManager: settingsManager,
		Validator:       validator,
		LeaderLock:      leaderLock,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the cron job execution.
func (s *CronChecker) Start() {
	logrus.Debug("Starting CronChecker...")
	s.wg.Add(1)
	go s.runLoop()
}

// Stop stops the cron job.
func (s *CronChecker) Stop() {
	logrus.Info("Stopping CronChecker...")
	close(s.stopChan)
	s.wg.Wait()
	logrus.Info("CronChecker stopped.")
}

func (s *CronChecker) runLoop() {
	defer s.wg.Done()

	if s.LeaderLock.IsLeader() {
		s.submitValidationJobs()
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.LeaderLock.IsLeader() {
				logrus.Debug("CronChecker: Running as leader, submitting validation jobs.")
				s.submitValidationJobs()
			} else {
				logrus.Debug("CronChecker: Not the leader. Standing by.")
			}
		case <-s.stopChan:
			return
		}
	}
}

// submitValidationJobs finds groups whose keys need validation and validates them.
func (s *CronChecker) submitValidationJobs() {
	var groups []models.Group
	if err := s.DB.Find(&groups).Error; err != nil {
		logrus.Errorf("CronChecker: Failed to get groups: %v", err)
		return
	}

	validationStartTime := time.Now()

	for i := range groups {
		group := &groups[i]
		effectiveSettings := s.SettingsManager.GetEffectiveConfig(group.Config)
		interval := time.Duration(effectiveSettings.KeyValidationIntervalMinutes) * time.Minute

		if group.LastValidatedAt == nil || validationStartTime.Sub(*group.LastValidatedAt) > interval {
			groupProcessStart := time.Now()
			var invalidKeys []models.APIKey
			err := s.DB.Where("group_id = ? AND status = ?", group.ID, models.KeyStatusInvalid).Find(&invalidKeys).Error
			if err != nil {
				logrus.Errorf("CronChecker: Failed to get invalid keys for group %s: %v", group.Name, err)
				continue
			}

			validatedCount := len(invalidKeys)
			becameValidCount := 0
			if validatedCount > 0 {
				logrus.Debugf("CronChecker: Found %d invalid keys to validate for group %s.", validatedCount, group.Name)
				for j := range invalidKeys {
					key := &invalidKeys[j]
					isValid, _ := s.Validator.ValidateSingleKey(key, group)

					if isValid {
						becameValidCount++
					}
				}
			}

			if err := s.DB.Model(group).Update("last_validated_at", time.Now()).Error; err != nil {
				logrus.Errorf("CronChecker: Failed to update last_validated_at for group %s: %v", group.Name, err)
			}

			duration := time.Since(groupProcessStart)
			logrus.Infof(
				"CronChecker: Group '%s' validation finished. Total checked: %d, became valid: %d. Duration: %s.",
				group.Name,
				validatedCount,
				becameValidCount,
				duration.String(),
			)
		}
	}
}
