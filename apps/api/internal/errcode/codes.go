// Package errcode is the single source of truth for stable, machine-readable
// error codes returned over the public API. Codes are UPPER_SNAKE_CASE
// strings carried in JSON responses; the corresponding human-readable
// message text lives in lib/shared/i18n/messages/{locale}/errors.json so the
// catalog can render it per-request.
//
// Adding a new code:
//
//  1. Add the const here (alphabetized within its module section).
//  2. Add a row to statusMap in http.go.
//  3. Add a translation entry under both cn/ and en/ errors.json.
//  4. CI lint (Phase 5) will fail the build if any of those three steps is
//     missing — handlers themselves should never include a literal user-
//     facing message string anymore.
package errcode

// Code is the typed representation of an error code. Using a named string
// type prevents accidental swapping with arbitrary strings at the call
// site (e.g. response.ErrorCode would refuse a plain "foo").
type Code string

// String returns the underlying wire value. Exposed so logger / metrics can
// emit codes without explicit type conversions.
func (c Code) String() string { return string(c) }

// --- Auth & session ---------------------------------------------------------

const (
	AuthRequired     Code = "AUTH_REQUIRED"
	AuthInvalidToken Code = "AUTH_INVALID_TOKEN"
	AuthExpired      Code = "AUTH_EXPIRED"
)

// --- Generic boundary errors -----------------------------------------------

const (
	ValidationFailed  Code = "VALIDATION_FAILED"
	RateLimitExceeded Code = "RATE_LIMIT_EXCEEDED"
	NotFound          Code = "NOT_FOUND"
	Forbidden         Code = "FORBIDDEN"
	Conflict          Code = "CONFLICT"
)

// --- Server / infrastructure -----------------------------------------------

const (
	InternalError Code = "INTERNAL_ERROR"
	Unavailable   Code = "UNAVAILABLE"
	Unknown       Code = "UNKNOWN"
)

// --- Monitors --------------------------------------------------------------

const (
	MonitorNotFound Code = "MONITOR_NOT_FOUND"
)

// --- Alerts ----------------------------------------------------------------

const (
	AlertRuleConflict Code = "ALERT_RULE_CONFLICT"
)

// --- Billing ---------------------------------------------------------------

const (
	BillingInsufficientCredits Code = "BILLING_INSUFFICIENT_CREDITS"
)

// All returns every code declared in this package. Used by CI lint to
// assert errcode <-> messages parity and by tests that want table-driven
// coverage of every code.
func All() []Code {
	return []Code{
		AuthRequired,
		AuthInvalidToken,
		AuthExpired,
		ValidationFailed,
		RateLimitExceeded,
		NotFound,
		Forbidden,
		Conflict,
		InternalError,
		Unavailable,
		Unknown,
		MonitorNotFound,
		AlertRuleConflict,
		BillingInsufficientCredits,
	}
}
