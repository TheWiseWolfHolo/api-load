package store

import (
	"errors"
	"time"
)

// ErrNotFound is the error returned when a key is not found in the store.
var ErrNotFound = errors.New("store: key not found")

// Store is a generic key-value store interface.
// Implementations of this interface must be safe for concurrent use.
type Store interface {
	// Set stores a key-value pair with an optional TTL.
	// - key: The key (string).
	// - value: The value ([]byte).
	// - ttl: The expiration time. If ttl is 0, the key never expires.
	Set(key string, value []byte, ttl time.Duration) error

	// Get retrieves a value by its key.
	// It must return store.ErrNotFound if the key does not exist.
	Get(key string) ([]byte, error)

	// Delete removes a value by its key.
	// If the key does not exist, this operation should be considered successful (idempotent) and not return an error.
	Delete(key string) error

	// Exists checks if a key exists in the store.
	Exists(key string) (bool, error)

	// Close closes the store and releases any underlying resources.
	Close() error
}
