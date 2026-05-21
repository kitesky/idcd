package contracts_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/contracts"
)

func TestRefundRetryEvent_RoundTrip(t *testing.T) {
	t.Parallel()
	scheduled := time.Date(2026, 5, 21, 10, 35, 0, 0, time.UTC)
	original := contracts.RefundRetryEvent{
		SchemaVer:   contracts.RefundRetryEventSchemaV1,
		OrderID:     "v_abc",
		ExtEventID:  "evt_x",
		Attempt:     1,
		ScheduledAt: scheduled,
	}
	vals := original.ToStreamValues()
	got, err := contracts.ParseRefundRetryEvent(vals)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if got.SchemaVer != original.SchemaVer ||
		got.OrderID != original.OrderID ||
		got.ExtEventID != original.ExtEventID ||
		got.Attempt != original.Attempt {
		t.Errorf("scalar round-trip drift:\n  want %+v\n   got %+v", original, got)
	}
	if !got.ScheduledAt.Equal(original.ScheduledAt) {
		t.Errorf("ScheduledAt drift: got %s, want %s", got.ScheduledAt, original.ScheduledAt)
	}
}

func TestRefundRetryEvent_AutoSchemaVer(t *testing.T) {
	t.Parallel()
	e := contracts.RefundRetryEvent{
		OrderID:     "v_1",
		Attempt:     1,
		ScheduledAt: time.Now().UTC(),
	}
	vals := e.ToStreamValues()
	if got := vals["schema_ver"]; got != "1" {
		t.Errorf("expected schema_ver auto-set to '1', got %v", got)
	}
}

func TestRefundRetryEvent_AttemptWireFormat(t *testing.T) {
	// Producer historically wrote attempt as a string ("1"). Confirm the
	// contract preserves that.
	t.Parallel()
	e := contracts.RefundRetryEvent{
		OrderID:     "v_1",
		Attempt:     2,
		ScheduledAt: time.Now().UTC(),
	}
	vals := e.ToStreamValues()
	if got := vals["attempt"]; got != "2" {
		t.Errorf("attempt: expected wire format \"2\" (string), got %v (%T)", got, got)
	}
}

func TestRefundRetryEvent_OptionalExtEventID(t *testing.T) {
	t.Parallel()
	// Producer always supplies ext_event_id today, but the contract
	// permits empty (parse-side leniency only — wire still emits "")
	// because it's needed for completeness, just no enforcement.
	e := contracts.RefundRetryEvent{
		OrderID:     "v_1",
		Attempt:     1,
		ScheduledAt: time.Now().UTC(),
	}
	vals := e.ToStreamValues()
	// ext_event_id always written even when empty (it's not omitempty).
	if _, present := vals["ext_event_id"]; !present {
		t.Error("ext_event_id should be present (possibly empty) in stream values")
	}
	got, err := contracts.ParseRefundRetryEvent(vals)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.ExtEventID != "" {
		t.Errorf("ExtEventID: got %q, want empty", got.ExtEventID)
	}
}

func TestParseRefundRetryEvent_MissingOrderID(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"attempt":      "1",
		"scheduled_at": "2026-05-21T10:35:00Z",
	})
	if err == nil {
		t.Fatal("expected error when order_id is missing")
	}
	if !strings.Contains(err.Error(), "order_id is required") {
		t.Errorf("err = %q, want substring 'order_id is required'", err)
	}
}

func TestParseRefundRetryEvent_MissingAttempt(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"scheduled_at": "2026-05-21T10:35:00Z",
	})
	if err == nil {
		t.Fatal("expected error when attempt is missing")
	}
	if !strings.Contains(err.Error(), "attempt is required") {
		t.Errorf("err = %q, want substring 'attempt is required'", err)
	}
}

func TestParseRefundRetryEvent_AttemptZeroRejected(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"attempt":      "0",
		"scheduled_at": "2026-05-21T10:35:00Z",
	})
	if err == nil {
		t.Fatal("expected error when attempt < 1")
	}
}

func TestParseRefundRetryEvent_AttemptNotNumeric(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"attempt":      "abc",
		"scheduled_at": "2026-05-21T10:35:00Z",
	})
	if err == nil {
		t.Fatal("expected error when attempt is not numeric")
	}
}

func TestParseRefundRetryEvent_MissingScheduledAt(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id": "v_1",
		"attempt":  "1",
	})
	if err == nil {
		t.Fatal("expected error when scheduled_at is missing")
	}
	if !strings.Contains(err.Error(), "scheduled_at is required") {
		t.Errorf("err = %q, want substring 'scheduled_at is required'", err)
	}
}

func TestParseRefundRetryEvent_BadScheduledAt(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"attempt":      "1",
		"scheduled_at": "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for malformed scheduled_at")
	}
}

func TestParseRefundRetryEvent_UnknownSchemaVer(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"attempt":      "1",
		"scheduled_at": "2026-05-21T10:35:00Z",
		"schema_ver":   "999",
	})
	if err == nil {
		t.Fatal("expected error for future schema_ver")
	}
	if !errors.Is(err, contracts.ErrUnknownSchemaVer) {
		t.Errorf("err should wrap ErrUnknownSchemaVer, got: %v", err)
	}
}

func TestParseRefundRetryEvent_LegacyMessage(t *testing.T) {
	// schema_ver missing → treated as legacy (SchemaVer=0), must still parse.
	t.Parallel()
	got, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_abc",
		"ext_event_id": "evt_x",
		"attempt":      "1",
		"scheduled_at": "2026-05-21T10:35:00.000000000Z",
	})
	if err != nil {
		t.Fatalf("legacy parse failed: %v", err)
	}
	if got.SchemaVer != 0 {
		t.Errorf("legacy SchemaVer: got %d, want 0", got.SchemaVer)
	}
	if got.OrderID != "v_abc" || got.ExtEventID != "evt_x" || got.Attempt != 1 {
		t.Errorf("legacy fields drift: %+v", got)
	}
}

func TestParseRefundRetryEvent_SchemaVerTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"string", "1", 1},
		{"int", int(1), 1},
		{"int64", int64(1), 1},
		{"float64", float64(1), 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := contracts.ParseRefundRetryEvent(map[string]any{
				"order_id":     "v_1",
				"attempt":      "1",
				"scheduled_at": "2026-05-21T10:35:00Z",
				"schema_ver":   tc.in,
			})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got.SchemaVer != tc.want {
				t.Errorf("SchemaVer: got %d, want %d", got.SchemaVer, tc.want)
			}
		})
	}
}

func TestParseRefundRetryEvent_SchemaVerBadString(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"attempt":      "1",
		"scheduled_at": "2026-05-21T10:35:00Z",
		"schema_ver":   "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid schema_ver string")
	}
}

func TestParseRefundRetryEvent_SchemaVerUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"attempt":      "1",
		"scheduled_at": "2026-05-21T10:35:00Z",
		"schema_ver":   []byte("1"),
	})
	if err == nil {
		t.Fatal("expected error for []byte schema_ver")
	}
}

func TestParseRefundRetryEvent_AttemptTypes(t *testing.T) {
	// parseInt64 accepts string / int / int64 / float64 — confirm the
	// consumer can decode whatever form go-redis hands back.
	t.Parallel()
	cases := []any{"1", 1, int64(1), float64(1)}
	for _, in := range cases {
		got, err := contracts.ParseRefundRetryEvent(map[string]any{
			"order_id":     "v_1",
			"attempt":      in,
			"scheduled_at": "2026-05-21T10:35:00Z",
		})
		if err != nil {
			t.Errorf("attempt=%v (%T): %v", in, in, err)
			continue
		}
		if got.Attempt != 1 {
			t.Errorf("attempt=%v (%T): got %d, want 1", in, in, got.Attempt)
		}
	}
}

func TestParseRefundRetryEvent_RFC3339Fallback(t *testing.T) {
	// Plain RFC3339 (no fractional seconds) must also parse.
	t.Parallel()
	got, err := contracts.ParseRefundRetryEvent(map[string]any{
		"order_id":     "v_1",
		"attempt":      "1",
		"scheduled_at": "2026-05-21T10:35:00Z",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.ScheduledAt.IsZero() {
		t.Error("expected ScheduledAt to be parsed from plain RFC3339")
	}
}
