package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kite365/idcd/lib/ratelimit"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/apps/api/internal/response"
)

// RateLimitFunc is the interface for rate limiting functionality.
// This interface allows for easy mocking in tests.
type RateLimitFunc interface {
	Allow(ctx context.Context, key string) (*ratelimit.Result, error)
}

// RateLimitByUser creates a chi middleware that enforces rate limiting keyed
// on the authenticated user. Use this for endpoints where IP-based limiting is
// too lenient (e.g. TOTP verification — an attacker holding a stolen JWT can
// brute-force 6-digit codes against a single victim from anywhere).
//
// Must be applied AFTER the auth middleware so UserIDFromContext is populated.
// Requests without an authenticated user fall through unchecked because the
// auth middleware itself will already have rejected them.
func RateLimitByUser(limiter RateLimitFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())
			if userID == "" {
				next.ServeHTTP(w, r)
				return
			}

			result, err := limiter.Allow(r.Context(), "user:"+userID)
			if err != nil {
				// Fail open on limiter errors so a Redis blip does not lock
				// every user out of MFA flows.
				next.ServeHTTP(w, r)
				return
			}

			setRateLimitHeaders(w, result)
			if !result.Allowed {
				response.Error(w, r, apperr.RateLimit("Rate limit exceeded"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit creates a chi middleware that enforces rate limiting based on client IP.
// When rate limit is exceeded, it returns 429 Too Many Requests with appropriate headers.
func RateLimit(limiter RateLimitFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP
			clientIP := getClientIP(r)
			if clientIP == "" {
				// If we can't get client IP, allow the request to proceed
				// This prevents blocking legitimate requests due to proxy issues
				next.ServeHTTP(w, r)
				return
			}

			// Check rate limit
			result, err := limiter.Allow(r.Context(), ratelimit.KeyIP(clientIP))
			if err != nil {
				// On rate limiter error, log and allow request to proceed
				// This prevents service outage due to Redis issues
				// TODO: Add proper logging here
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			setRateLimitHeaders(w, result)

			if !result.Allowed {
				// Rate limit exceeded - return 429
				err := apperr.RateLimit("Rate limit exceeded")
				response.Error(w, r, err)
				return
			}

			// Request allowed - continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// setRateLimitHeaders sets standard rate limiting headers as per RFC 6585 and common practices.
func setRateLimitHeaders(w http.ResponseWriter, result *ratelimit.Result) {
	// X-RateLimit-Remaining: requests left in current window
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))

	// X-RateLimit-Reset: Unix timestamp when the rate limit resets
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

	// When rate limited, set Retry-After header (seconds until reset)
	if !result.Allowed {
		retryAfterSeconds := int64(time.Until(result.ResetAt).Seconds())
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1
		}
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
	}
}

// trustedProxyCIDRs are the networks from which X-Forwarded-For / X-Real-IP headers are trusted.
// Only loopback and RFC1918 ranges are trusted by default — direct Internet traffic never
// comes from these addresses, so spoofing via a forged header is not possible when the
// TCP connection itself originates from one of these ranges.
var trustedProxyCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 ULA
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, ipNet)
		}
	}
	return nets
}()

// isTrustedProxy reports whether the IP belongs to a trusted proxy range.
func isTrustedProxy(ip net.IP) bool {
	for _, n := range trustedProxyCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP is the exported entry point for extracting the real client IP.
// Use this instead of duplicating the logic in each handler.
func ClientIP(r *http.Request) string { return getClientIP(r) }

// getClientIP extracts the real client IP from the request.
// X-Forwarded-For and X-Real-IP are only trusted when the direct TCP connection
// comes from a trusted proxy (loopback / RFC1918). This prevents IP spoofing by
// external clients who set these headers themselves.
func getClientIP(r *http.Request) string {
	// Resolve the direct connection IP.
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	remoteIP := net.ParseIP(remoteHost)

	// Only honour proxy headers when the direct connection is from a trusted proxy.
	if remoteIP != nil && isTrustedProxy(remoteIP) {
		// X-Real-IP: set by nginx to the single real client IP.
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if ip := net.ParseIP(xri); ip != nil {
				return ip.String()
			}
		}

		// X-Forwarded-For: "client, proxy1, proxy2" — use the leftmost non-private IP.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for _, p := range parts {
				candidate := strings.TrimSpace(p)
				if ip := net.ParseIP(candidate); ip != nil && !isTrustedProxy(ip) {
					return ip.String()
				}
			}
		}
	}

	// Fall back to the direct connection address.
	if remoteIP != nil {
		return remoteIP.String()
	}
	return remoteHost
}