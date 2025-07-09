package store

import (
	"errors"
	"time"
)

// ErrNotFound is the error returned when a key is not found in the store.
var ErrNotFound = errors.New("store: key not found")

// Message is the struct for received pub/sub messages.
type Message struct {
	Channel string
	Payload []byte
}

// Subscription represents an active subscription to a pub/sub channel.
type Subscription interface {
	// Channel returns the channel for receiving messages.
	Channel() <-chan *Message
	// Close unsubscribes and releases any resources associated with the subscription.
	Close() error
}

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

	// SetNX sets a key-value pair if the key does not already exist.
	// It returns true if the key was set, false otherwise.
	SetNX(key string, value []byte, ttl time.Duration) (bool, error)

	// HASH operations
	HSet(key string, values map[string]any) error
	HGetAll(key string) (map[string]string, error)
	HIncrBy(key, field string, incr int64) (int64, error)

	// LIST operations
	LPush(key string, values ...any) error
	LRem(key string, count int64, value any) error
	Rotate(key string) (string, error)

	// Close closes the store and releases any underlying resources.
	Close() error

	// Publish sends a message to a given channel.
	Publish(channel string, message []byte) error

	// Subscribe listens for messages on a given channel.
	// It returns a Subscription object that can be used to receive messages and to close the subscription.
	Subscribe(channel string) (Subscription, error)
}

// Pipeliner defines an interface for executing a batch of commands.
type Pipeliner interface {
	HSet(key string, values map[string]any)
	Exec() error
}

// RedisPipeliner is an optional interface that a Store can implement to provide pipelining.
type RedisPipeliner interface {
	Pipeline() Pipeliner
}

// LuaScripter is an optional interface that a Store can implement to provide Lua script execution.
type LuaScripter interface {
	Eval(script string, keys []string, args ...interface{}) (interface{}, error)
}
