// attest-refund-worker is the D5 refund-pipeline consumer.
//
// It drains the two Redis Streams populated in S2:
//
//   - refund_initiate_queue: Self-Verify caught a bad PDF and asks us
//     to refund the user (apps/attest/cmd/verifier/refund_enqueue.go).
//   - refund_retry_queue: a Paddle webhook indicated a refund event but
//     the inline UpdateStatus / lookup failed
//     (apps/attest/internal/handler/paddle/paddle.go).
//
// Both streams funnel into a shared delay-zone Redis ZSET keyed on
// (order_id, attempt). A 30 s tick goroutine scans the ZSET and runs
// the Paddle refund call for each due entry — up to two retries per
// order, then a flip to refund_failed plus an apology email
// (DECISIONS.md §M D5).
//
// D6: this is a stand-alone process. It owns its own Redis client and
// its own Postgres pool, and it deliberately does NOT import
// lib/attest/sign — nothing here touches the chain of custody for the
// original verdict.
//
// Wiring summary:
//
//   - cfg.RedisAddr → main Redis (streams + delay zone).
//   - cfg.RefundNotifierAddr → notifier asynq broker (apology emails).
//     Empty disables the mailer; the worker still flips orders to
//     refund_failed but logs a P0 warning that no email was sent.
//   - cfg.DatabaseDSN → idcd_attest pgxpool.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/attest/internal/config"
	"github.com/kite365/idcd/apps/attest/internal/refund"
	"github.com/kite365/idcd/apps/attest/internal/repo"
	"github.com/kite365/idcd/apps/attest/internal/streamconsumer"
)

// apologyTaskType is the notifier asynq task type the refund worker
// emits for terminal refund_failed apology emails. The notifier-side
// consumer (handler for this task type) is tracked separately — until
// it lands the task simply waits in the queue, which is the correct
// fail-open posture for early S2 deploys.
const apologyTaskType = "payment:refund_apology"

// shutdownTimeout caps how long graceful shutdown may take. Beyond
// this we abandon any in-flight Paddle call (which is fine: Paddle is
// idempotent on idem keys, and the retry ladder picks up where we left
// off after restart).
const shutdownTimeout = 10 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attest-refund-worker: load config: %v\n", err)
		os.Exit(1)
	}
	if strings.TrimSpace(cfg.DatabaseDSN) == "" {
		fmt.Fprintln(os.Stderr, "attest-refund-worker: ATTEST_DB_DSN is required")
		os.Exit(1)
	}
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		fmt.Fprintln(os.Stderr, "attest-refund-worker: ATTEST_REDIS_ADDR is required")
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)
	log.Info("attest-refund-worker starting",
		"env", cfg.Env,
		"initiate_stream", cfg.RefundInitiateStream,
		"retry_stream", cfg.RefundRetryStream,
		"delay_zone", cfg.RefundDelayZoneKey,
	)

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("attest-refund-worker: pgxpool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	repos := repo.New(pool)

	// D6: brand-new Redis client, NOT shared with verifier or generator.
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	defer func() { _ = rdb.Close() }()

	// Optional asynq client for apology emails. Empty addr → mailer
	// disabled (worker still functions; flips to refund_failed but
	// skips the user email).
	var mailer refund.ApologyMailer
	var asynqClient *asynq.Client
	if addr := strings.TrimSpace(cfg.RefundNotifierAddr); addr != "" {
		asynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: addr})
		mailer = &asynqApologyMailer{
			client: asynqClient,
			queue:  cfg.RefundNotifierQueue,
			now:    time.Now,
		}
		log.Info("attest-refund-worker: apology mailer wired",
			"queue", cfg.RefundNotifierQueue, "task", apologyTaskType)
	} else {
		log.Warn("attest-refund-worker: ATTEST_REFUND_NOTIFIER_REDIS_ADDR unset; apology email disabled")
	}
	if asynqClient != nil {
		defer func() { _ = asynqClient.Close() }()
	}

	refunder, err := newPaddleRefunder()
	if err != nil {
		log.Error("attest-refund-worker: paddle refunder init failed", "err", err)
		os.Exit(1)
	}

	orderStore := &repoOrderStore{
		orders:  repos.Orders,
		reports: repos.Reports,
		pool:    pool,
		now:     time.Now,
	}

	handler := refund.New(refund.Config{
		Orders:       orderStore,
		Refunder:     refunder,
		Mailer:       mailer,
		Redis:        rdb,
		DelayZoneKey: cfg.RefundDelayZoneKey,
		Logger:       log,
		Now:          time.Now,
	})

	initiateConsumer := streamconsumer.New(streamconsumer.Config{
		Redis:    rdb,
		Stream:   cfg.RefundInitiateStream,
		Group:    cfg.RefundGroup,
		Consumer: cfg.RefundConsumer,
		Handler:  handler.HandleInitiate,
		Logger:   log.With("stream", cfg.RefundInitiateStream),
	})
	retryConsumer := streamconsumer.New(streamconsumer.Config{
		Redis:    rdb,
		Stream:   cfg.RefundRetryStream,
		Group:    cfg.RefundGroup,
		Consumer: cfg.RefundConsumer,
		Handler:  handler.HandleRetryEnqueue,
		Logger:   log.With("stream", cfg.RefundRetryStream),
	})

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		if err := initiateConsumer.Run(ctx); err != nil {
			log.Error("initiate consumer exited with error", "err", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := retryConsumer.Run(ctx); err != nil {
			log.Error("retry consumer exited with error", "err", err)
		}
	}()
	go func() {
		defer wg.Done()
		runTickLoop(ctx, handler, refund.DefaultTickInterval, log)
	}()

	wg.Wait()

	// Shutdown grace — give in-flight handlers a moment to ACK before
	// process exit.
	shutCtx, shutCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutCancel()
	<-shutCtx.Done()
	log.Info("attest-refund-worker exited cleanly")
}

// runTickLoop fires Handler.TickDelayZone every interval until ctx is
// Done. Exits cleanly on cancellation; errors are logged but do not
// crash the worker (the delay zone is persisted in Redis, so the next
// process picks up where we left off).
func runTickLoop(ctx context.Context, h *refund.Handler, interval time.Duration, log *slog.Logger) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("tick loop stopping")
			return
		case <-t.C:
			n, err := h.TickDelayZone(ctx)
			if err != nil {
				log.Warn("tick: TickDelayZone failed", "err", err)
				continue
			}
			if n > 0 {
				log.Debug("tick: processed members", "count", n)
			}
		}
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

// ---------------------------------------------------------------------------
// repoOrderStore: refund.OrderStore over the two repos. The report → order
// hop is an application-level join (D1: no cross-schema FK; we just chain
// two SELECTs and emit a single Order projection).
// ---------------------------------------------------------------------------

type repoOrderStore struct {
	orders  *repo.VerdictOrdersRepo
	reports *repo.VerdictReportsRepo
	// pool is held directly (rather than going through a repo) so the
	// refund worker can run the one cross-schema read it needs — the
	// owner_id → idcd_main."user".email lookup driving the apology
	// email. D1: no FK, application-level join only.
	pool repo.Pool
	now  func() time.Time
}

// userEmailLookupSQL fetches the recipient email for the apology
// email. Schema-qualified so the query is correct regardless of which
// schema the connection's search_path defaults to. citext column ↔
// Go string maps via pgx's default codec.
//
// D1: NOT a foreign key. This is an application-level join executed at
// refund-failure time only (≤ projected ~50 refunds/day), so the cost
// is negligible and the cross-schema dependency stays read-only.
const userEmailLookupSQL = `SELECT email::text FROM idcd_main."user" WHERE id = $1`

func (s *repoOrderStore) GetByReportID(ctx context.Context, reportID string) (*refund.Order, error) {
	rep, err := s.reports.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, refund.ErrOrderNotFound
		}
		return nil, fmt.Errorf("report lookup: %w", err)
	}
	return s.loadOrder(ctx, rep.OrderID)
}

func (s *repoOrderStore) GetByID(ctx context.Context, orderID string) (*refund.Order, error) {
	return s.loadOrder(ctx, orderID)
}

func (s *repoOrderStore) loadOrder(ctx context.Context, id string) (*refund.Order, error) {
	o, err := s.orders.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, refund.ErrOrderNotFound
		}
		return nil, fmt.Errorf("order lookup: %w", err)
	}
	paddle := ""
	if o.PaddleOrderID != nil {
		paddle = *o.PaddleOrderID
	}
	lastErr := ""
	if o.RefundLastError != nil {
		lastErr = *o.RefundLastError
	}
	// D1 application-level join: chase owner_id into idcd_main."user"
	// to pull the email. We swallow "not found" so the refund flow
	// still flips the order to refund_failed even if the user row was
	// hard-deleted; D5 fail-open posture (no email beats no refund).
	email, lookupErr := s.lookupUserEmail(ctx, o.OwnerID)
	if lookupErr != nil {
		// Transient DB errors: surface so the caller can decide whether
		// to retry (initiate/tick wraps this into a non-fatal log).
		return nil, fmt.Errorf("user email lookup: %w", lookupErr)
	}
	return &refund.Order{
		ID:             o.ID,
		Status:         o.Status,
		OwnerID:        o.OwnerID,
		UserEmail:      email,
		PaddleOrderID:  paddle,
		PriceCNYYuan:   o.PriceCNY,
		Currency:       "CNY",
		RefundAttempts: o.RefundAttemptCount,
		LastError:      lastErr,
		RefundedAt:     o.RefundedAt,
		ApologySentAt:  o.RefundApologySentAt,
	}, nil
}

// lookupUserEmail runs the cross-schema "owner_id → user.email" join.
// Returns ("", nil) when the user row is gone (D5 fail-open: the
// refund pipeline must still close out the order even without a
// recipient). Any other DB error bubbles up so the caller can retry.
func (s *repoOrderStore) lookupUserEmail(ctx context.Context, ownerID string) (string, error) {
	if strings.TrimSpace(ownerID) == "" {
		return "", nil
	}
	if s.pool == nil {
		// Test wiring may omit the pool (no email needed); treat as
		// "no recipient" rather than a hard error.
		return "", nil
	}
	var email string
	if err := s.pool.QueryRow(ctx, userEmailLookupSQL, ownerID).Scan(&email); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return email, nil
}

func (s *repoOrderStore) MarkRefunded(ctx context.Context, orderID, fromStatus string, at time.Time) error {
	if err := s.orders.UpdateStatus(ctx, orderID, fromStatus, repo.OrderStatusRefunded, nil); err != nil {
		return fmt.Errorf("update status -> refunded: %w", err)
	}
	if err := s.orders.StampRefundedAt(ctx, orderID, at); err != nil {
		return fmt.Errorf("stamp refunded_at: %w", err)
	}
	return nil
}

func (s *repoOrderStore) MarkRefundFailed(ctx context.Context, orderID, fromStatus, errReason string) error {
	r := errReason
	return s.orders.UpdateStatus(ctx, orderID, fromStatus, repo.OrderStatusRefundFailed, &r)
}

func (s *repoOrderStore) BumpRefundAttempt(ctx context.Context, orderID, errReason string, _ int) error {
	// repo.IncrementRefundAttempt does its own +1; the caller-supplied
	// newAttempt is only an informational signal for tests.
	return s.orders.IncrementRefundAttempt(ctx, orderID, errReason)
}

func (s *repoOrderStore) MarkApologySent(ctx context.Context, orderID string, at time.Time) error {
	return s.orders.SetRefundApologySent(ctx, orderID, at)
}

// ---------------------------------------------------------------------------
// asynqApologyMailer: pushes a single payment:refund_apology task onto the
// notifier billing queue per failed refund. The task payload mirrors the
// notifier-side RefundRetryPayload's relevant fields so a future consumer
// can render an apology without re-reading the DB.
// ---------------------------------------------------------------------------

type asynqApologyMailer struct {
	client *asynq.Client
	queue  string
	now    func() time.Time
}

// apologyPayload is the wire format of payment:refund_apology asynq
// tasks. The notifier-side RefundApologyPayload mirrors this struct;
// JSON field names form the contract — DO NOT rename without
// coordinating with apps/notifier/internal/worker/handlers.go.
//
// The producer (refund worker) embeds enough state for the notifier
// to render and send the apology email without any further DB read.
// This avoids a notifier-side cross-schema lookup (which would have
// re-implemented the same D1 join we already do here) and keeps the
// notifier deployable without the idcd_attest DSN.
type apologyPayload struct {
	OrderID           string `json:"order_id"`
	UserEmail         string `json:"user_email"`
	PaddleOrderID     string `json:"paddle_order_id"`
	RefundAmountCents int64  `json:"refund_amount_cents"`
	Currency          string `json:"currency"`
	FailureReason     string `json:"failure_reason"`
	EnqueuedAt        string `json:"enqueued_at"`
}

func (m *asynqApologyMailer) SendApology(ctx context.Context, order *refund.Order, reason string) error {
	if order == nil {
		return fmt.Errorf("apology mailer: nil order")
	}
	currency := order.Currency
	if strings.TrimSpace(currency) == "" {
		currency = "CNY"
	}
	body, err := jsonMarshal(apologyPayload{
		OrderID:           order.ID,
		UserEmail:         order.UserEmail,
		PaddleOrderID:     order.PaddleOrderID,
		RefundAmountCents: order.PriceCents(),
		Currency:          currency,
		FailureReason:     reason,
		EnqueuedAt:        m.now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("marshal apology payload: %w", err)
	}
	queue := m.queue
	if strings.TrimSpace(queue) == "" {
		queue = "billing"
	}
	_, err = m.client.EnqueueContext(ctx,
		asynq.NewTask(apologyTaskType, body),
		asynq.Queue(queue),
		// Notifier-side handler is responsible for at-most-once delivery
		// keyed on order_id; the worker does not need any additional
		// retention beyond asynq's defaults.
	)
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", apologyTaskType, err)
	}
	return nil
}

// jsonMarshal is a tiny wrapper to keep encoding/json out of the import
// block in the imports-driven SDK adapter below — and to give tests an
// override seam if we ever need one.
var jsonMarshal = defaultJSONMarshal
