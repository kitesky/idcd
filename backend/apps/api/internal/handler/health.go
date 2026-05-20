// Package handler provides HTTP request handlers for the API Gateway.
package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/response"
)

// PgPinger is the minimal Postgres surface the health check needs.
// Implemented by *pgxpool.Pool; lets tests pass a nil or fake without
// pulling in database/sql.
type PgPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	db    PgPinger
	redis *redis.Client
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client) *HealthHandler {
	var p PgPinger
	if db != nil {
		p = db
	}
	return &HealthHandler{
		db:    p,
		redis: redis,
	}
}

// HealthResponse represents the health check response structure.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// DeepHealthResponse represents the deep health check response structure.
type DeepHealthResponse struct {
	Status  string                    `json:"status"`
	Version string                    `json:"version"`
	Checks  map[string]CheckStatus    `json:"checks"`
}

// CheckStatus represents the status of an individual health check.
type CheckStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Health handles GET /health - basic health check.
// Returns 200 OK with status and version.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	version := h.getVersion()

	healthResp := HealthResponse{
		Status:  "ok",
		Version: version,
	}

	response.JSON(w, r, http.StatusOK, healthResp)
}

// DeepHealth handles GET /health/deep - comprehensive health check.
// Checks PostgreSQL and Redis connectivity with 3-second timeout.
func (h *HealthHandler) DeepHealth(w http.ResponseWriter, r *http.Request) {
	version := h.getVersion()
	checks := make(map[string]CheckStatus)

	// Create a context with 3-second timeout for all checks
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Check PostgreSQL
	pgStatus := h.checkPostgreSQL(ctx)
	checks["postgres"] = pgStatus

	// Check Redis
	redisStatus := h.checkRedis(ctx)
	checks["redis"] = redisStatus

	// Determine overall status
	status := "ok"
	if pgStatus.Status == "error" || redisStatus.Status == "error" {
		status = "degraded"
	}

	deepHealthResp := DeepHealthResponse{
		Status:  status,
		Version: version,
		Checks:  checks,
	}

	// Return 503 if degraded, 200 if ok
	statusCode := http.StatusOK
	if status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	response.JSON(w, r, statusCode, deepHealthResp)
}

// checkPostgreSQL performs a PostgreSQL connectivity check via pgxpool.Ping.
func (h *HealthHandler) checkPostgreSQL(ctx context.Context) CheckStatus {
	if h.db == nil {
		return CheckStatus{
			Status: "error",
			Error:  "database connection not configured",
		}
	}
	if err := h.db.Ping(ctx); err != nil {
		return CheckStatus{
			Status: "error",
			Error:  fmt.Sprintf("database ping failed: %v", err),
		}
	}
	return CheckStatus{Status: "ok"}
}

// checkRedis performs a Redis connectivity check.
func (h *HealthHandler) checkRedis(ctx context.Context) CheckStatus {
	if h.redis == nil {
		return CheckStatus{
			Status: "error",
			Error:  "redis connection not configured",
		}
	}

	// Simple PING command to test connectivity
	pong, err := h.redis.Ping(ctx).Result()
	if err != nil {
		return CheckStatus{
			Status: "error",
			Error:  fmt.Sprintf("redis ping failed: %v", err),
		}
	}

	if pong != "PONG" {
		return CheckStatus{
			Status: "error",
			Error:  fmt.Sprintf("redis returned unexpected response: %s", pong),
		}
	}

	return CheckStatus{Status: "ok"}
}

// getVersion reads the VERSION file and returns its content.
// Falls back to "unknown" if the file doesn't exist or can't be read.
func (h *HealthHandler) getVersion() string {
	// Try to read VERSION file from project root
	content, err := os.ReadFile("../../VERSION")
	if err != nil {
		// Fallback: try current directory
		content, err = os.ReadFile("VERSION")
		if err != nil {
			// Fallback: try reading from embedded data or return unknown
			return "unknown"
		}
	}

	version := string(content)
	// Trim whitespace
	version = trimWhitespace(version)

	if version == "" {
		return "unknown"
	}

	return version
}

// trimWhitespace removes leading and trailing whitespace characters.
func trimWhitespace(s string) string {
	start := 0
	end := len(s)

	// Find first non-whitespace character
	for start < end {
		if s[start] != ' ' && s[start] != '\t' && s[start] != '\n' && s[start] != '\r' {
			break
		}
		start++
	}

	// Find last non-whitespace character
	for end > start {
		if s[end-1] != ' ' && s[end-1] != '\t' && s[end-1] != '\n' && s[end-1] != '\r' {
			break
		}
		end--
	}

	return s[start:end]
}