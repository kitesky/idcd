package errcode

import "net/http"

// statusMap is the canonical mapping from a Code to its HTTP status. Any
// code not present here defaults to 500 — see HTTPStatus.
//
// Keep alphabetized within the same status band to make conflicts easy to
// spot in code review.
var statusMap = map[Code]int{
	// 400 — client validation / bad input
	ValidationFailed:           http.StatusBadRequest,
	RequestBodyBad:             http.StatusBadRequest,
	MonitorInvalidTarget:       http.StatusBadRequest,
	MonitorInvalidInterval:     http.StatusBadRequest,
	AlertChannelInvalid:        http.StatusBadRequest,
	StatusPageDomainInvalid:    http.StatusBadRequest,
	ProbeTargetInvalid:         http.StatusBadRequest,
	DiagnoseReportTooLong:      http.StatusBadRequest,
	AuthPasswordWeak:           http.StatusBadRequest,
	AuthPasswordReused:         http.StatusBadRequest,
	AccountLocaleUnsupported:   http.StatusBadRequest,
	AccountTimezoneInvalid:     http.StatusBadRequest,
	TeamInviteInvalid:          http.StatusBadRequest,
	NodeEnrollmentTokenInvalid: http.StatusBadRequest,

	// 401 — authentication
	AuthRequired:              http.StatusUnauthorized,
	AuthInvalidToken:          http.StatusUnauthorized,
	AuthExpired:               http.StatusUnauthorized,
	AuthCredentialsInvalid:    http.StatusUnauthorized,
	AuthEmailNotVerified:      http.StatusUnauthorized,
	AuthAccountDisabled:       http.StatusUnauthorized,
	Auth2FARequired:           http.StatusUnauthorized,
	Auth2FAInvalid:            http.StatusUnauthorized,
	AuthOAuthStateInvalid:     http.StatusUnauthorized,
	AuthWebAuthnChallengeBad:  http.StatusUnauthorized,
	AuthSessionInvalid:        http.StatusUnauthorized,
	AuthOTPInvalid:            http.StatusUnauthorized,
	AuthOTPExpired:            http.StatusUnauthorized,
	AuthOTPAttemptsExceeded:   http.StatusUnauthorized,
	TokenExpired:              http.StatusUnauthorized,

	// 402-ish billing failure surfaces as 402 Payment Required so clients
	// can branch (the API has no strict 402 semantics today, but reserving
	// it now avoids re-mapping later).
	BillingInsufficientCredits: http.StatusPaymentRequired,

	// 403 — authorization
	Forbidden:  http.StatusForbidden,
	APIKeyRevoked: http.StatusForbidden,

	// 404 — missing resources
	NotFound:               http.StatusNotFound,
	MonitorNotFound:        http.StatusNotFound,
	AccountNotFound:        http.StatusNotFound,
	AlertRuleNotFound:      http.StatusNotFound,
	AlertChannelNotFound:   http.StatusNotFound,
	AlertNoiseRuleNotFound: http.StatusNotFound,
	BillingPlanNotFound:    http.StatusNotFound,
	BillingInvoiceNotFound: http.StatusNotFound,
	StatusPageNotFound:     http.StatusNotFound,
	NodeNotFound:           http.StatusNotFound,
	TeamNotFound:           http.StatusNotFound,
	APIKeyNotFound:         http.StatusNotFound,
	PATNotFound:            http.StatusNotFound,
	PostmortemNotFound:     http.StatusNotFound,
	IncidentNotFound:       http.StatusNotFound,
	ProbeJobNotFound:       http.StatusNotFound,

	// 408 — request timeout
	RequestTimeout: http.StatusRequestTimeout,

	// 409 — conflicts / duplicates
	Conflict:                  http.StatusConflict,
	Duplicate:                 http.StatusConflict,
	AlertRuleConflict:         http.StatusConflict,
	AccountEmailTaken:         http.StatusConflict,
	AccountUsernameTaken:      http.StatusConflict,
	MonitorNameTaken:          http.StatusConflict,
	StatusPageSlugTaken:       http.StatusConflict,
	StatusPageDomainTaken:     http.StatusConflict,
	TeamMemberExists:          http.StatusConflict,
	AuthOAuthLinkedOtherUser:  http.StatusConflict,
	AccountDeleted:            http.StatusGone,
	MonitorPaused:             http.StatusConflict,

	// 410 — gone (re-using for soft-deleted resources is OK)
	// AccountDeleted already covered above.

	// 422 — semantic error / quota / limits
	MonitorLimitExceeded: http.StatusUnprocessableEntity,
	TokenLimitHit:        http.StatusUnprocessableEntity,
	BillingQuotaExceeded: http.StatusUnprocessableEntity,
	BillingPaymentFailed: http.StatusUnprocessableEntity,
	BillingRefundFailed:  http.StatusUnprocessableEntity,
	NodeOffline:          http.StatusUnprocessableEntity,
	NodeUpgradeFailed:    http.StatusUnprocessableEntity,

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
