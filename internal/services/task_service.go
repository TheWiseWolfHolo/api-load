package services

import (
	"errors"
	"sync"
	"time"
)

// TaskStatus represents the status of a long-running task.
type TaskStatus struct {
	IsRunning   bool      `json:"is_running"`
	GroupName   string    `json:"group_name,omitempty"`
	Processed   int       `json:"processed,omitempty"`
	Total       int       `json:"total,omitempty"`
	TaskID      string    `json:"task_id,omitempty"`
	ExpiresAt   time.Time `json:"-"` // Internal field to handle zombie tasks
	lastUpdated time.Time
}

// TaskService manages the state of a single, global, long-running task.
type TaskService struct {
	mu           sync.Mutex
	status       TaskStatus
	resultsCache map[string]interface{}
	cacheOrder   []string
	maxCacheSize int
}

// NewTaskService creates a new TaskService.
func NewTaskService() *TaskService {
	return &TaskService{
		resultsCache: make(map[string]interface{}),
		cacheOrder:   make([]string, 0),
		maxCacheSize: 100, // Store results for the last 100 tasks
	}
}

// StartTask attempts to start a new task. It returns an error if a task is already running.
func (s *TaskService) StartTask(taskID, groupName string, total int, timeout time.Duration) (*TaskStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Zombie task check
	if s.status.IsRunning && time.Now().After(s.status.ExpiresAt) {
		// The previous task is considered a zombie, reset it.
		s.status = TaskStatus{}
	}

	if s.status.IsRunning {
		return nil, errors.New("a task is already running")
	}

	s.status = TaskStatus{
		IsRunning:   true,
		TaskID:      taskID,
		GroupName:   groupName,
		Total:       total,
		Processed:   0,
		ExpiresAt:   time.Now().Add(timeout),
		lastUpdated: time.Now(),
	}

	return &s.status, nil
}

// GetTaskStatus returns the current status of the task.
func (s *TaskService) GetTaskStatus() *TaskStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Zombie task check
	if s.status.IsRunning && time.Now().After(s.status.ExpiresAt) {
		s.status = TaskStatus{} // Reset if expired
	}

	// Return a copy to prevent race conditions on the caller's side
	statusCopy := s.status
	return &statusCopy
}

// UpdateProgress updates the progress of the current task.
func (s *TaskService) UpdateProgress(processed int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.status.IsRunning {
		return
	}

	s.status.Processed = processed
	s.status.lastUpdated = time.Now()
}

// EndTask marks the current task as finished.
func (s *TaskService) EndTask() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.IsRunning = false
}

// StoreResult stores the result of a finished task.
func (s *TaskService) StoreResult(taskID string, result interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.resultsCache[taskID]; !exists {
		if len(s.cacheOrder) >= s.maxCacheSize {
			oldestTaskID := s.cacheOrder[0]
			delete(s.resultsCache, oldestTaskID)
			s.cacheOrder = s.cacheOrder[1:]
		}
		s.cacheOrder = append(s.cacheOrder, taskID)
	}
	s.resultsCache[taskID] = result
}

// GetResult retrieves the result of a finished task.
func (s *TaskService) GetResult(taskID string) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, found := s.resultsCache[taskID]
	return result, found
}
