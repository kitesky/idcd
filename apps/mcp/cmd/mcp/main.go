package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
	"github.com/kite365/idcd/apps/mcp/internal/tools"
	"github.com/kite365/idcd/lib/shared/config"
)

func envInt64(name string, def int64) int64 {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return v
}

func envDuration(name string, def time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return v
}

func main() {
	apiKey := flag.String("api-key", "", "API key (overrides IDCD_API_KEY)")
	apiURL := flag.String("api-url", "", "API base URL (overrides IDCD_API_URL)")
	transport := flag.String("transport", "stdio", "Transport: stdio or http")
	port := flag.Int("port", 8082, "HTTP port (only for --transport http)")
	flag.Parse()

	key := *apiKey
	if key == "" {
		key = os.Getenv("IDCD_API_KEY")
	}

	baseURL := *apiURL
	if baseURL == "" {
		baseURL = os.Getenv("IDCD_API_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.idcd.com"
	}

	client := apiclient.New(baseURL, key)
	srv := protocol.NewServer()
	tools.RegisterAll(srv, client)

	switch *transport {
	case "http":
		// HTTP transport requires bearer-token auth + CORS allowlist
		// (see protocol/auth.go + REVIEW-FINDINGS-2026-05-16 P0#7).
		// We load cfg lazily so stdio mode keeps working without a
		// config file present (common in dev / Claude Desktop).
		if err := runHTTP(*port); err != nil {
			log.Fatalf("mcp http: %v", err)
		}
	default:
		if err := srv.RunStdio(context.Background()); err != nil {
			os.Exit(1)
		}
	}
}

// runHTTP wires the production HTTP transport: pgx-backed token validator
// + CORS allowlist + 1 MiB body cap. All applied via SetHTTPConfig; the
// existing SSEHandler / MessagesHandler entry points stay unchanged.
func runHTTP(port int) error {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	if cfg.Database.Main.DSN == "" {
		return fmt.Errorf("config: database.main.dsn is required for --transport http")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Database.Main.DSN)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pgxpool ping: %w", err)
	}

	validator := protocol.NewPGTokenValidator(pool)
	origins := parseCORSOrigins(os.Getenv("MCP_CORS_ORIGINS"))

	// Per-token rate limit. Defaults: 60 req/min/token. Override via
	// MCP_RATE_MAX (int per window) + MCP_RATE_WINDOW (Go duration string).
	rlMax := envInt64("MCP_RATE_MAX", 60)
	rlWindow := envDuration("MCP_RATE_WINDOW", time.Minute)
	limiter := protocol.NewMemoryLimiter(rlWindow, rlMax)

	// SSE heartbeat interval. Defaults to 15s; raise via MCP_SSE_HEARTBEAT.
	heartbeat := envDuration("MCP_SSE_HEARTBEAT", 15*time.Second)

	protocol.SetHTTPConfig(protocol.HTTPConfig{
		Validator:        validator,
		AllowedOrigins:   origins,
		AllowCredentials: true,
		RateLimiter:      limiter,
		SSEHeartbeat:     heartbeat,
	})

	// Rebuild server + tools after config is ready (the apiclient is the
	// same instance regardless of transport — re-using parent scope's
	// srv would also work, but keeping the wiring here makes the http
	// branch self-contained for readability).
	srv := protocol.NewServer()
	apiBase := os.Getenv("IDCD_API_URL")
	if apiBase == "" {
		apiBase = "https://api.idcd.com"
	}
	tools.RegisterAll(srv, apiclient.New(apiBase, os.Getenv("IDCD_API_KEY")))

	mux := http.NewServeMux()
	mux.Handle("/sse", protocol.SSEHandler(srv))
	mux.Handle("/messages", protocol.MessagesHandler(srv))

	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, mux)
}

// parseCORSOrigins splits a comma-separated env value into a normalized
// allowlist. Empty / whitespace-only entries are dropped. Returns nil for
// an empty input so SetHTTPConfig stays in its fail-closed default (no
// origins accepted) rather than accidentally allowing the empty-string
// origin.
func parseCORSOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
