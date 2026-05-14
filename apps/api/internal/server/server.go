// Package server provides HTTP server setup and lifecycle management.
package server

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/handler"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/ratelimit"
	"github.com/kite365/idcd/lib/shared/config"
	"github.com/kite365/idcd/lib/shared/stream"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// Server represents the HTTP server with its dependencies.
type Server struct {
	router     chi.Router
	httpServer *http.Server
	logger     *slog.Logger
	config     *config.Config
	db         *sql.DB
	pgxPool    *pgxpool.Pool
	redis      *redis.Client

	// Prometheus metrics
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

// New creates a new server instance with all middleware and routes configured.
func New(cfg *config.Config, db *sql.DB, pgxPool *pgxpool.Pool, redis *redis.Client, logger *slog.Logger) *Server {
	s := &Server{
		config:  cfg,
		db:      db,
		pgxPool: pgxPool,
		redis:   redis,
		logger:  logger,
	}

	s.setupMetrics()
	s.setupRouter()
	s.setupHTTPServer()

	return s
}

// setupMetrics initializes Prometheus metrics.
func (s *Server) setupMetrics() {
	s.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "path", "status"},
	)

	s.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Register metrics — tolerate duplicate registration (e.g., tests creating multiple Server instances).
	for _, c := range []prometheus.Collector{s.requestsTotal, s.requestDuration} {
		if err := prometheus.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic("failed to register prometheus metric: " + err.Error())
			}
		}
	}
}

// setupRouter configures the chi router with middleware and routes.
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware chain: Recover → TraceMiddleware → RequestID → Logger → SecurityHeaders → CORS → CSRF
	r.Use(middleware.Recover(s.logger))
	r.Use(telemetry.TraceMiddleware("idcd-api"))
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(s.logger))
	r.Use(middleware.SecurityHeaders(s.config.Server.Env))
	r.Use(middleware.CORS(s.config.Server.Env, s.config.Server.CORSOrigins))

	// Add CSRF protection (with exemptions for auth, probe, info endpoints)
	r.Use(middleware.CSRF())

	// Add metrics middleware
	r.Use(s.metricsMiddleware())

	// Health check routes
	healthHandler := handler.NewHealthHandler(s.db, s.redis)
	r.Get("/health", healthHandler.Health)
	r.Get("/health/deep", healthHandler.DeepHealth)

	// Prometheus metrics are served on a separate internal port (see startMetricsServer).
	// Do NOT expose /metrics on the public router.

	// CSP violation reporting endpoint
	cspReportHandler := handler.NewCSPReportHandler(s.logger)
	r.Post("/v1/csp-report", cspReportHandler.Report)

	// Auth & Account routes (requires pgxPool and JWT config)
	if s.pgxPool != nil && s.config.JWT.Secret != "" {
		jwtSvc, err := jwt.NewService(jwt.Config{SecretKey: s.config.JWT.Secret})
		if err != nil {
			panic("invalid JWT config: " + err.Error())
		}
		sessSvc := session.NewService(s.redis)
		q := idcdmain.New(s.pgxPool)

		authH := handler.NewAuthHandler(q, jwtSvc, sessSvc, s.config.JWT.Secret)
		acctH := handler.NewAccountHandler(q)
		authnMW := middleware.Authn(jwtSvc, sessSvc)

		// Strict rate limiter for auth endpoints: 5 requests/IP/minute.
		authLimiter := ratelimit.NewLimiter(s.redis, ratelimit.Config{
			WindowSize:  time.Minute,
			MaxRequests: 5,
			KeyPrefix:   "rl:auth:",
		})
		authRateMW := middleware.RateLimit(authLimiter)

		r.Route("/v1", func(r chi.Router) {
			r.Route("/auth", func(r chi.Router) {
				r.Use(authRateMW)
				r.Post("/register", authH.Register)
				r.Post("/login", authH.Login)
				r.With(authnMW).Post("/logout", authH.Logout)
				r.Post("/verify-email", authH.VerifyEmail)
				r.Post("/forgot-password", authH.ForgotPassword)
				r.Post("/reset-password", authH.ResetPassword)
			})
			r.Route("/account", func(r chi.Router) {
				r.Use(authnMW)
				r.Get("/profile", acctH.GetProfile)
				r.Patch("/profile", acctH.UpdateProfile)
				r.Delete("/", acctH.DeleteAccount)
			})
			r.Route("/info", func(r chi.Router) {
				infoH := handler.NewInfoHandler()
				r.Get("/ip", infoH.IP)
				r.Get("/whois", infoH.Whois)
				r.Get("/dns", infoH.DNS)
				r.Get("/ssl", infoH.SSL)
				r.Get("/icp", infoH.ICP)
			})
			// Probe endpoints
			streamClient := stream.New(s.redis)
			probeH := handler.NewProbeHandler(s.pgxPool, streamClient)
			r.Route("/probe", func(r chi.Router) {
				r.Post("/http", probeH.HTTP)
				r.Post("/ping", probeH.Ping)
				r.Post("/tcp", probeH.TCP)
				r.Post("/dns", probeH.DNS)
				r.Post("/traceroute", probeH.Traceroute)
			})
			r.Post("/diagnose", probeH.Diagnose)
			// Node directory endpoint
			nodesH := handler.NewNodesHandler(s.pgxPool)
			r.Get("/nodes", nodesH.List)

			// Monitor CRUD endpoints (authentication required)
			monitorH := handler.NewMonitorHandler(idcdmain.New(s.pgxPool))
			r.Route("/monitors", func(r chi.Router) {
				r.Use(authnMW)
				r.Post("/", monitorH.Create)
				r.Get("/", monitorH.List)
				r.Get("/{id}", monitorH.Get)
				r.Patch("/{id}", monitorH.Update)
				r.Delete("/{id}", monitorH.Delete)
				r.Post("/{id}/pause", monitorH.Pause)
				r.Post("/{id}/resume", monitorH.Resume)
			})

			// Admin billing endpoints
			// TODO: Add role=admin middleware once users.is_admin column exists.
			// Until then these routes should be restricted to VPN/internal network only.
			adminBillingH := handler.NewAdminBillingHandler(s.pgxPool)
			r.Route("/admin", func(r chi.Router) {
				r.Get("/refund-failed", adminBillingH.ListRefundFailed)
				r.Post("/refund-failed/{id}/retry", adminBillingH.RetryRefund)
			})

			// Alert channels, policies, and events (authentication required)
			alertH := handler.NewAlertHandler(s.pgxPool)
			r.Route("/alert-channels", func(r chi.Router) {
				r.Use(authnMW)
				r.Post("/", alertH.CreateChannel)
				r.Get("/", alertH.ListChannels)
				r.Delete("/{id}", alertH.DeleteChannel)
				r.Post("/{id}/test", alertH.TestChannel)
			})
			r.Route("/alert-policies", func(r chi.Router) {
				r.Use(authnMW)
				r.Post("/", alertH.CreatePolicy)
				r.Get("/", alertH.ListPolicies)
				r.Patch("/{id}", alertH.UpdatePolicy)
				r.Delete("/{id}", alertH.DeletePolicy)
			})
			r.Route("/alert-events", func(r chi.Router) {
				r.Use(authnMW)
				r.Get("/", alertH.ListEvents)
				r.Post("/{id}/ack", alertH.AcknowledgeEvent)
			})
		})
	}

	s.router = r
}

// setupHTTPServer configures the HTTP server.
func (s *Server) setupHTTPServer() {
	port := s.config.Server.Port
	if port == 0 {
		port = 8080 // Default port
	}

	s.httpServer = &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: s.router,

		// Timeouts
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,

		// Headers
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// metricsMiddleware updates Prometheus metrics for each request.
func (s *Server) metricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status — reuse middleware.StatusRecorder.
			rw := &middleware.StatusRecorder{ResponseWriter: w, StatusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(rw, r)

			// Update metrics
			duration := time.Since(start).Seconds()
			method := r.Method
			path := r.URL.Path
			status := strconv.Itoa(rw.StatusCode)

			s.requestsTotal.WithLabelValues(method, path, status).Inc()
			s.requestDuration.WithLabelValues(method, path).Observe(duration)
		})
	}
}


// Start starts the HTTP server and the internal metrics server.
func (s *Server) Start() error {
	go s.startMetricsServer()
	s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// startMetricsServer exposes Prometheus metrics on an internal-only port (:9091).
// This port must NOT be exposed to public traffic (bind to loopback or internal VPC only).
func (s *Server) startMetricsServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:        ":9091",
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
	}
	s.logger.Info("starting metrics server", "addr", metricsServer.Addr)
	if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("metrics server error", "error", err)
	}
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// Router returns the configured router (useful for testing).
func (s *Server) Router() chi.Router {
	return s.router
}