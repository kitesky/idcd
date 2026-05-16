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
	"github.com/kite365/idcd/lib/db"
	"github.com/kite365/idcd/lib/shared/stream"
	"github.com/kite365/idcd/lib/shared/telemetry"
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
		OTLPEndpoint:   cfg.Observability.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Observability.Telemetry.SamplingRate,
		Enabled:        cfg.Observability.Telemetry.Enabled,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		log.Printf("[telemetry] failed to init: %v", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

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

	// Use DBNodeSelector when a DB pool is available (production).
	// Fall back to StaticNodeSelector for local dev without DB.
	var nodeSelector scheduler.NodeSelector
	if pool != nil {
		nodeSelector = scheduler.NewDBNodeSelector(pool)
	} else {
		nodeSelector = scheduler.NewStaticNodeSelector([]string{
			"nd_us_ny_01_aws",
			"nd_eu_de_01_hetzner",
			"nd_ap_jp_01_vultr",
		})
	}

	// Wire the DB-backed MonitorStore so monitorPoller fires (B6 wiring).
	monitorStore := scheduler.NewPGMonitorStore(pool)

	// Create scheduler
	sched := scheduler.New(scheduler.Config{
		Leader:       leaderElection,
		Queue:        taskQueue,
		Selector:     nodeSelector,
		Stream:       streamClient,
		Pool:         pool,
		MonitorStore: monitorStore,
		WorkerCount:  cfg.Worker.Count,
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
