package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kite365/idcd/lib/shared/apperr"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       any
		requestID  string
		wantStatus int
	}{
		{
			name:       "successful response with data",
			status:     http.StatusOK,
			data:       map[string]string{"message": "success"},
			requestID:  "req_test123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "created response with complex data",
			status:     http.StatusCreated,
			data:       struct{ ID string }{"user_123"},
			requestID:  "req_test456",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "response with nil data",
			status:     http.StatusNoContent,
			data:       nil,
			requestID:  "req_test789",
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)

			// Set request ID in context
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), "request_id", tt.requestID)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()

			JSON(rr, req, tt.status, tt.data)

			// Check status code
			if rr.Code != tt.wantStatus {
				t.Errorf("expected status code %d, got %d", tt.wantStatus, rr.Code)
			}

			// 204 No Content has no body — skip body checks.
			if tt.wantStatus == http.StatusNoContent {
				if rr.Body.Len() != 0 {
					t.Errorf("expected empty body for 204, got %q", rr.Body.String())
				}
				return
			}

			// Check content type
			expectedContentType := "application/json"
			if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
				t.Errorf("expected Content-Type %q, got %q", expectedContentType, ct)
			}

			// Parse response
			var response SuccessResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse JSON response: %v", err)
			}

			// Check request ID
			if response.RequestID != tt.requestID {
				t.Errorf("expected request ID %q, got %q", tt.requestID, response.RequestID)
			}

			// Check data (compare JSON representations for simplicity)
			expectedDataJSON, _ := json.Marshal(tt.data)
			actualDataJSON, _ := json.Marshal(response.Data)
			if string(expectedDataJSON) != string(actualDataJSON) {
				t.Errorf("expected data %s, got %s", expectedDataJSON, actualDataJSON)
			}
		})
	}
}

func TestJSONWithoutRequestID(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	data := map[string]string{"test": "value"}
	JSON(rr, req, http.StatusOK, data)

	var response SuccessResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	// Should default to "unknown" when no request ID
	if response.RequestID != "unknown" {
		t.Errorf("expected request ID %q when none set, got %q", "unknown", response.RequestID)
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		requestID      string
		expectedStatus int
		expectedCode   string
		expectedMsg    string
	}{
		{
			name:           "not found error",
			err:            apperr.NotFound("user not found"),
			requestID:      "req_test123",
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
			expectedMsg:    "user not found",
		},
		{
			name:           "validation error",
			err:            apperr.Validation("invalid email", ""),
			requestID:      "req_test456",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "VALIDATION",
			expectedMsg:    "invalid email",
		},
		{
			name:           "unauthorized error",
			err:            apperr.Unauthorized("token expired"),
			requestID:      "req_test789",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
			expectedMsg:    "token expired",
		},
		{
			name:           "rate limit error",
			err:            apperr.RateLimit("too many requests"),
			requestID:      "req_test999",
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "RATE_LIMIT",
			expectedMsg:    "too many requests",
		},
		{
			name:           "generic error fallback",
			err:            apperr.Internal("database error", nil),
			requestID:      "req_test111",
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL",
			expectedMsg:    "database error",
		},
		{
			name:           "non-apperr error fallback",
			err:            errors.New("some random error"),
			requestID:      "req_test222",
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL",
			expectedMsg:    "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)

			// Set request ID in context
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), "request_id", tt.requestID)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()

			Error(rr, req, tt.err)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status code %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check content type
			expectedContentType := "application/json"
			if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
				t.Errorf("expected Content-Type %q, got %q", expectedContentType, ct)
			}

			// Parse response
			var response ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse JSON response: %v", err)
			}

			// Check request ID
			if response.RequestID != tt.requestID {
				t.Errorf("expected request ID %q, got %q", tt.requestID, response.RequestID)
			}

			// Check error details
			if response.Error.Code != tt.expectedCode {
				t.Errorf("expected error code %q, got %q", tt.expectedCode, response.Error.Code)
			}

			if response.Error.Message != tt.expectedMsg {
				t.Errorf("expected error message %q, got %q", tt.expectedMsg, response.Error.Message)
			}

			if response.Error.RequestID != tt.requestID {
				t.Errorf("expected error request ID %q, got %q", tt.requestID, response.Error.RequestID)
			}
		})
	}
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name          string
		contextValue  string
		headerValue   string
		expectedValue string
	}{
		{
			name:          "from context",
			contextValue:  "req_from_context",
			headerValue:   "req_from_header",
			expectedValue: "req_from_context",
		},
		{
			name:          "from header when context empty",
			contextValue:  "",
			headerValue:   "req_from_header",
			expectedValue: "req_from_header",
		},
		{
			name:          "unknown when both empty",
			contextValue:  "",
			headerValue:   "",
			expectedValue: "unknown",
		},
		{
			name:          "context takes precedence",
			contextValue:  "req_context",
			headerValue:   "req_header",
			expectedValue: "req_context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)

			if tt.contextValue != "" {
				ctx := context.WithValue(req.Context(), "request_id", tt.contextValue)
				req = req.WithContext(ctx)
			}

			if tt.headerValue != "" {
				req.Header.Set("X-Request-ID", tt.headerValue)
			}

			actual := getRequestID(req)
			if actual != tt.expectedValue {
				t.Errorf("expected request ID %q, got %q", tt.expectedValue, actual)
			}
		})
	}
}