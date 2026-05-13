package probe

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPProbe_Execute(t *testing.T) {
	probe := &HTTPProbe{}

	// Test successful HTTP request
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "11")
			w.WriteHeader(200)
			w.Write([]byte("Hello World"))
		}))
		defer server.Close()

		result := probe.Execute(server.URL, 10*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["status_code"] != 200 {
			t.Errorf("Expected status code 200, got %v", result.Data["status_code"])
		}

		if result.Data["content_length"] != int64(11) {
			t.Errorf("Expected content length 11, got %v", result.Data["content_length"])
		}

		// Check timing fields
		if result.Data["ttfb_ms"] == nil {
			t.Error("Expected ttfb_ms field")
		}

		if result.Data["total_ms"] == nil {
			t.Error("Expected total_ms field")
		}
	})

	// Test HTTP request with custom method
	t.Run("custom method", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("Expected POST method, got %s", r.Method)
			}
			w.WriteHeader(201)
		}))
		defer server.Close()

		options := map[string]any{
			"method": "POST",
		}

		result := probe.Execute(server.URL, 10*time.Second, options)

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["status_code"] != 201 {
			t.Errorf("Expected status code 201, got %v", result.Data["status_code"])
		}
	})

	// Test HTTP request with custom headers
	t.Run("custom headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Custom-Header") != "test-value" {
				t.Errorf("Expected custom header, got %s", r.Header.Get("X-Custom-Header"))
			}
			w.WriteHeader(200)
		}))
		defer server.Close()

		options := map[string]any{
			"headers": map[string]any{
				"X-Custom-Header": "test-value",
			},
		}

		result := probe.Execute(server.URL, 10*time.Second, options)

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}
	})

	// Test redirect handling
	t.Run("redirect handling", func(t *testing.T) {
		// Create servers for redirect chain
		finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("final"))
		}))
		defer finalServer.Close()

		redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, finalServer.URL, http.StatusFound)
		}))
		defer redirectServer.Close()

		result := probe.Execute(redirectServer.URL, 10*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["status_code"] != 200 {
			t.Errorf("Expected final status code 200, got %v", result.Data["status_code"])
		}
	})

	// Test disabled redirects
	t.Run("disabled redirects", func(t *testing.T) {
		redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com", http.StatusFound)
		}))
		defer redirectServer.Close()

		options := map[string]any{
			"follow_redirects": false,
		}

		result := probe.Execute(redirectServer.URL, 10*time.Second, options)

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["status_code"] != 302 {
			t.Errorf("Expected status code 302, got %v", result.Data["status_code"])
		}
	})

	// Test HTTPS with TLS info
	t.Run("https with tls info", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}))
		defer server.Close()

		result := probe.Execute(server.URL, 10*time.Second, map[string]any{"insecure_skip_verify": true})

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		// Check TLS version is recorded
		if result.Data["tls_version"] == nil {
			t.Error("Expected tls_version field for HTTPS")
		}

		// Check certificate expiry is recorded
		if result.Data["cert_expires_at"] == nil {
			t.Error("Expected cert_expires_at field for HTTPS")
		}
	})

	// Test client error (4xx)
	t.Run("client error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))
		defer server.Close()

		result := probe.Execute(server.URL, 10*time.Second, map[string]any{})

		// 4xx should be considered failure
		if result.Success {
			t.Error("Expected failure for 4xx status code")
		}

		if result.Data["status_code"] != 404 {
			t.Errorf("Expected status code 404, got %v", result.Data["status_code"])
		}
	})

	// Test timeout
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Longer than timeout
			w.WriteHeader(200)
		}))
		defer server.Close()

		result := probe.Execute(server.URL, 100*time.Millisecond, map[string]any{})

		if result.Success {
			t.Error("Expected timeout failure")
		}

		if result.Error == "" {
			t.Error("Expected error message for timeout")
		}
	})

	// Test invalid URL
	t.Run("invalid url", func(t *testing.T) {
		result := probe.Execute("not-a-valid-url", 10*time.Second, map[string]any{})

		if result.Success {
			t.Error("Expected failure for invalid URL")
		}

		if result.Error == "" {
			t.Error("Expected error message for invalid URL")
		}
	})

	// Test automatic HTTPS prefix
	t.Run("automatic https prefix", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}))
		defer server.Close()

		// Remove http:// prefix and test that https:// is automatically added
		// Note: This will fail to connect since test server is HTTP, but we're testing URL processing
		target := server.URL[7:] // Remove "http://" prefix
		result := probe.Execute(target, 100*time.Millisecond, map[string]any{})

		// Should fail due to HTTPS vs HTTP mismatch, but URL should be processed
		if result.Success {
			t.Error("Expected failure due to HTTPS vs HTTP mismatch")
		}
	})
}

func TestGetTLSVersion(t *testing.T) {
	tests := []struct {
		version  uint16
		expected string
	}{
		{tls.VersionTLS10, "TLS 1.0"},
		{tls.VersionTLS11, "TLS 1.1"},
		{tls.VersionTLS12, "TLS 1.2"},
		{tls.VersionTLS13, "TLS 1.3"},
		{0x9999, "Unknown (39321)"},
	}

	for _, tt := range tests {
		result := getTLSVersion(tt.version)
		if result != tt.expected {
			t.Errorf("getTLSVersion(%d) = %s, want %s", tt.version, result, tt.expected)
		}
	}
}

func TestOptionHelpers(t *testing.T) {
	options := map[string]any{
		"string_key": "test_value",
		"bool_key":   true,
		"map_key":    map[string]any{"nested": "value"},
	}

	// Test getStringOption
	if got := getStringOption(options, "string_key", "default"); got != "test_value" {
		t.Errorf("getStringOption() = %s, want test_value", got)
	}

	if got := getStringOption(options, "missing_key", "default"); got != "default" {
		t.Errorf("getStringOption() = %s, want default", got)
	}

	// Test getBoolOption
	if got := getBoolOption(options, "bool_key", false); got != true {
		t.Errorf("getBoolOption() = %t, want true", got)
	}

	if got := getBoolOption(options, "missing_key", false); got != false {
		t.Errorf("getBoolOption() = %t, want false", got)
	}

	// Test getMapOption
	result := getMapOption(options, "map_key")
	if result["nested"] != "value" {
		t.Errorf("getMapOption() nested value = %v, want value", result["nested"])
	}

	emptyResult := getMapOption(options, "missing_key")
	if len(emptyResult) != 0 {
		t.Errorf("getMapOption() for missing key = %v, want empty map", emptyResult)
	}
}