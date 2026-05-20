// Package server provides HTTP server setup for the Gateway service.
package server

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/gateway/internal/config"
	"github.com/kite365/idcd/apps/gateway/internal/handler"
	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/lib/shared/stream"
)

// Server represents the Gateway HTTP server.
type Server struct {
	config     *config.Config
	router     chi.Router
	httpServer *http.Server
	hub        *hub.Hub
	redis      *redis.Client
	pgxPool    handler.NodeAuthPool
	streamCli  *stream.Client
	logger     *slog.Logger
}

// New creates a new Gateway server.
func New(cfg *config.Config, h *hub.Hub, rdb *redis.Client, pool handler.NodeAuthPool, streamCli *stream.Client, logger *slog.Logger) *Server {
	s := &Server{
		config:    cfg,
		hub:       h,
		redis:     rdb,
		pgxPool:   pool,
		streamCli: streamCli,
		logger:    logger,
	}

	s.setupRouter()
	s.setupHTTPServer()

	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// NOTE: middleware.Timeout intentionally not mounted at the router level.
	// It cancels r.Context() after the configured duration and wraps the
	// ResponseWriter, which kills long-lived WebSocket connections on
	// /agent/ws (the wrapper closes the underlying conn the moment the ctx
	// expires, even though the WS body has been hijacked). Per-handler
	// timeouts (chi has middleware.WithValue, REST endpoints can use their
	// own per-request ctx) cover the slow-handler threat surface without
	// the WS-killer side effect.

	healthHandler := handler.NewHealthHandler(s.hub, s.redis, s.logger)
	r.Get("/health", healthHandler.Health)
	r.Get("/health/deep", healthHandler.DeepHealth)

	// Prometheus metrics — require Bearer token from config to prevent public exposure.
	metricsToken := s.config.MetricsToken
	r.Handle("/metrics", metricsAuthMiddleware(metricsToken, promhttp.Handler()))

	// WebSocket endpoint for agent nodes
	wsHandler := handler.NewWSHandler(s.hub, nil, s.pgxPool, s.streamCli, s.logger)
	r.Get("/agent/ws", wsHandler.ServeWS)

	// Internal endpoint for OTA broadcast (called by API service, not public)
	upgradeHandler := handler.NewUpgradeHandler(s.hub)
	r.Post("/internal/broadcast-upgrade", upgradeHandler.BroadcastUpgrade)

	// S2 W8: reverse-proxy /v1/cert/* to cert-svc. The proxy preserves
	// Authorization headers and cookies so the user's session / JWT flows
	// through unchanged. The one-shot /v1/cert/certs/{id}/download endpoint
	// is mounted explicitly (rather than relying solely on the /v1/cert/*
	// wildcard) so its OUTSIDE-auth-middleware shape on cert-svc is
	// preserved end-to-end; gateway adds no extra auth either.
	if s.config.CertSvcURL != "" {
		if proxy, err := newCertSvcProxy(s.config.CertSvcURL, s.logger); err == nil {
			r.Handle("/v1/cert/*", proxy)
			r.Handle("/v1/cert", proxy)
			// Admin surface on the same upstream; cert-svc enforces a
			// separate Bearer admin token on /v1/admin/cert/*. Gateway
			// adds no extra auth — the upstream is the source of truth.
			r.Handle("/v1/admin/cert/*", proxy)
			r.Handle("/v1/admin/cert", proxy)
		} else {
			s.logger.Error("cert-svc reverse proxy disabled — invalid CertSvcURL",
				"url", s.config.CertSvcURL, "error", err)
		}
	}

	s.router = r
}

// newCertSvcProxy returns an http.Handler that reverse-proxies requests to
// the cert-svc upstream identified by rawURL. The returned proxy:
//
//   - preserves Authorization headers and cookies (httputil.ReverseProxy's
//     default Director copies r.Header verbatim, so no extra wiring needed)
//   - responds with 502 Bad Gateway when the upstream is unreachable or
//     returns a transport-level error (avoids leaking Go's default
//     "http: proxy error" text)
//   - logs upstream errors via the gateway's structured logger so SREs can
//     correlate 502s with cert-svc availability
//
// rawURL must be a fully qualified scheme://host[:port] base URL, e.g.
// "http://cert-svc:8082" — paths in rawURL are preserved as a prefix.
func newCertSvcProxy(rawURL string, logger *slog.Logger) (http.Handler, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, &url.Error{Op: "parse", URL: rawURL, Err: http.ErrAbortHandler}
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if logger != nil {
			logger.Error("cert-svc upstream unreachable",
				"method", r.Method, "path", r.URL.Path, "upstream", target.String(), "error", err)
		}
		http.Error(w, "bad gateway: cert-svc unreachable", http.StatusBadGateway)
	}
	return proxy, nil
}

func (s *Server) setupHTTPServer() {
	s.httpServer = &http.Server{
		Addr:    s.config.ListenAddr,
		Handler: s.router,
		// ReadTimeout / WriteTimeout intentionally unset: the gateway carries
		// long-lived WebSocket connections on /agent/ws, and Go's
		// ReadTimeout fires from-request-start through body-end, which on
		// a hijacked WS connection severs the underlying TCP at ~30s and
		// causes "websocket: close 1006 (abnormal closure): unexpected EOF"
		// reconnect storms. Per-handler timeouts (chi middleware.Timeout
		// for non-WS routes, hub heartbeat/pong deadlines for WS) cover
		// the same threat surface without the WS-killer side effect.
		//
		// Slowloris is still mitigated by ReadHeaderTimeout, which only
		// gates the time spent reading request headers.
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("starting Gateway server", "addr", s.httpServer.Addr)

	if s.config.UseTLS() {
		s.logger.Info("TLS enabled", "cert", s.config.TLSCert, "key", s.config.TLSKey)
		return s.httpServer.ListenAndServeTLS(s.config.TLSCert, s.config.TLSKey)
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down Gateway server")
	return s.httpServer.Shutdown(ctx)
}

// Router returns the configured chi router (used in tests to mount test handlers).
func (s *Server) Router() chi.Router {
	return s.router
}

// metricsAuthMiddleware wraps a handler with optional Bearer token protection.
// If token is empty (dev mode), requests pass through unauthenticated.
func metricsAuthMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + token
		if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
