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
	LeaderService   *LeaderService
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewKeyCronService creates a new KeyCronService.
func NewKeyCronService(
	db *gorm.DB,
	settingsManager *config.SystemSettingsManager,
	pool *KeyValidationPool,
	leaderService *LeaderService,
) *KeyCronService {
	return &KeyCronService{
		DB:              db,
		SettingsManager: settingsManager,
		Pool:            pool,
		LeaderService:   leaderService,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the cron job execution.
func (s *KeyCronService) Start() {
	logrus.Info("Starting KeyCronService...")
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
				logrus.Info("KeyCronService: Running as leader, submitting validation jobs.")
				s.submitValidationJobs()
			} else {
				logrus.Debug("KeyCronService: Not the leader. Standing by.")
			}
		case <-s.stopChan:
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
