// Package apperr defines typed application errors with HTTP status mapping.
// All service boundaries should return *apperr.Error so HTTP / gRPC handlers
// can translate without switch-on-string logic.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a machine-readable error identifier, returned in JSON responses.
type Code string

const (
	CodeNotFound     Code = "NOT_FOUND"
	CodeDuplicate    Code = "DUPLICATE"
	CodeConflict     Code = "CONFLICT"
	CodeValidation   Code = "VALIDATION"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeRateLimit    Code = "RATE_LIMIT"
	CodeInternal     Code = "INTERNAL"
	CodeUnavailable  Code = "UNAVAILABLE"
)

// Error is a structured application error.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`           // user-facing summary
	Detail  string `json:"detail,omitempty"`  // optional extra context (not shown in prod)
	Err     error  `json:"-"`                 // underlying cause (for logging)
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// HTTPStatus returns the appropriate HTTP status code for this error.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeDuplicate, CodeConflict:
		return http.StatusConflict
	case CodeValidation:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimit:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// --- Constructors ---

func NotFound(msg string) *Error {
	return &Error{Code: CodeNotFound, Message: msg}
}

func NotFoundf(format string, args ...any) *Error {
	return &Error{Code: CodeNotFound, Message: fmt.Sprintf(format, args...)}
}

func Duplicate(msg string) *Error {
	return &Error{Code: CodeDuplicate, Message: msg}
}

func Conflict(msg string) *Error {
	return &Error{Code: CodeConflict, Message: msg}
}

func Validation(msg, detail string) *Error {
	return &Error{Code: CodeValidation, Message: msg, Detail: detail}
}

func Validationf(format string, args ...any) *Error {
	return &Error{Code: CodeValidation, Message: fmt.Sprintf(format, args...)}
}

func Unauthorized(msg string) *Error {
	return &Error{Code: CodeUnauthorized, Message: msg}
}

func Forbidden(msg string) *Error {
	return &Error{Code: CodeForbidden, Message: msg}
}

func RateLimit(msg string) *Error {
	return &Error{Code: CodeRateLimit, Message: msg}
}

func Internal(msg string, cause error) *Error {
	return &Error{Code: CodeInternal, Message: msg, Err: cause}
}

func Internalf(cause error, format string, args ...any) *Error {
	return &Error{Code: CodeInternal, Message: fmt.Sprintf(format, args...), Err: cause}
}

func Unavailable(msg string, cause error) *Error {
	return &Error{Code: CodeUnavailable, Message: msg, Err: cause}
}

// --- Inspection helpers ---

// Is reports whether err (or any wrapped error) is an *Error with the given code.
func Is(err error, code Code) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// AsError returns the *Error if err wraps one, else nil.
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}
