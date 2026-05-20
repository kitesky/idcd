package probe

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Execute performs a TCP connection probe.
func (p *TCPProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	// Parse target (host:port)
	host, port, err := parseTarget(target, 80) // default port 80 if not specified
	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("parse target: %v", err),
			Data:       map[string]any{},
			Timestamp:  start,
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	// Attempt TCP connection
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)

	connectTime := time.Since(start)
	data := map[string]any{
		"connect_ms": connectTime.Milliseconds(),
		"host":       host,
		"port":       port,
	}

	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("connection failed: %v", err),
			Data:       data,
			Timestamp:  start,
			DurationMs: connectTime.Milliseconds(),
		}
	}

	// Close connection immediately after successful connect
	conn.Close()

	return &Result{
		Success:    true,
		Data:       data,
		Timestamp:  start,
		DurationMs: connectTime.Milliseconds(),
	}
}

// parseTarget parses host:port from target, using defaultPort if port is not specified.
func parseTarget(target string, defaultPort int) (host string, port int, err error) {
	if strings.Contains(target, ":") {
		host, portStr, err := net.SplitHostPort(target)
		if err != nil {
			return "", 0, fmt.Errorf("invalid host:port format: %w", err)
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}

		if port < 1 || port > 65535 {
			return "", 0, fmt.Errorf("port out of range: %d", port)
		}

		return host, port, nil
	}

	// No port specified, use default
	return target, defaultPort, nil
}