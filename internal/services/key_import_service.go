package services

import (
	"api-load/internal/models"
	"fmt"

	"github.com/sirupsen/logrus"
)

// KeyImportResult holds the result of an import task.
type KeyImportResult struct {
	AddedCount     int `json:"added_count"`
	IgnoredCount   int `json:"ignored_count"`
	DuplicateCount int `json:"duplicate_count"`
	UpdatedCount   int `json:"updated_count"`
}

// KeyImportService handles the asynchronous import of a large number of keys.
type KeyImportService struct {
	TaskService *TaskService
	KeyService  *KeyService
}

// NewKeyImportService creates a new KeyImportService.
func NewKeyImportService(taskService *TaskService, keyService *KeyService) *KeyImportService {
	return &KeyImportService{
		TaskService: taskService,
		KeyService:  keyService,
	}
}

// StartImportTask initiates a new asynchronous key import task.
func (s *KeyImportService) StartImportTask(group *models.Group, keysText string) (*TaskStatus, error) {
	records, err := ParseKeyImportInput(keysText)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no valid keys found in the input text")
	}

	initialStatus, err := s.TaskService.StartTask(TaskTypeKeyImport, group.Name, len(records))
	if err != nil {
		return nil, err
	}

	go s.runImport(group, records)

	return initialStatus, nil
}

func (s *KeyImportService) runImport(group *models.Group, records []KeyImportRecord) {
	result, err := s.KeyService.ImportKeyRecords(group.ID, records, KeyImportOptions{DuplicatePolicy: DuplicatePolicyKeep})
	if err != nil {
		if endErr := s.TaskService.EndTask(nil, err); endErr != nil {
			logrus.Errorf("Failed to end task with error for group %d: %v (original error: %v)", group.ID, endErr, err)
		}
		return
	}

	if err := s.TaskService.UpdateProgress(len(records)); err != nil {
		logrus.Warnf("Failed to update task progress for group %d: %v", group.ID, err)
	}

	if endErr := s.TaskService.EndTask(*result, nil); endErr != nil {
		logrus.Errorf("Failed to end task with success result for group %d: %v", group.ID, endErr)
	}
}
