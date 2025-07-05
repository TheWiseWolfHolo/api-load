package services

import (
	"context"
	"gpt-load/internal/models"
	"gpt-load/internal/types"
	"sync"

	"github.com/sirupsen/logrus"
)

// ValidationJob represents a single key validation task for the worker pool.
type ValidationJob struct {
	TaskID     string
	Key        models.APIKey
	Group      *models.Group
	Ctx        context.Context
	CancelFunc context.CancelFunc
}

// ValidationResult holds the outcome of a validation job.
type ValidationResult struct {
	Job     ValidationJob
	IsValid bool
	Error   error
}

// KeyValidationPool manages a global worker pool for key validation.
type KeyValidationPool struct {
	validator     *KeyValidatorService
	configManager types.ConfigManager
	jobs          chan ValidationJob
	results       chan ValidationResult // 定时任务结果
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewKeyValidationPool creates a new KeyValidationPool.
func NewKeyValidationPool(validator *KeyValidatorService, configManager types.ConfigManager) *KeyValidationPool {
	return &KeyValidationPool{
		validator:     validator,
		configManager: configManager,
		jobs:          make(chan ValidationJob, 1024),
		results:       make(chan ValidationResult, 1024),
		stopChan:      make(chan struct{}),
	}
}

// Start initializes and runs the worker pool.
func (p *KeyValidationPool) Start() {
	performanceConfig := p.configManager.GetPerformanceConfig()
	concurrency := performanceConfig.KeyValidationPoolSize
	if concurrency <= 0 {
		concurrency = 10
	}

	logrus.Infof("Starting KeyValidationPool with %d workers...", concurrency)

	p.wg.Add(concurrency)
	for range concurrency {
		go p.worker()
	}
}

// Stop gracefully stops the worker pool.
func (p *KeyValidationPool) Stop() {
	logrus.Info("Stopping KeyValidationPool...")
	close(p.stopChan)
	close(p.jobs)
	p.wg.Wait()

	// 关闭结果通道
	close(p.results)

	logrus.Info("KeyValidationPool stopped.")
}

// worker is a single goroutine that processes jobs.
func (p *KeyValidationPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			ctx := job.Ctx
			if ctx == nil {
				ctx = context.Background()
			}
			isValid, err := p.validator.ValidateSingleKey(ctx, &job.Key, job.Group)
			if job.CancelFunc != nil {
				job.CancelFunc()
			}
			result := ValidationResult{
				Job:     job,
				IsValid: isValid,
				Error:   err,
			}

			// Block until the result can be sent or the pool is stopped.
			// This provides back-pressure and prevents result loss.
			select {
			case p.results <- result:
			case <-p.stopChan:
				logrus.Infof("Worker stopping, discarding result for key %d", job.Key.ID)
				return
			}
		case <-p.stopChan:
			return
		}
	}
}

// SubmitJob adds a new validation job to the pool.
func (p *KeyValidationPool) SubmitJob(job ValidationJob) {
	p.jobs <- job
}

// ResultsChannel returns the channel for reading validation results.
func (p *KeyValidationPool) ResultsChannel() <-chan ValidationResult {
	return p.results
}
