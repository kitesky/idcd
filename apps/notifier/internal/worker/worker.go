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
	"time"

	"github.com/hibiken/asynq"

	"github.com/kite365/idcd/apps/notifier/internal/config"
)

// Worker manages the asynq server and task processing.
type Worker struct {
	server   *asynq.Server
	handlers *Handlers
	logger   *slog.Logger
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

// Start starts the worker and begins processing tasks.
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("启动邮件通知 Worker")

	// Register handlers
	mux := w.handlers.GetMux()

	// Start server
	if err := w.server.Start(mux); err != nil {
		return fmt.Errorf("启动 asynq server 失败: %w", err)
	}

	w.logger.Info("邮件通知 Worker 已启动")
	return nil
}

// Stop gracefully stops the worker.
func (w *Worker) Stop(ctx context.Context) error {
	w.logger.Info("停止邮件通知 Worker")

	// Create a timeout context for graceful shutdown
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