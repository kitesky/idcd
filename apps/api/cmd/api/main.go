// main.go is the entry point for the API Gateway service.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	acmemgr "github.com/kite365/idcd/apps/api/internal/acme"
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

	// Connect to PostgreSQL via pgxpool (single pool used by all handlers +
	// health checks; the legacy database/sql pool was removed to avoid
	// duplicating connection counts against the DB).
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.Main.DSN)
	if err != nil {
		slogLogger.Error("failed to parse postgres DSN", "error", err)
		os.Exit(1)
	}
	if v := cfg.Database.Main.MaxOpenConns; v > 0 {
		poolCfg.MaxConns = int32(v)
	}
	if v := cfg.Database.Main.MaxIdleConns; v > 0 {
		poolCfg.MinConns = int32(v)
	}
	if cfg.Database.Main.ConnMaxLifetime.Duration > 0 {
		poolCfg.MaxConnLifetime = cfg.Database.Main.ConnMaxLifetime.Duration
	}
	pgxPool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		slogLogger.Error("failed to create pgx pool", "error", err)
		os.Exit(1)
	}
	defer pgxPool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pgxPool.Ping(ctx); err != nil {
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

	// Create and start server
	srv := server.New(cfg, pgxPool, redisClient, slogLogger)

	// ACME / Let's Encrypt for custom-domain status pages (M11 / K8).
	// Feature-flagged via env vars; default OFF.  See wireACME for details.
	if mgr := wireACME(pgxPool, slogLogger); mgr != nil {
		srv.MountACME(mgr)
		srv.StartACMEHTTPListener(mgr, acmeHTTPAddr())
	}

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

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		slogLogger.Error("failed to close Redis connection", "error", err)
	}

	slogLogger.Info("server shutdown complete")
}

// maskDSN returns a safe-to-log representation of a Postgres DSN by stripping
// any embedded credentials. Both URL-style (postgres://user:pass@host/db) and
// keyword-style (host=... password=... dbname=...) DSNs are accepted.
func maskDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" && u.Host != "" {
		u.User = nil
		// Drop query parameters that commonly carry secrets (sslcert, sslkey,
		// passfile, etc.). Keep only the scheme/host/dbname path.
		u.RawQuery = ""
		return u.Redacted()
	}
	// keyword-style: drop password=... segments
	parts := strings.Fields(dsn)
	out := parts[:0]
	for _, p := range parts {
		if strings.HasPrefix(strings.ToLower(p), "password=") {
			out = append(out, "password=***")
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}

// =====================================================================
// ACME / Let's Encrypt wiring for custom-domain status pages (M11 / K8).
//
// Feature-flagged via environment variables so prod can enable per-env
// without a config-schema change:
//
//   IDCD_ACME_ENABLED   "true" to turn on (default off)
//   IDCD_ACME_EMAIL     contact address registered with Let's Encrypt
//   IDCD_ACME_CACHE_DIR filesystem path for autocert.DirCache
//                       (default "/var/cache/idcd-acme", falls back to
//                       "./.acme-cache" if the default isn't writable)
//   IDCD_ACME_HTTP_ADDR address for the HTTP-01 listener (default ":80")
//
// The HostPolicy queries status_pages.custom_domain_verified_at IS NOT
// NULL — only domains that completed DNS verification can request a
// cert.  This is identical to the read in
// internal/handler/status_page_public.go.
// =====================================================================

// wireACME returns an *acme.Manager when IDCD_ACME_ENABLED=true, else nil.
// pgxPool is required (the host policy hits the DB on every TLS hello).
func wireACME(pool *pgxpool.Pool, log *slog.Logger) *acmemgr.Manager {
	if !envBool("IDCD_ACME_ENABLED") {
		return nil
	}
	if pool == nil {
		log.Warn("acme: IDCD_ACME_ENABLED=true but pgxPool is nil; ACME disabled")
		return nil
	}

	email := os.Getenv("IDCD_ACME_EMAIL")
	if email == "" {
		// Email is not strictly required by autocert but Let's Encrypt
		// strongly recommends one for expiry notifications.  Log a
		// warning but proceed — staging / non-prod often runs without.
		log.Warn("acme: IDCD_ACME_EMAIL not set; Let's Encrypt expiry notices will be skipped")
	}

	cacheDir := os.Getenv("IDCD_ACME_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "/var/cache/idcd-acme"
	}

	checker := &pgxDomainChecker{pool: pool}
	mgr := acmemgr.New(acmemgr.Config{
		CacheDir: cacheDir,
		Email:    email,
	}, checker)
	log.Info("acme: manager enabled",
		"cache_dir", cacheDir,
		"http_addr", acmeHTTPAddr(),
		"email_set", email != "",
	)
	return mgr
}

// acmeHTTPAddr returns the listen address for the HTTP-01 challenge
// listener.  Defaults to ":80".
func acmeHTTPAddr() string {
	if v := os.Getenv("IDCD_ACME_HTTP_ADDR"); v != "" {
		return v
	}
	return ":80"
}

// envBool parses IDCD_* boolean env vars.  Accepts "true", "1", "yes"
// (case-insensitive).
func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

// pgxDomainChecker satisfies acmemgr.DomainChecker against the
// status_pages table.
type pgxDomainChecker struct {
	pool *pgxpool.Pool
}

// IsVerifiedDomain reports whether host has a verified custom domain.
// Uses a 2s timeout to keep TLS hello handshakes snappy even if the
// DB is degraded — a slow DB causes a TLS handshake failure rather
// than blocking the connection indefinitely.
func (c *pgxDomainChecker) IsVerifiedDomain(ctx context.Context, host string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var one int
	err := c.pool.QueryRow(ctx,
		`SELECT 1 FROM status_pages
		 WHERE custom_domain = $1
		   AND custom_domain_verified_at IS NOT NULL
		 LIMIT 1`,
		host,
	).Scan(&one)
	if err != nil {
		// pgx returns ErrNoRows when nothing matches — that's "not verified",
		// not an error.
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}