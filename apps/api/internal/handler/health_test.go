package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestHealthHandler_Health(t *testing.T) {
	// Create handler (db and redis not used for basic health check)
	handler := NewHealthHandler(nil, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	// Set request ID in context for response package
	ctx := context.WithValue(req.Context(), "request_id", "test-req-123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
		t.Logf("Response body: %s", rr.Body.String())
	}

	// Check content type
	expectedContentType := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
		t.Errorf("expected Content-Type %q, got %q", expectedContentType, ct)
	}

	// Parse the unified response format
	var response struct {
		Data struct {
			Status  string `json:"status"`
			Version string `json:"version"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v\nBody: %s", err, rr.Body.String())
	}

	// Verify response structure
	if response.Data.Status != "ok" {
		t.Errorf("expected status %q, got %q", "ok", response.Data.Status)
	}

	if response.Data.Version == "" {
		t.Error("expected version to be set")
	}

	if response.RequestID != "test-req-123" {
		t.Errorf("expected request ID %q, got %q", "test-req-123", response.RequestID)
	}
}

func TestHealthHandler_DeepHealth_NilDB(t *testing.T) {
	// Test with nil database
	handler := NewHealthHandler(nil, nil)

	req := httptest.NewRequest("GET", "/health/deep", nil)
	// Set request ID in context for response package
	ctx := context.WithValue(req.Context(), "request_id", "test-req-456")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.DeepHealth(rr, req)

	// Should return 503 because both db and redis are nil
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status code %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	// Parse the unified response format
	var response struct {
		Data struct {
			Status  string                 `json:"status"`
			Version string                 `json:"version"`
			Checks  map[string]CheckStatus `json:"checks"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v\nBody: %s", err, rr.Body.String())
	}

	// Verify degraded status
	if response.Data.Status != "degraded" {
		t.Errorf("expected status %q, got %q", "degraded", response.Data.Status)
	}

	// Verify error messages
	if pgCheck, exists := response.Data.Checks["postgres"]; exists {
		if pgCheck.Status != "error" {
			t.Errorf("expected postgres status %q, got %q", "error", pgCheck.Status)
		}
		if pgCheck.Error == "" {
			t.Error("expected postgres error message to be set")
		}
	} else {
		t.Error("expected postgres check to be present")
	}

	if redisCheck, exists := response.Data.Checks["redis"]; exists {
		if redisCheck.Status != "error" {
			t.Errorf("expected redis status %q, got %q", "error", redisCheck.Status)
		}
		if redisCheck.Error == "" {
			t.Error("expected redis error message to be set")
		}
	} else {
		t.Error("expected redis check to be present")
	}
}

func TestCheckPostgreSQL(t *testing.T) {
	tests := []struct {
		name        string
		db          *sql.DB
		expectedStatus string
		expectError bool
	}{
		{
			name:           "nil database",
			db:             nil,
			expectedStatus: "error",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &HealthHandler{db: tt.db}

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			result := handler.checkPostgreSQL(ctx)

			if result.Status != tt.expectedStatus {
				t.Errorf("expected status %q, got %q", tt.expectedStatus, result.Status)
			}

			if tt.expectError && result.Error == "" {
				t.Error("expected error message to be set")
			}

			if !tt.expectError && result.Error != "" {
				t.Errorf("unexpected error message: %q", result.Error)
			}
		})
	}
}

func TestCheckRedis(t *testing.T) {
	tests := []struct {
		name           string
		redis          *redis.Client
		expectedStatus string
		expectError    bool
	}{
		{
			name:           "nil redis client",
			redis:          nil,
			expectedStatus: "error",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &HealthHandler{redis: tt.redis}

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			result := handler.checkRedis(ctx)

			if result.Status != tt.expectedStatus {
				t.Errorf("expected status %q, got %q", tt.expectedStatus, result.Status)
			}

			if tt.expectError && result.Error == "" {
				t.Error("expected error message to be set")
			}

			if !tt.expectError && result.Error != "" {
				t.Errorf("unexpected error message: %q", result.Error)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	handler := &HealthHandler{}

	version := handler.getVersion()

	// Version should be a non-empty string (either from file or "unknown")
	if version == "" {
		t.Error("getVersion should return a non-empty string")
	}

	// Should return a reasonable version string
	if len(version) > 100 {
		t.Errorf("version string seems too long: %q", version)
	}
}

func TestTrimWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{" hello ", "hello"},
		{"  hello  ", "hello"},
		{"\nhello\n", "hello"},
		{"\t\rhello\t\r", "hello"},
		{"", ""},
		{"   ", ""},
		{" \n\t ", ""},
	}

	for _, tt := range tests {
		actual := trimWhitespace(tt.input)
		if actual != tt.expected {
			t.Errorf("trimWhitespace(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}