package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/kite365/idcd/lib/shared/logger"
)

// StatusRecorder wraps http.ResponseWriter to capture the response status code.
// Exported so that other packages (e.g., server metrics middleware) can reuse it.
type StatusRecorder struct {
	http.ResponseWriter
	StatusCode int
	written    bool
}

func (rw *StatusRecorder) WriteHeader(code int) {
	if !rw.written {
		rw.StatusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *StatusRecorder) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// responseWriter is kept as an alias for package-internal use.
type responseWriter = StatusRecorder

// sanitizeHeader strips control characters (newlines, ANSI escapes) from header values
// before writing them to structured logs, preventing log injection attacks.
func sanitizeHeader(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// Logger middleware logs HTTP requests with structured logging.
// It captures method, path, status code, latency, and includes request_id if available.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			rw := &responseWriter{
				ResponseWriter: w,
				StatusCode:     http.StatusOK,
			}

			// Extract request ID from context for logging
			requestLog := logger.FromContext(r.Context(), log)

			// Process request
			next.ServeHTTP(rw, r)

			// Log the completed request
			latency := time.Since(start)
			requestLog.Info("HTTP request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.StatusCode,
				"latency", latency.String(),
				"user_agent", sanitizeHeader(r.Header.Get("User-Agent")),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}