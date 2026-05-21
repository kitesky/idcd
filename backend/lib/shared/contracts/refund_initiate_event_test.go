package contracts_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/contracts"
)

func TestRefundInitiateEvent_RoundTrip(t *testing.T) {
	t.Parallel()
	enqueued := time.Date(2026, 5, 21, 9, 30, 15, 123_456_789, time.UTC)
	original := contracts.RefundInitiateEvent{
		SchemaVer:  contracts.RefundInitiateEventSchemaV1,
		ReportID:   "vr_42",
		Reason:     "bad signature on PDF",
		EnqueuedAt: enqueued,
	}
	vals := original.ToStreamValues()
	got, err := contracts.ParseRefundInitiateEvent(vals)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if got.SchemaVer != original.SchemaVer ||
		got.ReportID != original.ReportID ||
		got.Reason != original.Reason {
		t.Errorf("scalar round-trip drift:\n  want %+v\n   got %+v", original, got)
	}
	if !got.EnqueuedAt.Equal(original.EnqueuedAt) {
		t.Errorf("EnqueuedAt drift: got %s, want %s", got.EnqueuedAt, original.EnqueuedAt)
	}
}

func TestRefundInitiateEvent_AutoSchemaVer(t *testing.T) {
	t.Parallel()
	e := contracts.RefundInitiateEvent{ReportID: "vr_1"}
	vals := e.ToStreamValues()
	if got := vals["schema_ver"]; got != "1" {
		t.Errorf("expected schema_ver auto-set to '1', got %v", got)
	}
}

func TestRefundInitiateEvent_AutoEnqueuedAt(t *testing.T) {
	t.Parallel()
	e := contracts.RefundInitiateEvent{ReportID: "vr_1"}
	vals := e.ToStreamValues()
	s, ok := vals["enqueued_at"].(string)
	if !ok || s == "" {
		t.Fatalf("expected enqueued_at auto-filled, got %v", vals["enqueued_at"])
	}
	if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
		t.Errorf("enqueued_at is not RFC3339Nano: %q (%v)", s, err)
	}
}

func TestRefundInitiateEvent_OptionalReasonOmitted(t *testing.T) {
	t.Parallel()
	e := contracts.RefundInitiateEvent{ReportID: "vr_1"}
	vals := e.ToStreamValues()
	if _, present := vals["reason"]; present {
		t.Error("reason should be omitted when empty, but was present")
	}
	// Required keys still present.
	for _, k := range []string{"schema_ver", "report_id", "enqueued_at"} {
		if _, present := vals[k]; !present {
			t.Errorf("expected required key %q to be present", k)
		}
	}
	// Reparse — reason should be zero value.
	got, err := contracts.ParseRefundInitiateEvent(vals)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Reason != "" {
		t.Errorf("Reason: got %q, want empty", got.Reason)
	}
}

func TestParseRefundInitiateEvent_MissingReportID(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundInitiateEvent(map[string]any{
		"reason": "x",
	})
	if err == nil {
		t.Fatal("expected error when report_id is missing")
	}
	if !strings.Contains(err.Error(), "report_id is required") {
		t.Errorf("err = %q, want substring 'report_id is required'", err)
	}
}

func TestParseRefundInitiateEvent_UnknownSchemaVer(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundInitiateEvent(map[string]any{
		"report_id":  "vr_1",
		"schema_ver": "999",
	})
	if err == nil {
		t.Fatal("expected error for future schema_ver")
	}
	if !errors.Is(err, contracts.ErrUnknownSchemaVer) {
		t.Errorf("err should wrap ErrUnknownSchemaVer, got: %v", err)
	}
}

func TestParseRefundInitiateEvent_LegacyMessage(t *testing.T) {
	// schema_ver missing → treated as legacy (SchemaVer=0), must still parse.
	t.Parallel()
	got, err := contracts.ParseRefundInitiateEvent(map[string]any{
		"report_id":   "vr_42",
		"reason":      "bad sig",
		"enqueued_at": "2026-05-21T09:30:15.123456789Z",
	})
	if err != nil {
		t.Fatalf("legacy parse failed: %v", err)
	}
	if got.SchemaVer != 0 {
		t.Errorf("legacy SchemaVer: got %d, want 0", got.SchemaVer)
	}
	if got.ReportID != "vr_42" || got.Reason != "bad sig" {
		t.Errorf("legacy fields drift: %+v", got)
	}
	if got.EnqueuedAt.IsZero() {
		t.Errorf("EnqueuedAt should be parsed from RFC3339Nano legacy string")
	}
}

func TestParseRefundInitiateEvent_SchemaVerTypes(t *testing.T) {
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
			got, err := contracts.ParseRefundInitiateEvent(map[string]any{
				"report_id":  "vr_1",
				"schema_ver": tc.in,
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

func TestParseRefundInitiateEvent_SchemaVerBadString(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundInitiateEvent(map[string]any{
		"report_id":  "vr_1",
		"schema_ver": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid schema_ver string")
	}
}

func TestParseRefundInitiateEvent_SchemaVerUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseRefundInitiateEvent(map[string]any{
		"report_id":  "vr_1",
		"schema_ver": []byte("1"),
	})
	if err == nil {
		t.Fatal("expected error for []byte schema_ver")
	}
}

func TestParseRefundInitiateEvent_MalformedTimestampIgnored(t *testing.T) {
	// Malformed enqueued_at should leave the field zero-valued (not error)
	// — best-effort decoding per the doc contract; the report_id is still
	// the only required field, the timestamp is for log/observability.
	t.Parallel()
	got, err := contracts.ParseRefundInitiateEvent(map[string]any{
		"report_id":   "vr_1",
		"enqueued_at": "not-a-timestamp",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.EnqueuedAt.IsZero() {
		t.Errorf("EnqueuedAt: got %s, want zero on malformed input", got.EnqueuedAt)
	}
}

func TestParseRefundInitiateEvent_RFC3339Fallback(t *testing.T) {
	// Pre-W3 producers always wrote RFC3339Nano; but for forward
	// compatibility (and consistency with refund.parseScheduledAt),
	// plain RFC3339 must also parse.
	t.Parallel()
	got, err := contracts.ParseRefundInitiateEvent(map[string]any{
		"report_id":   "vr_1",
		"enqueued_at": "2026-05-21T09:30:15Z",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.EnqueuedAt.IsZero() {
		t.Error("expected EnqueuedAt to be parsed from plain RFC3339")
	}
}
