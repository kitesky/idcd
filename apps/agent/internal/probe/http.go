package probe

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
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

	// Track TTFB
	var ttfb time.Duration
	var redirectChain []string

	// Execute request
	resp, err := client.Do(req)
	ttfb = time.Since(start)

	data := map[string]any{
		"ttfb_ms":   ttfb.Milliseconds(),
		"total_ms":  time.Since(start).Milliseconds(),
	}

	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("request failed: %v", err),
			Data:       data,
			Timestamp:  start,
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	// Extract response data
	data["status_code"] = resp.StatusCode
	data["content_length"] = resp.ContentLength
	data["total_ms"] = time.Since(start).Milliseconds()

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
		DurationMs: time.Since(start).Milliseconds(),
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