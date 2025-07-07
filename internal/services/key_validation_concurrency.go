package services

import (
	"context"
	"gpt-load/internal/keypool"
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

// KeyValidationPool manages a global worker pool for key validation.
type KeyValidationPool struct {
	validator     *keypool.KeyValidator
	configManager types.ConfigManager
	jobs          chan ValidationJob
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewKeyValidationPool creates a new KeyValidationPool.
func NewKeyValidationPool(validator *keypool.KeyValidator, configManager types.ConfigManager) *KeyValidationPool {
	return &KeyValidationPool{
		validator:     validator,
		configManager: configManager,
		jobs:     make(chan ValidationJob, 1024),
		stopChan: make(chan struct{}),
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
			p.validator.ValidateSingleKey(ctx, &job.Key, job.Group)
			if job.CancelFunc != nil {
				job.CancelFunc()
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
