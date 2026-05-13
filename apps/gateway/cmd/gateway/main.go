// Package main is the entry point for the Gateway service.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/gateway/internal/config"
	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/apps/gateway/internal/scheduler"
	"github.com/kite365/idcd/apps/gateway/internal/server"
	"github.com/kite365/idcd/packages/shared/stream"
)

func main() {
	// Load configuration
	cfg := config.Default()

	// Setup logger
	var logLevel slog.Level
	if cfg.IsDev() {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}

	loggerInst := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	loggerInst.Info("starting Gateway service", "env", cfg.Env)

	// Setup Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	// Ping Redis to verify connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	loggerInst.Info("connected to Redis", "addr", cfg.RedisAddr)

	// Setup PostgreSQL connection pool (optional, only if PGDSN is configured)
	var pool *pgxpool.Pool
	if cfg.PGDSN != "" {
		poolConfig, err := pgxpool.ParseConfig(cfg.PGDSN)
		if err != nil {
			log.Fatalf("failed to parse PostgreSQL DSN: %v", err)
		}

		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			log.Fatalf("failed to create PostgreSQL pool: %v", err)
		}

		// Ping database to verify connection
		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("failed to ping PostgreSQL: %v", err)
		}
		loggerInst.Info("connected to PostgreSQL", "dsn", cfg.PGDSN)
	} else {
		loggerInst.Warn("PostgreSQL DSN not configured, node cleanup scheduler will not run")
	}

	// Create stream client
	streamCli := stream.New(rdb)

	// Create Hub
	h := hub.New(cfg.HeartbeatTimeout, loggerInst)

	// Start heartbeat monitor
	monitorCtx, cancelMonitor := context.WithCancel(ctx)
	go h.StartHeartbeatMonitor(monitorCtx)

	// Start cleanup scheduler (if PostgreSQL is configured)
	var cleanupCtx context.Context
	var cancelCleanup context.CancelFunc
	if pool != nil {
		cleanupCtx, cancelCleanup = context.WithCancel(ctx)
		cleanupScheduler := scheduler.NewCleanupScheduler(pool, 5*time.Minute, loggerInst)
		go cleanupScheduler.Run(cleanupCtx)
	}

	// Create and start HTTP server
	srv := server.New(cfg, h, rdb, streamCli, loggerInst)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			loggerInst.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	loggerInst.Info("received shutdown signal")

	// Cancel heartbeat monitor
	cancelMonitor()

	// Cancel cleanup scheduler
	if cancelCleanup != nil {
		cancelCleanup()
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		loggerInst.Error("server shutdown error", "error", err)
	}

	// Close PostgreSQL pool
	if pool != nil {
		pool.Close()
		loggerInst.Info("PostgreSQL pool closed")
	}

	// Close Redis connection
	if err := rdb.Close(); err != nil {
		loggerInst.Error("redis close error", "error", err)
	}

	loggerInst.Info("Gateway service stopped")
}
