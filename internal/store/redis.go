package store

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Redis-backed key-value store.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new RedisStore instance.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Set stores a key-value pair in Redis.
func (s *RedisStore) Set(key string, value []byte, ttl time.Duration) error {
	return s.client.Set(context.Background(), key, value, ttl).Err()
}

// Get retrieves a value from Redis.
func (s *RedisStore) Get(key string) ([]byte, error) {
	val, err := s.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return val, nil
}

// Delete removes a value from Redis.
func (s *RedisStore) Delete(key string) error {
	return s.client.Del(context.Background(), key).Err()
}

// Exists checks if a key exists in Redis.
func (s *RedisStore) Exists(key string) (bool, error) {
	val, err := s.client.Exists(context.Background(), key).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// Close closes the Redis client connection.
func (s *RedisStore) Close() error {
	return s.client.Close()
}
