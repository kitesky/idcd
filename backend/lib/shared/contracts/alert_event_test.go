package contracts_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/contracts"
)

func TestAlertEvent_RoundTrip(t *testing.T) {
	t.Parallel()
	// UnixMilli round-trips lose monotonic clock and sub-ms precision, so
	// pick a whole-millisecond timestamp for clean equality.
	ts := time.UnixMilli(time.Date(2026, 5, 21, 8, 30, 0, 0, time.UTC).UnixMilli()).UTC()
	original := contracts.AlertEvent{
		SchemaVer:    contracts.AlertEventSchemaV1,
		AlertEventID: "ae_abc123",
		MonitorID:    "mon_xyz789",
		Kind:         "down",
		TsMs:         ts,
	}
	vals := original.ToStreamValues()
	got, err := contracts.ParseAlertEvent(vals)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if got.SchemaVer != original.SchemaVer ||
		got.AlertEventID != original.AlertEventID ||
		got.MonitorID != original.MonitorID ||
		got.Kind != original.Kind {
		t.Errorf("scalar round-trip drift:\n  want %+v\n   got %+v", original, got)
	}
	if !got.TsMs.Equal(original.TsMs) {
		t.Errorf("TsMs drift: got %s (unix_ms=%d), want %s (unix_ms=%d)",
			got.TsMs, got.TsMs.UnixMilli(), original.TsMs, original.TsMs.UnixMilli())
	}
}

func TestAlertEvent_DefaultSchemaVer(t *testing.T) {
	t.Parallel()
	e := contracts.AlertEvent{
		AlertEventID: "ae_1",
		MonitorID:    "mon_1",
		Kind:         "down",
	}
	vals := e.ToStreamValues()
	if got := vals["schema_ver"]; got != "1" {
		t.Errorf("expected schema_ver auto-set to '1', got %v", got)
	}
}

func TestAlertEvent_DefaultTimestamp(t *testing.T) {
	t.Parallel()
	before := time.Now()
	e := contracts.AlertEvent{
		AlertEventID: "ae_1",
		MonitorID:    "mon_1",
		Kind:         "down",
		// TsMs left zero — ToStreamValues should auto-fill.
	}
	vals := e.ToStreamValues()
	after := time.Now()

	s, ok := vals["ts"].(string)
	if !ok || s == "" {
		t.Fatalf("expected ts auto-filled, got %v (%T)", vals["ts"], vals["ts"])
	}
	// Parse the int64 ms back to time.Time and check it lies in [before-1s, after+1s].
	// Round to ms for the lower bound since UnixMilli truncates sub-ms precision.
	got, err := contracts.ParseAlertEvent(vals)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	lo := before.Add(-time.Second)
	hi := after.Add(time.Second)
	if got.TsMs.Before(lo) || got.TsMs.After(hi) {
		t.Errorf("auto-filled TsMs out of range: got %s, want in [%s, %s]",
			got.TsMs, lo, hi)
	}
}

func TestParseAlertEvent_UnknownSchemaVer(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseAlertEvent(map[string]any{
		"schema_ver":     "999",
		"alert_event_id": "ae_1",
		"monitor_id":     "mon_1",
		"kind":           "down",
	})
	if err == nil {
		t.Fatal("expected error for future schema_ver")
	}
	if !errors.Is(err, contracts.ErrUnknownSchemaVer) {
		t.Errorf("err should wrap ErrUnknownSchemaVer, got: %v", err)
	}
}

func TestParseAlertEvent_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		vals        map[string]any
		wantSubstr  string
	}{
		{
			name: "missing alert_event_id",
			vals: map[string]any{
				"monitor_id": "mon_1",
				"kind":       "down",
			},
			wantSubstr: "alert_event_id is required",
		},
		{
			name: "missing monitor_id",
			vals: map[string]any{
				"alert_event_id": "ae_1",
				"kind":           "down",
			},
			wantSubstr: "monitor_id is required",
		},
		{
			name: "missing kind",
			vals: map[string]any{
				"alert_event_id": "ae_1",
				"monitor_id":     "mon_1",
			},
			wantSubstr: "kind is required",
		},
		{
			name: "empty alert_event_id",
			vals: map[string]any{
				"alert_event_id": "",
				"monitor_id":     "mon_1",
				"kind":           "down",
			},
			wantSubstr: "alert_event_id is required",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := contracts.ParseAlertEvent(tc.vals)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("err = %q, want substring %q", err, tc.wantSubstr)
			}
		})
	}
}

func TestParseAlertEvent_BadTimestampType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ts   any
	}{
		{"non-numeric string", "not-a-number"},
		{"slice", []string{"100"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := contracts.ParseAlertEvent(map[string]any{
				"alert_event_id": "ae_1",
				"monitor_id":     "mon_1",
				"kind":           "down",
				"ts":             tc.ts,
			})
			if err == nil {
				t.Fatalf("expected error for ts=%v (%T)", tc.ts, tc.ts)
			}
			if !strings.Contains(err.Error(), "ts") {
				t.Errorf("err = %q, want substring 'ts'", err)
			}
		})
	}
}

func TestParseAlertEvent_LegacyMessage(t *testing.T) {
	// schema_ver missing → treated as legacy (SchemaVer=0), must still parse.
	t.Parallel()
	got, err := contracts.ParseAlertEvent(map[string]any{
		"alert_event_id": "ae_1",
		"monitor_id":     "mon_1",
		"kind":           "down",
		"ts":             int64(1747800000000),
	})
	if err != nil {
		t.Fatalf("legacy parse failed: %v", err)
	}
	if got.SchemaVer != 0 {
		t.Errorf("legacy SchemaVer: got %d, want 0", got.SchemaVer)
	}
	if got.AlertEventID != "ae_1" || got.MonitorID != "mon_1" || got.Kind != "down" {
		t.Errorf("legacy fields drift: %+v", got)
	}
	if got.TsMs.UnixMilli() != 1747800000000 {
		t.Errorf("legacy TsMs: got %d, want 1747800000000", got.TsMs.UnixMilli())
	}
}

func TestParseAlertEvent_SchemaVerTypes(t *testing.T) {
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
			t.Parallel()
			got, err := contracts.ParseAlertEvent(map[string]any{
				"schema_ver":     tc.in,
				"alert_event_id": "ae_1",
				"monitor_id":     "mon_1",
				"kind":           "down",
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

func TestParseAlertEvent_SchemaVerBadString(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseAlertEvent(map[string]any{
		"schema_ver":     "not-a-number",
		"alert_event_id": "ae_1",
		"monitor_id":     "mon_1",
		"kind":           "down",
	})
	if err == nil {
		t.Fatal("expected error for invalid schema_ver string")
	}
}

func TestParseAlertEvent_SchemaVerUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseAlertEvent(map[string]any{
		"schema_ver":     []byte("1"),
		"alert_event_id": "ae_1",
		"monitor_id":     "mon_1",
		"kind":           "down",
	})
	if err == nil {
		t.Fatal("expected error for []byte schema_ver")
	}
}

func TestParseAlertEvent_MissingTsLeavesZero(t *testing.T) {
	// ts missing is lenient at parse time — caller decides whether to fill.
	t.Parallel()
	got, err := contracts.ParseAlertEvent(map[string]any{
		"alert_event_id": "ae_1",
		"monitor_id":     "mon_1",
		"kind":           "down",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.TsMs.IsZero() {
		t.Errorf("expected TsMs zero when ts missing, got %s", got.TsMs)
	}
}

func TestAlertEvent_ExplicitSchemaVerPreserved(t *testing.T) {
	t.Parallel()
	e := contracts.AlertEvent{
		SchemaVer:    contracts.AlertEventSchemaV1,
		AlertEventID: "ae_1",
		MonitorID:    "mon_1",
		Kind:         "down",
	}
	vals := e.ToStreamValues()
	if got := vals["schema_ver"]; got != "1" {
		t.Errorf("explicit schema_ver: got %v, want '1'", got)
	}
}
