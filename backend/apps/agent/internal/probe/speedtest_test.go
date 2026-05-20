package probe

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSpeedtestProbe_Success verifies that a working download+upload server
// produces a successful result with non-zero Mbps values.
func TestSpeedtestProbe_Success(t *testing.T) {
	const bodySize = 512 * 1024 // 512 KiB — fast in CI

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusPartialContent)
			buf := make([]byte, bodySize)
			_, _ = w.Write(buf)
		case http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	p := &SpeedtestProbe{}
	result := p.Execute(srv.URL, 10*time.Second, map[string]any{
		"download_bytes": int64(bodySize),
	})

	if result == nil {
		t.Fatal("Execute must never return nil")
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Type != TaskSpeedtest {
		t.Errorf("expected type %q, got %q", TaskSpeedtest, result.Type)
	}

	dlMbps, ok := result.Data["download_mbps"].(float64)
	if !ok || dlMbps <= 0 {
		t.Errorf("expected positive download_mbps, got %v", result.Data["download_mbps"])
	}
	ulMbps, ok := result.Data["upload_mbps"].(float64)
	if !ok || ulMbps <= 0 {
		t.Errorf("expected positive upload_mbps, got %v", result.Data["upload_mbps"])
	}
	if result.Data["latency_ms"] == nil {
		t.Error("expected latency_ms in result data")
	}
	if result.Data["server"] == nil {
		t.Error("expected server in result data")
	}
}

// TestSpeedtestProbe_UploadFails verifies that when the server rejects POST
// the result is still Success=true (download succeeded) and upload_mbps stays 0.
func TestSpeedtestProbe_UploadFails(t *testing.T) {
	const bodySize = 128 * 1024

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			buf := make([]byte, bodySize)
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(buf)
		case http.MethodPost:
			// Simulate server refusing uploads.
			http.Error(w, "not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	p := &SpeedtestProbe{}
	result := p.Execute(srv.URL, 10*time.Second, map[string]any{
		"download_bytes": int64(bodySize),
	})

	if !result.Success {
		t.Errorf("download succeeded so overall Success must be true, got error: %s", result.Error)
	}
	ulMbps, _ := result.Data["upload_mbps"].(float64)
	if ulMbps != 0 {
		t.Errorf("expected upload_mbps=0 on failed upload, got %f", ulMbps)
	}
}

// TestSpeedtestProbe_Timeout verifies that the probe honours the timeout and
// returns Success=false rather than hanging.
func TestSpeedtestProbe_Timeout(t *testing.T) {
	// Use a channel so the handler goroutine exits cleanly when the server closes,
	// preventing httptest.Server.Close() from blocking on active connections.
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the test is done (connection will be closed by the server).
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer func() {
		close(done)
		srv.CloseClientConnections()
		srv.Close()
	}()

	p := &SpeedtestProbe{}
	result := p.Execute(srv.URL, 50*time.Millisecond, nil)

	if result.Success {
		t.Error("expected failure on timeout")
	}
	if result.Error == "" {
		t.Error("expected non-empty Error on timeout")
	}
}

// TestSpeedtestProbe_InvalidTarget verifies that a completely invalid target
// returns Success=false with a descriptive error.
func TestSpeedtestProbe_InvalidTarget(t *testing.T) {
	p := &SpeedtestProbe{}
	result := p.Execute("://not-a-url", 5*time.Second, nil)

	if result.Success {
		t.Error("expected failure for invalid target")
	}
	if result.Error == "" {
		t.Error("expected non-empty Error for invalid target")
	}
}

// TestSpeedtestProbe_BareHostname verifies that a bare hostname gets https://
// prepended and then fails gracefully (no panic, no nil result).
func TestSpeedtestProbe_BareHostname(t *testing.T) {
	p := &SpeedtestProbe{}
	// Use a non-routable address so the dial fails predictably.
	result := p.Execute("192.0.2.1", 200*time.Millisecond, nil)

	if result == nil {
		t.Fatal("Execute must never return nil")
	}
	// We only care it does not panic and returns a result.
}

// TestGetInt64Option covers the helper function branches.
func TestGetInt64Option(t *testing.T) {
	opts := map[string]any{
		"int64val":   int64(100),
		"float64val": float64(200),
		"intval":     int(300),
		"wrongtype":  "string",
	}

	if v := getInt64Option(opts, "int64val", 0); v != 100 {
		t.Errorf("int64 branch: got %d", v)
	}
	if v := getInt64Option(opts, "float64val", 0); v != 200 {
		t.Errorf("float64 branch: got %d", v)
	}
	if v := getInt64Option(opts, "intval", 0); v != 300 {
		t.Errorf("int branch: got %d", v)
	}
	if v := getInt64Option(opts, "wrongtype", 42); v != 42 {
		t.Errorf("wrong type should return default: got %d", v)
	}
	if v := getInt64Option(opts, "missing", 99); v != 99 {
		t.Errorf("missing key should return default: got %d", v)
	}
}
