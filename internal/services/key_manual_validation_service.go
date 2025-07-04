package services

import (
	"context"
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ManualValidationResult holds the result of a manual validation task.
type ManualValidationResult struct {
	TotalKeys   int `json:"total_keys"`
	ValidKeys   int `json:"valid_keys"`
	InvalidKeys int `json:"invalid_keys"`
}

// KeyManualValidationService handles user-initiated key validation for a group.
type KeyManualValidationService struct {
	DB              *gorm.DB
	Validator       *KeyValidatorService
	TaskService     *TaskService
	SettingsManager *config.SystemSettingsManager
}

// NewKeyManualValidationService creates a new KeyManualValidationService.
func NewKeyManualValidationService(db *gorm.DB, validator *KeyValidatorService, taskService *TaskService, settingsManager *config.SystemSettingsManager) *KeyManualValidationService {
	return &KeyManualValidationService{
		DB:              db,
		Validator:       validator,
		TaskService:     taskService,
		SettingsManager: settingsManager,
	}
}

// StartValidationTask starts a new manual validation task for a given group.
func (s *KeyManualValidationService) StartValidationTask(group *models.Group) (*TaskStatus, error) {
	var keys []models.APIKey
	if err := s.DB.Where("group_id = ?", group.ID).Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to get keys for group %s: %w", group.Name, err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys to validate in group %s", group.Name)
	}

	taskID := uuid.New().String()
	timeoutMinutes := s.SettingsManager.GetInt("key_validation_task_timeout_minutes", 60)
	timeout := time.Duration(timeoutMinutes) * time.Minute

	taskStatus, err := s.TaskService.StartTask(taskID, group.Name, len(keys), timeout)
	if err != nil {
		return nil, err // A task is already running
	}

	// Run the validation in a separate goroutine
	go s.runValidation(group, keys, taskStatus)

	return taskStatus, nil
}

func (s *KeyManualValidationService) runValidation(group *models.Group, keys []models.APIKey, task *TaskStatus) {
	defer s.TaskService.EndTask()

	logrus.Infof("Starting manual validation for group %s (TaskID: %s)", group.Name, task.TaskID)

	jobs := make(chan models.APIKey, len(keys))
	results := make(chan bool, len(keys))

	concurrency := s.SettingsManager.GetInt("key_validation_concurrency", 10)
	if concurrency <= 0 {
		concurrency = 10 // Fallback to a safe default
	}

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go s.validationWorker(&wg, group, jobs, results)
	}

	for _, key := range keys {
		jobs <- key
	}
	close(jobs)

	wg.Wait()
	close(results)

	validCount := 0
	processedCount := 0
	for isValid := range results {
		processedCount++
		if isValid {
			validCount++
		}
		// Update progress
		s.TaskService.UpdateProgress(processedCount)
	}

	result := ManualValidationResult{
		TotalKeys:   len(keys),
		ValidKeys:   validCount,
		InvalidKeys: len(keys) - validCount,
	}

	// Store the final result
	s.TaskService.StoreResult(task.TaskID, result)
	logrus.Infof("Manual validation finished for group %s (TaskID: %s): %+v", group.Name, task.TaskID, result)
}

func (s *KeyManualValidationService) validationWorker(wg *sync.WaitGroup, group *models.Group, jobs <-chan models.APIKey, results chan<- bool) {
	defer wg.Done()
	for key := range jobs {
		isValid, _ := s.Validator.ValidateSingleKey(context.Background(), &key, group)
		results <- isValid
	}
}
