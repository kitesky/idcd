package contracts_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kite365/idcd/lib/shared/contracts"
)

func TestMonitorEvent_RoundTrip(t *testing.T) {
	t.Parallel()
	original := contracts.MonitorEvent{
		SchemaVer: contracts.MonitorEventSchemaV1,
		MonitorID: "m_001",
		Event:     "down",
		TsMs:      1716200000000,
		Severity:  "critical",
		Reason:    "timeout",
		ExtraJSON: []byte(`{"region":"sg","retries":3}`),
	}
	vals := original.ToStreamValues()
	got, err := contracts.ParseMonitorEvent(vals)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if got.MonitorID != original.MonitorID ||
		got.Event != original.Event ||
		got.TsMs != original.TsMs ||
		got.Severity != original.Severity ||
		got.Reason != original.Reason ||
		got.SchemaVer != original.SchemaVer {
		t.Errorf("round-trip scalar mismatch:\n  want %+v\n   got %+v", original, got)
	}
	if !bytes.Equal(got.ExtraJSON, original.ExtraJSON) {
		t.Errorf("ExtraJSON mismatch: got %s, want %s", got.ExtraJSON, original.ExtraJSON)
	}
}

func TestMonitorEvent_AutoSchemaVer(t *testing.T) {
	t.Parallel()
	e := contracts.MonitorEvent{MonitorID: "m_a", Event: "up"}
	vals := e.ToStreamValues()
	if got := vals["schema_ver"]; got != "1" {
		t.Errorf("expected schema_ver auto-set to '1', got %v", got)
	}
}

func TestMonitorEvent_AutoTs(t *testing.T) {
	t.Parallel()
	e := contracts.MonitorEvent{MonitorID: "m_a", Event: "up"}
	vals := e.ToStreamValues()
	tsRaw, ok := vals["ts"].(string)
	if !ok || tsRaw == "" {
		t.Fatalf("expected ts to be auto-set, got %v", vals["ts"])
	}
}

func TestMonitorEvent_OmitEmpty(t *testing.T) {
	t.Parallel()
	e := contracts.MonitorEvent{
		MonitorID: "m_a",
		Event:     "up",
		TsMs:      123,
	}
	vals := e.ToStreamValues()
	for _, k := range []string{"severity", "reason", "extra"} {
		if _, present := vals[k]; present {
			t.Errorf("expected key %q to be omitted when zero-valued", k)
		}
	}
}

func TestParseMonitorEvent_RequiredFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		vals map[string]any
		want string
	}{
		{
			name: "missing monitor_id",
			vals: map[string]any{"event": "up"},
			want: "monitor_id is required",
		},
		{
			name: "missing event",
			vals: map[string]any{"monitor_id": "m_a"},
			want: "event is required",
		},
		{
			name: "empty event",
			vals: map[string]any{"monitor_id": "m_a", "event": ""},
			want: "event is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := contracts.ParseMonitorEvent(tc.vals)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want contains %q", err, tc.want)
			}
		})
	}
}

func TestParseMonitorEvent_UnknownSchemaVer(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"schema_ver": "42",
	})
	if err == nil {
		t.Fatal("expected error for future schema_ver")
	}
	if !errors.Is(err, contracts.ErrUnknownSchemaVer) {
		t.Errorf("err should wrap ErrUnknownSchemaVer, got: %v", err)
	}
}

func TestParseMonitorEvent_LegacyMessage(t *testing.T) {
	t.Parallel()
	e, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_legacy",
		"event":      "down",
		"reason":     "timeout",
		// no schema_ver, no ts
	})
	if err != nil {
		t.Fatalf("parse legacy failed: %v", err)
	}
	if e.SchemaVer != 0 {
		t.Errorf("legacy SchemaVer: got %d, want 0", e.SchemaVer)
	}
	if e.Reason != "timeout" {
		t.Errorf("Reason: got %q, want %q", e.Reason, "timeout")
	}
}

func TestParseMonitorEvent_TsTypeFlexibility(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"string", "1716200000000", 1716200000000},
		{"int64", int64(1716200000000), 1716200000000},
		{"float64", float64(1716200000000), 1716200000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := contracts.ParseMonitorEvent(map[string]any{
				"monitor_id": "m_a",
				"event":      "up",
				"ts":         tc.in,
			})
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if e.TsMs != tc.want {
				t.Errorf("TsMs: got %d, want %d", e.TsMs, tc.want)
			}
		})
	}
}

func TestParseMonitorEvent_ExtraAsBytes(t *testing.T) {
	t.Parallel()
	e, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"extra":      []byte(`{"k":"v"}`),
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if string(e.ExtraJSON) != `{"k":"v"}` {
		t.Errorf("ExtraJSON: got %s", e.ExtraJSON)
	}
}

func TestParseMonitorEvent_ExtraInvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"extra":      `{not-json`,
	})
	if err == nil {
		t.Fatal("expected error for invalid extra JSON")
	}
}

func TestParseMonitorEvent_ExtraInvalidBytes(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"extra":      []byte(`{nope`),
	})
	if err == nil {
		t.Fatal("expected error for invalid extra bytes")
	}
}

func TestParseMonitorEvent_ExtraUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"extra":      42,
	})
	if err == nil {
		t.Fatal("expected error for unsupported extra type")
	}
}

func TestParseMonitorEvent_ExtraNilSkipped(t *testing.T) {
	t.Parallel()
	e, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"extra":      nil,
	})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if e.ExtraJSON != nil {
		t.Errorf("expected nil ExtraJSON, got %v", e.ExtraJSON)
	}
}

func TestParseMonitorEvent_SchemaVerInvalidString(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"schema_ver": "abc",
	})
	if err == nil {
		t.Fatal("expected error for invalid schema_ver string")
	}
}

func TestParseMonitorEvent_SchemaVerUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"schema_ver": []string{"1"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported schema_ver type")
	}
}

func TestParseMonitorEvent_TsInvalid(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseMonitorEvent(map[string]any{
		"monitor_id": "m_a",
		"event":      "up",
		"ts":         "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid ts")
	}
}

func TestParseMonitorEvent_SchemaVerNumericTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"int", int(1), 1},
		{"int64", int64(1), 1},
		{"float64", float64(1), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := contracts.ParseMonitorEvent(map[string]any{
				"monitor_id": "m_a",
				"event":      "up",
				"schema_ver": tc.in,
			})
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if e.SchemaVer != tc.want {
				t.Errorf("SchemaVer: got %d, want %d", e.SchemaVer, tc.want)
			}
		})
	}
}

func TestMonitorEvent_BoolHelpersCovered(t *testing.T) {
	// Cross-coverage: exercise parseBool's full type matrix via
	// ProbeResult.Success since MonitorEvent doesn't expose a bool.
	// (parseBool is package-private; tested transitively to keep
	//  coverage 100% on helpers.go.)
	t.Parallel()
	cases := []any{int64(1), float64(0)}
	for _, c := range cases {
		_, err := contracts.ParseProbeResult(map[string]any{
			"task_id": "pt_a",
			"node_id": "nd_a",
			"success": c,
		})
		if err != nil {
			t.Errorf("parseBool failed for %T(%v): %v", c, c, err)
		}
	}
}
