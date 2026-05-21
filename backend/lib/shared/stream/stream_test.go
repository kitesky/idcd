package stream_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/kite365/idcd/lib/shared/contracts"
	"github.com/kite365/idcd/lib/shared/stream"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// newTestClient spins up a miniredis instance and returns a stream.Client.
// The miniredis server is automatically stopped when the test ends.
func newTestClient(t *testing.T) *stream.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return stream.New(rdb)
}

func TestNewFromConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	c, rdb := stream.NewFromConfig(mr.Addr(), "", 0)
	defer rdb.Close()
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("NewFromConfig: Ping failed: %v", err)
	}
}

func TestPing(t *testing.T) {
	c := newTestClient(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestAdd_returnsID(t *testing.T) {
	c := newTestClient(t)
	id, err := c.Add(context.Background(), "test.stream", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id == "" {
		t.Error("Add should return a non-empty message ID")
	}
	// Redis stream IDs are "<ms>-<seq>"
	if !strings.Contains(id, "-") {
		t.Errorf("unexpected ID format: %q", id)
	}
}

func TestLen_empty(t *testing.T) {
	c := newTestClient(t)
	n, err := c.Len(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestLen_afterAdd(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	for range 5 {
		if _, err := c.Add(ctx, "test.len", map[string]any{"x": "y"}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := c.Len(ctx, "test.len")
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("expected 5, got %d", n)
	}
}

func TestAddProbeResult(t *testing.T) {
	c := newTestClient(t)
	id, err := c.AddProbeResult(context.Background(), "pt_abc", "nd_jp_01", map[string]any{
		"duration_ms": 42,
		"success":     true,
	})
	if err != nil {
		t.Fatalf("AddProbeResult failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty ID")
	}
	// Verify stream has 1 entry
	n, _ := c.Len(context.Background(), stream.Probe)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Probe, n)
	}
}

func TestAddMonitorEvent(t *testing.T) {
	c := newTestClient(t)
	_, err := c.AddMonitorEvent(context.Background(), "m_abc", "down", map[string]any{
		"reason": "timeout",
	})
	if err != nil {
		t.Fatalf("AddMonitorEvent failed: %v", err)
	}
	n, _ := c.Len(context.Background(), stream.Monitor)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Monitor, n)
	}
}

func TestAddAlertEvent(t *testing.T) {
	c := newTestClient(t)
	_, err := c.AddAlertEvent(context.Background(), "ae_xyz", "m_abc", "down")
	if err != nil {
		t.Fatalf("AddAlertEvent failed: %v", err)
	}
	n, _ := c.Len(context.Background(), stream.Alert)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Alert, n)
	}
}

func TestAddAuditEvent(t *testing.T) {
	c := newTestClient(t)
	_, err := c.AddAuditEvent(context.Background(), map[string]any{
		"actor_user_id": "u_abc",
		"action":        "login",
		"result":        "ok",
	})
	if err != nil {
		t.Fatalf("AddAuditEvent failed: %v", err)
	}
	n, _ := c.Len(context.Background(), stream.Audit)
	if n != 1 {
		t.Errorf("expected 1 message in %s, got %d", stream.Audit, n)
	}
}

// newTestClientWithRDB is like newTestClient but also returns the underlying
// *redis.Client so tests can read back stream entries for assertion.
func newTestClientWithRDB(t *testing.T) (*stream.Client, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return stream.New(rdb), rdb
}

// newStoppedClient returns a client pointing at a stopped Redis server.
func newStoppedClient(t *testing.T) *stream.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := stream.New(rdb)
	mr.Close()
	return c
}

func TestErrorPaths(t *testing.T) {
	c := newStoppedClient(t)
	ctx := context.Background()
	t.Run("Add", func(t *testing.T) {
		_, err := c.Add(ctx, "test.stream", map[string]any{"k": "v"})
		if err == nil {
			t.Error("expected error when Redis is unavailable")
		}
	})
	t.Run("Ping", func(t *testing.T) {
		if err := c.Ping(ctx); err == nil {
			t.Error("expected error when Redis is unavailable")
		}
	})
	t.Run("Len", func(t *testing.T) {
		_, err := c.Len(ctx, "some.stream")
		if err == nil {
			t.Error("expected error when Redis is unavailable")
		}
	})
}

func TestAddProbeResult_requiredFieldsNotOverridden(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	_, err := c.AddProbeResult(ctx, "pt_real", "nd_real", map[string]any{
		"task_id": "OVERRIDE",
		"node_id": "OVERRIDE",
		"extra":   "value",
	})
	if err != nil {
		t.Fatalf("AddProbeResult failed: %v", err)
	}
	msgs, err := rdb.XRange(ctx, stream.Probe, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if got := msgs[0].Values["task_id"]; got != "pt_real" {
		t.Errorf("task_id: expected %q, got %v", "pt_real", got)
	}
	if got := msgs[0].Values["node_id"]; got != "nd_real" {
		t.Errorf("node_id: expected %q, got %v", "nd_real", got)
	}
}

func TestAddMonitorEvent_requiredFieldsNotOverridden(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	_, err := c.AddMonitorEvent(ctx, "m_real", "up", map[string]any{
		"monitor_id": "OVERRIDE",
		"event":      "OVERRIDE",
	})
	if err != nil {
		t.Fatalf("AddMonitorEvent failed: %v", err)
	}
	msgs, err := rdb.XRange(ctx, stream.Monitor, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if got := msgs[0].Values["monitor_id"]; got != "m_real" {
		t.Errorf("monitor_id: expected %q, got %v", "m_real", got)
	}
	if got := msgs[0].Values["event"]; got != "up" {
		t.Errorf("event: expected %q, got %v", "up", got)
	}
}

func TestAddProbeResultTyped(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	r := contracts.ProbeResult{
		TaskID:     "pt_typed",
		NodeID:     "nd_typed",
		DurationMs: 99,
		Success:    true,
		MonitorID:  "m_typed",
	}
	id, err := c.AddProbeResultTyped(ctx, r)
	if err != nil {
		t.Fatalf("AddProbeResultTyped failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty stream ID")
	}
	msgs, err := rdb.XRange(ctx, stream.Probe, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got, err := contracts.ParseProbeResult(msgs[0].Values)
	if err != nil {
		t.Fatalf("ParseProbeResult on round-tripped message failed: %v", err)
	}
	if got.TaskID != r.TaskID || got.NodeID != r.NodeID || got.DurationMs != r.DurationMs ||
		got.Success != r.Success || got.MonitorID != r.MonitorID {
		t.Errorf("round-trip drift:\n  want %+v\n   got %+v", r, got)
	}
	if got.SchemaVer != contracts.ProbeResultSchemaV1 {
		t.Errorf("expected schema_ver auto-inject = %d, got %d", contracts.ProbeResultSchemaV1, got.SchemaVer)
	}
}

func TestAddProbeResultTyped_RespectsExplicitSchemaVer(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	r := contracts.ProbeResult{
		SchemaVer:  contracts.ProbeResultSchemaV1, // explicit
		TaskID:     "pt_a",
		NodeID:     "nd_a",
		DurationMs: 1,
	}
	if _, err := c.AddProbeResultTyped(ctx, r); err != nil {
		t.Fatal(err)
	}
	msgs, _ := rdb.XRange(ctx, stream.Probe, "-", "+").Result()
	if got := msgs[0].Values["schema_ver"]; got != "1" {
		t.Errorf("schema_ver: got %v, want '1'", got)
	}
}

func TestAddCertNotificationTyped(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	notAfter := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	emitted := time.Date(2026, 5, 21, 8, 30, 0, 0, time.UTC)
	e := contracts.CertNotificationEvent{
		EventType:    "cert.issued",
		AccountID:    "42",
		CertID:       77,
		OrderID:      100,
		SANs:         []string{"a.example.com"},
		CA:           "lets-encrypt",
		DaysToExpire: 14,
		NotAfter:     notAfter,
		Subject:      "[idcd] cert ready",
		Body:         "Hello world",
		EmittedAt:    emitted,
	}
	id, err := c.AddCertNotificationTyped(ctx, e)
	if err != nil {
		t.Fatalf("AddCertNotificationTyped failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty stream ID")
	}
	msgs, err := rdb.XRange(ctx, stream.CertNotifications, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got, err := contracts.ParseCertNotificationEvent(msgs[0].Values)
	if err != nil {
		t.Fatalf("ParseCertNotificationEvent failed: %v", err)
	}
	if got.EventType != e.EventType || got.AccountID != e.AccountID ||
		got.CertID != e.CertID || got.OrderID != e.OrderID ||
		got.CA != e.CA || got.DaysToExpire != e.DaysToExpire ||
		got.Subject != e.Subject || got.Body != e.Body {
		t.Errorf("round-trip drift:\n  want %+v\n   got %+v", e, got)
	}
	if !got.NotAfter.Equal(e.NotAfter) {
		t.Errorf("NotAfter drift: got %s, want %s", got.NotAfter, e.NotAfter)
	}
	if !got.EmittedAt.Equal(e.EmittedAt) {
		t.Errorf("EmittedAt drift: got %s, want %s", got.EmittedAt, e.EmittedAt)
	}
	if got.SchemaVer != contracts.CertNotificationEventSchemaV1 {
		t.Errorf("expected schema_ver auto-inject = %d, got %d",
			contracts.CertNotificationEventSchemaV1, got.SchemaVer)
	}
	if len(got.SANs) != 1 || got.SANs[0] != "a.example.com" {
		t.Errorf("SANs drift: got %v, want [a.example.com]", got.SANs)
	}
}

func TestAddCertNotificationTyped_RespectsExplicitSchemaVer(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	e := contracts.CertNotificationEvent{
		SchemaVer: contracts.CertNotificationEventSchemaV1,
		EventType: "cert.issued",
		AccountID: "1",
	}
	if _, err := c.AddCertNotificationTyped(ctx, e); err != nil {
		t.Fatal(err)
	}
	msgs, _ := rdb.XRange(ctx, stream.CertNotifications, "-", "+").Result()
	if got := msgs[0].Values["schema_ver"]; got != "1" {
		t.Errorf("schema_ver: got %v, want '1'", got)
	}
}

func TestAddMonitorEventTyped(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	e := contracts.MonitorEvent{
		MonitorID: "m_typed",
		Event:     "recovery",
		Severity:  "info",
		Reason:    "ping resumed",
		ExtraJSON: []byte(`{"down_for_sec":42}`),
	}
	id, err := c.AddMonitorEventTyped(ctx, e)
	if err != nil {
		t.Fatalf("AddMonitorEventTyped failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty stream ID")
	}
	msgs, err := rdb.XRange(ctx, stream.Monitor, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got, err := contracts.ParseMonitorEvent(msgs[0].Values)
	if err != nil {
		t.Fatalf("ParseMonitorEvent failed: %v", err)
	}
	if got.MonitorID != e.MonitorID || got.Event != e.Event ||
		got.Severity != e.Severity || got.Reason != e.Reason {
		t.Errorf("round-trip drift:\n  want %+v\n   got %+v", e, got)
	}
	if string(got.ExtraJSON) != string(e.ExtraJSON) {
		t.Errorf("ExtraJSON drift: got %s, want %s", got.ExtraJSON, e.ExtraJSON)
	}
	if got.TsMs == 0 {
		t.Error("expected TsMs to be auto-set by ToStreamValues")
	}
}

// setupTracingGlobals installs an always-sample TracerProvider + W3C/Baggage
// propagator for the duration of the test, restoring globals on cleanup.
// Mirrors the helper in telemetry/stream_carrier_test.go so this file remains
// self-contained.
func setupTracingGlobals(t *testing.T) trace.Tracer {
	t.Helper()
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})
	return tp.Tracer("stream-test")
}

// TestAddPropagatesTraceContext: P1-5 — stream.Client.Add must inject the
// active OTel trace context into the XAdd values so the consumer side can
// Extract it and continue the same trace.
func TestAddPropagatesTraceContext(t *testing.T) {
	tracer := setupTracingGlobals(t)
	c, rdb := newTestClientWithRDB(t)

	ctx, span := tracer.Start(context.Background(), "producer")
	defer span.End()
	wantTraceID := span.SpanContext().TraceID()

	_, err := c.Add(ctx, "trace.stream", map[string]any{"task_id": "pt_trace"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	msgs, err := rdb.XRange(ctx, "trace.stream", "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	// Round-trip through ExtractStream — the consumer-side helper — and
	// confirm trace_id continuity.
	gotCtx := telemetry.ExtractStream(context.Background(), msgs[0].Values)
	gotSC := trace.SpanContextFromContext(gotCtx)
	if !gotSC.IsValid() {
		t.Fatal("ExtractStream produced invalid SpanContext (no traceparent on the message)")
	}
	if gotSC.TraceID() != wantTraceID {
		t.Errorf("trace_id drift across stream: got %s, want %s",
			gotSC.TraceID(), wantTraceID)
	}
	// Business field still present.
	if msgs[0].Values["task_id"] != "pt_trace" {
		t.Errorf("business field clobbered by trace inject: %v", msgs[0].Values)
	}
}

// TestAddProbeResultTyped_PropagatesTrace: typed Add paths go through Add()
// at the bottom, so they get inject for free. Spot-check that this is true
// in practice.
func TestAddProbeResultTyped_PropagatesTrace(t *testing.T) {
	tracer := setupTracingGlobals(t)
	c, rdb := newTestClientWithRDB(t)
	ctx, span := tracer.Start(context.Background(), "producer-typed")
	defer span.End()

	r := contracts.ProbeResult{TaskID: "pt_typed", NodeID: "nd_typed", DurationMs: 5}
	if _, err := c.AddProbeResultTyped(ctx, r); err != nil {
		t.Fatal(err)
	}
	msgs, _ := rdb.XRange(ctx, stream.Probe, "-", "+").Result()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message")
	}
	gotCtx := telemetry.ExtractStream(context.Background(), msgs[0].Values)
	gotSC := trace.SpanContextFromContext(gotCtx)
	if !gotSC.IsValid() || gotSC.TraceID() != span.SpanContext().TraceID() {
		t.Errorf("AddProbeResultTyped did not propagate trace context: span_ctx=%v", gotSC)
	}
	// Typed parse must still succeed — trace fields are stream-level metadata,
	// not part of the contract, so ParseProbeResult ignores them.
	parsed, err := contracts.ParseProbeResult(msgs[0].Values)
	if err != nil {
		t.Fatalf("ParseProbeResult after trace inject failed: %v", err)
	}
	if parsed.TaskID != "pt_typed" {
		t.Errorf("Parse drift: %+v", parsed)
	}
}

func TestAdd_NoActiveSpan_NoTraceparent(t *testing.T) {
	setupTracingGlobals(t)
	c, rdb := newTestClientWithRDB(t)
	// Bare ctx with no active span — Inject is a no-op, no traceparent field
	// should appear on the message.
	if _, err := c.Add(context.Background(), "no.trace.stream", map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	msgs, _ := rdb.XRange(context.Background(), "no.trace.stream", "-", "+").Result()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message")
	}
	if _, ok := msgs[0].Values["traceparent"]; ok {
		t.Errorf("traceparent should not be present without active span: %v", msgs[0].Values)
	}
	if msgs[0].Values["k"] != "v" {
		t.Errorf("business field lost: %v", msgs[0].Values)
	}
}

func TestStreamConstants(t *testing.T) {
	// Ensure constants haven't been accidentally changed — these names are
	// shared across services and changing them would break consumers.
	cases := map[string]string{
		"Probe":               stream.Probe,
		"Monitor":             stream.Monitor,
		"Alert":               stream.Alert,
		"Audit":               stream.Audit,
		"Usage":               stream.Usage,
		"CertNotifications":   stream.CertNotifications,
		"RefundInitiateQueue": stream.RefundInitiateQueue,
		"RefundRetryQueue":    stream.RefundRetryQueue,
	}
	expected := map[string]string{
		"Probe":               "probe.results",
		"Monitor":             "monitor.events",
		"Alert":               "alert.events",
		"Audit":               "audit.events",
		"Usage":               "usage.events",
		"CertNotifications":   "cert:notifications",
		"RefundInitiateQueue": "refund_initiate_queue",
		"RefundRetryQueue":    "refund_retry_queue",
	}
	for name, got := range cases {
		if got != expected[name] {
			t.Errorf("%s constant: expected %q, got %q", name, expected[name], got)
		}
	}
}

func TestAddRefundInitiateTyped(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	enqueued := time.Date(2026, 5, 21, 9, 30, 15, 0, time.UTC)
	e := contracts.RefundInitiateEvent{
		ReportID:   "vr_typed",
		Reason:     "bad signature on PDF",
		EnqueuedAt: enqueued,
	}
	id, err := c.AddRefundInitiateTyped(ctx, e)
	if err != nil {
		t.Fatalf("AddRefundInitiateTyped failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty stream ID")
	}
	msgs, err := rdb.XRange(ctx, stream.RefundInitiateQueue, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got, err := contracts.ParseRefundInitiateEvent(msgs[0].Values)
	if err != nil {
		t.Fatalf("ParseRefundInitiateEvent failed: %v", err)
	}
	if got.ReportID != e.ReportID || got.Reason != e.Reason {
		t.Errorf("round-trip drift:\n  want %+v\n   got %+v", e, got)
	}
	if !got.EnqueuedAt.Equal(e.EnqueuedAt) {
		t.Errorf("EnqueuedAt drift: got %s, want %s", got.EnqueuedAt, e.EnqueuedAt)
	}
	if got.SchemaVer != contracts.RefundInitiateEventSchemaV1 {
		t.Errorf("expected schema_ver auto-inject = %d, got %d",
			contracts.RefundInitiateEventSchemaV1, got.SchemaVer)
	}
}

func TestAddRefundInitiateTyped_RespectsExplicitSchemaVer(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	e := contracts.RefundInitiateEvent{
		SchemaVer: contracts.RefundInitiateEventSchemaV1,
		ReportID:  "vr_a",
	}
	if _, err := c.AddRefundInitiateTyped(ctx, e); err != nil {
		t.Fatal(err)
	}
	msgs, _ := rdb.XRange(ctx, stream.RefundInitiateQueue, "-", "+").Result()
	if got := msgs[0].Values["schema_ver"]; got != "1" {
		t.Errorf("schema_ver: got %v, want '1'", got)
	}
}

func TestAddRefundRetryTyped(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	scheduled := time.Date(2026, 5, 21, 10, 35, 0, 0, time.UTC)
	e := contracts.RefundRetryEvent{
		OrderID:     "v_abc",
		ExtEventID:  "evt_x",
		Attempt:     1,
		ScheduledAt: scheduled,
	}
	id, err := c.AddRefundRetryTyped(ctx, e)
	if err != nil {
		t.Fatalf("AddRefundRetryTyped failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty stream ID")
	}
	msgs, err := rdb.XRange(ctx, stream.RefundRetryQueue, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got, err := contracts.ParseRefundRetryEvent(msgs[0].Values)
	if err != nil {
		t.Fatalf("ParseRefundRetryEvent failed: %v", err)
	}
	if got.OrderID != e.OrderID || got.ExtEventID != e.ExtEventID || got.Attempt != e.Attempt {
		t.Errorf("round-trip drift:\n  want %+v\n   got %+v", e, got)
	}
	if !got.ScheduledAt.Equal(e.ScheduledAt) {
		t.Errorf("ScheduledAt drift: got %s, want %s", got.ScheduledAt, e.ScheduledAt)
	}
	if got.SchemaVer != contracts.RefundRetryEventSchemaV1 {
		t.Errorf("expected schema_ver auto-inject = %d, got %d",
			contracts.RefundRetryEventSchemaV1, got.SchemaVer)
	}
	// Pin wire-level attempt encoding so future "Attempt: int → JSON number"
	// drift fails CI rather than silently breaking the consumer's strconv.Atoi.
	if rawAttempt := msgs[0].Values["attempt"]; rawAttempt != "1" {
		t.Errorf("attempt wire format: got %v (%T), want \"1\"", rawAttempt, rawAttempt)
	}
}

func TestAddRefundRetryTyped_RespectsExplicitSchemaVer(t *testing.T) {
	c, rdb := newTestClientWithRDB(t)
	ctx := context.Background()
	e := contracts.RefundRetryEvent{
		SchemaVer:   contracts.RefundRetryEventSchemaV1,
		OrderID:     "v_a",
		Attempt:     1,
		ScheduledAt: time.Now().UTC(),
	}
	if _, err := c.AddRefundRetryTyped(ctx, e); err != nil {
		t.Fatal(err)
	}
	msgs, _ := rdb.XRange(ctx, stream.RefundRetryQueue, "-", "+").Result()
	if got := msgs[0].Values["schema_ver"]; got != "1" {
		t.Errorf("schema_ver: got %v, want '1'", got)
	}
}
