// server is the cert-svc HTTP API entry point.
//
// S1 W3 wires the full dependency graph: pgx pool, Redis client, vault,
// DNS provider registry, ACME service handle and the JWT/session-based
// auth middleware. The Service constructed here drives only the
// HTTP-side write entry points (EnqueueOrder, RetryOrder,
// MarkManualChallengeReady) — the long-running Redis-Stream consumer
// runs in cmd/worker.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/cert-svc/internal/config"
	"github.com/kite365/idcd/apps/cert-svc/internal/handler"
	certmw "github.com/kite365/idcd/apps/cert-svc/internal/middleware"
	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/ca/buypass"
	"github.com/kite365/idcd/lib/cert/ca/letsencrypt"
	"github.com/kite365/idcd/lib/cert/ca/zerossl"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/dns/aliyun"
	"github.com/kite365/idcd/lib/cert/dns/cloudflare"
	"github.com/kite365/idcd/lib/cert/dns/dnspod"
	"github.com/kite365/idcd/lib/cert/dns/gcloud"
	"github.com/kite365/idcd/lib/cert/dns/manual"
	"github.com/kite365/idcd/lib/cert/dns/route53"
	"github.com/kite365/idcd/lib/cert/vault"
	"github.com/kite365/idcd/lib/cert/vault/alikms"
	"github.com/kite365/idcd/lib/cert/vault/envmaster"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("cert-svc: load config: %v", err)
	}

	slogLogger := logger.New(cfg.Env)
	slogLogger.Info("cert-svc starting",
		"port", cfg.Port,
		"env", cfg.Env,
		"log_level", cfg.LogLevel,
	)

	telCfg := telemetry.Config{
		ServiceName:    "idcd-cert-svc",
		ServiceVersion: "v0.1.0",
		SamplingRate:   0.1,
		Enabled:        false, // flip via config once OTLP endpoint is wired
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		slogLogger.Warn("telemetry init failed; continuing without traces", "error", err)
		shutdownTelemetry = func(context.Context) error { return nil }
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Postgres. cert-svc reads from cert.* schema; the pool size is
	// tiny because the handlers do at most one query per request.
	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		slogLogger.Error("pgx pool init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Redis. Used for the JWT session store AND for the order stream
	// publisher inside Service. Same instance, two consumer surfaces.
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()

	vlt, err := buildVault(cfg)
	if err != nil {
		slogLogger.Error("vault init failed", "backend", cfg.VaultBackend, "error", err)
		os.Exit(1)
	}
	slogLogger.Info("vault wired", "backend", cfg.VaultBackend, "key_id", vlt.KeyID())

	// DNS provider registry. Same providers as the worker; the server
	// only uses ValidateCredential + HealthCheck (BuildSolver is
	// worker-only) but we register both so future endpoints don't
	// silently break.
	reg := dns.NewRegistry()
	for _, p := range []dns.Provider{
		cloudflare.New(cloudflare.Config{}),
		manual.New(manual.Config{}),
		aliyun.New(aliyun.Config{}),
		dnspod.New(dnspod.Config{}),
		route53.New(route53.Config{}),
		gcloud.New(gcloud.Config{}),
	} {
		if err := reg.Register(p); err != nil {
			slogLogger.Error("register dns provider failed", "kind", p.Kind(), "error", err)
			os.Exit(1)
		}
	}

	repos := repo.New(pool)

	// Service is constructed without an AccountKey — the server never
	// drives the orchestrator (that's worker territory). It only
	// publishes new orders and proxies manual-ready confirmations.
	// S2: multi-CA registry. Let's Encrypt is always the default;
	// ZeroSSL + Buypass register conditionally when their env vars are
	// set. Order ca-field dispatch happens inside the Router.
	leCA := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.Env(cfg.LEEnv)})
	var extras []ca.AcmeCA
	if cfg.ZeroSSLEABKID != "" && cfg.ZeroSSLEABHMACKey != "" {
		extras = append(extras, zerossl.New(zerossl.Config{
			EABKID:     cfg.ZeroSSLEABKID,
			EABHMACKey: cfg.ZeroSSLEABHMACKey,
		}))
	}
	if cfg.BuypassEnv != "" {
		extras = append(extras, buypass.New(buypass.Config{Env: buypass.Env(cfg.BuypassEnv)}))
	}
	router := service.NewRouter(leCA, extras...)
	slogLogger.Info("ca router wired", "cas", router.Names())
	abuse := service.NewAbuseDetector(repos,
		service.WithAbuseLogger(slogLogger))

	// W5: HMAC key for one-shot download URLs. Empty + non-production
	// is tolerated (download endpoint will 503 cleanly); production
	// deploys MUST set it or we refuse to start so we never serve a
	// forgeable token.
	if len(cfg.DownloadSecret) == 0 && cfg.Env == "production" {
		slogLogger.Error("CERT_DOWNLOAD_SECRET is required in production")
		os.Exit(1)
	}
	if len(cfg.DownloadSecret) == 0 {
		slogLogger.Warn("CERT_DOWNLOAD_SECRET not set — /v1/cert/certs/{id}/download will 503")
	}

	svc := service.New(service.Config{
		Repos:          repos,
		Redis:          rdb,
		Vault:          vlt,
		DNSReg:         reg,
		Router:         router,
		AccountEmail:   cfg.AccountEmail,
		Abuse:          abuse,
		DownloadSecret: cfg.DownloadSecret,
		Logger:         slogLogger,
	})

	// Auth. cert-svc shares the JWT secret + Redis session store with
	// apps/api, so a browser session signed in via /v1/auth/login works
	// transparently against /v1/cert/* on this service.
	var authnMW func(http.Handler) http.Handler
	if cfg.JWTSecret != "" {
		jwtSvc, jerr := jwt.NewServiceWithOptions(jwt.Config{SecretKey: cfg.JWTSecret})
		if jerr != nil {
			slogLogger.Error("jwt service init failed", "error", jerr)
			os.Exit(1)
		}
		sessSvc := session.NewService(rdb)
		authnMW = certmw.Authn(jwtSvc, sessSvc)
	} else {
		slogLogger.Warn("CERT_JWT_SECRET not set — /v1/cert/* will reject all requests")
	}

	router2 := handler.New(handler.Deps{
		DB:              pgxPinger{pool: pool},
		Redis:           redisPinger{client: rdb},
		Service:         svc,
		Repos:           repos,
		Vault:           vlt,
		DNSReg:          reg,
		AuthnMiddleware: authnMW,
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router2,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slogLogger.Info("cert-svc listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			slogLogger.Error("cert-svc server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slogLogger.Info("cert-svc shutdown signal received")
	}

	shutdownCtx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer scancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slogLogger.Error("cert-svc graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slogLogger.Info("cert-svc shutdown complete")
}

// buildVault picks the vault.Vault implementation per Config.VaultBackend.
// envmaster is the S1 default; alikms is the D-FC-04 production path.
func buildVault(cfg *config.Config) (vault.Vault, error) {
	switch cfg.VaultBackend {
	case config.VaultBackendAliKMS:
		return alikms.New(alikms.Config{
			RegionID:        cfg.AliKMSRegionID,
			AccessKeyID:     cfg.AliKMSAccessKeyID,
			AccessKeySecret: cfg.AliKMSAccessKeySecret,
			KeyID:           cfg.AliKMSKeyID,
		})
	default:
		return envmaster.NewFromEnv("CERT_MASTER_KEY")
	}
}

// pgxPinger / redisPinger adapt the pgx and redis client to the
// handler.Pinger contract so readyz can probe them.
type pgxPinger struct{ pool *pgxpool.Pool }

func (p pgxPinger) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

type redisPinger struct{ client *redis.Client }

func (p redisPinger) Ping(ctx context.Context) error { return p.client.Ping(ctx).Err() }
