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
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/cert-svc/internal/config"
	"github.com/kite365/idcd/apps/cert-svc/internal/handler"
	"github.com/kite365/idcd/apps/cert-svc/internal/metrics"
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
	"github.com/kite365/idcd/lib/cert/vault/awskms"
	"github.com/kite365/idcd/lib/cert/vault/envmaster"
	"github.com/kite365/idcd/lib/cert/vault/hashivault"
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

	// Admin surface (/v1/admin/cert/*). Bearer-token middleware matches
	// apps/api's admin pattern (lib/shared/config Server.AdminToken).
	// Empty CERT_ADMIN_TOKEN disables admin endpoints entirely — they
	// fall through to the handler's rejectAllAdminUnauthenticated 401.
	var adminAuthMW func(http.Handler) http.Handler
	if cfg.AdminToken != "" {
		adminAuthMW = adminBearerAuth(cfg.AdminToken)
	} else {
		slogLogger.Warn("CERT_ADMIN_TOKEN not set — /v1/admin/cert/* will reject all requests")
	}

	adminQuota := service.NewRepoQuotaChecker(repos.Orders, repos.Certs, nil)
	adminAbuse := newAuditAbuseGate(repos.AuditLogs, slogLogger)

	router2 := handler.New(handler.Deps{
		DB:                   pgxPinger{pool: pool},
		Redis:                redisPinger{client: rdb},
		Service:              svc,
		Repos:                repos,
		Vault:                vlt,
		DNSReg:               reg,
		AuthnMiddleware:      authnMW,
		AdminAuthnMiddleware: adminAuthMW,
		AdminQuota:           adminQuota,
		AdminAbuse:           adminAbuse,
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router2,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Prometheus /metrics on a dedicated port. Kept on a separate
	// listener so the scraper ACL (VPN-only) does not need to overlap
	// with the public /v1/cert/* HTTP API.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{
		Addr:              cfg.MetricsAddr(),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slogLogger.Info("cert-svc metrics listening", "addr", metricsSrv.Addr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slogLogger.Error("metrics listener failed", "addr", metricsSrv.Addr, "error", err)
		}
	}()

	// Periodic gauge collector — refreshes queue_depth + ca_quota_used
	// every 30s from Redis + DB. The orchestrator records counters /
	// histograms in-line; this goroutine owns gauges only.
	quotaSampler := samplerAdapter{
		inner: service.NewRepoQuotaChecker(repos.Orders, repos.Certs, nil),
	}
	collector := metrics.NewCollector(rdb, quotaSampler,
		metrics.WithLogger(slogLogger),
		metrics.WithStreams(service.DefaultStream, "cert:notifications"),
		metrics.WithCAs(router.Names()...),
	)
	go func() {
		if err := collector.Run(ctx); err != nil {
			slogLogger.Error("metrics collector failed", "error", err)
		}
	}()

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
	_ = metricsSrv.Shutdown(shutdownCtx)
	slogLogger.Info("cert-svc shutdown complete")
}

// samplerAdapter bridges service.RepoQuotaChecker (which returns
// service.QuotaUsage) onto metrics.QuotaSampler (which expects
// metrics.UsageRatio). The metrics package deliberately does not import
// the service package to avoid an import cycle once the collector
// graduates to lib/.
type samplerAdapter struct {
	inner *service.RepoQuotaChecker
}

func (a samplerAdapter) Usage(ctx context.Context, caName string) (metrics.UsageRatio, error) {
	u, err := a.inner.Usage(ctx, caName)
	if err != nil {
		return metrics.UsageRatio{}, err
	}
	return metrics.UsageRatio{
		PerRegisteredDomain: u.PerRegisteredDomain,
		PerAccount3h:        u.PerAccount3h,
	}, nil
}

// buildVault picks the vault.Vault implementation per Config.VaultBackend.
// envmaster is the S1 default; alikms / awskms / hashivault are the
// D-FC-04 production paths (国内 / 海外 / 自托管).
func buildVault(cfg *config.Config) (vault.Vault, error) {
	switch cfg.VaultBackend {
	case config.VaultBackendAliKMS:
		return alikms.New(alikms.Config{
			RegionID:        cfg.AliKMSRegionID,
			AccessKeyID:     cfg.AliKMSAccessKeyID,
			AccessKeySecret: cfg.AliKMSAccessKeySecret,
			KeyID:           cfg.AliKMSKeyID,
		})
	case config.VaultBackendAWSKMS:
		return awskms.New(awskms.Config{
			Region:          cfg.AWSKMSRegion,
			AccessKeyID:     cfg.AWSKMSAccessKeyID,
			SecretAccessKey: cfg.AWSKMSSecretAccessKey,
			KeyID:           cfg.AWSKMSKeyID,
		})
	case config.VaultBackendHashiVault:
		return hashivault.New(hashivault.Config{
			Address:   cfg.HashiVaultAddress,
			Token:     cfg.HashiVaultToken,
			Namespace: cfg.HashiVaultNamespace,
			KeyName:   cfg.HashiVaultKeyName,
			MountPath: cfg.HashiVaultMountPath,
		})
	default:
		return envmaster.NewFromEnv("CERT_MASTER_KEY")
	}
}

// adminBearerAuth returns a middleware that requires
//
//	Authorization: Bearer <CERT_ADMIN_TOKEN>
//
// Token comparison uses crypto/subtle so timing leaks cannot probe the
// secret. Pattern mirrors apps/api's AdminAuthMiddleware.
func adminBearerAuth(token string) func(http.Handler) http.Handler {
	want := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix {
				http.Error(w, `{"error":{"code":"CERT_UNAUTHORIZED","message":"missing bearer token"}}`, http.StatusUnauthorized)
				return
			}
			got := []byte(auth[len(prefix):])
			if subtle.ConstantTimeCompare(got, want) != 1 {
				http.Error(w, `{"error":{"code":"CERT_UNAUTHORIZED","message":"invalid admin token"}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// auditAbuseGate satisfies handler.AdminAbuseGate by recording each ban
// as an immutable cert.audit_logs row. Enforcement (rejecting future
// orders from banned accounts) requires a cert.abuse_bans table — a
// post-S2 addition; for now the audit row is the system of record so
// operators can review and a later migration can backfill.
type auditAbuseGate struct {
	repo   *repo.AuditLogsRepo
	logger *slog.Logger
}

func newAuditAbuseGate(r *repo.AuditLogsRepo, l *slog.Logger) *auditAbuseGate {
	return &auditAbuseGate{repo: r, logger: l}
}

func (g *auditAbuseGate) Ban(ctx context.Context, accountID int64, reason string) error {
	if g == nil || g.repo == nil {
		return errors.New("audit gate not configured")
	}
	tk := "account"
	payload, _ := json.Marshal(map[string]string{"reason": reason})
	entry := &repo.AuditLog{
		AccountID:  &accountID,
		Actor:      "admin",
		Action:     "abuse.ban",
		TargetKind: &tk,
		TargetID:   &accountID,
		Payload:    payload,
	}
	if err := g.repo.Append(ctx, entry); err != nil {
		if g.logger != nil {
			g.logger.Error("audit ban append failed", "account_id", accountID, "error", err)
		}
		return err
	}
	if g.logger != nil {
		g.logger.Info("admin ban recorded", "account_id", accountID, "reason", reason)
	}
	return nil
}

// pgxPinger / redisPinger adapt the pgx and redis client to the
// handler.Pinger contract so readyz can probe them.
type pgxPinger struct{ pool *pgxpool.Pool }

func (p pgxPinger) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

type redisPinger struct{ client *redis.Client }

func (p redisPinger) Ping(ctx context.Context) error { return p.client.Ping(ctx).Err() }
