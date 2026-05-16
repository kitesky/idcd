// worker is the cert-svc ACME orchestrator.
//
// S1 W2: drives the Redis-Stream consumer and the order state machine.
// Liveness logs every minute so an operator can confirm the loop hasn't
// silently parked; the real signal is the state-machine log.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/cert-svc/internal/config"
	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/cert/ca/letsencrypt"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/dns/cloudflare"
	"github.com/kite365/idcd/lib/cert/dns/manual"
	"github.com/kite365/idcd/lib/cert/vault"
	"github.com/kite365/idcd/lib/cert/vault/envmaster"
	"github.com/kite365/idcd/lib/shared/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("cert-worker: load config: " + err.Error())
	}
	log := logger.New(cfg.Env)
	log.Info("cert-worker starting", "env", cfg.Env, "redis", cfg.RedisAddr, "le_env", cfg.LEEnv)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("cert-worker: pgx pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()

	vlt, err := envmaster.NewFromEnv("CERT_MASTER_KEY")
	if err != nil {
		log.Error("cert-worker: vault init", "err", err)
		os.Exit(1)
	}

	reg := dns.NewRegistry()
	if err := reg.Register(cloudflare.New(cloudflare.Config{})); err != nil {
		log.Error("cert-worker: register cloudflare", "err", err)
		os.Exit(1)
	}
	if err := reg.Register(manual.New(manual.Config{})); err != nil {
		log.Error("cert-worker: register manual", "err", err)
		os.Exit(1)
	}

	leCA := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.Env(cfg.LEEnv)})
	router := service.NewRouter(leCA)

	// S1 simplification: account key is a fresh ECDSA P256 derived per
	// process via vault.GenerateKey. lego auto-registers on first use.
	// S2 will load this from cert.acme_accounts.
	_, ek, err := vlt.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		log.Error("cert-worker: acme account key", "err", err)
		os.Exit(1)
	}
	plain, err := vlt.DecryptKey(ctx, ek)
	if err != nil {
		log.Error("cert-worker: decrypt acme account key", "err", err)
		os.Exit(1)
	}
	signer, err := service.DecodeAccountKey(plain)
	if err != nil {
		log.Error("cert-worker: parse acme account key", "err", err)
		os.Exit(1)
	}

	svc := service.New(service.Config{
		Repos:        repo.New(pool),
		Redis:        rdb,
		Vault:        vlt,
		DNSReg:       reg,
		Router:       router,
		AccountKey:   signer,
		AccountEmail: cfg.AccountEmail,
		Logger:       log,
	})

	log.Info("cert-worker consuming", "stream", service.DefaultStream, "group", service.DefaultGroup)
	if err := svc.RunConsumer(ctx); err != nil {
		log.Error("cert-worker: consumer", "err", err)
		os.Exit(1)
	}
	log.Info("cert-worker stopped")
}
