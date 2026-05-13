package probe

import (
	"net"
	"strconv"
	"testing"
	"time"
)

func TestTCPProbe_Execute(t *testing.T) {
	probe := &TCPProbe{}

	// Test successful TCP connection
	t.Run("successful connection", func(t *testing.T) {
		// Start a test server
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start test server: %v", err)
		}
		defer listener.Close()

		// Accept connections in background
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				conn.Close() // Immediately close to simulate successful connection
			}
		}()

		target := listener.Addr().String()
		result := probe.Execute(target, 5*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["connect_ms"] == nil {
			t.Error("Expected connect_ms field")
		}

		connectMs, ok := result.Data["connect_ms"].(int64)
		if !ok {
			t.Error("Expected connect_ms to be int64")
		}

		if connectMs < 0 {
			t.Error("Expected non-negative connect time")
		}

		// Check host and port extraction
		host, portStr, _ := net.SplitHostPort(target)
		port, _ := strconv.Atoi(portStr)

		if result.Data["host"] != host {
			t.Errorf("Expected host %s, got %v", host, result.Data["host"])
		}

		if result.Data["port"] != port {
			t.Errorf("Expected port %d, got %v", port, result.Data["port"])
		}
	})

	// Test connection refused
	t.Run("connection refused", func(t *testing.T) {
		// Use a port that's likely to be closed
		target := "127.0.0.1:1" // Port 1 is typically not in use

		result := probe.Execute(target, 1*time.Second, map[string]any{})

		if result.Success {
			t.Error("Expected failure for connection refused")
		}

		if result.Error == "" {
			t.Error("Expected error message for connection refused")
		}

		// Should still have timing and host/port info
		if result.Data["connect_ms"] == nil {
			t.Error("Expected connect_ms field even for failed connection")
		}

		if result.Data["host"] != "127.0.0.1" {
			t.Errorf("Expected host 127.0.0.1, got %v", result.Data["host"])
		}

		if result.Data["port"] != 1 {
			t.Errorf("Expected port 1, got %v", result.Data["port"])
		}
	})

	// Test timeout
	t.Run("connection timeout", func(t *testing.T) {
		// Use a non-routable address to trigger timeout
		target := "10.255.255.1:12345"

		result := probe.Execute(target, 100*time.Millisecond, map[string]any{})

		if result.Success {
			t.Error("Expected failure for timeout")
		}

		if result.Error == "" {
			t.Error("Expected error message for timeout")
		}
	})

	// Test target with default port
	t.Run("default port", func(t *testing.T) {
		// Start a test server
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start test server: %v", err)
		}
		defer listener.Close()

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}()

		// Use just the host part (should default to port 80)
		host, _, _ := net.SplitHostPort(listener.Addr().String())
		target := host // No port specified

		result := probe.Execute(target, 1*time.Second, map[string]any{})

		// This will likely fail since we're not listening on port 80,
		// but we're testing the port defaulting logic
		if result.Data["port"] != 80 {
			t.Errorf("Expected default port 80, got %v", result.Data["port"])
		}
	})
}

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		defaultPort int
		wantHost    string
		wantPort    int
		wantErr     bool
	}{
		{
			name:        "host and port",
			target:      "example.com:8080",
			defaultPort: 80,
			wantHost:    "example.com",
			wantPort:    8080,
			wantErr:     false,
		},
		{
			name:        "host only",
			target:      "example.com",
			defaultPort: 80,
			wantHost:    "example.com",
			wantPort:    80,
			wantErr:     false,
		},
		{
			name:        "IPv4 with port",
			target:      "192.168.1.1:443",
			defaultPort: 80,
			wantHost:    "192.168.1.1",
			wantPort:    443,
			wantErr:     false,
		},
		{
			name:        "IPv6 with port",
			target:      "[::1]:8080",
			defaultPort: 80,
			wantHost:    "::1",
			wantPort:    8080,
			wantErr:     false,
		},
		{
			name:        "invalid port",
			target:      "example.com:invalid",
			defaultPort: 80,
			wantErr:     true,
		},
		{
			name:        "port out of range high",
			target:      "example.com:99999",
			defaultPort: 80,
			wantErr:     true,
		},
		{
			name:        "port out of range low",
			target:      "example.com:0",
			defaultPort: 80,
			wantErr:     true,
		},
		{
			name:        "malformed host:port",
			target:      "example.com:8080:extra",
			defaultPort: 80,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseTarget(tt.target, tt.defaultPort)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if host != tt.wantHost {
				t.Errorf("Expected host %s, got %s", tt.wantHost, host)
			}

			if port != tt.wantPort {
				t.Errorf("Expected port %d, got %d", tt.wantPort, port)
			}
		})
	}
}

func TestTCPProbe_ExecuteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	probe := &TCPProbe{}

	// Test connecting to a real service (DNS on port 53)
	t.Run("real service", func(t *testing.T) {
		result := probe.Execute("8.8.8.8:53", 5*time.Second, map[string]any{})

		if !result.Success {
			// This might fail in some network environments, so just log
			t.Logf("Could not connect to 8.8.8.8:53: %s", result.Error)
		} else {
			t.Logf("Successfully connected to 8.8.8.8:53 in %d ms", result.Data["connect_ms"])
		}
	})
}