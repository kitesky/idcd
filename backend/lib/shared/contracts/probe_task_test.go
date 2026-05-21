package contracts

import (
	"errors"
	"strings"
	"testing"
)

// epochPtr is a test helper for the *int64 Epoch field. The scheduler
// producer sets a real address; the api ad-hoc producer leaves Epoch nil.
func epochPtr(n int64) *int64 { return &n }

func TestProbeTask_ToStreamValues_RequiredOnly(t *testing.T) {
	pt := ProbeTask{TaskID: "task_001", Type: "ping", Target: "1.1.1.1"}
	v := pt.ToStreamValues()

	if got, want := v["schema_ver"], "1"; got != want {
		t.Errorf("schema_ver: got %v, want %v", got, want)
	}
	for k, want := range map[string]any{"task_id": "task_001", "type": "ping", "target": "1.1.1.1"} {
		if v[k] != want {
			t.Errorf("%s: got %v, want %v", k, v[k], want)
		}
	}
	for _, k := range []string{"node_id", "monitor_id", "priority", "params", "epoch",
		"target_normalized", "node_selection", "created_at"} {
		if _, present := v[k]; present {
			t.Errorf("optional field %q should be omitted when zero", k)
		}
	}
}

// TestProbeTask_ToStreamValues_ShapeMatrix covers the two real producer
// shapes side-by-side so a wire-format drift on either side fails loudly.
func TestProbeTask_ToStreamValues_ShapeMatrix(t *testing.T) {
	cases := []struct {
		name    string
		task    ProbeTask
		want    map[string]any
		omitted []string
	}{
		{
			name: "scheduler",
			task: ProbeTask{
				TaskID:     "task_002",
				Type:       "http",
				Target:     "https://x.io/",
				NodeID:     "node-sg",
				MonitorID:  "mon_42",
				Priority:   2,
				ParamsJSON: `{"timeout_ms":5000}`,
				Epoch:      epochPtr(7),
			},
			want: map[string]any{
				"node_id":    "node-sg",
				"monitor_id": "mon_42",
				"priority":   2,
				"params":     `{"timeout_ms":5000}`,
				"epoch":      "7", // decimal string per parseInt64 round-trip
			},
			omitted: []string{"target_normalized", "node_selection", "created_at"},
		},
		{
			name: "api_ad_hoc",
			task: ProbeTask{
				TaskID:            "task_003",
				Type:              "dns",
				Target:            "example.com",
				NodeID:            "node-hk",
				TargetNormalized:  "example.com.",
				NodeSelectionJSON: `["node-hk"]`,
				CreatedAt:         1729600000,
			},
			want: map[string]any{
				"target_normalized": "example.com.",
				"node_selection":    `["node-hk"]`,
				"created_at":        "1729600000",
			},
			omitted: []string{"epoch", "monitor_id", "priority"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.task.ToStreamValues()
			for k, want := range tc.want {
				if v[k] != want {
					t.Errorf("%s: got %v, want %v", k, v[k], want)
				}
			}
			for _, k := range tc.omitted {
				if _, ok := v[k]; ok {
					t.Errorf("field %q must be omitted but is present: %v", k, v[k])
				}
			}
		})
	}
}

func TestParseProbeTask_AcceptsBothWireShapes(t *testing.T) {
	cases := []struct {
		name        string
		vals        map[string]any
		wantEpoch   *int64
		wantPrio    int
		wantTNorm   string
		wantCreated int64
	}{
		{
			name: "scheduler wire (has epoch)",
			vals: map[string]any{
				"schema_ver": "1",
				"task_id":    "task_010",
				"type":       "ping",
				"target":     "1.0.0.1",
				"node_id":    "n1",
				"priority":   "2",
				"params":     `{"timeout_ms":3000}`,
				"epoch":      "5",
			},
			wantEpoch: epochPtr(5),
			wantPrio:  2,
		},
		{
			name: "api wire (no epoch)",
			vals: map[string]any{
				"schema_ver":        "1",
				"task_id":           "task_011",
				"type":              "http",
				"target":            "https://idcd.com",
				"target_normalized": "idcd.com",
				"node_selection":    `["a","b"]`,
				"created_at":        "1729600000",
				"node_id":           "a",
			},
			wantEpoch:   nil,
			wantTNorm:   "idcd.com",
			wantCreated: 1729600000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseProbeTask(tc.vals)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			switch {
			case tc.wantEpoch == nil && got.Epoch != nil:
				t.Errorf("epoch: got %d, want nil", *got.Epoch)
			case tc.wantEpoch != nil && got.Epoch == nil:
				t.Errorf("epoch: got nil, want %d", *tc.wantEpoch)
			case tc.wantEpoch != nil && *got.Epoch != *tc.wantEpoch:
				t.Errorf("epoch: got %d, want %d", *got.Epoch, *tc.wantEpoch)
			}
			if got.Priority != tc.wantPrio {
				t.Errorf("priority: got %d, want %d", got.Priority, tc.wantPrio)
			}
			if got.TargetNormalized != tc.wantTNorm {
				t.Errorf("target_normalized: got %q, want %q", got.TargetNormalized, tc.wantTNorm)
			}
			if got.CreatedAt != tc.wantCreated {
				t.Errorf("created_at: got %d, want %d", got.CreatedAt, tc.wantCreated)
			}
		})
	}
}

// TestParseProbeTask_LegacyMissingSchemaVer covers pre-W5 producers (no
// schema_ver). The required identity fields must still be enforced.
func TestParseProbeTask_LegacyMissingSchemaVer(t *testing.T) {
	got, err := ParseProbeTask(map[string]any{
		"task_id": "task_legacy",
		"type":    "ping",
		"target":  "8.8.8.8",
	})
	if err != nil {
		t.Fatalf("legacy parse: %v", err)
	}
	if got.SchemaVer != 0 {
		t.Errorf("legacy schema_ver: got %d, want 0", got.SchemaVer)
	}
}

func TestParseProbeTask_UnknownSchemaVer(t *testing.T) {
	_, err := ParseProbeTask(map[string]any{
		"schema_ver": "99",
		"task_id":    "x",
		"type":       "x",
		"target":     "x",
	})
	if !errors.Is(err, ErrUnknownSchemaVer) {
		t.Errorf("got %v, want ErrUnknownSchemaVer chain", err)
	}
}

func TestParseProbeTask_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		vals map[string]any
		want string
	}{
		{"no task_id", map[string]any{"type": "ping", "target": "x"}, "task_id is required"},
		{"no type", map[string]any{"task_id": "x", "target": "x"}, "type is required"},
		{"no target", map[string]any{"task_id": "x", "type": "ping"}, "target is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseProbeTask(tc.vals)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestParseProbeTask_BadEpoch(t *testing.T) {
	_, err := ParseProbeTask(map[string]any{
		"task_id": "x", "type": "ping", "target": "x",
		"epoch": "notanumber",
	})
	if err == nil || !strings.Contains(err.Error(), "epoch") {
		t.Errorf("got %v, want error mentioning epoch", err)
	}
}
