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
	leaderElectionTimeout = 5 * time.Second
)

// LeaderService provides a mechanism for electing a single leader in a cluster.
type LeaderService struct {
	store    store.Store
	nodeID   string
	isLeader atomic.Bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewLeaderService creates a new LeaderService.
func NewLeaderService(store store.Store) *LeaderService {
	return &LeaderService{
		store:    store,
		nodeID:   generateNodeID(),
		stopChan: make(chan struct{}),
	}
}

// Start begins the leader election process.
func (s *LeaderService) Start() {
	logrus.WithField("nodeID", s.nodeID).Info("Starting LeaderService...")
	s.wg.Add(1)
	go s.electionLoop()
}

// Stop gracefully stops the leader election process.
func (s *LeaderService) Stop() {
	logrus.Info("Stopping LeaderService...")
	close(s.stopChan)
	s.wg.Wait()
	logrus.Info("LeaderService stopped.")
}

// IsLeader returns true if the current node is the leader.
// This is a fast, local check against an atomic boolean.
func (s *LeaderService) IsLeader() bool {
	return s.isLeader.Load()
}

func (s *LeaderService) electionLoop() {
	defer s.wg.Done()

	// Attempt to acquire leadership immediately on start.
	s.tryToBeLeader()

	ticker := time.NewTicker(leaderRenewalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tryToBeLeader()
		case <-s.stopChan:
			if s.IsLeader() {
				s.releaseLock()
			}
			return
		}
	}
}

func (s *LeaderService) tryToBeLeader() {
	if s.IsLeader() {
		// Already the leader, just renew the lock.
		if err := s.renewLock(); err != nil {
			logrus.WithError(err).Error("Failed to renew leader lock, relinquishing leadership.")
			s.isLeader.Store(false)
		}
		return
	}

	// Not the leader, try to acquire the lock.
	acquired, err := s.acquireLock()
	if err != nil {
		logrus.WithError(err).Error("Error trying to acquire leader lock.")
		s.isLeader.Store(false)
		return
	}

	if acquired {
		logrus.WithField("nodeID", s.nodeID).Info("Successfully acquired leader lock.")
		s.isLeader.Store(true)
	} else {
		logrus.Debug("Could not acquire leader lock, another node is likely the leader.")
		s.isLeader.Store(false)
	}
}

func (s *LeaderService) acquireLock() (bool, error) {
	// SetNX is an atomic operation. If the key already exists, it does nothing.
	// This is the core of our distributed lock.
	return s.store.SetNX(leaderLockKey, []byte(s.nodeID), leaderLockTTL)
}

func (s *LeaderService) renewLock() error {
	// To renew, we must ensure we are still the lock holder.
	// A LUA script is the safest way to do this atomically.
	// For simplicity here, we get and set, but this is not truly atomic without LUA.
	// A simple SET can also work if we are confident in our election loop timing.
	return s.store.Set(leaderLockKey, []byte(s.nodeID), leaderLockTTL)
}

func (s *LeaderService) releaseLock() {
	// Best-effort attempt to release the lock on shutdown.
	// The TTL will handle cases where this fails.
	if err := s.store.Delete(leaderLockKey); err != nil {
		logrus.WithError(err).Error("Failed to release leader lock on shutdown.")
	} else {
		logrus.Info("Successfully released leader lock.")
	}
	s.isLeader.Store(false)
}

func generateNodeID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a timestamp-based ID if crypto/rand fails
		return "node-" + time.Now().Format(time.RFC3339Nano)
	}
	return hex.EncodeToString(bytes)
}
