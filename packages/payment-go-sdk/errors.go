package payment

import (
	"errors"
	"fmt"
)

// API error codes.
const (
	ErrBadRequest        = 10001
	ErrInvalidSign       = 10002
	ErrRateLimited       = 10003
	ErrIdempotent        = 10004
	ErrUnauthorized      = 20001
	ErrInvalidAPIKey     = 20004
	ErrAppSuspended      = 20005
	ErrChannelNotEnabled = 30004
	ErrMethodNotEnabled  = 30005
	ErrOrderNotFound     = 40001
	ErrVerifyFailed      = 40010
	ErrRefundExceedAmt   = 40012
	ErrInternalError     = 50001
	ErrChannelTimeout    = 50002
)

// APIError represents an error returned by the payment API.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("payment api error: code=%d message=%s", e.Code, e.Message)
}


// IsCode checks whether the error is an APIError with the given code.
func IsCode(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}
