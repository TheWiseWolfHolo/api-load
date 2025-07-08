package services

import (
	"crypto/rand"
	"encoding/hex"
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

// Lua script for atomic lock renewal.
// KEYS[1]: lock key, ARGV[1]: node ID, ARGV[2]: TTL in seconds.
const renewLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], ARGV[2])
else
    return 0
end`

// Lua script for atomic lock release.
// KEYS[1]: lock key, ARGV[1]: node ID.
const releaseLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end`

// LeaderService provides a mechanism for electing a single leader in a cluster.
type LeaderService struct {
	store             store.Store
	nodeID            string
	isLeader          atomic.Bool
	stopChan          chan struct{}
	wg                sync.WaitGroup
	isSingleNode      bool
	firstElectionDone chan struct{}
	firstElectionOnce sync.Once
}

// NewLeaderService creates a new LeaderService.
func NewLeaderService(s store.Store) *LeaderService {
	// Check if the store supports Lua scripting to determine if we are in a distributed environment.
	_, isDistributed := s.(store.LuaScripter)

	service := &LeaderService{
		store:             s,
		nodeID:            generateNodeID(),
		stopChan:          make(chan struct{}),
		isSingleNode:      !isDistributed,
		firstElectionDone: make(chan struct{}),
	}

	if service.isSingleNode {
		logrus.Info("Store does not support Lua, running in single-node mode. Assuming leadership.")
		service.isLeader.Store(true)
		close(service.firstElectionDone)
	} else {
		logrus.Info("Store supports Lua, running in distributed mode.")
	}

	return service
}

// Start begins the leader election process.
func (s *LeaderService) Start() {
	if s.isSingleNode {
		return
	}
	s.wg.Add(1)
	go s.electionLoop()
}

// Stop gracefully stops the leader election process.
func (s *LeaderService) Stop() {
	if s.isSingleNode {
		return
	}
	close(s.stopChan)
	s.wg.Wait()
}

// IsLeader returns true if the current node is the leader.
// In distributed mode, this call will block until the first election attempt is complete.
func (s *LeaderService) IsLeader() bool {
	<-s.firstElectionDone
	return s.isLeader.Load()
}

func (s *LeaderService) electionLoop() {
	defer s.wg.Done()
	logrus.WithField("nodeID", s.nodeID).Info("Starting leader election loop...")

	// Attempt to acquire leadership immediately on start.
	s.tryToBeLeader()

	ticker := time.NewTicker(leaderRenewalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tryToBeLeader()
		case <-s.stopChan:
			logrus.Info("Stopping leader election loop...")
			if s.isLeader.Load() {
				s.releaseLock()
			}
			return
		}
	}
}

func (s *LeaderService) tryToBeLeader() {
	defer s.firstElectionOnce.Do(func() {
		close(s.firstElectionDone)
	})

	if s.isLeader.Load() {
		if err := s.renewLock(); err != nil {
			logrus.WithError(err).Error("Failed to renew leader lock, relinquishing leadership.")
			s.isLeader.Store(false)
		}
		return
	}

	acquired, err := s.acquireLock()
	if err != nil {
		logrus.WithError(err).Error("Error trying to acquire leader lock.")
		s.isLeader.Store(false)
		return
	}

	if acquired {
		logrus.WithField("nodeID", s.nodeID).Info("Successfully acquired leader lock.")
		s.isLeader.Store(true)
	}
}

func (s *LeaderService) acquireLock() (bool, error) {
	return s.store.SetNX(leaderLockKey, []byte(s.nodeID), leaderLockTTL)
}

func (s *LeaderService) renewLock() error {
	luaStore := s.store.(store.LuaScripter) // Already checked in NewLeaderService
	ttlSeconds := int(leaderLockTTL.Seconds())

	res, err := luaStore.Eval(renewLockScript, []string{leaderLockKey}, s.nodeID, ttlSeconds)
	if err != nil {
		return err
	}

	if i, ok := res.(int64); !ok || i == 0 {
		return store.ErrNotFound // Not our lock anymore
	}
	return nil
}

func (s *LeaderService) releaseLock() {
	luaStore := s.store.(store.LuaScripter) // Already checked in NewLeaderService
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
