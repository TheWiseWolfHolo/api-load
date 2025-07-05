package store

import (
	"sync"
	"time"
)

// memoryStoreItem holds the value and expiration timestamp for a key.
type memoryStoreItem struct {
	value     []byte
	expiresAt int64 // Unix-nano timestamp. 0 for no expiry.
}

// MemoryStore is an in-memory key-value store that is safe for concurrent use.
type MemoryStore struct {
	mu     sync.RWMutex
	data   map[string]memoryStoreItem
	stopCh chan struct{} // Channel to stop the cleanup goroutine
}

// NewMemoryStore creates and returns a new MemoryStore instance.
// It also starts a background goroutine to periodically clean up expired keys.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		data:   make(map[string]memoryStoreItem),
		stopCh: make(chan struct{}),
	}
	go s.cleanupLoop(1 * time.Minute)
	return s
}

// Close stops the background cleanup goroutine.
func (s *MemoryStore) Close() error {
	close(s.stopCh)
	return nil
}

// cleanupLoop periodically iterates through the store and removes expired keys.
func (s *MemoryStore) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now().UnixNano()
			for key, item := range s.data {
				if item.expiresAt > 0 && now > item.expiresAt {
					delete(s.data, key)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// Set stores a key-value pair.
func (s *MemoryStore) Set(key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expiresAt int64
	if ttl > 0 {
		expiresAt = time.Now().UnixNano() + ttl.Nanoseconds()
	}

	s.data[key] = memoryStoreItem{
		value:     value,
		expiresAt: expiresAt,
	}
	return nil
}

// Get retrieves a value by its key.
func (s *MemoryStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	item, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrNotFound
	}

	// Check for expiration
	if item.expiresAt > 0 && time.Now().UnixNano() > item.expiresAt {
		// Lazy deletion
		s.mu.Lock()
		delete(s.data, key)
		s.mu.Unlock()
		return nil, ErrNotFound
	}

	return item.value, nil
}

// Delete removes a value by its key.
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// Exists checks if a key exists.
func (s *MemoryStore) Exists(key string) (bool, error) {
	s.mu.RLock()
	item, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return false, nil
	}

	if item.expiresAt > 0 && time.Now().UnixNano() > item.expiresAt {
		// Lazy deletion
		s.mu.Lock()
		delete(s.data, key)
		s.mu.Unlock()
		return false, nil
	}

	return true, nil
}
