// Package cache owns the Redis client used for caching and as the backing store
// for the background-job queue. As with the database pool, the client is
// constructed once and injected; nothing dials Redis independently.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"beacon/internal/config"
)

// Connect builds and verifies a Redis client.
func Connect(ctx context.Context, cfg config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: ping: %w", err)
	}
	return client, nil
}
