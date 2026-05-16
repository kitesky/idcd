// Package main is the entry point for the Gateway service.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/gateway/internal/config"
	"github.com/kite365/idcd/apps/gateway/internal/dispatcher"
	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/apps/gateway/internal/scheduler"
	"github.com/kite365/idcd/apps/gateway/internal/server"
	sharedconfig "github.com/kite365/idcd/lib/shared/config"
	"github.com/kite365/idcd/lib/shared/stream"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// defaultMetricsPort is the listen port for the gateway Prometheus scrape
// endpoint when observability.prometheus_port is unset in config. Picked to
// sit alongside the other service metric ports (aggregator 9091, notifier
// 9092, scheduler 9093) — see docs/REVIEW-FINDINGS-2026-05-16.md.
const defaultMetricsPort = 9094

func main() {
	// Load configuration from dev.env.yaml (falls back to defaults if missing).
	cfg := config.Load()

	// Setup logger: text+DEBUG in dev, JSON+INFO in prod.
	var loggerInst *slog.Logger
	if cfg.IsDev() {
		loggerInst = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	} else {
		loggerInst = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	loggerInst.Info("starting Gateway service", "env", cfg.Env)

	// Initialize OpenTelemetry
	telCfg := telemetry.Config{
		ServiceName:    "idcd-gateway",
		ServiceVersion: "v1.0.0",
		OTLPEndpoint:   cfg.Observability.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Observability.Telemetry.SamplingRate,
		Enabled:        cfg.Observability.Telemetry.Enabled,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		loggerInst.Error("failed to init telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	// Setup Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
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

	// Start task dispatcher: reads probe.tasks stream and forwards to connected agents.
	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	defer cancelDispatch()
	taskDispatcher := dispatcher.New(rdb, h, loggerInst)
	go taskDispatcher.Run(dispatchCtx)

	// Create and start HTTP server
	srv := server.New(cfg, h, rdb, pool, streamCli, loggerInst)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			loggerInst.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start dedicated Prometheus /metrics listener (P1#19). Runs on its own
	// port so dashboards can scrape gateway-specific counters / gauges
	// (see internal/hub/metrics.go) without going through the WS server's
	// auth-gated /metrics route. Errors are logged but never fatal — broken
	// scraping should never bring the gateway down.
	startMetricsServer(gatewayMetricsPort(cfg), loggerInst)

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

// gatewayMetricsPort returns the listener port for the Prometheus scrape
// endpoint. config.Load currently does not propagate Observability from the
// shared config file, so we re-read the shared file here to honour the
// operator's observability.prometheus_port knob. Any I/O error falls back to
// defaultMetricsPort so a missing config still produces a working scrape
// endpoint.
func gatewayMetricsPort(cfg *config.Config) int {
	if cfg != nil && cfg.Observability.PrometheusPort > 0 {
		return cfg.Observability.PrometheusPort
	}
	if shared, err := sharedconfig.Load(sharedconfig.DefaultPath()); err == nil {
		if shared.Observability.PrometheusPort > 0 {
			return shared.Observability.PrometheusPort
		}
	}
	return defaultMetricsPort
}

// startMetricsServer launches the dedicated Prometheus /metrics HTTP listener
// in a background goroutine. The listener exposes the default Prometheus
// registry, where promauto-registered metrics in internal/hub/metrics.go
// land. ErrServerClosed is treated as a clean shutdown; any other failure is
// logged but does not panic the gateway process.
func startMetricsServer(port int, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	addr := ":" + strconv.Itoa(port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("Prometheus /metrics listener started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics listener failed", "addr", addr, "error", err)
		}
	}()
}
