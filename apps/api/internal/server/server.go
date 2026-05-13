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
	"github.com/kite365/idcd/packages/auth/jwt"
	"github.com/kite365/idcd/packages/auth/session"
	"github.com/kite365/idcd/packages/db/gen/idcdmain"
	"github.com/kite365/idcd/packages/shared/config"
	"github.com/kite365/idcd/packages/shared/stream"
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

	// Register metrics
	prometheus.MustRegister(s.requestsTotal)
	prometheus.MustRegister(s.requestDuration)
}

// setupRouter configures the chi router with middleware and routes.
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware chain: Recover → RequestID → Logger → SecurityHeaders → CORS → CSRF
	r.Use(middleware.Recover(s.logger))
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

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

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

		authH := handler.NewAuthHandler(q, jwtSvc, sessSvc)
		acctH := handler.NewAccountHandler(q)
		authnMW := middleware.Authn(jwtSvc, sessSvc)

		r.Route("/v1", func(r chi.Router) {
			r.Route("/auth", func(r chi.Router) {
				r.Post("/register", authH.Register)
				r.Post("/login", authH.Login)
				r.Post("/logout", authH.Logout)
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

			// Wrap response writer to capture status
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(rw, r)

			// Update metrics
			duration := time.Since(start).Seconds()
			method := r.Method
			path := r.URL.Path
			status := strconv.Itoa(rw.statusCode)

			s.requestsTotal.WithLabelValues(method, path, status).Inc()
			s.requestDuration.WithLabelValues(method, path).Observe(duration)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
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