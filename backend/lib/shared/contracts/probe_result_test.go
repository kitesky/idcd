package contracts_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kite365/idcd/lib/shared/contracts"
)

func TestProbeResult_RoundTrip(t *testing.T) {
	t.Parallel()
	original := contracts.ProbeResult{
		SchemaVer:   contracts.ProbeResultSchemaV1,
		TaskID:      "pt_abc",
		NodeID:      "nd_jp_01",
		RawJSON:     `{"latency_ms":12}`,
		SummaryJSON: `{"summary":"ok"}`,
		DurationMs:  42,
		Success:     true,
		Error:       "",
		Signature:   "sig_xyz",
		MonitorID:   "m_001",
	}
	vals := original.ToStreamValues()
	got, err := contracts.ParseProbeResult(vals)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if got != original {
		t.Errorf("round-trip mismatch:\n  want %+v\n   got %+v", original, got)
	}
}

func TestProbeResult_AutoSchemaVer(t *testing.T) {
	t.Parallel()
	r := contracts.ProbeResult{TaskID: "pt_a", NodeID: "nd_a"}
	vals := r.ToStreamValues()
	if got := vals["schema_ver"]; got != "1" {
		t.Errorf("expected schema_ver auto-set to '1', got %v", got)
	}
}

func TestProbeResult_OmitEmpty(t *testing.T) {
	t.Parallel()
	r := contracts.ProbeResult{
		TaskID:     "pt_a",
		NodeID:     "nd_a",
		DurationMs: 10,
		Success:    true,
	}
	vals := r.ToStreamValues()
	for _, k := range []string{"raw", "summary", "error", "signature", "monitor_id"} {
		if _, present := vals[k]; present {
			t.Errorf("expected key %q to be omitted when zero-valued, but it was present", k)
		}
	}
	// required keys
	for _, k := range []string{"schema_ver", "task_id", "node_id", "duration_ms", "success"} {
		if _, present := vals[k]; !present {
			t.Errorf("expected required key %q to be present", k)
		}
	}
}

func TestParseProbeResult_RequiredFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		vals map[string]any
		want string
	}{
		{
			name: "missing task_id",
			vals: map[string]any{"node_id": "nd_a", "schema_ver": "1"},
			want: "task_id is required",
		},
		{
			name: "missing node_id",
			vals: map[string]any{"task_id": "pt_a", "schema_ver": "1"},
			want: "node_id is required",
		},
		{
			name: "empty task_id",
			vals: map[string]any{"task_id": "", "node_id": "nd_a"},
			want: "task_id is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := contracts.ParseProbeResult(tc.vals)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want contains %q", err, tc.want)
			}
		})
	}
}

func TestParseProbeResult_DurationMsTypeFlexibility(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"string", "123", 123},
		{"int", int(123), 123},
		{"int32", int32(123), 123},
		{"int64", int64(123), 123},
		{"float64", float64(123), 123},
		{"empty string", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := contracts.ParseProbeResult(map[string]any{
				"task_id":     "pt_a",
				"node_id":     "nd_a",
				"duration_ms": tc.in,
			})
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if r.DurationMs != tc.want {
				t.Errorf("DurationMs: got %d, want %d", r.DurationMs, tc.want)
			}
		})
	}
}

func TestParseProbeResult_DurationMsInvalid(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseProbeResult(map[string]any{
		"task_id":     "pt_a",
		"node_id":     "nd_a",
		"duration_ms": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid duration_ms")
	}
	if !strings.Contains(err.Error(), "duration_ms") {
		t.Errorf("err = %q, want contains 'duration_ms'", err)
	}
}

func TestParseProbeResult_SuccessTypeFlexibility(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want bool
	}{
		{`bool true`, true, true},
		{`bool false`, false, false},
		{`string "true"`, "true", true},
		{`string "false"`, "false", false},
		{`string "1"`, "1", true},
		{`string "0"`, "0", false},
		{`string "TRUE"`, "TRUE", true},
		{`int 1`, 1, true},
		{`int 0`, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := contracts.ParseProbeResult(map[string]any{
				"task_id": "pt_a",
				"node_id": "nd_a",
				"success": tc.in,
			})
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if r.Success != tc.want {
				t.Errorf("Success: got %v, want %v", r.Success, tc.want)
			}
		})
	}
}

func TestParseProbeResult_SuccessInvalid(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseProbeResult(map[string]any{
		"task_id": "pt_a",
		"node_id": "nd_a",
		"success": "maybe",
	})
	if err == nil {
		t.Fatal("expected error for invalid success value")
	}
}

func TestParseProbeResult_LegacyMessage(t *testing.T) {
	// schema_ver == 0 / missing → treated as legacy, must still parse.
	t.Parallel()
	r, err := contracts.ParseProbeResult(map[string]any{
		"task_id":     "pt_legacy",
		"node_id":     "nd_legacy",
		"duration_ms": "100",
		"success":     "true",
		"raw":         `{"a":1}`,
	})
	if err != nil {
		t.Fatalf("parse legacy failed: %v", err)
	}
	if r.SchemaVer != 0 {
		t.Errorf("legacy SchemaVer: got %d, want 0", r.SchemaVer)
	}
	if r.DurationMs != 100 {
		t.Errorf("DurationMs: got %d, want 100", r.DurationMs)
	}
}

func TestParseProbeResult_UnknownSchemaVer(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseProbeResult(map[string]any{
		"task_id":    "pt_a",
		"node_id":    "nd_a",
		"schema_ver": "99",
	})
	if err == nil {
		t.Fatal("expected error for future schema_ver")
	}
	if !errors.Is(err, contracts.ErrUnknownSchemaVer) {
		t.Errorf("err should wrap ErrUnknownSchemaVer, got: %v", err)
	}
}

func TestParseProbeResult_SchemaVerTypes(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			r, err := contracts.ParseProbeResult(map[string]any{
				"task_id":    "pt_a",
				"node_id":    "nd_a",
				"schema_ver": tc.in,
			})
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if r.SchemaVer != tc.want {
				t.Errorf("SchemaVer: got %d, want %d", r.SchemaVer, tc.want)
			}
		})
	}
}

func TestParseProbeResult_SchemaVerInvalidString(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseProbeResult(map[string]any{
		"task_id":    "pt_a",
		"node_id":    "nd_a",
		"schema_ver": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid schema_ver string")
	}
}

func TestParseProbeResult_SchemaVerUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseProbeResult(map[string]any{
		"task_id":    "pt_a",
		"node_id":    "nd_a",
		"schema_ver": []byte("1"),
	})
	if err == nil {
		t.Fatal("expected error for []byte schema_ver")
	}
}

func TestParseProbeResult_DurationMsUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseProbeResult(map[string]any{
		"task_id":     "pt_a",
		"node_id":     "nd_a",
		"duration_ms": []string{"100"},
	})
	if err == nil {
		t.Fatal("expected error for []string duration_ms")
	}
}

func TestParseProbeResult_SuccessUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseProbeResult(map[string]any{
		"task_id": "pt_a",
		"node_id": "nd_a",
		"success": []string{"true"},
	})
	if err == nil {
		t.Fatal("expected error for []string success")
	}
}

func TestProbeResult_ErrorFieldEncoded(t *testing.T) {
	t.Parallel()
	r := contracts.ProbeResult{
		TaskID:  "pt_a",
		NodeID:  "nd_a",
		Success: false,
		Error:   "dial timeout",
	}
	vals := r.ToStreamValues()
	if vals["error"] != "dial timeout" {
		t.Errorf("expected error field encoded, got %v", vals["error"])
	}
}

func TestProbeResult_BoolEncoding(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   bool
		want string
	}{
		{true, "true"},
		{false, "false"},
	}
	for _, tc := range cases {
		r := contracts.ProbeResult{TaskID: "pt_a", NodeID: "nd_a", Success: tc.in}
		vals := r.ToStreamValues()
		if got := vals["success"]; got != tc.want {
			t.Errorf("Success=%v encoded as %v, want %q", tc.in, got, tc.want)
		}
	}
}
