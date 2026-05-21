package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// setupPropagator wires up a real TracerProvider + W3C/Baggage propagator
// for the duration of the test, restoring previous globals on cleanup so
// other tests aren't affected.
func setupPropagator(t *testing.T) trace.Tracer {
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
	return tp.Tracer("test")
}

func TestStreamCarrier_GetSet(t *testing.T) {
	c := StreamCarrier{}
	c.Set("k", "v")
	if got := c.Get("k"); got != "v" {
		t.Errorf("Get(k) = %q, want %q", got, "v")
	}
}

func TestStreamCarrier_GetMissing(t *testing.T) {
	c := StreamCarrier{}
	if got := c.Get("nope"); got != "" {
		t.Errorf("Get on empty carrier = %q, want \"\"", got)
	}
}

func TestStreamCarrier_GetNonStringReturnsEmpty(t *testing.T) {
	c := StreamCarrier{
		"int_field":   42,
		"bool_field":  true,
		"float_field": 3.14,
		"nil_field":   nil,
		"str_field":   "hello",
	}
	cases := map[string]string{
		"int_field":   "",
		"bool_field":  "",
		"float_field": "",
		"nil_field":   "",
		"str_field":   "hello",
	}
	for key, want := range cases {
		if got := c.Get(key); got != want {
			t.Errorf("Get(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestStreamCarrier_GetBytesCoercedToString(t *testing.T) {
	// go-redis sometimes hands back []byte values; the carrier should still
	// surface them as strings rather than dropping the trace header.
	c := StreamCarrier{"traceparent": []byte("00-deadbeef-cafef00d-01")}
	if got := c.Get("traceparent"); got != "00-deadbeef-cafef00d-01" {
		t.Errorf("Get on []byte value = %q", got)
	}
}

func TestStreamCarrier_Keys(t *testing.T) {
	c := StreamCarrier{"a": "1", "b": "2", "c": "3"}
	keys := c.Keys()
	if len(keys) != 3 {
		t.Fatalf("len(Keys) = %d, want 3", len(keys))
	}
	seen := map[string]bool{}
	for _, k := range keys {
		seen[k] = true
	}
	for _, k := range []string{"a", "b", "c"} {
		if !seen[k] {
			t.Errorf("key %q missing from Keys()", k)
		}
	}
}

func TestStreamCarrier_SetOverwrites(t *testing.T) {
	c := StreamCarrier{}
	c.Set("k", "v1")
	c.Set("k", "v2")
	if got := c.Get("k"); got != "v2" {
		t.Errorf("Get after overwrite = %q, want \"v2\"", got)
	}
}

func TestInjectExtract_RoundTrip(t *testing.T) {
	tracer := setupPropagator(t)

	// Start a span so ctx has a real trace_id / span_id.
	ctx, span := tracer.Start(context.Background(), "producer")
	defer span.End()

	wantTraceID := span.SpanContext().TraceID()

	vals := map[string]any{
		"task_id": "pt_abc",
		"node_id": "nd_jp",
	}
	InjectStream(ctx, vals)

	// W3C propagator must have written traceparent.
	if _, ok := vals["traceparent"]; !ok {
		t.Fatalf("InjectStream did not write traceparent; vals=%v", vals)
	}

	// Business fields must remain intact.
	if vals["task_id"] != "pt_abc" || vals["node_id"] != "nd_jp" {
		t.Errorf("InjectStream clobbered business fields: vals=%v", vals)
	}

	// Extract on a fresh ctx and verify the same trace_id flows through.
	gotCtx := ExtractStream(context.Background(), vals)
	gotSC := trace.SpanContextFromContext(gotCtx)
	if !gotSC.IsValid() {
		t.Fatal("ExtractStream returned invalid SpanContext")
	}
	if gotSC.TraceID() != wantTraceID {
		t.Errorf("trace_id drift: got %s, want %s", gotSC.TraceID(), wantTraceID)
	}
}

func TestInjectStream_NoActiveSpan(t *testing.T) {
	setupPropagator(t)

	// Bare context with no span — Inject should be a no-op but must not panic.
	vals := map[string]any{"k": "v"}
	InjectStream(context.Background(), vals)

	// With no active SpanContext, W3C propagator declines to write traceparent.
	// (Confirmed behaviour as of otel v1.43.)
	if _, ok := vals["traceparent"]; ok {
		t.Errorf("InjectStream wrote traceparent without active span: vals=%v", vals)
	}
	// Business field still present.
	if vals["k"] != "v" {
		t.Errorf("business field clobbered: vals=%v", vals)
	}
}

func TestInjectStream_NilVals(t *testing.T) {
	setupPropagator(t)
	// Should not panic.
	InjectStream(context.Background(), nil)
}

func TestExtractStream_EmptyVals(t *testing.T) {
	setupPropagator(t)
	ctx := context.Background()
	got := ExtractStream(ctx, nil)
	if got != ctx {
		t.Error("ExtractStream(ctx, nil) should return ctx unchanged")
	}
	got = ExtractStream(ctx, map[string]any{})
	if got != ctx {
		t.Error("ExtractStream(ctx, empty map) should return ctx unchanged")
	}
}

func TestExtractStream_NoTraceparent(t *testing.T) {
	setupPropagator(t)
	// vals has only business fields — extracted ctx should have no valid span.
	vals := map[string]any{"task_id": "pt_x", "node_id": "nd_y"}
	gotCtx := ExtractStream(context.Background(), vals)
	gotSC := trace.SpanContextFromContext(gotCtx)
	if gotSC.IsValid() {
		t.Errorf("expected invalid SpanContext on no-traceparent extract, got %v", gotSC)
	}
}

func TestInjectExtract_PreservesBusinessFields(t *testing.T) {
	tracer := setupPropagator(t)
	ctx, span := tracer.Start(context.Background(), "test")
	defer span.End()

	vals := map[string]any{
		"task_id":     "pt_a",
		"node_id":     "nd_b",
		"duration_ms": int64(42), // non-string field
		"success":     true,      // non-string field
	}
	InjectStream(ctx, vals)

	// Business fields untouched.
	if vals["task_id"] != "pt_a" || vals["node_id"] != "nd_b" {
		t.Errorf("string business fields lost: %v", vals)
	}
	if vals["duration_ms"] != int64(42) || vals["success"] != true {
		t.Errorf("non-string business fields lost / coerced: %v", vals)
	}
}
