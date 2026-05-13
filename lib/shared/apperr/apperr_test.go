package apperr_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/kite365/idcd/packages/shared/apperr"
)

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		err  *apperr.Error
		want int
	}{
		{apperr.NotFound("x"), http.StatusNotFound},
		{apperr.Duplicate("x"), http.StatusConflict},
		{apperr.Conflict("x"), http.StatusConflict},
		{apperr.Validation("x", ""), http.StatusBadRequest},
		{apperr.Unauthorized("x"), http.StatusUnauthorized},
		{apperr.Forbidden("x"), http.StatusForbidden},
		{apperr.RateLimit("x"), http.StatusTooManyRequests},
		{apperr.Internal("x", nil), http.StatusInternalServerError},
		{apperr.Unavailable("x", nil), http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		if got := c.err.HTTPStatus(); got != c.want {
			t.Errorf("[%s] HTTPStatus() = %d, want %d", c.err.Code, got, c.want)
		}
	}
}

func TestError_wrapsUnderlying(t *testing.T) {
	cause := errors.New("db failure")
	e := apperr.Internal("query failed", cause)
	if !errors.Is(e, cause) {
		t.Error("expected errors.Is to find cause via Unwrap")
	}
}

func TestIs(t *testing.T) {
	e := apperr.NotFound("user")
	if !apperr.Is(e, apperr.CodeNotFound) {
		t.Error("apperr.Is should return true for matching code")
	}
	if apperr.Is(e, apperr.CodeInternal) {
		t.Error("apperr.Is should return false for non-matching code")
	}
}

func TestIs_wrapped(t *testing.T) {
	inner := apperr.NotFound("resource")
	outer := fmt.Errorf("handler: %w", inner)
	if !apperr.Is(outer, apperr.CodeNotFound) {
		t.Error("apperr.Is should find error through wrapping")
	}
}

func TestAsError(t *testing.T) {
	e := apperr.Validation("bad input", "field x required")
	got := apperr.AsError(e)
	if got == nil {
		t.Fatal("AsError returned nil")
	}
	if got.Detail != "field x required" {
		t.Errorf("unexpected detail: %q", got.Detail)
	}
}

func TestAsError_nil(t *testing.T) {
	if apperr.AsError(errors.New("plain")) != nil {
		t.Error("AsError should return nil for non-apperr errors")
	}
}

func TestError_string(t *testing.T) {
	e := apperr.Internal("boom", errors.New("cause"))
	s := e.Error()
	if s == "" {
		t.Error("Error() should not be empty")
	}
}
