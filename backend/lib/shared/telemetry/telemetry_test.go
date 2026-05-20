package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInit_Disabled(t *testing.T) {
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Enabled:        false,
	}

	shutdown, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer shutdown(context.Background())

	// Should succeed without error when disabled
}

func TestInit_Enabled(t *testing.T) {
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		OTLPEndpoint:   "", // stdout exporter
		SamplingRate:   1.0, // 100% sampling for tests
		Enabled:        true,
	}

	shutdown, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer shutdown(context.Background())

	// Should succeed with stdout exporter
}

func TestInit_DefaultSamplingRate(t *testing.T) {
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		SamplingRate:   0, // should default to 0.1
		Enabled:        true,
	}

	shutdown, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer shutdown(context.Background())
}

func TestTraceMiddleware(t *testing.T) {
	// Initialize telemetry for test
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		SamplingRate:   1.0,
		Enabled:        true,
	}
	shutdown, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer shutdown(context.Background())

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with trace middleware
	middleware := TraceMiddleware("test-service")
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rr, req)

	// Verify response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := "test response"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestTraceMiddleware_ErrorStatus(t *testing.T) {
	// Initialize telemetry for test
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		SamplingRate:   1.0,
		Enabled:        true,
	}
	shutdown, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer shutdown(context.Background())

	// Create test handler that returns 500
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	// Wrap with trace middleware
	middleware := TraceMiddleware("test-service")
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/error", nil)
	rr := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rr, req)

	// Verify response
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)

	if rw.statusCode != http.StatusCreated {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusCreated)
	}

	// Second WriteHeader should be ignored
	rw.WriteHeader(http.StatusBadRequest)
	if rw.statusCode != http.StatusCreated {
		t.Errorf("statusCode = %d, want %d (should not change)", rw.statusCode, http.StatusCreated)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	data := []byte("test data")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d bytes, want %d", n, len(data))
	}

	// Should have written default status code
	if !rw.written {
		t.Error("written flag should be true after Write")
	}
}
