package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/kite365/idcd/packages/shared/logger"
)

// Recover middleware catches panics and returns a 500 error response.
// It logs the panic with stack trace for debugging.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Get request-aware logger
					requestLog := logger.FromContext(r.Context(), log)

					// Log the panic with stack trace
					requestLog.Error("HTTP handler panic",
						"error", err,
						"stack", string(debug.Stack()),
						"method", r.Method,
						"path", r.URL.Path,
						"user_agent", r.Header.Get("User-Agent"),
						"remote_addr", r.RemoteAddr,
					)

					// Check if response has already been written
					if w.Header().Get("Content-Type") == "" {
						// Write error response directly to avoid circular dependency
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)

						// Get request ID
						requestID := "unknown"
						if val := r.Context().Value("request_id"); val != nil {
							if id, ok := val.(string); ok && id != "" {
								requestID = id
							}
						}

						// Write minimal error response
						errorResponse := `{"error":{"code":"INTERNAL","message":"Internal server error","request_id":"` + requestID + `"},"request_id":"` + requestID + `"}`
						w.Write([]byte(errorResponse))
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}