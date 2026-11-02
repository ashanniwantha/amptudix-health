package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ashanniwantha/amptudix-health/internal/config"
)

// NewPool creates a pgxpool from the typed config, verifies connectivity
// with a ping, and returns a ready-to-use pool. The caller owns Close().
func NewPool(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}

	// Pool sizing and lifecycle. These are the values the official pgx
	// examples and production guides converge on for a single API instance:
	//   MaxConns 25, MaxConnLifetime 30m, MaxConnIdleTime 5m,
	//   HealthCheckPeriod 1m[reference:1].
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgxpool: %w", err)
	}

	// NewWithConfig is lazy — it does not dial until first use. Ping forces
	// a real connection so startup fails loudly if the DB is unreachable[reference:2].
	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
