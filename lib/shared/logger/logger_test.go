package logger_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kite365/idcd/packages/shared/logger"
)

func TestNew_dev_text(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewWithWriter("development", &buf)
	l.Info("hello", "key", "value")
	got := buf.String()
	if !strings.Contains(got, "hello") {
		t.Errorf("expected 'hello' in output, got: %q", got)
	}
	_ = logger.New("development") // cover the os.Stdout path
}

func TestNew_prod_json(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewWithWriter("production", &buf)
	l.Info("hello", "key", "value")
	got := buf.String()
	// JSON output should contain quotes
	if !strings.Contains(got, `"msg"`) && !strings.Contains(got, `"message"`) {
		t.Errorf("expected JSON output, got: %q", got)
	}
}

func TestFromContext_injectsRequestID(t *testing.T) {
	var buf bytes.Buffer
	base := logger.NewWithWriter("development", &buf)

	ctx := logger.WithRequestID(context.Background(), "req-123")
	l := logger.FromContext(ctx, base)
	l.Info("test")

	got := buf.String()
	if !strings.Contains(got, "req-123") {
		t.Errorf("expected request_id in output, got: %q", got)
	}
}

func TestFromContext_injectsUserID(t *testing.T) {
	var buf bytes.Buffer
	base := logger.NewWithWriter("development", &buf)

	ctx := logger.WithUserID(context.Background(), "u_abc123")
	l := logger.FromContext(ctx, base)
	l.Info("test")

	got := buf.String()
	if !strings.Contains(got, "u_abc123") {
		t.Errorf("expected user_id in output, got: %q", got)
	}
}

func TestFromContext_emptyContext(t *testing.T) {
	var buf bytes.Buffer
	base := logger.NewWithWriter("development", &buf)
	l := logger.FromContext(context.Background(), base)
	l.Info("no ids")
	// should not panic, just log normally
}

func TestFromContext_injectsTraceID(t *testing.T) {
	var buf bytes.Buffer
	base := logger.NewWithWriter("development", &buf)

	ctx := logger.WithTraceID(context.Background(), "trace-xyz")
	l := logger.FromContext(ctx, base)
	l.Info("test")

	got := buf.String()
	if !strings.Contains(got, "trace-xyz") {
		t.Errorf("expected trace_id in output, got: %q", got)
	}
}

func TestDiscard(t *testing.T) {
	l := logger.Discard()
	l.Info("this should not appear anywhere")
	l.Error("silent error")
	// no panic, no output
}
