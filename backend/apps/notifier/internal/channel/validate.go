package channel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kite365/idcd/lib/shared/netutil"
)

// blockedHosts lists hostnames that must never be used as webhook targets,
// regardless of whether they parse as IPs.
var blockedHosts = []string{
	"metadata.google.internal",
	"169.254.169.254",
	"fd00:ec2::254",
	"localhost",
}

// validateWebhookURL rejects URLs that could be used for SSRF attacks.
// Rules:
//  1. Scheme must be "https" (plain http is not allowed for production webhooks).
//  2. The resolved host must not be a loopback, link-local, private, or unspecified address.
//
// Returns a descriptive error if the URL is unsafe or malformed.
func validateWebhookURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("webhook url must not be empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("webhook url parse error: %w", err)
	}

	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("webhook url scheme must be https, got %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook url missing host")
	}

	// Reject bare IP literals that are in private/reserved ranges.
	if ip := net.ParseIP(host); ip != nil {
		if netutil.IsPrivateIP(ip) {
			return fmt.Errorf("webhook url resolves to a private/reserved address: %s", host)
		}
	}

	// Block well-known cloud-metadata hostnames by name as a defence-in-depth measure.
	lower := strings.ToLower(host)
	for _, b := range blockedHosts {
		if lower == b {
			return fmt.Errorf("webhook url host %q is blocked", host)
		}
	}

	return nil
}

// safeTransport is a shared *http.Transport that re-validates the resolved IP
// on every TCP dial, preventing DNS rebinding (TOCTOU) attacks. Shared across
// all channel instances to allow connection-pool reuse.
var safeTransport = newSafeTransport()

func newSafeTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("safe transport: invalid addr %q: %w", addr, err)
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return nil, fmt.Errorf("safe transport: non-IP address %q after dial resolution", host)
			}
			if netutil.IsPrivateIP(ip) {
				return nil, fmt.Errorf("safe transport: resolved address %s is private/reserved", host)
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
	}
}
