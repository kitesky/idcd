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
	AuthRequired              Code = "AUTH_REQUIRED"
	AuthInvalidToken          Code = "AUTH_INVALID_TOKEN"
	AuthExpired               Code = "AUTH_EXPIRED"
	AuthCredentialsInvalid    Code = "AUTH_CREDENTIALS_INVALID"
	AuthEmailNotVerified      Code = "AUTH_EMAIL_NOT_VERIFIED"
	AuthAccountDisabled       Code = "AUTH_ACCOUNT_DISABLED"
	Auth2FARequired           Code = "AUTH_2FA_REQUIRED"
	Auth2FAInvalid            Code = "AUTH_2FA_INVALID"
	AuthOAuthLinkedOtherUser  Code = "AUTH_OAUTH_LINKED_OTHER_USER"
	AuthOAuthStateInvalid     Code = "AUTH_OAUTH_STATE_INVALID"
	AuthWebAuthnChallengeBad  Code = "AUTH_WEBAUTHN_CHALLENGE_INVALID"
	AuthSessionInvalid        Code = "AUTH_SESSION_INVALID"
	AuthOTPInvalid            Code = "AUTH_OTP_INVALID"
	AuthOTPExpired            Code = "AUTH_OTP_EXPIRED"
	AuthOTPAttemptsExceeded   Code = "AUTH_OTP_ATTEMPTS_EXCEEDED"
	AuthPasswordWeak          Code = "AUTH_PASSWORD_WEAK"
	AuthPasswordReused        Code = "AUTH_PASSWORD_REUSED"
)

// --- Account / user profile -------------------------------------------------

const (
	AccountNotFound          Code = "ACCOUNT_NOT_FOUND"
	AccountEmailTaken        Code = "ACCOUNT_EMAIL_TAKEN"
	AccountUsernameTaken     Code = "ACCOUNT_USERNAME_TAKEN"
	AccountLocaleUnsupported Code = "ACCOUNT_LOCALE_UNSUPPORTED"
	AccountTimezoneInvalid   Code = "ACCOUNT_TIMEZONE_INVALID"
	AccountDeleted           Code = "ACCOUNT_DELETED"
)

// --- Generic boundary errors -----------------------------------------------

const (
	ValidationFailed  Code = "VALIDATION_FAILED"
	RateLimitExceeded Code = "RATE_LIMIT_EXCEEDED"
	NotFound          Code = "NOT_FOUND"
	Forbidden         Code = "FORBIDDEN"
	Conflict          Code = "CONFLICT"
	Duplicate         Code = "DUPLICATE"
	RequestTimeout    Code = "REQUEST_TIMEOUT"
	RequestBodyBad    Code = "REQUEST_BODY_INVALID"
)

// --- Server / infrastructure -----------------------------------------------

const (
	InternalError Code = "INTERNAL_ERROR"
	Unavailable   Code = "UNAVAILABLE"
	Unknown       Code = "UNKNOWN"
)

// --- Monitors --------------------------------------------------------------

const (
	MonitorNotFound        Code = "MONITOR_NOT_FOUND"
	MonitorNameTaken       Code = "MONITOR_NAME_TAKEN"
	MonitorLimitExceeded   Code = "MONITOR_LIMIT_EXCEEDED"
	MonitorInvalidTarget   Code = "MONITOR_INVALID_TARGET"
	MonitorInvalidInterval Code = "MONITOR_INVALID_INTERVAL"
	MonitorPaused          Code = "MONITOR_PAUSED"
)

// --- Alerts ----------------------------------------------------------------

const (
	AlertRuleConflict      Code = "ALERT_RULE_CONFLICT"
	AlertRuleNotFound      Code = "ALERT_RULE_NOT_FOUND"
	AlertChannelNotFound   Code = "ALERT_CHANNEL_NOT_FOUND"
	AlertChannelInvalid    Code = "ALERT_CHANNEL_INVALID"
	AlertNoiseRuleNotFound Code = "ALERT_NOISE_RULE_NOT_FOUND"
)

// --- Billing ---------------------------------------------------------------

const (
	BillingInsufficientCredits Code = "BILLING_INSUFFICIENT_CREDITS"
	BillingPlanNotFound        Code = "BILLING_PLAN_NOT_FOUND"
	BillingInvoiceNotFound     Code = "BILLING_INVOICE_NOT_FOUND"
	BillingPaymentFailed       Code = "BILLING_PAYMENT_FAILED"
	BillingRefundFailed        Code = "BILLING_REFUND_FAILED"
	BillingQuotaExceeded       Code = "BILLING_QUOTA_EXCEEDED"
)

// --- Status pages ----------------------------------------------------------

const (
	StatusPageNotFound      Code = "STATUS_PAGE_NOT_FOUND"
	StatusPageSlugTaken     Code = "STATUS_PAGE_SLUG_TAKEN"
	StatusPageDomainInvalid Code = "STATUS_PAGE_DOMAIN_INVALID"
	StatusPageDomainTaken   Code = "STATUS_PAGE_DOMAIN_TAKEN"
)

// --- Node / aggregator -----------------------------------------------------

const (
	NodeNotFound               Code = "NODE_NOT_FOUND"
	NodeEnrollmentTokenInvalid Code = "NODE_ENROLLMENT_TOKEN_INVALID"
	NodeOffline                Code = "NODE_OFFLINE"
	NodeUpgradeFailed          Code = "NODE_UPGRADE_FAILED"
)

// --- Teams / organizations -------------------------------------------------

const (
	TeamNotFound      Code = "TEAM_NOT_FOUND"
	TeamMemberExists  Code = "TEAM_MEMBER_EXISTS"
	TeamInviteInvalid Code = "TEAM_INVITE_INVALID"
)

// --- API keys / PATs / tokens ----------------------------------------------

const (
	APIKeyNotFound  Code = "API_KEY_NOT_FOUND"
	APIKeyRevoked   Code = "API_KEY_REVOKED"
	PATNotFound     Code = "PAT_NOT_FOUND"
	TokenExpired    Code = "TOKEN_EXPIRED"
	TokenLimitHit   Code = "TOKEN_LIMIT_EXCEEDED"
)

// --- Postmortem / incidents -----------------------------------------------

const (
	PostmortemNotFound Code = "POSTMORTEM_NOT_FOUND"
	IncidentNotFound   Code = "INCIDENT_NOT_FOUND"
)

// --- Probe / diagnostics ---------------------------------------------------

const (
	ProbeJobNotFound      Code = "PROBE_JOB_NOT_FOUND"
	ProbeTargetInvalid    Code = "PROBE_TARGET_INVALID"
	DiagnoseReportTooLong Code = "DIAGNOSE_REPORT_TOO_LONG"
)

// All returns every code declared in this package. Used by CI lint to
// assert errcode <-> messages parity and by tests that want table-driven
// coverage of every code.
func All() []Code {
	return []Code{
		// Auth
		AuthRequired,
		AuthInvalidToken,
		AuthExpired,
		AuthCredentialsInvalid,
		AuthEmailNotVerified,
		AuthAccountDisabled,
		Auth2FARequired,
		Auth2FAInvalid,
		AuthOAuthLinkedOtherUser,
		AuthOAuthStateInvalid,
		AuthWebAuthnChallengeBad,
		AuthSessionInvalid,
		AuthOTPInvalid,
		AuthOTPExpired,
		AuthOTPAttemptsExceeded,
		AuthPasswordWeak,
		AuthPasswordReused,
		// Account
		AccountNotFound,
		AccountEmailTaken,
		AccountUsernameTaken,
		AccountLocaleUnsupported,
		AccountTimezoneInvalid,
		AccountDeleted,
		// Generic
		ValidationFailed,
		RateLimitExceeded,
		NotFound,
		Forbidden,
		Conflict,
		Duplicate,
		RequestTimeout,
		RequestBodyBad,
		// Server
		InternalError,
		Unavailable,
		Unknown,
		// Monitors
		MonitorNotFound,
		MonitorNameTaken,
		MonitorLimitExceeded,
		MonitorInvalidTarget,
		MonitorInvalidInterval,
		MonitorPaused,
		// Alerts
		AlertRuleConflict,
		AlertRuleNotFound,
		AlertChannelNotFound,
		AlertChannelInvalid,
		AlertNoiseRuleNotFound,
		// Billing
		BillingInsufficientCredits,
		BillingPlanNotFound,
		BillingInvoiceNotFound,
		BillingPaymentFailed,
		BillingRefundFailed,
		BillingQuotaExceeded,
		// Status pages
		StatusPageNotFound,
		StatusPageSlugTaken,
		StatusPageDomainInvalid,
		StatusPageDomainTaken,
		// Nodes
		NodeNotFound,
		NodeEnrollmentTokenInvalid,
		NodeOffline,
		NodeUpgradeFailed,
		// Teams
		TeamNotFound,
		TeamMemberExists,
		TeamInviteInvalid,
		// API keys / PATs
		APIKeyNotFound,
		APIKeyRevoked,
		PATNotFound,
		TokenExpired,
		TokenLimitHit,
		// Postmortem
		PostmortemNotFound,
		IncidentNotFound,
		// Probe
		ProbeJobNotFound,
		ProbeTargetInvalid,
		DiagnoseReportTooLong,
	}
}
