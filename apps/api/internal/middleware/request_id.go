// Package middleware provides HTTP middleware for the API Gateway.
package middleware

import (
	"context"
	"net/http"

	"github.com/kite365/idcd/packages/shared/idgen"
	"github.com/kite365/idcd/packages/shared/logger"
)

// RequestID middleware injects a unique request ID into each request.
// It generates a new ID using idgen.New("req") and stores it in both
// the request context and X-Request-ID header.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if request ID already exists in header
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				// Generate new request ID
				requestID = idgen.New("req_")
			}

			// Set response header
			w.Header().Set("X-Request-ID", requestID)

			// Store in context for downstream use
			ctx := context.WithValue(r.Context(), "request_id", requestID)
			ctx = logger.WithRequestID(ctx, requestID)

			// Continue with updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}