package errcode

import "net/http"

// statusMap is the canonical mapping from a Code to its HTTP status. Any
// code not present here defaults to 500 — see HTTPStatus.
//
// Keep alphabetized within the same status band to make conflicts easy to
// spot in code review.
var statusMap = map[Code]int{
	// 400 — client validation
	ValidationFailed: http.StatusBadRequest,

	// 401 — authentication
	AuthRequired:     http.StatusUnauthorized,
	AuthInvalidToken: http.StatusUnauthorized,
	AuthExpired:      http.StatusUnauthorized,

	// 402-ish billing failure surfaces as 402 Payment Required so clients
	// can branch (the API has no strict 402 semantics today, but reserving
	// it now avoids re-mapping later).
	BillingInsufficientCredits: http.StatusPaymentRequired,

	// 403 — authorization
	Forbidden: http.StatusForbidden,

	// 404 — missing resources
	NotFound:        http.StatusNotFound,
	MonitorNotFound: http.StatusNotFound,

	// 409 — conflicts
	Conflict:          http.StatusConflict,
	AlertRuleConflict: http.StatusConflict,

	// 429 — rate limiting
	RateLimitExceeded: http.StatusTooManyRequests,

	// 500 / 503 — server-side
	InternalError: http.StatusInternalServerError,
	Unknown:       http.StatusInternalServerError,
	Unavailable:   http.StatusServiceUnavailable,
}

// HTTPStatus returns the HTTP status mapped to c, defaulting to 500 for
// codes we forgot to map. The default exists so a missing entry never
// crashes a request — but the CI lint should ensure no such gap remains.
func HTTPStatus(c Code) int {
	if s, ok := statusMap[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}
