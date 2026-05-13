// Package main implements the idcd scheduler service.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/scheduler/internal/config"
	"github.com/kite365/idcd/apps/scheduler/internal/leader"
	"github.com/kite365/idcd/apps/scheduler/internal/queue"
	"github.com/kite365/idcd/apps/scheduler/internal/scheduler"
	"github.com/kite365/idcd/packages/db"
	"github.com/kite365/idcd/packages/shared/stream"
	"github.com/kite365/idcd/packages/shared/telemetry"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}

func run() error {
	// Load config
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("[main] Config loaded from %s", cfgPath)
	log.Printf("[main] Redis: %s", cfg.Redis.Addr)
	log.Printf("[main] Leader key: %s, TTL: %v", cfg.Leader.Key, cfg.Leader.TTL)
	log.Printf("[main] Workers: %d", cfg.Worker.Count)

	// Initialize OpenTelemetry
	telCfg := telemetry.Config{
		ServiceName:    "idcd-scheduler",
		ServiceVersion: "v1.0.0",
		OTLPEndpoint:   "", // S1: stdout exporter
		SamplingRate:   0.1,
		Enabled:        true,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		log.Printf("[telemetry] failed to init: %v", err)
	}
	defer shutdownTelemetry(context.Background())

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})
	defer rdb.Close()

	// Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	log.Println("[main] Redis connection OK")

	// Create DB pool
	pool, err := db.NewPool(ctx, db.Config{
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()
	log.Println("[main] Database connection OK")

	// Create components
	nodeID := fmt.Sprintf("scheduler-%s-%d", os.Getenv("HOSTNAME"), os.Getpid())
	if os.Getenv("HOSTNAME") == "" {
		nodeID = fmt.Sprintf("scheduler-%d", os.Getpid())
	}

	leaderElection := leader.New(rdb, cfg.Leader.Key, cfg.Leader.TTL, nodeID)
	taskQueue := queue.New(rdb, "scheduler:tasks")
	streamClient := stream.New(rdb)

	// S1: Use static node selector (will be replaced with DB-based selector in S2)
	// For now, use a hardcoded list of nodes for testing
	nodeSelector := scheduler.NewStaticNodeSelector([]string{
		"nd_us_ny_01_aws",
		"nd_eu_de_01_hetzner",
		"nd_ap_jp_01_vultr",
	})

	// Create scheduler
	sched := scheduler.New(scheduler.Config{
		Leader:      leaderElection,
		Queue:       taskQueue,
		Selector:    nodeSelector,
		Stream:      streamClient,
		Pool:        pool,
		WorkerCount: cfg.Worker.Count,
	})

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start scheduler in goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Println("[main] Starting scheduler...")
		errChan <- sched.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("[main] Received signal %v, shutting down...", sig)
		cancel()
		// Wait for scheduler to stop
		<-errChan
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("scheduler error: %w", err)
		}
	}

	log.Println("[main] Scheduler stopped gracefully")
	return nil
}
