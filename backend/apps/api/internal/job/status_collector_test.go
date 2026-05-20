package job

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test the probe-classification rules end-to-end against a real httptest server.
// Each subtest configures one handler behavior and verifies (status, detail
// prefix) matches the spec laid out in status_collector.go.
func TestProbeOne(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		handler    http.HandlerFunc
		degradedMs int
		timeout    time.Duration
		wantStatus int16
		// detail is a free-form string used for logs; we just assert
		// it starts with the expected category prefix.
		wantPrefix string
	}{
		{
			name:       "2xx fast → operational",
			handler:    func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) },
			degradedMs: 1000,
			timeout:    2 * time.Second,
			wantStatus: StatusOperational,
			wantPrefix: "ok",
		},
		{
			name: "2xx slow → degraded",
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(60 * time.Millisecond)
				w.WriteHeader(200)
			},
			degradedMs: 1, // anything ≥ 1ms counts as slow
			timeout:    2 * time.Second,
			wantStatus: StatusDegraded,
			wantPrefix: "slow_",
		},
		{
			name:       "503 → outage",
			handler:    func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) },
			degradedMs: 1000,
			timeout:    2 * time.Second,
			wantStatus: StatusOutage,
			wantPrefix: "http_503",
		},
		{
			name: "timeout → outage",
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(200 * time.Millisecond)
				w.WriteHeader(200)
			},
			degradedMs: 1000,
			timeout:    50 * time.Millisecond,
			wantStatus: StatusOutage,
			wantPrefix: "transport:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(nil, Options{
				Timeout:    tc.timeout,
				DegradedMs: tc.degradedMs,
			})
			got, _, detail := c.probeOne(context.Background(), srv.URL)
			if got != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (detail=%q)", got, tc.wantStatus, detail)
			}
			if len(tc.wantPrefix) > 0 && len(detail) < len(tc.wantPrefix) {
				t.Fatalf("detail %q does not start with %q", detail, tc.wantPrefix)
			}
			if got := detail[:len(tc.wantPrefix)]; got != tc.wantPrefix {
				t.Fatalf("detail prefix: got %q, want %q (full=%q)", got, tc.wantPrefix, detail)
			}
		})
	}
}

// classifyNodeAge encodes the staleness thresholds used to color the
// node fleet bars on idcd.com/status. The boundaries are spec, not
// implementation detail — guard them.
func TestClassifyNodeAge(t *testing.T) {
	t.Parallel()
	now := time.Now()
	mk := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }

	cases := []struct {
		name       string
		lastSeen   *time.Time
		wantStatus int16
	}{
		{"NULL → outage", nil, StatusOutage},
		{"1m old → operational", mk(1 * time.Minute), StatusOperational},
		{"just under 5m → operational", mk(4*time.Minute + 59*time.Second), StatusOperational},
		{"7m old → degraded", mk(7 * time.Minute), StatusDegraded},
		{"just under 15m → degraded", mk(14*time.Minute + 59*time.Second), StatusDegraded},
		{"20m old → outage", mk(20 * time.Minute), StatusOutage},
		{"1h old → outage", mk(1 * time.Hour), StatusOutage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := classifyNodeAge(tc.lastSeen, now)
			if got != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", got, tc.wantStatus)
			}
		})
	}
}

// New() must apply defaults for zero-value Options — main wires
// Options straight from yaml which is full of zero values when fields
// are omitted by an operator.
func TestNewDefaults(t *testing.T) {
	t.Parallel()
	c := New(nil, Options{})
	if c.interval != defaultProbeInterval {
		t.Errorf("interval: got %v, want %v", c.interval, defaultProbeInterval)
	}
	if c.timeout != defaultProbeTimeout {
		t.Errorf("timeout: got %v, want %v", c.timeout, defaultProbeTimeout)
	}
	if c.degradedMs != defaultDegradedMs {
		t.Errorf("degradedMs: got %d, want %d", c.degradedMs, defaultDegradedMs)
	}
	if c.httpClient == nil {
		t.Error("httpClient must be non-nil")
	}
}
