package errcode

import (
	"errors"

	"github.com/kite365/idcd/lib/shared/apperr"
)

// FromAppErr inspects err for a wrapped *apperr.Error and translates its
// apperr.Code (the older, coarse-grained typing) into the finer-grained
// errcode.Code used by the localized response helpers.
//
// The mapping is intentionally one-way: apperr only knows a handful of
// generic kinds (NotFound, Validation, Conflict, …), so all callers that
// surface domain-specific codes (MONITOR_NAME_TAKEN, AUTH_OTP_EXPIRED, …)
// must call response.ErrorCode directly. FromAppErr is the safety net that
// keeps legacy response.Error() call sites usable through the i18n catalog
// without forcing every handler to migrate in one giant churn.
//
// The boolean return indicates whether a mapping was found. When false,
// callers should fall back to the legacy response.Error() path so existing
// behaviour (literal Message field, no translation) is preserved.
func FromAppErr(err error) (Code, bool) {
	if err == nil {
		return "", false
	}
	var aerr *apperr.Error
	if !errors.As(err, &aerr) {
		return "", false
	}
	switch aerr.Code {
	case apperr.CodeNotFound:
		return NotFound, true
	case apperr.CodeDuplicate:
		return Duplicate, true
	case apperr.CodeConflict:
		return Conflict, true
	case apperr.CodeValidation:
		return ValidationFailed, true
	case apperr.CodeUnauthorized:
		return AuthRequired, true
	case apperr.CodeForbidden:
		return Forbidden, true
	case apperr.CodeRateLimit:
		return RateLimitExceeded, true
	case apperr.CodeInternal:
		return InternalError, true
	case apperr.CodeUnavailable:
		return Unavailable, true
	default:
		return "", false
	}
}
