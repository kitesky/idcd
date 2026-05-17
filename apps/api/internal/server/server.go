// Package server provides HTTP server setup and lifecycle management.
package server

import (
	"context"
	"crypto/tls"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	acmemgr "github.com/kite365/idcd/apps/api/internal/acme"
	"github.com/kite365/idcd/apps/api/internal/billing"
	"github.com/kite365/idcd/apps/api/internal/handler"
	apiI18n "github.com/kite365/idcd/apps/api/internal/i18n"
	_ "github.com/kite365/idcd/apps/api/internal/metrics" // register business metrics with Prometheus default registry
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/quota"
	"github.com/kite365/idcd/apps/api/internal/repository"
	"github.com/kite365/idcd/lib/auth/jwt"
	"github.com/kite365/idcd/lib/auth/session"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/ratelimit"
	"github.com/kite365/idcd/lib/shared/aesenc"
	"github.com/kite365/idcd/lib/shared/config"
	sharedi18n "github.com/kite365/idcd/lib/shared/i18n"
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

// redisBlocklistAdapter adapts *redis.Client to the IPBlocklistStore and BlocklistStore interfaces.
type redisBlocklistAdapter struct {
	client *redis.Client
}

func (a *redisBlocklistAdapter) SIsMember(ctx context.Context, key, member string) (bool, error) {
	return a.client.SIsMember(ctx, key, member).Result()
}

func (a *redisBlocklistAdapter) SAdd(ctx context.Context, key string, members ...string) error {
	args := make([]any, len(members))
	for i, m := range members {
		args[i] = m
	}
	return a.client.SAdd(ctx, key, args...).Err()
}

func (a *redisBlocklistAdapter) SRem(ctx context.Context, key string, members ...string) error {
	args := make([]any, len(members))
	for i, m := range members {
		args[i] = m
	}
	return a.client.SRem(ctx, key, args...).Err()
}

// asynqEnqueuer adapts *asynq.Client to the handler.AuthEnqueuer interface.
type asynqEnqueuer struct {
	client *asynq.Client
}

func (a *asynqEnqueuer) EnqueueTask(ctx context.Context, taskType string, payload []byte, queue string) error {
	_, err := a.client.EnqueueContext(ctx,
		asynq.NewTask(taskType, payload),
		asynq.Queue(queue),
	)
	return err
}

// buildJWTService constructs the JWT service used by auth handlers and the
// authn middleware. When Redis is wired we attach a Redis-backed JTI
// blocklist so refresh / logout actually revokes the old token across all
// API replicas; without this the legacy NewService() path would skip
// revocation entirely and a leaked JWT would remain valid until its
// natural exp. See lib/auth/jwt/blocklist.go for the contract.
//
// When s.redis is nil (test / minimal harness), no blocklist is attached
// and the Service falls back to the legacy "no revocation" behavior so
// existing callers that don't run Redis still work.
func (s *Server) buildJWTService() (*jwt.Service, error) {
	cfg := jwt.Config{SecretKey: s.config.JWT.Secret}
	if s.redis == nil {
		return jwt.NewServiceWithOptions(cfg)
	}
	return jwt.NewServiceWithOptions(cfg, jwt.WithBlocklist(jwt.NewRedisBlocklist(s.redis)))
}

// setupRouter configures the chi router with middleware and routes.
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// IP blocklist adapter — wraps *redis.Client; nil-safe (middleware is fail-open when redis is nil).
	var blocklistAdapter middleware.IPBlocklistStore
	if s.redis != nil {
		blocklistAdapter = &redisBlocklistAdapter{client: s.redis}
	}

	// Middleware chain: Recover → TraceMiddleware → RequestID → i18n → Logger → SecurityHeaders → CORS → CSRF
	r.Use(middleware.Recover(s.logger))
	r.Use(telemetry.TraceMiddleware("idcd-api"))
	r.Use(middleware.RequestID())
	// i18n locale resolution must run before Authn so claims-stashing in
	// authn.go has somewhere to write to; it doesn't depend on any auth
	// state itself (the JWT claim is read from ctx if Authn already ran on
	// a sub-route, otherwise we fall through to Accept-Language / default).
	r.Use(apiI18n.Middleware(sharedi18n.MustDefault()))
	r.Use(middleware.Logger(s.logger))
	r.Use(middleware.SecurityHeaders(s.config.Server.Env))
	r.Use(middleware.CORS(s.config.Server.Env, s.config.Server.CORSOrigins))

	// Add CSRF protection (with exemptions for auth, probe, info endpoints)
	r.Use(middleware.CSRF())

	// IP blocklist check — runs before rate limiting so banned IPs are rejected early.
	r.Use(middleware.IPBlocklist(blocklistAdapter))

	// Add metrics middleware
	r.Use(s.metricsMiddleware())

	// Health check routes
	healthHandler := handler.NewHealthHandler(s.db, s.redis)
	r.Get("/health", healthHandler.Health)
	r.Get("/health/deep", healthHandler.DeepHealth)

	// OpenAPI spec endpoint — no auth required
	openAPIHandler := handler.NewOpenAPIHandler()
	r.Get("/v1/openapi.json", openAPIHandler.OpenAPI)

	// Transparency dashboard — public, no auth required
	transparencyH := handler.NewTransparencyHandler(s.pgxPool)
	r.Get("/v1/transparency", transparencyH.Get)

	// CDN leaderboard — public, no auth required
	leaderboardH := handler.NewLeaderboardHandler(s.pgxPool)
	r.Get("/v1/leaderboard/cdn", leaderboardH.CDNLeaderboard)

	// Prometheus metrics are served on a separate internal port (see startMetricsServer).
	// Do NOT expose /metrics on the public router.

	// CSP violation reporting endpoint
	cspReportHandler := handler.NewCSPReportHandler(s.logger)
	r.Post("/v1/csp-report", cspReportHandler.Report)

	// Auth & Account routes (requires pgxPool and JWT config)
	if s.pgxPool != nil && s.config.JWT.Secret != "" {
		jwtSvc, err := s.buildJWTService()
		if err != nil {
			panic("invalid JWT config: " + err.Error())
		}
		sessSvc := session.NewService(s.redis)
		q := idcdmain.New(s.pgxPool)

		fieldCipher := newFieldCipher(s.config.Encryption.FieldKey, s.logger)

		// Wire async dispatch via asynq (uses the same Redis instance as the notifier).
		// One asynq.Client is shared across all enqueuer adapters (auth emails,
		// billing refund retries, …) — they all hit the same Redis broker.
		var (
			asynqClient      *asynq.Client
			enqueuer         handler.AuthEnqueuer
			asynqBillingEnq  handler.BillingEnqueuer
		)
		if s.redis != nil {
			asynqClient = asynq.NewClient(asynq.RedisClientOpt{
				Addr:     s.config.Redis.Addr,
				Password: s.config.Redis.Password,
				DB:       s.config.Redis.DB,
			})
			enqueuer = &asynqEnqueuer{client: asynqClient}
			asynqBillingEnq = repository.NewBillingEnqueuer(asynqClient)
		}

		authH := handler.NewAuthHandler(q, jwtSvc, sessSvc, s.config.JWT.Secret).
			WithReferralPool(s.pgxPool).
			WithMFA(s.pgxPool, s.redis).
			WithFieldCipher(fieldCipher).
			WithEnqueuer(enqueuer).
			WithAppBaseURL(s.config.Server.AppBaseURL)
		acctH := handler.NewAccountHandler(q)
		apiKeyH := handler.NewAPIKeyHandler(q)
		patH := handler.NewPATHandler(s.pgxPool)
		totpH := handler.NewTOTPHandler(s.pgxPool, s.redis, fieldCipher)
		webauthnH := handler.NewWebAuthnHandler(s.pgxPool, s.redis, "").
			WithAuth(jwtSvc, sessSvc).
			WithOrigins(s.config.Server.CORSOrigins)
		sessionH := handler.NewSessionHandler(sessSvc, s.redis)

		// Multi-modal auth — accept JWT (browser sessions), PATs (idcd_pat_*),
		// and API keys (sk_live_* / sk_test_*). The PAT/APIKey verifiers run
		// against the same pgx pool the handlers write to, so newly minted
		// tokens are immediately usable.
		patVerifier := repository.NewPATVerifier(s.pgxPool)
		apiKeyVerifier := repository.NewAPIKeyVerifier(s.pgxPool)
		authnMW := middleware.AuthnWithTokens(jwtSvc, sessSvc, patVerifier, apiKeyVerifier)

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
				r.With(authnMW).Post("/resend-verify", authH.ResendVerifyEmail)
				r.Post("/forgot-password", authH.ForgotPassword)
				r.Post("/reset-password", authH.ResetPassword)
				r.Post("/2fa-login", authH.TwoFactorLogin)
				r.Post("/passkeys/begin", webauthnH.AuthBegin)
				r.Post("/passkeys/complete", webauthnH.AuthComplete)

				oauthCfg := handler.OAuthConfig{
					DingTalkAppID:  s.config.OAuth.DingTalk.AppID,
					DingTalkSecret: s.config.OAuth.DingTalk.AppSecret,
					FeishuAppID:    s.config.OAuth.Feishu.AppID,
					FeishuSecret:   s.config.OAuth.Feishu.AppSecret,
					CallbackBase:   s.config.OAuth.CallbackBase,
				}
				stateStore := handler.NewRedisStateStore(s.redis)
				oauthH := handler.NewOAuthHandler(oauthCfg, q, jwtSvc, sessSvc, stateStore)
				r.Get("/dingtalk", oauthH.DingTalkLogin)
				r.Get("/dingtalk/callback", oauthH.DingTalkCallback)
				r.Get("/feishu", oauthH.FeishuLogin)
				r.Get("/feishu/callback", oauthH.FeishuCallback)
			})
			r.Route("/account", func(r chi.Router) {
				r.Use(authnMW)
				r.Get("/profile", acctH.GetProfile)
				r.Patch("/profile", acctH.UpdateProfile)
				r.Post("/avatar", acctH.UploadAvatar)
				r.Patch("/password", acctH.ChangePassword)
				r.Delete("/", acctH.DeleteAccount)
				// API key management
				r.Get("/api-keys", apiKeyH.ListAPIKeys)
				r.Post("/api-keys", apiKeyH.CreateAPIKey)
				r.Delete("/api-keys/{id}", apiKeyH.RevokeAPIKey)
				// Personal Access Token management
				r.Route("/tokens", func(r chi.Router) {
					r.Post("/", patH.Create)
					r.Get("/", patH.List)
					r.Delete("/{id}", patH.Delete)
				})
				// 2FA / TOTP management
				r.Route("/2fa", func(r chi.Router) {
					r.Post("/setup", totpH.Setup)
					r.Post("/verify", totpH.Verify)
					r.Post("/disable", totpH.Disable)
					r.Get("/status", totpH.Status)
				})
				// Passkey (WebAuthn) management
				r.Route("/passkeys", func(r chi.Router) {
					r.Post("/register/begin", webauthnH.RegisterBegin)
					r.Post("/register/complete", webauthnH.RegisterComplete)
					r.Get("/", webauthnH.List)
					r.Delete("/{id}", webauthnH.Delete)
				})
				// Session management
				r.Route("/sessions", func(r chi.Router) {
					r.Get("/", sessionH.ListSessions)
					r.Delete("/{session_id}", sessionH.RevokeSession)
				})
			})
			r.Route("/info", func(r chi.Router) {
				infoH := handler.NewInfoHandler()
				r.Get("/ip", infoH.IP)
				r.Get("/whois", infoH.Whois)
				r.Get("/dns", infoH.DNS)
				r.Get("/ssl", infoH.SSL)
				r.Get("/icp", infoH.ICP)
				// T5–T11 — handler 已存在但未挂载，导致前端 404
				r.Get("/rdns", infoH.RDNS)
				r.Get("/mx", infoH.MX)
				r.Get("/spf", infoH.SPF)
				r.Get("/dmarc", infoH.DMARC)
				r.Get("/dkim", infoH.DKIM)
				r.Get("/asn", infoH.ASN)
				r.Get("/bgp", infoH.BGP)
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
				// T12–T14 + N4 — handler 已存在但未挂载
				r.Post("/mtr", probeH.MTR)
				r.Post("/smtp", probeH.SMTP)
				r.Post("/ntp", probeH.NTP)
				r.Post("/speedtest", probeH.Speedtest)
				// T1 — async 任务结果查询
				r.Get("/tasks/{taskId}", probeH.TaskResult)
			})
			r.Post("/diagnose", probeH.Diagnose)

			// T2 — 一键诊断报告存档（Redis backed）
			if s.redis != nil {
				diagReportH := handler.NewDiagnoseReportHandler(s.redis)
				r.Post("/diagnose/reports", diagReportH.SaveReport)
				r.Get("/diagnose/reports/{id}", diagReportH.GetReport)
			}
			// Node directory endpoint
			nodesH := handler.NewNodesHandler(s.pgxPool)
			r.Get("/nodes", nodesH.List)
			// Node diagnostics endpoint (public, no auth)
			nodeDiagH := handler.NewNodeDiagnosticsHandler(s.pgxPool)
			r.Get("/nodes/{id}/diagnostics", nodeDiagH.Diagnostics)

			// API quota rate limiter (per-user daily limit)
			apiRateLimiter := quota.NewAPIRateLimiter(s.redis)

			// planLookup fetches the user's active subscription plan.
			planLookup := middleware.APIPlanLookup(func(ctx context.Context, userID string) string {
				var plan string
				err := s.pgxPool.QueryRow(ctx,
					`SELECT plan FROM subscriptions WHERE user_id = $1 AND status = 'active' LIMIT 1`,
					userID,
				).Scan(&plan)
				if err != nil {
					return "free"
				}
				return plan
			})

			// Quota middleware: applied to authenticated routes after authnMW.
			apiQuotaMW := middleware.APIQuotaMiddleware(apiRateLimiter, planLookup)

			// Monitor CRUD endpoints (authentication required)
			monitorH := handler.NewMonitorHandler(idcdmain.New(s.pgxPool)).WithQuotaPool(s.pgxPool).WithBulkPool(s.pgxPool)
			monitorStreamH := handler.NewMonitorStreamHandler(idcdmain.New(s.pgxPool), s.pgxPool)
			monitorChecksH := handler.NewMonitorChecksHandler(idcdmain.New(s.pgxPool), s.pgxPool)
			anchorH := handler.NewAnchorHandler(idcdmain.New(s.pgxPool), s.pgxPool)
			agentObsH := handler.NewAgentObsHandler(idcdmain.New(s.pgxPool), s.pgxPool)
			r.Route("/monitors", func(r chi.Router) {
				r.Use(authnMW)
				r.Use(apiQuotaMW)
				r.Post("/", monitorH.Create)
				r.Get("/", monitorH.List)
				r.With(authnMW).Post("/bulk", monitorH.BulkAction)
				r.Get("/{id}", monitorH.Get)
				r.Patch("/{id}", monitorH.Update)
				r.Delete("/{id}", monitorH.Delete)
				r.Post("/{id}/pause", monitorH.Pause)
				r.Post("/{id}/resume", monitorH.Resume)
				r.With(authnMW).Get("/{id}/stream", monitorStreamH.Stream)
				r.With(authnMW).Get("/{id}/checks", monitorChecksH.List)
				r.With(authnMW).Get("/{id}/baseline", anchorH.GetBaseline)
				r.With(authnMW).Get("/{id}/deviations", anchorH.ListDeviations)
				r.With(authnMW).Post("/{id}/agent-obs", agentObsH.CreateConfig)
				r.With(authnMW).Get("/{id}/agent-obs", agentObsH.GetConfig)
				r.With(authnMW).Patch("/{id}/agent-obs", agentObsH.UpdateConfig)
				r.With(authnMW).Delete("/{id}/agent-obs", agentObsH.DeleteConfig)
				r.With(authnMW).Get("/{id}/agent-obs/checks", agentObsH.ListChecks)
			})

			// Admin billing endpoints — require admin token via Authorization: Bearer header.
			adminBillingH := handler.NewAdminBillingHandler(s.pgxPool).WithEnqueuer(asynqBillingEnq)
			billingAdminAuthH := handler.NewAdminHandler(s.pgxPool, s.config.Server.AdminToken)
			r.Route("/admin", func(r chi.Router) {
				r.Use(billingAdminAuthH.AdminAuthMiddleware)
				r.Get("/refund-failed", adminBillingH.ListRefundFailed)
				r.Post("/refund-failed/{id}/retry", adminBillingH.RetryRefund)
			})

			// Billing endpoints (subscribe, cancel, invoices, webhook, stub-confirm)
			var billingProvider billing.Provider
			if s.config.Payment.Enabled && s.config.Payment.APIKey != "" {
				billingProvider = billing.NewPaymentHubProvider(billing.PaymentHubConfig{
					BaseURL:       s.config.Payment.BaseURL,
					APIKey:        s.config.Payment.APIKey,
					APISecret:     s.config.Payment.APISecret,
					WebhookSecret: s.config.Payment.WebhookSecret,
					Channel:       s.config.Payment.Channel,
					Currency:      s.config.Payment.Currency,
				})
			} else {
				billingProvider = billing.NewStubProvider()
			}
			billingH := handler.NewBillingHandler(s.pgxPool, billingProvider).WithEnqueuer(asynqBillingEnq).WithVerdictPublisher(repository.NewVerdictPublisher(s.redis))
			r.Route("/billing", func(r chi.Router) {
				// Authenticated routes
				r.With(authnMW).Post("/subscribe", billingH.Subscribe)
				r.With(authnMW).Post("/cancel", billingH.Cancel)
				r.With(authnMW).Get("/subscription", billingH.GetSubscription)
				r.With(authnMW).Get("/invoices", billingH.ListInvoices)
				// Unauthenticated routes
				r.Post("/webhook", billingH.Webhook)
				r.Get("/stub-confirm", billingH.StubConfirm)
			})

			// Quota status endpoint
			quotaH := handler.NewQuotaHandler(s.pgxPool, apiRateLimiter)
			r.Route("/account/quota", func(r chi.Router) {
				r.Use(authnMW)
				r.Use(apiQuotaMW)
				r.Get("/", quotaH.GetQuota)
			})

			// Status page user CRUD endpoints (authentication required)
			statusPageUserH := handler.NewStatusPageUserHandler(s.pgxPool)
			r.Route("/status-pages", func(r chi.Router) {
				r.Use(authnMW)
				r.Get("/", statusPageUserH.List)
				r.Post("/", statusPageUserH.Create)
				r.Get("/{id}", statusPageUserH.Get)
				r.Delete("/{id}", statusPageUserH.Delete)
			})

			// Status page custom domain endpoints (authentication required)
			statusPageDomainH := handler.NewStatusPageDomainHandler(idcdmain.New(s.pgxPool), s.logger)
			r.Route("/status-pages/{id}/domain", func(r chi.Router) {
				r.Use(authnMW)
				r.Patch("/", statusPageDomainH.SetStatusPageDomain)
				r.Get("/verify", statusPageDomainH.VerifyStatusPageDomain)
			})

			// Status page public endpoint (no auth)
			statusPagePublicH := handler.NewStatusPagePublicHandler(s.pgxPool)
			r.Get("/status-pages/{slug}/public", statusPagePublicH.Get)

			// Status page subscription endpoints
			statusSubH := handler.NewStatusSubscriptionHandler(s.pgxPool)
			r.Post("/status-pages/{slug}/subscribe", statusSubH.Subscribe)
			r.Get("/status-pages/{slug}/verify", statusSubH.Verify)
			r.Delete("/status-pages/{slug}/unsubscribe", statusSubH.Unsubscribe)
			r.With(authnMW).Get("/status-pages/{slug}/subscriptions", statusSubH.List)
			r.With(authnMW).Delete("/status-pages/{slug}/subscriptions/{id}", statusSubH.Delete)

			// Dashboard summary and pins endpoints (authentication required)
			dashboardH := handler.NewDashboardHandler(s.pgxPool, s.redis)
			r.Route("/dashboard", func(r chi.Router) {
				r.Use(authnMW)
				r.Get("/summary", dashboardH.Summary)
				r.Get("/pins", dashboardH.GetPins)
				r.Put("/pins", dashboardH.UpdatePins)
			})

			// SLA monthly report endpoint + noise report (authentication required)
			slaH := handler.NewSLAHandler(s.pgxPool)
			noiseH := handler.NewAlertNoiseHandler(s.pgxPool)
			r.Route("/reports", func(r chi.Router) {
				r.Use(authnMW)
				r.Get("/sla", slaH.GetSLA)
				r.Get("/alert-noise", noiseH.NoiseReport)
			})

			// Alert channels, policies, events, silences, groups (authentication required)
			alertH := handler.NewAlertHandler(s.pgxPool)
			alertNotifH := handler.NewAlertNotificationHandler(s.pgxPool)
			r.Route("/alert-channels", func(r chi.Router) {
				r.Use(authnMW)
				r.Use(apiQuotaMW)
				r.Post("/", alertH.CreateChannel)
				r.Get("/", alertH.ListChannels)
				r.Delete("/{id}", alertH.DeleteChannel)
				r.Post("/{id}/test", alertH.TestChannel)
				r.With(authnMW).Get("/{id}/notifications", alertNotifH.List)
			})
			r.Route("/alert-policies", func(r chi.Router) {
				r.Use(authnMW)
				r.Use(apiQuotaMW)
				r.Post("/", alertH.CreatePolicy)
				r.Get("/", alertH.ListPolicies)
				r.Patch("/{id}", alertH.UpdatePolicy)
				r.Delete("/{id}", alertH.DeletePolicy)
			})
			r.Route("/alert-events", func(r chi.Router) {
				r.Use(authnMW)
				r.Use(apiQuotaMW)
				r.Get("/", alertH.ListEvents)
				r.Post("/{id}/ack", alertH.AcknowledgeEvent)
			})
			r.Route("/alert-silences", func(r chi.Router) {
				r.Use(authnMW)
				r.Post("/", noiseH.CreateSilence)
				r.Get("/", noiseH.ListSilences)
				r.Delete("/{id}", noiseH.DeleteSilence)
			})
			r.Route("/alert-groups", func(r chi.Router) {
				r.Use(authnMW)
				r.Post("/", noiseH.CreateGroup)
				r.Get("/", noiseH.ListGroups)
				r.Delete("/{id}", noiseH.DeleteGroup)
			})

			// Team / Org endpoints (authentication required)
			teamH := handler.NewTeamHandler(s.pgxPool)
			teamKeyH := handler.NewTeamAPIKeyHandler(s.pgxPool)
			teamBillingH := handler.NewTeamBillingHandler(s.pgxPool, billingProvider)
			r.Route("/teams", func(r chi.Router) {
				r.Use(authnMW)
				r.Post("/", teamH.Create)
				r.Get("/", teamH.List)
				r.Post("/accept-invitation", teamH.AcceptInvitation)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", teamH.Get)
					r.Patch("/", teamH.Update)
					r.Delete("/", teamH.Delete)
					r.Get("/members", teamH.ListMembers)
					r.Patch("/members/{user_id}", teamH.UpdateMemberRole)
					r.Delete("/members/{user_id}", teamH.RemoveMember)
					r.Get("/invitations", teamH.ListInvitations)
					r.Post("/invitations", teamH.CreateInvitation)
					r.Delete("/invitations/{inv_id}", teamH.RevokeInvitation)
					r.Route("/api-keys", func(r chi.Router) {
						r.Post("/", teamKeyH.Create)
						r.Get("/", teamKeyH.List)
						r.Delete("/{key_id}", teamKeyH.Delete)
					})
					r.Route("/billing", func(r chi.Router) {
						r.Post("/subscribe", teamBillingH.Subscribe)
						r.Get("/subscription", teamBillingH.GetSubscription)
					})
				})
			})

			// Referral code and reward endpoints (authentication required)
			referralH := handler.NewReferralHandler(s.pgxPool)
			r.Route("/referral", func(r chi.Router) {
				r.Use(authnMW)
				r.Post("/code", referralH.GetOrCreateCode)
				r.Get("/code", referralH.GetCode)
				r.Get("/rewards", referralH.ListRewards)
			})
		})

		// Internal endpoints — consumed by the status Next.js app.
		// TODO: Restrict to VPN/internal network via network policy before production.
		statusPageInternalH := handler.NewStatusPageInternalHandler(idcdmain.New(s.pgxPool))
		r.Route("/internal/status-pages", func(r chi.Router) {
			r.Get("/by-domain", statusPageInternalH.ByDomain)
		})

		// Agent enrollment endpoints
		gatewayWSS := s.config.AgentGateway.PublicWSS
		if gatewayWSS == "" {
			gatewayWSS = "wss://gateway.idcd.com" // production default
		}
		enrollH := handler.NewNodeEnrollmentHandler(s.pgxPool, gatewayWSS, s.config.Server.AdminToken)
		r.Post("/v1/agent/enroll", enrollH.Enroll)

		// Admin node lifecycle endpoint (P0 #9 — flip pending/offline → active).
		// NodeActivate runs its own admin-token check (h.isAdmin), matching the
		// other NodeEnrollmentHandler routes; no extra middleware needed here.
		r.Post("/v1/admin/nodes/{node_id}/activate", enrollH.NodeActivate)

		// Agent node management (admin)
		nodeCmdH := handler.NewNodeCommandHandler(s.pgxPool, s.config.Server.AdminToken)
		r.Route("/internal/admin/nodes", func(r chi.Router) {
			r.Post("/enrollment-tokens", enrollH.CreateEnrollmentToken)
			r.Get("/", nodeCmdH.ListNodes)
			r.Post("/{node_id}/upgrade", nodeCmdH.QueueUpgrade)
			r.Post("/{node_id}/reload-config", nodeCmdH.QueueReloadConfig)
		})

		// Prometheus metrics endpoint — VPN/internal only (no auth; rely on network policy).
		// Exposes the default Prometheus registry including idcd business metrics from
		// the internal/metrics package.
		r.Get("/internal/metrics", promhttp.Handler().ServeHTTP)

		// Admin management endpoints (token-protected, VPN-only in production).
		adminH := handler.NewAdminHandler(s.pgxPool, s.config.Server.AdminToken)
		var blocklistHandlerStore handler.BlocklistStore
		if s.redis != nil {
			blocklistHandlerStore = &redisBlocklistAdapter{client: s.redis}
		}
		blocklistH := handler.NewAdminBlocklistHandler(blocklistHandlerStore)
		cdnAdminH := handler.NewAdminCDNHandler(s.pgxPool)
		// N3 — agent OTA 灰度升级 admin 端点（前端 /admin/upgrades 调用）。
		// 注意：NodeUpgradeHandler.triggerBroadcast 走 HTTP POST 调 gateway 的
		// /internal/broadcast-upgrade，必须传内部 HTTP URL，不能传 wss://。
		gatewayInternalURL := s.config.AgentGateway.InternalURL
		nodeUpgradeH := handler.NewNodeUpgradeHandler(s.pgxPool, gatewayInternalURL)
		r.Route("/internal/admin", func(r chi.Router) {
			r.Use(adminH.AdminAuthMiddleware)
			r.Get("/metrics", adminH.AdminMetrics)
			r.Get("/users", adminH.AdminUsers)
			r.Get("/users/{id}", adminH.AdminUserDetail)
			r.Post("/block-ip", blocklistH.BlockIP)
			r.Delete("/block-ip", blocklistH.UnblockIP)
			r.Post("/cdn-monitors/seed", cdnAdminH.Seed)
			r.Post("/test-email", adminH.TestEmail)
			r.Post("/upgrade-rollouts", nodeUpgradeH.Create)
			r.Get("/upgrade-rollouts", nodeUpgradeH.List)
			r.Patch("/upgrade-rollouts/{id}", nodeUpgradeH.Update)
		})

		// Community node application and points endpoints.
		communityH := handler.NewCommunityNodeHandler(s.pgxPool)
		r.With(authnMW).Post("/v1/nodes/apply", communityH.Apply)
		r.With(authnMW).Get("/v1/nodes/my-applications", communityH.MyApplications)
		r.With(authnMW).Get("/v1/account/points", communityH.GetPoints)
		r.With(authnMW).Post("/v1/account/points/redeem", communityH.Redeem)
		communityAdminH := handler.NewAdminHandler(s.pgxPool, s.config.Server.AdminToken)
		r.With(communityAdminH.AdminAuthMiddleware).Get("/v1/admin/node-applications", communityH.AdminList)
		r.With(communityAdminH.AdminAuthMiddleware).Patch("/v1/admin/node-applications/{id}", communityH.AdminUpdate)
		// 前端实际调 PATCH .../{id}/review；保留 /review 别名以兼容（K8 路由对齐）
		r.With(communityAdminH.AdminAuthMiddleware).Patch("/v1/admin/node-applications/{id}/review", communityH.AdminUpdate)

		// On-Call rotation endpoints (authentication required)
		oncallH := handler.NewOncallHandler(s.pgxPool)
		r.Route("/v1/oncall/schedules", func(r chi.Router) {
			r.Use(authnMW)
			r.Post("/", oncallH.CreateSchedule)
			r.Get("/", oncallH.ListSchedules)
			r.Get("/{id}", oncallH.GetSchedule)
			r.Post("/{id}/participants", oncallH.AddParticipant)
			r.Delete("/{id}/participants/{user_id}", oncallH.RemoveParticipant)
			r.Post("/{id}/overrides", oncallH.CreateOverride)
			r.Get("/{id}/current", oncallH.GetCurrentOnCall)
		})

		// Incident postmortem endpoints (authentication required)
		pmH := handler.NewPostmortemHandler(s.pgxPool)
		r.With(authnMW).Post("/v1/incidents/{event_id}/draft", pmH.Draft)
		r.With(authnMW).Get("/v1/incidents/{event_id}/postmortem", pmH.Get)
		r.With(authnMW).Patch("/v1/incidents/{event_id}/postmortem", pmH.Update)
		r.With(authnMW).Get("/v1/incidents", pmH.List)

		// Beta invitation endpoints (user-facing + admin).
		betaH := handler.NewBetaInvitationHandler(s.pgxPool)
		r.Route("/v1/beta", func(r chi.Router) {
			r.With(authnMW).Post("/request", betaH.RequestBeta)
			r.With(authnMW).Get("/status", betaH.GetBetaStatus)
			r.With(authnMW).Post("/redeem", betaH.RedeemBeta)
		})
		betaAdminH := handler.NewAdminHandler(s.pgxPool, s.config.Server.AdminToken)
		r.Route("/v1/admin/beta-invitations", func(r chi.Router) {
			r.Use(betaAdminH.AdminAuthMiddleware)
			r.Get("/", betaH.AdminListBetaInvitations)
			r.Post("/", betaH.AdminCreateBetaInvitation)
			r.Patch("/{id}", betaH.AdminUpdateBetaInvitation)
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

// newFieldCipher creates the AES-256-GCM cipher used for field-level at-rest
// encryption.  If fieldKey is empty a dev-only all-zeros key is used with a
// warning; production should always supply a real key.
func newFieldCipher(fieldKey string, log *slog.Logger) *aesenc.Cipher {
	if fieldKey == "" {
		log.Warn("encryption.field_key not set — using dev fallback key, NOT safe for production")
		c, _ := aesenc.New(make([]byte, 32))
		return c
	}
	c, err := aesenc.NewFromHex(fieldKey)
	if err != nil {
		panic("encryption.field_key invalid: " + err.Error())
	}
	return c
}

// =====================================================================
// ACME / Let's Encrypt wiring (custom-domain status pages — M11 / K8).
//
// The Manager (apps/api/internal/acme) was implemented but never wired
// into the API server, causing custom-domain status pages to never
// receive TLS certificates.  These methods plug it in.
//
// HTTP-01 vs TLS-ALPN-01:
//   We use HTTP-01.  Easier to debug, no port-443 contention with the
//   main TLS terminator (Cloudflare / Caddy / nginx), and works through
//   any reverse proxy that forwards :80 to the API process.
//
// Port strategy:
//   The main API server keeps its existing port (8080 by default).
//   When ACME is enabled, StartACMEHTTPListener spins up an additional
//   listener on :80 (configurable) whose handler is
//   acme.Manager.HTTPHandler(fallback).  autocert short-circuits the
//   /.well-known/acme-challenge/{token} path itself; everything else
//   falls through to a 308 redirect to https:// (or the supplied
//   fallback handler).
//
//   For testability we ALSO mount the challenge path on the main chi
//   router via MountACME so smoke tests can hit it without a port-80
//   listener.  In production traffic only ever hits the :80 listener
//   for ACME validation, so the chi mount is harmless.
//
// TLS bootstrapping:
//   The API server itself does not terminate TLS today (Cloudflare in
//   front).  The Manager.GetCertificate hook is exposed via
//   ACMETLSConfig() for future use when we run a direct TLS listener
//   for custom-domain status pages.  Until then only HTTP-01 is wired.
// =====================================================================

// MountACME registers the HTTP-01 challenge route on the main chi router.
// Safe to call after New() and before Start().  No-op if mgr is nil.
//
// The path is fixed by RFC 8555 §8.3: /.well-known/acme-challenge/{token}
// The handler returned by mgr.HTTPHandler(nil) responds to that exact
// prefix; the chi route forwards the request to it unchanged.
func (s *Server) MountACME(mgr *acmemgr.Manager) {
	if mgr == nil || s.router == nil {
		return
	}
	h := mgr.HTTPHandler(nil)
	s.router.Get("/.well-known/acme-challenge/{token}", h.ServeHTTP)
	s.logger.Info("acme: HTTP-01 challenge route mounted on main router")
}

// StartACMEHTTPListener spins up an HTTP-only listener on the given addr
// (typically ":80") whose handler is mgr.HTTPHandler(fallback).
// autocert serves /.well-known/acme-challenge/* itself; everything else
// is delegated to fallback.  When fallback is nil a permanent redirect
// to the https:// equivalent of the request URL is returned.
//
// The listener runs in a goroutine and logs errors via s.logger.  It is
// not part of the main httpServer lifecycle — caller may rely on
// process termination to tear it down; this is acceptable for the
// long-lived API process.
//
// No-op if mgr is nil or addr is empty.
func (s *Server) StartACMEHTTPListener(mgr *acmemgr.Manager, addr string) {
	if mgr == nil || addr == "" {
		return
	}
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	})
	srv := &http.Server{
		Addr:              addr,
		Handler:           mgr.HTTPHandler(fallback),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		s.logger.Info("acme: starting HTTP-01 listener", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("acme: HTTP-01 listener error", "addr", addr, "error", err)
		}
	}()
}

// ACMETLSConfig returns a *tls.Config whose GetCertificate is wired to
// the autocert manager.  Use this when you stand up a direct TLS
// listener for custom-domain status pages.  Returns nil if mgr is nil.
//
// Not currently used in main.go — included for symmetry and future use.
func ACMETLSConfig(mgr *acmemgr.Manager) *tls.Config {
	if mgr == nil {
		return nil
	}
	return &tls.Config{
		GetCertificate: mgr.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1", "acme-tls/1"},
		MinVersion:     tls.VersionTLS12,
	}
}