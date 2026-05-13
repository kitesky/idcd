// Binary aggregator consumes the probe.results Redis Stream and persists results
// to TimescaleDB.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/aggregator/internal/config"
	"github.com/kite365/idcd/apps/aggregator/internal/consumer"
	"github.com/kite365/idcd/apps/aggregator/internal/dedup"
	"github.com/kite365/idcd/apps/aggregator/internal/processor"
	"github.com/kite365/idcd/packages/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.MustLoad(config.DefaultPath())

	// PostgreSQL pool
	pool, err := db.NewPool(context.Background(), db.Config{
		DSN:         cfg.Aggregator.PGDSN,
		MaxOpenConns: 10,
		MaxIdleConns: 2,
	})
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.Aggregator.RedisAddr,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to connect to Redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	dedupr := dedup.New(rdb)
	proc := processor.New(pool, dedupr)

	cons := consumer.New(rdb, consumer.Config{
		Stream:       cfg.Aggregator.StreamName,
		Group:        cfg.Aggregator.GroupName,
		ConsumerName: "aggregator-0",
		BatchSize:    cfg.Aggregator.BatchSize,
		BlockTimeout: cfg.Aggregator.BlockTimeout,
	}, proc, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cons.Run(ctx); err != nil {
		logger.Error("consumer exited with error", "err", err)
		os.Exit(1)
	}
}
