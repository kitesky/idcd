package errcode

import (
	"net/http"
	"strings"
	"testing"
)

func TestAllCodesUpperSnakeCase(t *testing.T) {
	for _, c := range All() {
		s := string(c)
		if s == "" {
			t.Error("empty code in All()")
			continue
		}
		if s != strings.ToUpper(s) {
			t.Errorf("code %q must be upper-case", s)
		}
		// Allowed characters: A-Z 0-9 _
		for _, r := range s {
			ok := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
			if !ok {
				t.Errorf("code %q has illegal character %q", s, r)
				break
			}
		}
	}
}

func TestAllCodesUnique(t *testing.T) {
	seen := map[Code]bool{}
	for _, c := range All() {
		if seen[c] {
			t.Errorf("duplicate code %q in All()", c)
		}
		seen[c] = true
	}
}

func TestHTTPStatusMapping(t *testing.T) {
	cases := []struct {
		code Code
		want int
	}{
		{AuthRequired, http.StatusUnauthorized},
		{AuthInvalidToken, http.StatusUnauthorized},
		{AuthExpired, http.StatusUnauthorized},
		{ValidationFailed, http.StatusBadRequest},
		{RateLimitExceeded, http.StatusTooManyRequests},
		{NotFound, http.StatusNotFound},
		{Forbidden, http.StatusForbidden},
		{Conflict, http.StatusConflict},
		{InternalError, http.StatusInternalServerError},
		{Unavailable, http.StatusServiceUnavailable},
		{Unknown, http.StatusInternalServerError},
		{MonitorNotFound, http.StatusNotFound},
		{AlertRuleConflict, http.StatusConflict},
		{BillingInsufficientCredits, http.StatusPaymentRequired},
	}
	for _, c := range cases {
		if got := HTTPStatus(c.code); got != c.want {
			t.Errorf("HTTPStatus(%q) = %d want %d", c.code, got, c.want)
		}
	}
}

func TestHTTPStatusUnknownCodeDefaults500(t *testing.T) {
	if got := HTTPStatus("MADE_UP_CODE_NOT_REGISTERED"); got != http.StatusInternalServerError {
		t.Errorf("unknown code should default to 500, got %d", got)
	}
}

func TestEveryAllCodeHasStatusMapping(t *testing.T) {
	for _, c := range All() {
		if _, ok := statusMap[c]; !ok {
			t.Errorf("code %q missing from statusMap", c)
		}
	}
}

func TestCodeString(t *testing.T) {
	if AuthRequired.String() != "AUTH_REQUIRED" {
		t.Errorf("Code.String mismatch: %q", AuthRequired.String())
	}
}
