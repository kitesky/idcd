package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/notifier/internal/billing"
	"github.com/kite365/idcd/apps/notifier/internal/config"
	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/apps/notifier/internal/worker"
	"github.com/kite365/idcd/lib/db"
	sharedconfig "github.com/kite365/idcd/lib/shared/config"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// defaultMetricsPort is the listen port for the notifier Prometheus
// scrape endpoint when observability.prometheus_port is unset in config.
// Picked to sit alongside the other service metric ports (aggregator
// 9091, scheduler 9093, gateway 9094) — see docs/REVIEW-FINDINGS-2026-05-16.md.
const defaultMetricsPort = 9092

func main() {
	// Load configuration. Honour IDCD_CONFIG env var so prod containers can
	// point at /config/prod.env.yaml without rebuilding the image.
	cfg := config.MustLoad(sharedconfig.DefaultPath())

	// Initialize logger
	log := logger.New(cfg.Server.Env)

	// Initialize OpenTelemetry
	telCfg := telemetry.Config{
		ServiceName:    "idcd-notifier",
		ServiceVersion: "v1.0.0",
		OTLPEndpoint:   cfg.Observability.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Observability.Telemetry.SamplingRate,
		Enabled:        cfg.Observability.Telemetry.Enabled,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		log.Error("failed to init telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	// Initialize templates
	templates, err := template.New()
	if err != nil {
		log.Error("初始化邮件模板失败", "error", err)
		os.Exit(1)
	}

	// Initialize email sender (prefer SMTP, fallback to SES if configured)
	var emailSender email.Sender
	if cfg.Notifier.SMTP.Host != "" {
		emailSender = email.NewSMTPSender(email.SMTPConfig{
			Host:     cfg.Notifier.SMTP.Host,
			Port:     cfg.Notifier.SMTP.Port,
			Username: cfg.Notifier.SMTP.Username,
			Password: cfg.Notifier.SMTP.Password,
			From:     cfg.Notifier.SMTP.From,
			FromName: cfg.Notifier.SMTP.FromName,
		})
		log.Info("使用 SMTP 发送邮件", "host", cfg.Notifier.SMTP.Host, "port", cfg.Notifier.SMTP.Port)
	} else if cfg.Notifier.SES.Region != "" {
		emailSender = email.NewSESSender(email.SESConfig{
			Region:    cfg.Notifier.SES.Region,
			AccessKey: cfg.Notifier.SES.AccessKey,
			SecretKey: cfg.Notifier.SES.SecretKey,
			From:      cfg.Notifier.SES.From,
			FromName:  cfg.Notifier.SES.FromName,
		})
		log.Info("使用 AWS SES 发送邮件", "region", cfg.Notifier.SES.Region)
	} else {
		log.Error("未配置邮件发送方式，请设置 SMTP 或 SES 配置")
		os.Exit(1)
	}

	// Initialize handlers (refund deps wired below if all prerequisites
	// are available — see wireRefundDeps).
	handlers := worker.NewHandlers(emailSender, templates, log)

	// Initialize worker (asynq.Server reads cfg.Notifier.Queues, which now
	// includes "billing" so payment:refund_retry tasks are picked up).
	w, err := worker.NewWorker(&cfg.Notifier, handlers, log)
	if err != nil {
		log.Error("初始化邮件 Worker 失败", "error", err)
		os.Exit(1)
	}

	// Wire D5 refund retry dependencies. We do this AFTER handlers/worker
	// are constructed so a missing dep doesn't block email delivery — if
	// the deps are wired the refund retry handler is fully functional; if
	// not, refund_retry tasks fail loudly (see HandleRefundRetry) and asynq
	// retries them, giving us a recoverable wiring bug rather than a silent
	// drop.
	asynqClient, dbPool := wireRefundDeps(cfg, handlers, log)
	if asynqClient != nil {
		defer asynqClient.Close()
	}
	if dbPool != nil {
		defer dbPool.Close()
	}

	// S2 W8: spin up the cert:notifications Redis Stream consumer alongside
	// the asynq email worker.  We deliberately use a SEPARATE go-redis client
	// (not asynq's internal pool) — the Stream API is stateful per-connection
	// (BLOCK / pending entry list) and we don't want asynq's queue
	// throughput / consumer offsets entangled with cert events.
	//
	// Wiring is best-effort: if the operator disabled the consumer via
	// cert_stream_enabled=false, OR if the DB pool isn't available (cert
	// events need an account lookup to resolve the recipient email), we log a
	// clear warn and skip — the email worker still runs.
	if cfg.Notifier.CertStreamEnabledOrDefault() {
		certRedis := newCertStreamRedis(cfg)
		defer certRedis.Close()
		certConsumer, wireErr := wireCertConsumer(cfg, certRedis, emailSender, templates, dbPool, log)
		if wireErr != nil {
			log.Warn("S2 W8 cert consumer 未启动",
				"reason", wireErr,
				"stream", cfg.Notifier.CertStreamName,
			)
		} else {
			w.WithCertConsumer(certConsumer)
			log.Info("S2 W8 cert consumer 已附加",
				"stream", cfg.Notifier.CertStreamName,
				"group", cfg.Notifier.CertConsumerGroup,
			)
		}
	} else {
		log.Info("S2 W8 cert consumer 已通过配置关闭", "cert_stream_enabled", false)
	}

	// Set up graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start worker
	go func() {
		if err := w.Start(ctx); err != nil {
			log.Error("启动邮件 Worker 失败", "error", err)
			cancel()
		}
	}()

	// Start Prometheus /metrics listener. Runs on its own port so the worker
	// process can expose runtime + notifier-specific metrics without standing
	// up a full HTTP server. Failures are logged but do not block startup —
	// the email worker keeps running even if Prometheus scraping is broken.
	startMetricsServer(metricsPort(cfg), log)

	log.Info("邮件通知服务已启动，等待任务...")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info("收到停止信号，正在优雅关闭...")

	// Stop worker gracefully
	if err := w.Stop(context.Background()); err != nil {
		log.Error("停止邮件 Worker 失败", "error", err)
		os.Exit(1)
	}

	log.Info("邮件通知服务已停止")
}

// wireRefundDeps initialises the three D5 refund retry dependencies and
// attaches them to handlers.  Returns the asynq client and pgx pool so main
// can defer their Close.
//
// Failure modes are non-fatal: each missing dep is logged at warn level and
// handlers.WithRefundDeps is invoked only when all three are present.  The
// refund retry handler surfaces a clear "deps not wired" error when a
// payment:refund_retry task arrives without full wiring, leaving asynq's
// own retry mechanism to recover after operators fix the config.  This
// keeps email delivery available even when (e.g.) the DB is briefly
// unreachable at boot.
func wireRefundDeps(cfg *config.Config, handlers *worker.Handlers, log *slog.Logger) (*asynq.Client, *pgxpool.Pool) {
	// --- pgxpool ---
	// Notifier only writes payment status flips here, so a small pool is enough.
	var pool *pgxpool.Pool
	var paymentStore *billing.PgPaymentStore
	if dsn := cfg.Database.Main.DSN; dsn != "" {
		dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		p, err := db.NewPool(dbCtx, db.Config{
			DSN:             dsn,
			MaxOpenConns:    4,
			MaxIdleConns:    1,
			ConnMaxLifetime: 5 * time.Minute,
		})
		cancel()
		if err != nil {
			log.Warn("D5 refund retry: pgx pool init failed — PaymentStore NOT wired",
				"error", err,
			)
		} else {
			pool = p
			paymentStore = billing.NewPgPaymentStore(p)
			log.Info("D5 refund retry: PaymentStore wired", "max_conns", 4)
		}
	} else {
		log.Warn("D5 refund retry: database.main.dsn empty — PaymentStore NOT wired")
	}

	// --- asynq.Client for re-enqueue ---
	redisOpt := parseAsynqRedis(cfg.Notifier.AsynqDSN)
	asynqClient := asynq.NewClient(redisOpt)
	enqueuer := billing.NewAsynqRefundEnqueuer(asynqClient)
	log.Info("D5 refund retry: RefundRetryEnqueuer wired",
		"redis_addr", redisOpt.Addr,
		"queue", billing.BillingQueue,
	)

	// --- PaymentRefunder (payment SDK) ---
	var refunder *billing.PaymentHubRefunder
	pc := cfg.Payment
	if pc.Enabled && pc.APIKey != "" && pc.APISecret != "" {
		refunder = billing.NewPaymentHubRefunder(pc.BaseURL, pc.APIKey, pc.APISecret)
		log.Info("D5 refund retry: PaymentRefunder wired (PaymentHub)",
			"base_url", pc.BaseURL,
		)
	} else {
		log.Warn("D5 refund retry: payment.enabled=false or credentials missing — PaymentRefunder NOT wired")
	}

	// Attach only when all three deps succeeded.  HandleRefundRetry's nil
	// check surfaces a partial wire as a retried internal error rather than
	// a silent corrupt state.
	if refunder != nil && paymentStore != nil {
		handlers.WithRefundDeps(refunder, paymentStore, enqueuer)
		log.Info("D5 refund retry: handler fully wired")
	} else {
		log.Warn("D5 refund retry: handler NOT wired — refund_retry tasks will fail until config is fixed",
			"refunder_ok", refunder != nil,
			"store_ok", paymentStore != nil,
			"enqueuer_ok", true,
		)
	}

	return asynqClient, pool
}

// metricsPort returns the Prometheus listener port, honouring the shared
// observability.prometheus_port config knob and falling back to the notifier-
// specific defaultMetricsPort. Negative / unset values use the default so a
// blank config file still produces a working scrape endpoint.
func metricsPort(cfg *config.Config) int {
	if cfg != nil && cfg.Observability.PrometheusPort > 0 {
		return cfg.Observability.PrometheusPort
	}
	return defaultMetricsPort
}

// startMetricsServer spins up the dedicated /metrics HTTP listener in a
// background goroutine. The listener exposes the default Prometheus registry,
// which is where promauto-registered metrics (see internal/worker/metrics.go)
// land. ErrServerClosed during shutdown is logged at debug level — anything
// else is logged at error level but never panics the worker.
func startMetricsServer(port int, log *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	addr := ":" + strconv.Itoa(port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("Prometheus /metrics 监听已启动", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics 监听失败", "addr", addr, "error", err)
		}
	}()
}

// newCertStreamRedis builds a dedicated *redis.Client for the S2 W8 cert
// consumer.  We deliberately use a NEW client (rather than reusing the
// shared lib/shared cache helper) so the Stream API's stateful semantics
// (BLOCK + pending entry list per-connection) don't entangle with other
// notifier Redis traffic.  The DSN reuses Notifier.AsynqDSN because the
// producer (apps/cert-svc) writes to the same Redis cluster.
func newCertStreamRedis(cfg *config.Config) *redis.Client {
	opt := parseAsynqRedis(cfg.Notifier.AsynqDSN)
	return redis.NewClient(&redis.Options{
		Addr:         opt.Addr,
		Password:     opt.Password,
		DB:           opt.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  0, // BLOCK 0 = read until message; the consumer caps via XReadGroupArgs.Block.
		WriteTimeout: 5 * time.Second,
	})
}

// wireCertConsumer constructs a worker.CertConsumer with all dependencies.
// Returns an error (non-fatal — caller logs + skips) when a hard prerequisite
// is missing (notably the DB pool).
func wireCertConsumer(
	cfg *config.Config,
	rdb *redis.Client,
	sender email.Sender,
	templates *template.Templates,
	pool *pgxpool.Pool,
	log *slog.Logger,
) (*worker.CertConsumer, error) {
	if pool == nil {
		return nil, errors.New("pgx pool not wired; cannot resolve account → email")
	}
	lookup := accountEmailLookup(pool)
	return worker.NewCertConsumer(
		rdb,
		sender,
		templates,
		lookup,
		log,
		cfg.Notifier.CertStreamName,
		cfg.Notifier.CertConsumerGroup,
	)
}

// accountEmailLookup returns an EmailLookup that queries the auth schema for
// (email, locale) by account id.
//
// D1 — cross-schema FK forbidden.  We're reading from auth.accounts using
// the same pgxpool that the notifier already opens for D5 refund retries;
// the *query* is intra-schema (auth.* only), so no Repository join layer is
// needed here.  If the row is missing or has no email, we return a soft
// miss ("","",nil) so the consumer ACKs and moves on.
//
// `locale_pref` is best-effort: when the column / value is null we leave
// locale empty so the consumer's registry fallback applies.
func accountEmailLookup(pool *pgxpool.Pool) worker.EmailLookup {
	const sqlAccount = `SELECT email, COALESCE(locale_pref, '') FROM auth.accounts WHERE id = $1`
	return func(ctx context.Context, accountID int64) (string, string, error) {
		queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var email, locale string
		err := pool.QueryRow(queryCtx, sqlAccount, accountID).Scan(&email, &locale)
		if err != nil {
			// pgx surfaces "no rows" as a typed error; we can't import its
			// sentinel without bloating this file, so a string check keeps
			// us decoupled.  Missing row = soft miss.
			if strings.Contains(err.Error(), "no rows") {
				return "", "", nil
			}
			return "", "", fmt.Errorf("query auth.accounts id=%d: %w", accountID, err)
		}
		return email, locale, nil
	}
}

// parseAsynqRedis mirrors the helper in apps/notifier/internal/worker/worker.go
// — supports both "host:port" and "redis://[:password@]host:port[/db]".
// Duplicated here (instead of exported from worker) to keep wiring concerns
// out of the handler-facing package's API surface.
func parseAsynqRedis(dsn string) asynq.RedisClientOpt {
	opt := asynq.RedisClientOpt{Addr: dsn}
	if strings.HasPrefix(dsn, "redis://") || strings.HasPrefix(dsn, "rediss://") {
		if u, err := url.Parse(dsn); err == nil {
			opt.Addr = u.Host
			if u.User != nil {
				opt.Password, _ = u.User.Password()
			}
			if dbStr := strings.TrimPrefix(u.Path, "/"); dbStr != "" {
				if dbN, err := strconv.Atoi(dbStr); err == nil {
					opt.DB = dbN
				}
			}
		}
	}
	return opt
}
