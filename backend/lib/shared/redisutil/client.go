// Package redisutil provides helpers for constructing Redis clients from
// the shared config.RedisConfig. It selects between a single-node Client
// and a Sentinel FailoverClient based on whether MasterName + SentinelAddrs
// are both present.
package redisutil

import (
	"github.com/kite365/idcd/lib/shared/config"
	"github.com/redis/go-redis/v9"
)

// NewClientFromConfig returns a redis.UniversalClient configured from cfg.
//
// Selection logic:
//   - If cfg.MasterName != "" AND len(cfg.SentinelAddrs) > 0 → FailoverClient (Sentinel mode).
//   - Otherwise → plain Client (single-node mode, uses cfg.Addr).
//
// Optional tuning fields (DialTimeout, ReadTimeout, WriteTimeout, PoolSize)
// are applied when non-zero; zero values fall back to go-redis defaults.
func NewClientFromConfig(cfg config.RedisConfig) redis.UniversalClient {
	if cfg.MasterName != "" && len(cfg.SentinelAddrs) > 0 {
		opts := &redis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    cfg.SentinelAddrs,
			SentinelPassword: cfg.SentinelPassword,
			Password:         cfg.Password,
			DB:               cfg.DB,
		}
		if cfg.DialTimeout.Duration > 0 {
			opts.DialTimeout = cfg.DialTimeout.Duration
		}
		if cfg.ReadTimeout.Duration > 0 {
			opts.ReadTimeout = cfg.ReadTimeout.Duration
		}
		if cfg.WriteTimeout.Duration > 0 {
			opts.WriteTimeout = cfg.WriteTimeout.Duration
		}
		if cfg.PoolSize > 0 {
			opts.PoolSize = cfg.PoolSize
		}
		return redis.NewFailoverClient(opts)
	}

	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.DialTimeout.Duration > 0 {
		opts.DialTimeout = cfg.DialTimeout.Duration
	}
	if cfg.ReadTimeout.Duration > 0 {
		opts.ReadTimeout = cfg.ReadTimeout.Duration
	}
	if cfg.WriteTimeout.Duration > 0 {
		opts.WriteTimeout = cfg.WriteTimeout.Duration
	}
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	return redis.NewClient(opts)
}
