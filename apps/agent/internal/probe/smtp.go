package probe

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// Execute performs an SMTP banner + EHLO test.
func (p *SMTPProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	// Determine port from options or target
	port := "25"
	if opts, ok := options["port"].(string); ok && opts != "" {
		port = opts
	} else if strings.Contains(target, ":") {
		host, p2, err := net.SplitHostPort(target)
		if err == nil {
			target = host
			port = p2
		}
	}

	addr := net.JoinHostPort(target, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return &Result{
			Type:       TaskSMTP,
			Target:     target,
			Success:    false,
			Error:      err.Error(),
			Data:       map[string]any{"port": port},
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  time.Now(),
		}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	reader := bufio.NewReader(conn)

	// Read banner
	banner, err := reader.ReadString('\n')
	if err != nil {
		return &Result{
			Type:       TaskSMTP,
			Target:     target,
			Success:    false,
			Error:      "failed to read banner: " + err.Error(),
			Data:       map[string]any{"port": port},
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  time.Now(),
		}
	}
	banner = strings.TrimSpace(banner)

	// Send EHLO
	if _, err := fmt.Fprintf(conn, "EHLO idcd-probe\r\n"); err != nil {
		return &Result{
			Type:       TaskSMTP,
			Target:     target,
			Success:    false,
			Error:      fmt.Sprintf("failed to send EHLO: %v", err),
			Data:       map[string]any{"port": port},
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  time.Now(),
		}
	}
	ehloResp, err := reader.ReadString('\n')
	if err != nil {
		ehloResp = ""
	}
	ehloResp = strings.TrimSpace(ehloResp)

	// Send QUIT — best-effort, ignore errors
	_, _ = fmt.Fprintf(conn, "QUIT\r\n")

	durationMs := time.Since(start).Milliseconds()
	success := strings.HasPrefix(banner, "220")

	return &Result{
		Type:    TaskSMTP,
		Target:  target,
		Success: success,
		Data: map[string]any{
			"port":      port,
			"banner":    banner,
			"ehlo_resp": ehloResp,
		},
		DurationMs: durationMs,
		Timestamp:  time.Now(),
	}
}
