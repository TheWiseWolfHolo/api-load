package services

import (
	"gpt-load/internal/config"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// KeyCronService is responsible for periodically validating invalid keys.
type KeyCronService struct {
	DB              *gorm.DB
	SettingsManager *config.SystemSettingsManager
	Validator       *keypool.KeyValidator
	LeaderService   *LeaderService
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewKeyCronService creates a new KeyCronService.
func NewKeyCronService(
	db *gorm.DB,
	settingsManager *config.SystemSettingsManager,
	validator *keypool.KeyValidator,
	leaderService *LeaderService,
) *KeyCronService {
	return &KeyCronService{
		DB:              db,
		SettingsManager: settingsManager,
		Validator:       validator,
		LeaderService:   leaderService,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the cron job execution.
func (s *KeyCronService) Start() {
	logrus.Debug("Starting KeyCronService...")
	s.wg.Add(1)
	go s.runLoop()
}

// Stop stops the cron job.
func (s *KeyCronService) Stop() {
	logrus.Info("Stopping KeyCronService...")
	close(s.stopChan)
	s.wg.Wait()
	logrus.Info("KeyCronService stopped.")
}

func (s *KeyCronService) runLoop() {
	defer s.wg.Done()

	if s.LeaderService.IsLeader() {
		s.submitValidationJobs()
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.LeaderService.IsLeader() {
				logrus.Debug("KeyCronService: Running as leader, submitting validation jobs.")
				s.submitValidationJobs()
			} else {
				logrus.Debug("KeyCronService: Not the leader. Standing by.")
			}
		case <-s.stopChan:
			return
		}
	}
}

// submitValidationJobs finds groups whose keys need validation and validates them.
func (s *KeyCronService) submitValidationJobs() {
	var groups []models.Group
	if err := s.DB.Find(&groups).Error; err != nil {
		logrus.Errorf("KeyCronService: Failed to get groups: %v", err)
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
				logrus.Errorf("KeyCronService: Failed to get invalid keys for group %s: %v", group.Name, err)
				continue
			}

			validatedCount := len(invalidKeys)
			becameValidCount := 0
			if validatedCount > 0 {
				logrus.Debugf("KeyCronService: Found %d invalid keys to validate for group %s.", validatedCount, group.Name)
				for j := range invalidKeys {
					key := &invalidKeys[j]
					isValid, _ := s.Validator.ValidateSingleKey(key, group)

					if isValid {
						becameValidCount++
					}
				}
			}

			if err := s.DB.Model(group).Update("last_validated_at", time.Now()).Error; err != nil {
				logrus.Errorf("KeyCronService: Failed to update last_validated_at for group %s: %v", group.Name, err)
			}

			duration := time.Since(groupProcessStart)
			logrus.Infof(
				"KeyCronService: Group '%s' validation finished. Total checked: %d, became valid: %d. Duration: %s.",
				group.Name,
				validatedCount,
				becameValidCount,
				duration.String(),
			)
		}
	}
}
