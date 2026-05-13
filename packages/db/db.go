// Package db provides PostgreSQL connection pool setup.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds connection pool configuration loaded from config/dev.env.yaml.
type Config struct {
	DSN             string
	MaxOpenConns    int32
	MaxIdleConns    int32
	ConnMaxLifetime time.Duration
}

// NewPool creates and verifies a *pgxpool.Pool using the given config.
// The pool is ready to use immediately; call pool.Close() on shutdown.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: parse dsn: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = cfg.MaxOpenConns
	}
	if cfg.MaxIdleConns > 0 {
		poolCfg.MinConns = cfg.MaxIdleConns
	}
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return pool, nil
}
