package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name           string
		existingHeader string
		wantPrefix     string
	}{
		{
			name:           "generates new request ID when none exists",
			existingHeader: "",
			wantPrefix:     "req_",
		},
		{
			name:           "preserves existing request ID",
			existingHeader: "existing-req-123",
			wantPrefix:     "existing-req-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that checks the request ID
			var receivedRequestID string
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check context value
				if val := r.Context().Value("request_id"); val != nil {
					if id, ok := val.(string); ok {
						receivedRequestID = id
					}
				}
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with RequestID middleware
			handler := RequestID()(testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.existingHeader != "" {
				req.Header.Set("X-Request-ID", tt.existingHeader)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(rr, req)

			// Verify response header is set
			responseHeader := rr.Header().Get("X-Request-ID")
			if responseHeader == "" {
				t.Error("X-Request-ID header not set in response")
			}

			// Verify context value was set
			if receivedRequestID == "" {
				t.Error("request ID not set in context")
			}

			// Verify the request ID format or value
			if tt.existingHeader != "" {
				// Should preserve existing ID
				if receivedRequestID != tt.existingHeader {
					t.Errorf("expected request ID %q, got %q", tt.existingHeader, receivedRequestID)
				}
				if responseHeader != tt.existingHeader {
					t.Errorf("expected response header %q, got %q", tt.existingHeader, responseHeader)
				}
			} else {
				// Should generate new ID with correct prefix
				if !strings.HasPrefix(receivedRequestID, tt.wantPrefix) {
					t.Errorf("expected request ID to start with %q, got %q", tt.wantPrefix, receivedRequestID)
				}
				if !strings.HasPrefix(responseHeader, tt.wantPrefix) {
					t.Errorf("expected response header to start with %q, got %q", tt.wantPrefix, responseHeader)
				}

				// Should be same in context and header
				if receivedRequestID != responseHeader {
					t.Errorf("request ID mismatch: context=%q, header=%q", receivedRequestID, responseHeader)
				}

				// Should be reasonable length (prefix + 12 chars)
				expectedLen := len(tt.wantPrefix) + 12
				if len(receivedRequestID) != expectedLen {
					t.Errorf("expected request ID length %d, got %d (ID: %q)", expectedLen, len(receivedRequestID), receivedRequestID)
				}
			}
		})
	}
}

// TestRequestIDUniqueness verifies that multiple requests get different IDs
func TestRequestIDUniqueness(t *testing.T) {
	var requestIDs []string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if val := r.Context().Value("request_id"); val != nil {
			if id, ok := val.(string); ok {
				requestIDs = append(requestIDs, id)
			}
		}
	})

	handler := RequestID()(testHandler)

	// Generate multiple request IDs
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	// Verify all IDs are unique
	if len(requestIDs) != 10 {
		t.Errorf("expected 10 request IDs, got %d", len(requestIDs))
		return
	}

	seenIDs := make(map[string]bool)
	for _, id := range requestIDs {
		if seenIDs[id] {
			t.Errorf("duplicate request ID found: %q", id)
		}
		seenIDs[id] = true
	}
}