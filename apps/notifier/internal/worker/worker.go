// Package worker provides asynq-based email task processing.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kite365/idcd/apps/notifier/internal/config"
)

// Worker manages the asynq server and task processing.
//
// S2 W8 added an optional Redis Stream consumer (CertConsumer) that runs
// alongside the asynq server in its own goroutine.  Stop() cancels its
// dedicated context and waits for the goroutine to return before reporting
// "fully stopped" upstream.
type Worker struct {
	server   *asynq.Server
	handlers *Handlers
	logger   *slog.Logger

	// Optional cert:notifications consumer.  Nil when CertStreamEnabled was
	// false at construction time or when the wiring layer chose not to attach
	// one (e.g. a DB-less deployment).
	certConsumer *CertConsumer

	// Cancel + wg are populated when Start spawns the cert consumer goroutine
	// so Stop can deterministically wait.
	certCancel context.CancelFunc
	certWG     sync.WaitGroup
}

// retryDelayFunc is the asynq.Config.RetryDelayFunc implementation used by
// NewWorker.  It is extracted to a package-level function (rather than an
// inline closure) so unit tests can pin the exact behaviour without spinning
// up a real asynq.Server.  See the long comment in NewWorker for the
// behavioural contract: refund tasks land on the D5 fixed 5-minute cadence;
// every other task type uses jittered exponential backoff capped at 60s.
func retryDelayFunc(n int, _ error, task *asynq.Task) time.Duration {
	if task != nil && task.Type() == TaskRefundRetry {
		// D5 alignment: even when asynq drives the retry (because the
		// handler returned a transient error before it could enqueue the
		// next attempt explicitly), respect the 5-minute primary cadence.
		// No jitter: D5 calls out specific timings the on-call uses to
		// reason about user-visible refund SLAs.
		return RefundRetryFirstDelay
	}
	// Exponential backoff: 1s, 4s, 16s with ±25% jitter, capped at 60s.
	base := time.Duration(1<<uint(n)) * time.Second
	if base > 60*time.Second {
		base = 60 * time.Second
	}
	jitterRange := time.Duration(float64(base) * 0.25)
	return base + time.Duration(float64(jitterRange)*(2.0*rand.Float64()-1.0)) //nolint:gosec — non-cryptographic jitter is intentional
}

// NewWorker creates a new Worker instance.
func NewWorker(cfg *config.NotifierConfig, handlers *Handlers, logger *slog.Logger) (*Worker, error) {
	// Parse Redis DSN (supports both "host:port" and "redis://[:password@]host:port[/db]")
	redisOpt := asynq.RedisClientOpt{Addr: cfg.AsynqDSN}
	if strings.HasPrefix(cfg.AsynqDSN, "redis://") || strings.HasPrefix(cfg.AsynqDSN, "rediss://") {
		if u, err := url.Parse(cfg.AsynqDSN); err == nil {
			redisOpt.Addr = u.Host
			if u.User != nil {
				redisOpt.Password, _ = u.User.Password()
			}
			if db, err := strconv.Atoi(strings.TrimPrefix(u.Path, "/")); err == nil {
				redisOpt.DB = db
			}
		}
	}

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Queues:      cfg.Queues,
			Concurrency: cfg.Workers,

			// Retry configuration.  Two regimes share this function:
			//
			//   1. Refund retry tasks (payment:refund_retry, D5) follow a
			//      *fixed* schedule (5min → 30min) implemented at the
			//      enqueuer side via asynq.ProcessIn.  The enqueuer always
			//      drives the next attempt by enqueuing a *new* task with
			//      AttemptCount incremented (see HandleRefundRetry).  Asynq's
			//      built-in retry path still kicks in if the handler returns
			//      a non-validation error before it can call the enqueuer
			//      (transient DB blip, etc.) — without an explicit override
			//      that bounce would happen at 1s/4s/16s, well shorter than
			//      D5's intent.  We therefore return RefundRetryFirstDelay
			//      here so even the asynq-driven retry respects the 5min
			//      cadence, keeping the refund pipeline aligned with D5
			//      regardless of which retry vector trips.
			//
			//   2. Every other task type (verify email, welcome email,
			//      password reset, alert notifications, …) keeps the
			//      exponential 1s/4s/16s/cap 60s backoff with ±25% jitter
			//      that has shipped since S1.  These are best-effort sends
			//      where fast retries help mask transient SMTP / webhook
			//      blips, and there is no D5-style fixed cadence to honour.
			RetryDelayFunc: retryDelayFunc,

			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error("任务处理最终失败",
					"task_type", task.Type(),
					"error", err,
				)
			}),

			// Logging
			Logger: &asynqLogger{logger: logger},
		},
	)

	return &Worker{
		server:   server,
		handlers: handlers,
		logger:   logger,
	}, nil
}

// WithCertConsumer attaches a cert:notifications Stream consumer.  Pass nil
// to disable (no-op).  Must be called before Start; calling after Start has
// no effect on the running loop (the existing consumer keeps going).
//
// The wiring contract: main.go constructs a CertConsumer once it has the
// Redis client / template renderer / EmailLookup ready, then hands it to the
// Worker so Stop() can fence the consumer's lifetime alongside asynq's own.
func (w *Worker) WithCertConsumer(c *CertConsumer) *Worker {
	w.certConsumer = c
	return w
}

// Start starts the worker and begins processing tasks.
//
// When a CertConsumer has been attached via WithCertConsumer, Start also
// spawns its Run loop as a managed goroutine.  The context passed to Start
// drives both: callers cancelling it also halts the consumer.  In addition,
// Stop() cancels the consumer's dedicated context so callers can use a
// fresh context for graceful shutdown (mirroring asynq.Server.Shutdown).
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("启动邮件通知 Worker")

	// Register handlers
	mux := w.handlers.GetMux()

	// Start server
	if err := w.server.Start(mux); err != nil {
		return fmt.Errorf("启动 asynq server 失败: %w", err)
	}

	// Spawn the cert:notifications Stream consumer (if wired). The consumer
	// owns its own context so Stop() can fence its lifetime independently of
	// the asynq server's shutdown sequence.
	if w.certConsumer != nil {
		consumerCtx, cancel := context.WithCancel(ctx)
		w.certCancel = cancel
		w.certWG.Add(1)
		go func() {
			defer w.certWG.Done()
			if err := w.certConsumer.Run(consumerCtx); err != nil {
				w.logger.Error("cert 通知消费者退出",
					"stream", w.certConsumer.Stream(), "err", err)
			}
		}()
		w.logger.Info("cert 通知消费者已启动",
			"stream", w.certConsumer.Stream(),
			"consumer", w.certConsumer.ConsumerName())
	}

	w.logger.Info("邮件通知 Worker 已启动")
	return nil
}

// Stop gracefully stops the worker.
//
// Order matters: we cancel the cert consumer first (so it stops pulling new
// stream entries), wait for its goroutine to drain, then shut down asynq.
// If asynq's shutdown exceeds 30s we still report the consumer as cleanly
// stopped because its drain completed earlier.
func (w *Worker) Stop(ctx context.Context) error {
	w.logger.Info("停止邮件通知 Worker")

	// Tell the cert consumer to stop and wait for the goroutine to finish.
	if w.certCancel != nil {
		w.certCancel()
	}
	// Wait with a generous bound — the consumer's BLOCK timeout is small so
	// in practice this returns in <500ms.  We still bound the wait with the
	// caller's context to avoid hanging shutdown on a wedged Redis call.
	waitDone := make(chan struct{})
	go func() {
		w.certWG.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
		if w.certConsumer != nil {
			w.logger.Info("cert 通知消费者已停止")
		}
	case <-ctx.Done():
		w.logger.Warn("cert 通知消费者停止超时，将强制关闭后续步骤")
	}

	// Create a timeout context for asynq's graceful shutdown.
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Shutdown server gracefully
	w.server.Shutdown()

	select {
	case <-shutdownCtx.Done():
		w.logger.Warn("Worker 停止超时，强制退出")
		return shutdownCtx.Err()
	default:
		w.logger.Info("邮件通知 Worker 已停止")
		return nil
	}
}

// asynqLogger adapter for bridging asynq logging to our logger.
type asynqLogger struct {
	logger *slog.Logger
}

func (l *asynqLogger) Debug(args ...interface{}) {
	l.logger.Debug(fmt.Sprint(args...))
}

func (l *asynqLogger) Info(args ...interface{}) {
	l.logger.Info(fmt.Sprint(args...))
}

func (l *asynqLogger) Warn(args ...interface{}) {
	l.logger.Warn(fmt.Sprint(args...))
}

func (l *asynqLogger) Error(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...))
}

func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...))
	// Note: We don't call os.Exit here as that would be too destructive
	// Let the application handle fatal errors appropriately
}