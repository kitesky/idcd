// Package server provides HTTP server setup for the Gateway service.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/gateway/internal/config"
	"github.com/kite365/idcd/apps/gateway/internal/handler"
	"github.com/kite365/idcd/apps/gateway/internal/hub"
	"github.com/kite365/idcd/packages/shared/stream"
)

// Server represents the Gateway HTTP server.
type Server struct {
	config     *config.Config
	router     chi.Router
	httpServer *http.Server
	hub        *hub.Hub
	redis      *redis.Client
	streamCli  *stream.Client
	logger     *slog.Logger
}

// New creates a new Gateway server.
func New(cfg *config.Config, h *hub.Hub, rdb *redis.Client, streamCli *stream.Client, logger *slog.Logger) *Server {
	s := &Server{
		config:    cfg,
		hub:       h,
		redis:     rdb,
		streamCli: streamCli,
		logger:    logger,
	}

	s.setupRouter()
	s.setupHTTPServer()

	return s
}

// setupRouter configures the HTTP router.
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware chain
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health checks
	healthHandler := handler.NewHealthHandler(s.hub, s.redis, s.logger)
	r.Get("/health", healthHandler.Health)
	r.Get("/health/deep", healthHandler.DeepHealth)

	// Prometheus metrics
	r.Handle("/metrics", promhttp.Handler())

	// WebSocket endpoint for agent nodes
	wsHandler := handler.NewWSHandler(s.hub, nil, s.streamCli, s.logger)
	r.Get("/agent/ws", wsHandler.ServeWS)

	s.router = r
}

// setupHTTPServer configures the HTTP server.
func (s *Server) setupHTTPServer() {
	s.httpServer = &http.Server{
		Addr:    s.config.ListenAddr,
		Handler: s.router,

		// Timeouts
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
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

// Router returns the configured router (useful for testing).
func (s *Server) Router() chi.Router {
	return s.router
}
