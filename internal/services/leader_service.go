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
	electionTimeout       = 15 * time.Second
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
		logrus.Info("Running in single-node mode.")
		service.isLeader.Store(true)
	} else {
		logrus.Info("Running in distributed mode.")
	}
	return service
}

// ElectLeader attempts to become the cluster leader. This is a blocking call.
func (s *LeaderService) ElectLeader() error {
	if s.isSingleNode {
		logrus.Info("In single-node mode, leadership is assumed. Skipping election.")
		return nil
	}

	logrus.WithField("nodeID", s.nodeID).Debug("Attempting to acquire leadership...")

	acquired, err := s.acquireLock()
	if err != nil {
		return fmt.Errorf("failed to acquire leader lock: %w", err)
	}

	if acquired {
		logrus.WithField("nodeID", s.nodeID).Info("Successfully acquired leadership. Starting renewal process.")
		s.isLeader.Store(true)
		s.wg.Add(1)
		go s.renewalLoop()
	} else {
		logrus.WithField("nodeID", s.nodeID).Info("Another node is already the leader.")
		s.isLeader.Store(false)
	}

	return nil
}

// Stop gracefully stops the leader renewal process if this node is the leader.
func (s *LeaderService) Stop() {
	if s.isSingleNode || !s.isLeader.Load() {
		return
	}
	logrus.Info("Stopping leader renewal process...")
	close(s.stopChan)
	s.wg.Wait()
	s.releaseLock()
	logrus.Info("Leader renewal process stopped.")
}

// IsLeader returns true if the current node is the leader.
func (s *LeaderService) IsLeader() bool {
	return s.isLeader.Load()
}

// renewalLoop is the background process that keeps the leader lock alive.
func (s *LeaderService) renewalLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(leaderRenewalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.renewLock(); err != nil {
				logrus.WithError(err).Error("Failed to renew leader lock, relinquishing leadership.")
				s.isLeader.Store(false)
				return
			}
			logrus.Debug("Successfully renewed leader lock.")
		case <-s.stopChan:
			logrus.Info("Leader renewal loop stopping.")
			return
		}
	}
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
	if !s.isLeader.Load() {
		return
	}
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
