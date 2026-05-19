package probe

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"time"
)

// Execute performs an HTTP/HTTPS probe.
func (p *HTTPProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	// Parse options
	method := getStringOption(options, "method", "GET")
	followRedirects := getBoolOption(options, "follow_redirects", true)
	headers := getMapOption(options, "headers")

	// Ensure target has scheme
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "https://" + target
	}

	insecureSkipVerify := getBoolOption(options, "insecure_skip_verify", false)

	// Create client with timeout
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !followRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= 5 { // max 5 redirects
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Create request
	req, err := http.NewRequest(method, target, nil)
	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("create request: %v", err),
			Data:       map[string]any{},
			Timestamp:  start,
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	// Add custom headers
	for key, value := range headers {
		if v, ok := value.(string); ok {
			req.Header.Set(key, v)
		}
	}

	// Set default User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "idcd-agent/1.0")
	}

	// httptrace breaks the request into named phases for the stacked-bar
	// timing chart in the UI (dns / connect / ssl / ttfb / download).
	// Accumulators because the same client.Do() may run multiple redirect
	// hops — each hop touches DNSStart/Done etc. exactly once.
	var (
		dnsMs, connectMs, tlsMs                  atomic.Int64
		dnsStart, connectStart, tlsStart         time.Time
	)
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				dnsMs.Add(time.Since(dnsStart).Milliseconds())
			}
		},
		ConnectStart: func(_, _ string) { connectStart = time.Now() },
		ConnectDone: func(_, _ string, _ error) {
			if !connectStart.IsZero() {
				connectMs.Add(time.Since(connectStart).Milliseconds())
			}
		},
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			if !tlsStart.IsZero() {
				tlsMs.Add(time.Since(tlsStart).Milliseconds())
			}
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Track TTFB + redirect
	var ttfb time.Duration
	var redirectChain []string

	// Execute request
	resp, err := client.Do(req)
	ttfb = time.Since(start)

	totalMsOnError := time.Since(start).Milliseconds()
	data := map[string]any{
		"dns_ms":     dnsMs.Load(),
		"connect_ms": connectMs.Load(),
		"ssl_ms":     tlsMs.Load(),
		"ttfb_ms":    ttfb.Milliseconds(),
		"total_ms":   totalMsOnError,
	}

	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("request failed: %v", err),
			Data:       data,
			Timestamp:  start,
			DurationMs: totalMsOnError,
		}
	}
	defer resp.Body.Close()

	// Read body so download_ms is meaningful — discard the bytes, callers
	// don't need them. Use a bounded reader to avoid pulling multi-GB
	// responses into memory just to time them.
	var downloadedBytes int64
	if resp.Body != nil {
		downloadedBytes, _ = copyAndDiscard(resp.Body, 16*1024*1024) // 16 MiB cap
	}
	totalMs := time.Since(start).Milliseconds()
	downloadMs := totalMs - ttfb.Milliseconds()
	if downloadMs < 0 {
		downloadMs = 0
	}

	// server_ms is the pure server-processing slice of ttfb: the time we
	// waited for the first byte *after* the network setup (dns + connect
	// + ssl) had already finished. This is what the UI stacked bar shows
	// as "首包" — stacking ttfb_ms directly would double-count the
	// network phases (ttfb wraps them by definition).
	serverMs := ttfb.Milliseconds() - dnsMs.Load() - connectMs.Load() - tlsMs.Load()
	if serverMs < 0 {
		serverMs = 0
	}

	// Extract response data
	data["server_ms"] = serverMs
	data["status_code"] = resp.StatusCode
	data["content_length"] = resp.ContentLength
	data["downloaded_bytes"] = downloadedBytes
	data["download_ms"] = downloadMs
	data["total_ms"] = totalMs

	// Extract TLS info if HTTPS
	if resp.TLS != nil {
		data["tls_version"] = getTLSVersion(resp.TLS.Version)
		if len(resp.TLS.PeerCertificates) > 0 {
			cert := resp.TLS.PeerCertificates[0]
			data["cert_expires_at"] = cert.NotAfter.Unix()
		}
	}

	// Track redirect chain
	if resp.Request.URL.String() != target {
		redirectChain = append(redirectChain, resp.Request.URL.String())
		data["redirect_chain"] = redirectChain
	}

	// Consider 2xx and 3xx as success for HTTP probes
	success := resp.StatusCode >= 200 && resp.StatusCode < 400

	return &Result{
		Success:    success,
		Data:       data,
		Timestamp:  start,
		DurationMs: totalMs,
	}
}

// copyAndDiscard reads up to `cap` bytes from r and discards them, returning
// the number of bytes consumed. We don't import io.Discard via io.Copy here
// because we want a hard memory cap (huge payload defenses).
func copyAndDiscard(r interface{ Read([]byte) (int, error) }, cap int64) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := r.Read(buf)
		total += int64(n)
		if total > cap {
			return total, nil
		}
		if err != nil {
			return total, nil
		}
	}
}

func getStringOption(options map[string]any, key, defaultValue string) string {
	if v, ok := options[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getBoolOption(options map[string]any, key string, defaultValue bool) bool {
	if v, ok := options[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func getMapOption(options map[string]any, key string) map[string]any {
	if v, ok := options[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return make(map[string]any)
}

func getTLSVersion(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (%d)", version)
	}
}