// renewer is the cert-svc renewal scheduler.
//
// S1 W3: scans cert.certs hourly for rows whose not_after is within the
// configured lead time and enqueues a fresh order + renewal_jobs row for
// each. The renewer pushes work onto the same Redis stream as the worker
// — it does not run the ACME orchestrator itself.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/cert-svc/internal/config"
	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/shared/logger"
)

// enqueueOnly is a slim Service-like adapter that only knows how to push
// an order id onto the Redis stream. The renewer does not need the full
// ACME orchestrator (CA, vault, DNS providers); a thin wrapper around the
// existing Service.EnqueueOrder keeps the dependency surface small.
type enqueueOnly struct{ svc *service.Service }

func (e *enqueueOnly) EnqueueOrder(ctx context.Context, orderID int64) error {
	return e.svc.EnqueueOrder(ctx, orderID)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("cert-renewer: load config: " + err.Error())
	}
	log := logger.New(cfg.Env)
	log.Info("cert-renewer started", "env", cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("cert-renewer: pgx pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer func() { _ = rdb.Close() }()

	repos := repo.New(pool)

	// We build a Service shell only to reach Service.EnqueueOrder — the
	// ACME orchestrator fields (Vault / DNSReg / Router / AccountKey) are
	// intentionally nil since the renewer never calls DriveOrder.
	svc := service.New(service.Config{
		Repos:  repos,
		Redis:  rdb,
		Logger: log,
	})

	scheduler := service.NewRenewalScheduler(repos, &enqueueOnly{svc: svc},
		service.WithRenewalLogger(log))

	if err := scheduler.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("cert-renewer: scheduler stopped", "err", err)
		os.Exit(1)
	}
	log.Info("cert-renewer shutting down")
}
