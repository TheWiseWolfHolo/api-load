package store

import (
	"context"
	"fmt"
	"gpt-load/internal/types"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// NewStore creates a new store based on the application configuration.
// It prioritizes Redis if a DSN is provided, otherwise it falls back to an in-memory store.
func NewStore(cfg types.ConfigManager) (Store, error) {
	redisDSN := cfg.GetRedisDSN()
	// Prioritize Redis if configured
	if redisDSN != "" {
		logrus.Info("Redis DSN found, initializing Redis store...")
		opts, err := redis.ParseURL(redisDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redis DSN: %w", err)
		}

		client := redis.NewClient(opts)
		// Ping the server to ensure a connection is established.
		if err := client.Ping(context.Background()).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to redis: %w", err)
		}

		logrus.Info("Successfully connected to Redis.")
		return NewRedisStore(client), nil
	}

	// Fallback to in-memory store
	logrus.Info("Redis DSN not configured, falling back to in-memory store.")
	return NewMemoryStore(), nil
}
