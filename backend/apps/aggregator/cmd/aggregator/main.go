// Binary aggregator consumes the probe.results Redis Stream and persists results
// to TimescaleDB.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/aggregator/internal/config"
	"github.com/kite365/idcd/apps/aggregator/internal/consumer"
	"github.com/kite365/idcd/apps/aggregator/internal/dedup"
	"github.com/kite365/idcd/apps/aggregator/internal/processor"
	"github.com/kite365/idcd/lib/db"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.MustLoad(config.DefaultPath())

	// Initialize OpenTelemetry
	telCfg := telemetry.Config{
		ServiceName:    "idcd-aggregator",
		ServiceVersion: "v1.0.0",
		OTLPEndpoint:   cfg.Observability.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Observability.Telemetry.SamplingRate,
		Enabled:        cfg.Observability.Telemetry.Enabled,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		logger.Error("failed to init telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	// PostgreSQL pool
	pool, err := db.NewPool(context.Background(), db.Config{
		DSN:          cfg.Aggregator.PGDSN,
		MaxOpenConns: 10,
		MaxIdleConns: 2,
	})
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Redis client. Read password from the shared Redis section so authenticated
	// remote dev Redis (e.g. 8.163.70.123:6379 with AUTH) works out of the box.
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Aggregator.RedisAddr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to connect to Redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	dedupr := dedup.New(rdb)
	proc := processor.New(pool, dedupr)
	proc.SetLogger(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the internal metrics listener (Prometheus default registry).
	metricsPort := 9091
	if cfg.Config != nil && cfg.Observability.PrometheusPort > 0 {
		metricsPort = cfg.Observability.PrometheusPort
	}
	metricsSrv := startMetricsServer(metricsPort, logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	// One consumer name per worker: "{pod-id}-{worker-idx}". pod-id comes from
	// the HOSTNAME env var (k8s auto-injects the pod name); a random UUID is
	// used outside k8s. This ensures multi-replica deployments do not collide.
	podID := os.Getenv("HOSTNAME")
	if podID == "" {
		podID = uuid.New().String()
	}

	workerCount := cfg.Aggregator.ConsumerCount
	if workerCount <= 0 {
		workerCount = 1
	}
	consumers := make([]*consumer.Consumer, 0, workerCount)
	for i := 0; i < workerCount; i++ {
		name := fmt.Sprintf("%s-%d", podID, i)
		c := consumer.New(rdb, consumer.Config{
			Stream:       cfg.Aggregator.StreamName,
			Group:        cfg.Aggregator.GroupName,
			ConsumerName: name,
			BatchSize:    cfg.Aggregator.BatchSize,
			BlockTimeout: cfg.Aggregator.BlockTimeout,
		}, proc, logger)
		consumers = append(consumers, c)
	}

	var wg sync.WaitGroup

	// One foreground goroutine per consumer (independent XREADGROUP).
	for _, c := range consumers {
		wg.Add(1)
		go func(c *consumer.Consumer) {
			defer wg.Done()
			if err := c.Run(ctx); err != nil {
				logger.Error("consumer exited with error", "consumer", c.Name(), "err", err)
			}
		}(c)
	}

	// Single maintenance goroutine for the whole replica — reclaim PEL + DLQ +
	// PEL gauge sampling. Picked the first consumer's identity so reclaimed
	// messages always have a real owner; XAUTOCLAIM is happy with that.
	if len(consumers) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			consumers[0].RunMaintenance(ctx, consumer.DefaultReclaimInterval)
		}()
	}

	wg.Wait()
}

// startMetricsServer exposes Prometheus default-registry metrics on
// :<port>/metrics. The listener must NOT be exposed to public traffic (bind to
// the internal VPC / loopback).
func startMetricsServer(port int, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", port),
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("starting metrics server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "err", err)
		}
	}()
	return srv
}
