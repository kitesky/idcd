package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/packages/shared/stream"
)

// setupTestProbeHandler creates a ProbeHandler with mock dependencies.
func setupTestProbeHandler(t *testing.T) (*ProbeHandler, *miniredis.Miniredis, pgxmock.PgxPoolIface) {
	// Create miniredis for stream client
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	streamClient := stream.New(redisClient)

	// Create pgxmock for database
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}

	handler := NewProbeHandler(mockPool, streamClient)

	return handler, mr, mockPool
}

func TestProbeHandler_HTTP(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	// Mock the database insert
	mockPool.ExpectExec("INSERT INTO probe_task").
		WithArgs(
			pgxmock.AnyArg(), // id
			"http",           // type
			"example.com",    // target
			"example.com",    // target_normalized
			pgxmock.AnyArg(), // params
			pgxmock.AnyArg(), // initiated_by
			pgxmock.AnyArg(), // api_key_id
			pgxmock.AnyArg(), // client_ip
			pgxmock.AnyArg(), // user_agent
			pgxmock.AnyArg(), // node_selection
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	reqBody := ProbeRequest{
		Target: "example.com",
		Nodes:  []string{"nd_us_nyc_01_vultr"},
		Params: map[string]interface{}{"method": "GET"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/probe/http", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "request_id", "test-req-123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.HTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		t.Logf("Response body: %s", rr.Body.String())
	}

	var response struct {
		Data struct {
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Data.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", response.Data.Status)
	}

	if response.Data.TaskID == "" {
		t.Error("expected task_id to be set")
	}

	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProbeHandler_SSRF_Protection(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	privateIPs := []string{
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"127.0.0.1",
		"169.254.169.254", // AWS metadata endpoint
		"localhost",
	}

	for _, privateIP := range privateIPs {
		t.Run("reject_"+privateIP, func(t *testing.T) {
			reqBody := ProbeRequest{
				Target: privateIP,
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/v1/probe/http", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), "request_id", "test-req-ssrf")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()

			handler.HTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status %d for private IP %s, got %d", http.StatusBadRequest, privateIP, rr.Code)
				t.Logf("Response body: %s", rr.Body.String())
			}

			var response struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}

			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse error response: %v", err)
			}

			if response.Error.Code != "VALIDATION" {
				t.Errorf("expected error code VALIDATION, got %q", response.Error.Code)
			}
		})
	}
}

func TestProbeHandler_Ping(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	mockPool.ExpectExec("INSERT INTO probe_task").
		WithArgs(
			pgxmock.AnyArg(),
			"ping",
			"8.8.8.8",
			"8.8.8.8",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	reqBody := ProbeRequest{Target: "8.8.8.8"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/probe/ping", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "request_id", "test-req-ping")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.Ping(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestProbeHandler_TCP(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	mockPool.ExpectExec("INSERT INTO probe_task").
		WithArgs(
			pgxmock.AnyArg(),
			"tcp",
			"example.com:443",
			"example.com:443",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	reqBody := ProbeRequest{Target: "example.com:443"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/probe/tcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "request_id", "test-req-tcp")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.TCP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestProbeHandler_DNS(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	mockPool.ExpectExec("INSERT INTO probe_task").
		WithArgs(
			pgxmock.AnyArg(),
			"dns",
			"example.com",
			"example.com",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	reqBody := ProbeRequest{Target: "example.com"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/probe/dns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "request_id", "test-req-dns")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.DNS(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestProbeHandler_Traceroute(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	mockPool.ExpectExec("INSERT INTO probe_task").
		WithArgs(
			pgxmock.AnyArg(),
			"traceroute",
			"example.com",
			"example.com",
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	reqBody := ProbeRequest{Target: "example.com"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/probe/traceroute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "request_id", "test-req-traceroute")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.Traceroute(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestProbeHandler_Diagnose(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	// Expect 5 inserts (one for each probe type)
	for i := 0; i < 5; i++ {
		mockPool.ExpectExec("INSERT INTO probe_task").
			WithArgs(
				pgxmock.AnyArg(),
				pgxmock.AnyArg(), // type varies
				"example.com",
				"example.com",
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
				pgxmock.AnyArg(),
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
	}

	reqBody := ProbeRequest{Target: "example.com"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/diagnose", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "request_id", "test-req-diagnose")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.Diagnose(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		t.Logf("Response body: %s", rr.Body.String())
	}

	var response struct {
		Data struct {
			DiagnosisID string   `json:"diagnosis_id"`
			TaskIDs     []string `json:"task_ids"`
			Status      string   `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Data.TaskIDs) != 5 {
		t.Errorf("expected 5 task IDs, got %d", len(response.Data.TaskIDs))
	}

	if response.Data.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", response.Data.Status)
	}

	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProbeHandler_InvalidRequest(t *testing.T) {
	handler, mr, mockPool := setupTestProbeHandler(t)
	defer mr.Close()
	defer mockPool.Close()

	// Test missing target
	reqBody := ProbeRequest{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/probe/http", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "request_id", "test-req-invalid")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.HTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

