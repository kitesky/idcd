// Package template provides locale-aware email templates loaded via
// go:embed. Each template base name (e.g. "verify_email") has one HTML
// file per locale ("verify_email.cn.html", "verify_email.en.html"). When a
// renderer is asked for an unsupported locale, it walks the shared i18n
// registry fallback chain so adding a new language is a data-only change
// (drop new {base}.{locale}.html files into this directory).
package template

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"

	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/i18n"
)

//go:embed *.html
var embedFS embed.FS

// Template base names registered with the loader. New email types should be
// added here and accompanied by `<base>.<locale>.html` files for every
// supported locale in the registry.
const (
	BaseVerifyEmail   = "verify_email"
	BaseWelcome       = "welcome"
	BaseResetPassword = "reset_password"
	BaseRefundFailed  = "refund_failed"
)

// registeredBases returns the known template base names. Tests assert every
// base is parseable for every supported locale.
func registeredBases() []string {
	return []string{
		BaseVerifyEmail,
		BaseWelcome,
		BaseResetPassword,
		BaseRefundFailed,
	}
}

// Bases is the exported view of registeredBases for callers that need to
// enumerate every email type (e.g. tests, doc generation).
func Bases() []string { return registeredBases() }

// TemplateData holds common template variables.
type TemplateData map[string]any

// VerifyEmailData holds variables for the email verification template.
type VerifyEmailData struct {
	Code      string // 6-digit OTP code
	ExpiresIn string // human-readable duration (caller already locale-formatted)
}

// WelcomeData holds variables for the welcome email template.
type WelcomeData struct {
	Username string // user's display name or email
}

// ResetPasswordData holds variables for the password reset template.
type ResetPasswordData struct {
	ResetURL  string // password reset URL (already locale-prefixed by the caller)
	ExpiresIn string // human-readable duration
}

// RefundFailedData holds variables for the post-retry-exhaustion apology
// email. The caller formats AmountDisplay (e.g. "99.00 CNY") because the
// template intentionally doesn't know about currency conventions.
type RefundFailedData struct {
	PaymentID     string
	ExtTxnID      string
	AmountDisplay string
}

// Templates provides access to all email templates. Internally it caches one
// parsed *html/template per (base, locale) pair so subsequent renders are
// allocation-free for the template tree itself.
type Templates struct {
	registry *i18n.Registry
	// parsed[base][locale] = *html/template
	parsed map[string]map[string]*htmltemplate.Template
}

// New creates a new Templates instance and pre-parses every (base, locale)
// pair declared in the registry. Returns an error if any locale is missing
// a template file (caught at startup rather than at first send).
func New() (*Templates, error) {
	reg := i18n.MustDefault()

	t := &Templates{
		registry: reg,
		parsed:   make(map[string]map[string]*htmltemplate.Template),
	}

	exister := EmbedExister()
	for _, base := range registeredBases() {
		t.parsed[base] = make(map[string]*htmltemplate.Template, len(reg.Codes()))
		for _, code := range reg.Codes() {
			path, err := TemplatePathFS(exister, base, code)
			if err != nil {
				return nil, apperr.Internal(fmt.Sprintf("template: no file for base=%s locale=%s", base, code), err)
			}
			body, err := embedFS.ReadFile(path)
			if err != nil {
				return nil, apperr.Internal(fmt.Sprintf("template: read embedded %s", path), err)
			}
			tpl, err := htmltemplate.New(base + "." + code).Parse(string(body))
			if err != nil {
				return nil, apperr.Internal(fmt.Sprintf("template: parse %s", path), err)
			}
			t.parsed[base][code] = tpl
		}
	}

	return t, nil
}

// lookup returns the parsed template for the base in the locale's fallback
// chain. Missing combinations should never happen after New() succeeds, so
// hitting the final "" branch indicates a programming error.
func (t *Templates) lookup(base, locale string) (*htmltemplate.Template, error) {
	bucket, ok := t.parsed[base]
	if !ok {
		return nil, apperr.Internal(fmt.Sprintf("template: unknown base %q", base), nil)
	}
	for _, loc := range t.registry.FallbackChain(locale) {
		if tpl, ok := bucket[loc]; ok {
			return tpl, nil
		}
	}
	return nil, apperr.Internal(fmt.Sprintf("template: no template for base=%q locale=%q (fallback chain exhausted)", base, locale), nil)
}

// render executes the template selected by lookup and returns its body.
func (t *Templates) render(base, locale string, data any) (string, error) {
	tpl, err := t.lookup(base, locale)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", apperr.Internal(fmt.Sprintf("template: execute %s/%s", base, locale), err)
	}
	return buf.String(), nil
}

// RenderVerifyEmail renders the email verification template for the locale.
func (t *Templates) RenderVerifyEmail(locale string, data VerifyEmailData) (string, error) {
	return t.render(BaseVerifyEmail, locale, data)
}

// RenderWelcome renders the welcome email template for the locale.
func (t *Templates) RenderWelcome(locale string, data WelcomeData) (string, error) {
	return t.render(BaseWelcome, locale, data)
}

// RenderResetPassword renders the password reset email template for the locale.
func (t *Templates) RenderResetPassword(locale string, data ResetPasswordData) (string, error) {
	return t.render(BaseResetPassword, locale, data)
}

// RenderRefundFailed renders the refund-failed apology email for the locale.
func (t *Templates) RenderRefundFailed(locale string, data RefundFailedData) (string, error) {
	return t.render(BaseRefundFailed, locale, data)
}
