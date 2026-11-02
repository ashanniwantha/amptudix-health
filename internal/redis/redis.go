package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/ashanniwantha/amptudix-health/internal/config"
)

// NewClient constructs a go-redis client, verifies connectivity with a ping,
// and returns it. The caller owns Close().
func NewClient(ctx context.Context, cfg config.Redis) (*goredis.Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,

		// Connection pool: a single client multiplexes many goroutines,
		// so the pool size is the concurrency ceiling, not the number of
		// clients[reference:4].
		PoolSize:        20,
		MinIdleConns:    5,
		MaxIdleConns:    10,
		ConnMaxIdleTime: 5 * 60 * 1e9,  // 5 minutes
		ConnMaxLifetime: 30 * 60 * 1e9, // 30 minutes

		// Fail fast instead of hanging a request on a dead Redis.
		DialTimeout:  5e9, // 5s
		ReadTimeout:  3e9, // 3s
		WriteTimeout: 3e9, // 3s
		PoolTimeout:  4e9, // 4s
	})

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return rdb, nil
}
