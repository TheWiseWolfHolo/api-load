package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gpt-load/internal/store"

	"github.com/sirupsen/logrus"
)

const (
	leaderLockKey         = "cluster:leader"
	leaderLockTTL         = 30 * time.Second
	leaderRenewalInterval = 10 * time.Second
)

const renewLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], ARGV[2])
else
    return 0
end`

const releaseLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end`

// LeaderService provides a mechanism for electing a single leader in a cluster.
type LeaderService struct {
	store        store.Store
	nodeID       string
	isLeader     atomic.Bool
	stopChan     chan struct{}
	wg           sync.WaitGroup
	isSingleNode bool
}

// NewLeaderService creates a new LeaderService.
func NewLeaderService(s store.Store) *LeaderService {
	_, isDistributed := s.(store.LuaScripter)
	service := &LeaderService{
		store:        s,
		nodeID:       generateNodeID(),
		stopChan:     make(chan struct{}),
		isSingleNode: !isDistributed,
	}
	if service.isSingleNode {
		logrus.Debug("Running in single-node mode. Assuming leadership.")
		service.isLeader.Store(true)
	} else {
		logrus.Debug("Running in distributed mode.")
	}
	return service
}

// Start performs an initial leader election and starts the background leadership maintenance loop.
func (s *LeaderService) Start() error {
	if s.isSingleNode {
		return nil
	}

	if err := s.tryToBeLeader(); err != nil {
		return fmt.Errorf("initial leader election failed: %w", err)
	}

	s.wg.Add(1)
	go s.maintainLeadershipLoop()

	return nil
}

// Stop gracefully stops the leadership maintenance process.
func (s *LeaderService) Stop() {
	if s.isSingleNode {
		return
	}
	logrus.Info("Stopping leadership maintenance process...")
	close(s.stopChan)
	s.wg.Wait()

	if s.isLeader.Load() {
		s.releaseLock()
	}
	logrus.Info("Leadership maintenance process stopped.")
}

// IsLeader returns true if the current node is the leader.
func (s *LeaderService) IsLeader() bool {
	return s.isLeader.Load()
}

// maintainLeadershipLoop is the background process that keeps trying to acquire or renew the lock.
func (s *LeaderService) maintainLeadershipLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(leaderRenewalInterval)
	defer ticker.Stop()

	logrus.Info("Leadership maintenance loop started.")
	for {
		select {
		case <-ticker.C:
			if err := s.tryToBeLeader(); err != nil {
				logrus.WithError(err).Warn("Error during leadership maintenance cycle.")
			}
		case <-s.stopChan:
			logrus.Info("Leadership maintenance loop stopping.")
			return
		}
	}
}

// tryToBeLeader is an idempotent function that attempts to acquire or renew the lock.
func (s *LeaderService) tryToBeLeader() error {
	if s.isLeader.Load() {
		err := s.renewLock()
		if err != nil {
			logrus.WithError(err).Error("Failed to renew leader lock, relinquishing leadership.")
			s.isLeader.Store(false)
		}
		return nil
	}

	acquired, err := s.acquireLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if acquired {
		logrus.WithField("nodeID", s.nodeID).Info("Successfully acquired leadership.")
		s.isLeader.Store(true)
	}
	return nil
}

func (s *LeaderService) acquireLock() (bool, error) {
	return s.store.SetNX(leaderLockKey, []byte(s.nodeID), leaderLockTTL)
}

func (s *LeaderService) renewLock() error {
	luaStore := s.store.(store.LuaScripter)
	ttlSeconds := int(leaderLockTTL.Seconds())
	res, err := luaStore.Eval(renewLockScript, []string{leaderLockKey}, s.nodeID, ttlSeconds)
	if err != nil {
		return err
	}
	if i, ok := res.(int64); !ok || i == 0 {
		return fmt.Errorf("failed to renew lock, another node may have taken over")
	}
	return nil
}

func (s *LeaderService) releaseLock() {
	luaStore := s.store.(store.LuaScripter)
	if _, err := luaStore.Eval(releaseLockScript, []string{leaderLockKey}, s.nodeID); err != nil {
		logrus.WithError(err).Error("Failed to release leader lock on shutdown.")
	} else {
		logrus.Info("Successfully released leader lock.")
	}
}

func generateNodeID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "node-" + time.Now().Format(time.RFC3339Nano)
	}
	return hex.EncodeToString(bytes)
}
