package errcode

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kite365/idcd/lib/shared/apperr"
)

func TestFromAppErr(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantCode Code
		wantOK  bool
	}{
		{"nil", nil, "", false},
		{"plain error", errors.New("boom"), "", false},
		{"not found", apperr.NotFound("x"), NotFound, true},
		{"duplicate", apperr.Duplicate("x"), Duplicate, true},
		{"conflict", apperr.Conflict("x"), Conflict, true},
		{"validation", apperr.Validation("x", ""), ValidationFailed, true},
		{"unauthorized", apperr.Unauthorized("x"), AuthRequired, true},
		{"forbidden", apperr.Forbidden("x"), Forbidden, true},
		{"rate limit", apperr.RateLimit("x"), RateLimitExceeded, true},
		{"internal", apperr.Internal("x", nil), InternalError, true},
		{"unavailable", apperr.Unavailable("x", nil), Unavailable, true},
		{"wrapped", fmt.Errorf("ctx: %w", apperr.NotFound("x")), NotFound, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FromAppErr(tt.err)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantCode {
				t.Errorf("code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}
