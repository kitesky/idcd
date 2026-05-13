// Package worker provides asynq-based email task processing.
package worker

import (
	"context"
	"fmt"
	"log/slog"
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

// NewWorker creates a new Worker instance.
func NewWorker(cfg *config.NotifierConfig, handlers *Handlers, logger *slog.Logger) (*Worker, error) {
	// Configure asynq server
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr: cfg.AsynqDSN,
		},
		asynq.Config{
			// Queue configuration with priorities
			Queues: cfg.Queues,

			// Worker concurrency
			Concurrency: cfg.Workers,

			// Retry configuration
			RetryDelayFunc: func(n int, err error, task *asynq.Task) time.Duration {
				// Exponential backoff: 1s, 4s, 16s with ±25% jitter
				base := time.Duration(1<<uint(n)) * time.Second
				if base > 60*time.Second {
					base = 60 * time.Second // cap at 1 minute
				}

				// Add jitter (±25%) to prevent thundering herd
				jitterRange := time.Duration(float64(base) * 0.25)
				return base + time.Duration(float64(jitterRange)*(2.0*0.5-1.0)) // simple jitter
			},

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