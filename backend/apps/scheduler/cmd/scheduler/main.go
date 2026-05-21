// Package main implements the idcd scheduler service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/scheduler/internal/config"
	"github.com/kite365/idcd/apps/scheduler/internal/leader"
	"github.com/kite365/idcd/apps/scheduler/internal/scheduler"
	"github.com/kite365/idcd/lib/db"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/stream"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// defaultMetricsPort is the listen port for the scheduler Prometheus scrape
// endpoint when observability.prometheus_port is unset in config. Picked to
// sit alongside the other service metric ports (aggregator 9091, notifier
// 9092, gateway 9094) — see docs/REVIEW-FINDINGS-2026-05-16.md.
const defaultMetricsPort = 9093

func main() {
	if err := run(); err != nil {
		// Logger may not be initialised yet if config loading failed, so fall
		// back to the slog default which writes to stderr.
		slog.Default().Error("fatal error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Load config
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Build the service logger. The scheduler config has no Server.Env field
	// (unlike apps/api); honour IDCD_ENV so dev gets text + DEBUG and prod
	// gets JSON + INFO. Tag every line with component=scheduler so Loki can
	// filter by service without us having to repeat "[main]" / "[scheduler]"
	// in every message.
	baseLogger := logger.New(os.Getenv("IDCD_ENV"))
	mainLog := baseLogger.With("component", "scheduler", "subsystem", "main")
	telLog := baseLogger.With("component", "scheduler", "subsystem", "telemetry")
	metricsLog := baseLogger.With("component", "scheduler", "subsystem", "metrics")

	mainLog.Info("config loaded", "path", cfgPath)
	mainLog.Info("redis configured", "addr", cfg.Redis.Addr)
	mainLog.Info("leader configured", "key", cfg.Leader.Key, "ttl", cfg.Leader.TTL)

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
		telLog.Error("failed to init", "err", err)
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
	mainLog.Info("redis connection ok")

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
	mainLog.Info("database connection ok")

	// Create components
	nodeID := fmt.Sprintf("scheduler-%s-%d", os.Getenv("HOSTNAME"), os.Getpid())
	if os.Getenv("HOSTNAME") == "" {
		nodeID = fmt.Sprintf("scheduler-%d", os.Getpid())
	}

	leaderElection := leader.New(rdb, cfg.Leader.Key, cfg.Leader.TTL, nodeID)

	// Claim a fencing token for this scheduler process before we ever touch
	// the probe.tasks stream. Tagged onto every stream write so the gateway
	// dispatcher can reject messages from a deposed leader that hasn't yet
	// noticed it lost the Redis lock. Failure here is fatal — running without
	// a token would re-enable the split-brain we're trying to prevent.
	// See backend/apps/scheduler/internal/leader/fencing.go for the full
	// rationale and Redis-persistence requirements.
	epochCtx, epochCancel := context.WithTimeout(ctx, 5*time.Second)
	epoch, err := leader.AcquireEpoch(epochCtx, rdb, leader.DefaultEpochKey)
	epochCancel()
	if err != nil {
		return fmt.Errorf("acquire fencing epoch: %w", err)
	}
	mainLog.Info("acquired fencing epoch", "epoch", epoch.Int64(), "key", leader.DefaultEpochKey)

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
	// Queue + WorkerCount fields removed 2026-05-16: the worker pool that
	// consumed scheduler:tasks ZSET had no producer; monitor poller is the
	// sole live path. See scheduler package doc.
	monitorStore := scheduler.NewPGMonitorStore(pool)

	sched := scheduler.New(scheduler.Config{
		Leader:       leaderElection,
		Selector:     nodeSelector,
		Stream:       streamClient,
		Pool:         pool,
		MonitorStore: monitorStore,
		NodeID:       nodeID,
		Logger:       baseLogger.With("component", "scheduler", "subsystem", "loop"),
		Epoch:        epoch,
	})

	// Start Prometheus /metrics listener. Runs in a goroutine so it doesn't
	// block the scheduler main loop; failures are logged but never fatal —
	// the scheduler keeps running even if observability scraping is broken.
	startMetricsServer(metricsPort(cfg), metricsLog)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start scheduler in goroutine
	errChan := make(chan error, 1)
	go func() {
		mainLog.Info("starting scheduler")
		errChan <- sched.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		mainLog.Info("received signal, shutting down", "signal", sig.String())
		cancel()
		// Wait for scheduler to stop
		<-errChan
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("scheduler error: %w", err)
		}
	}

	mainLog.Info("scheduler stopped gracefully")
	return nil
}

// metricsPort returns the Prometheus listener port, honouring the shared
// observability.prometheus_port config knob and falling back to the scheduler-
// specific defaultMetricsPort when unset. Zero / negative values use the
// default so a missing config block still produces a working scrape endpoint.
func metricsPort(cfg *config.Config) int {
	if cfg != nil && cfg.Observability.PrometheusPort > 0 {
		return cfg.Observability.PrometheusPort
	}
	return defaultMetricsPort
}

// startMetricsServer spins up a background HTTP listener serving the default
// Prometheus registry (where promauto-registered metrics in
// internal/scheduler/metrics.go land). ErrServerClosed is treated as a clean
// shutdown signal; any other failure is logged but never panics the scheduler.
func startMetricsServer(port int, log *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	addr := ":" + strconv.Itoa(port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("prometheus /metrics listener started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listener failed", "addr", addr, "err", err)
		}
	}()
}
