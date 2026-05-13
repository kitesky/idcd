// Package response provides unified JSON response formats for the API Gateway.
// All endpoints should use these functions to ensure consistent response structure.
package response

import (
	"encoding/json"
	"net/http"

	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/logger"
)

// SuccessResponse is the JSON structure for successful API responses.
type SuccessResponse struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

// ErrorResponse is the JSON structure for error API responses.
type ErrorResponse struct {
	Error     ErrorDetail `json:"error"`
	RequestID string      `json:"request_id"`
}

// ErrorDetail contains error information.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// JSON writes a successful JSON response with the given status code and data.
// The request_id is extracted from the request context.
func JSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	requestID := getRequestID(r)

	// 204 No Content must not have a body per HTTP spec.
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	resp := SuccessResponse{
		Data:      data,
		RequestID: requestID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log := logger.FromContext(r.Context(), logger.New("production"))
		log.Error("failed to encode JSON response", "error", err, "request_id", requestID)
		// Don't use http.Error as it would override our Content-Type header
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"code":"INTERNAL","message":"Internal Server Error","request_id":"` + requestID + `"},"request_id":"` + requestID + `"}`))
	}
}

// Error writes an error JSON response.
// It attempts to extract error details from apperr.Error, otherwise defaults to 500.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	requestID := getRequestID(r)

	var code string
	var message string
	var status int

	// Try to extract from apperr.Error
	if appErr := apperr.AsError(err); appErr != nil {
		code = string(appErr.Code)
		message = appErr.Message
		status = appErr.HTTPStatus()
	} else {
		// Fallback to generic internal error
		code = string(apperr.CodeInternal)
		message = "Internal server error"
		status = http.StatusInternalServerError
	}

	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
		RequestID: requestID,
	}

	// Log the error for debugging
	log := logger.FromContext(r.Context(), logger.New("production"))
	log.Error("API error response",
		"error", err,
		"code", code,
		"status", status,
		"request_id", requestID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		log.Error("failed to encode error response", "error", encErr, "request_id", requestID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getRequestID extracts the request ID from the request context or headers.
// Falls back to "unknown" if not found.
func getRequestID(r *http.Request) string {
	// Try to get from context (set by request_id middleware)
	if val := r.Context().Value("request_id"); val != nil {
		if id, ok := val.(string); ok && id != "" {
			return id
		}
	}

	// Fallback to header
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}

	return "unknown"
}