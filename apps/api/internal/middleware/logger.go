package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/kite365/idcd/packages/shared/logger"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
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
				statusCode:     http.StatusOK,
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
				"status", rw.statusCode,
				"latency", latency.String(),
				"user_agent", r.Header.Get("User-Agent"),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}