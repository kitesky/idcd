package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kite365/idcd/packages/ratelimit"
	"github.com/kite365/idcd/packages/shared/apperr"
	"github.com/kite365/idcd/apps/api/internal/response"
)

// RateLimitFunc is the interface for rate limiting functionality.
// This interface allows for easy mocking in tests.
type RateLimitFunc interface {
	Allow(ctx context.Context, key string) (*ratelimit.Result, error)
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
		retryAfterSeconds := int64(result.ResetAt.Sub(time.Now()).Seconds())
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1 // Minimum 1 second
		}
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
	}
}

// getClientIP extracts the real client IP from the request.
// Handles X-Forwarded-For, X-Real-IP headers for proxy scenarios.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (standard proxy header)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// We want the first (leftmost) IP which is the original client
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])
		if net.ParseIP(clientIP) != nil {
			return clientIP
		}
	}

	// Check X-Real-IP header (commonly used by nginx)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" && net.ParseIP(xri) != nil {
		return xri
	}

	// Check X-Forwarded (less common)
	xf := r.Header.Get("X-Forwarded")
	if xf != "" {
		// Format: X-Forwarded: for=192.0.2.60;proto=http;by=203.0.113.43
		parts := strings.Split(xf, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "for=") {
				ip := strings.TrimPrefix(part, "for=")
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
		}
	}

	// Fall back to remote address from connection
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // Return as-is if split fails
	}
	return host
}