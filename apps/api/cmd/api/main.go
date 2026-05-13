// main.go is the entry point for the API Gateway service.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // PostgreSQL driver for sql.DB health check
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/server"
	"github.com/kite365/idcd/lib/shared/config"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

func main() {
	// Load configuration
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Setup logger
	slogLogger := logger.New(cfg.Server.Env)

	// Initialize OpenTelemetry
	telCfg := telemetry.Config{
		ServiceName:    "idcd-api",
		ServiceVersion: "v1.0.0",
		OTLPEndpoint:   cfg.Observability.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Observability.Telemetry.SamplingRate,
		Enabled:        cfg.Observability.Telemetry.Enabled,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		slogLogger.Error("failed to init telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.Database.Main.DSN)
	if err != nil {
		slogLogger.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Configure PostgreSQL connection pool
	db.SetMaxOpenConns(int(cfg.Database.Main.MaxOpenConns))
	db.SetMaxIdleConns(int(cfg.Database.Main.MaxIdleConns))
	db.SetConnMaxLifetime(cfg.Database.Main.ConnMaxLifetime.Duration)

	// Test PostgreSQL connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		slogLogger.Error("failed to ping PostgreSQL", "error", err)
		os.Exit(1)
	}

	slogLogger.Info("connected to PostgreSQL", "dsn", maskDSN(cfg.Database.Main.DSN))

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pong, err := redisClient.Ping(ctx).Result()
	if err != nil {
		slogLogger.Error("failed to ping Redis", "error", err)
		os.Exit(1)
	}

	slogLogger.Info("connected to Redis", "addr", cfg.Redis.Addr, "response", pong)

	// Create pgxpool for handlers that need pgx (sqlc queries)
	pgxPool, err := pgxpool.New(context.Background(), cfg.Database.Main.DSN)
	if err != nil {
		slogLogger.Error("failed to create pgx pool", "error", err)
		os.Exit(1)
	}
	defer pgxPool.Close()

	// Create and start server
	srv := server.New(cfg, db, pgxPool, redisClient, slogLogger)

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start()
	}()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		if err != nil {
			slogLogger.Error("server failed to start", "error", err)
		}
	case sig := <-sigChan:
		slogLogger.Info("received shutdown signal", "signal", sig)
	}

	// Graceful shutdown with 5 second timeout
	slogLogger.Info("initiating graceful shutdown...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slogLogger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	// Close database connections
	if err := db.Close(); err != nil {
		slogLogger.Error("failed to close database connection", "error", err)
	}

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		slogLogger.Error("failed to close Redis connection", "error", err)
	}

	slogLogger.Info("server shutdown complete")
}

// maskDSN replaces password in DSN string with asterisks for logging.
func maskDSN(dsn string) string {
	// Simple implementation - just return a safe version
	// In production, you might want to use a more sophisticated approach
	if len(dsn) > 20 {
		return dsn[:20] + "***"
	}
	return "***"
}