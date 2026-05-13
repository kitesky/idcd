// Package logger provides a structured logger (log/slog) with context-aware
// request_id injection. All services should use this instead of log.Printf.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type contextKey string

const (
	keyRequestID contextKey = "request_id"
	keyUserID    contextKey = "user_id"
	keyTraceID   contextKey = "trace_id"
)

// New returns a *slog.Logger.
//   - env == "development": text handler, DEBUG level, colorless
//   - anything else: JSON handler, INFO level (production)
func New(env string) *slog.Logger {
	return NewWithWriter(env, os.Stdout)
}

// NewWithWriter allows injecting a custom writer (useful in tests).
func NewWithWriter(env string, w io.Writer) *slog.Logger {
	var handler slog.Handler
	if env == "development" {
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: false,
		})
	} else {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false,
		})
	}
	return slog.New(handler)
}

// WithRequestID stores a request_id in ctx for later extraction.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, keyRequestID, requestID)
}

// WithUserID stores a user_id in ctx.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

// WithTraceID stores a trace_id in ctx.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, keyTraceID, traceID)
}

// FromContext returns base enriched with any IDs found in ctx.
// Call this at the start of each request handler to get a pre-tagged logger.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	l := base
	if rid, ok := ctx.Value(keyRequestID).(string); ok && rid != "" {
		l = l.With("request_id", rid)
	}
	if uid, ok := ctx.Value(keyUserID).(string); ok && uid != "" {
		l = l.With("user_id", uid)
	}
	if tid, ok := ctx.Value(keyTraceID).(string); ok && tid != "" {
		l = l.With("trace_id", tid)
	}
	return l
}

// Discard returns a no-op logger — useful in tests.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}
