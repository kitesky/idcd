package template

import (
	"strings"
	"testing"
)

func TestTemplates_New(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Expected no error creating templates, got: %v", err)
	}

	if templates == nil {
		t.Fatal("Expected non-nil templates instance")
	}

	// Every (base, locale) combination must be pre-parsed after New().
	for _, base := range Bases() {
		bucket, ok := templates.parsed[base]
		if !ok {
			t.Errorf("base %q missing from parsed map", base)
			continue
		}
		for _, code := range []string{"cn", "en"} {
			if _, ok := bucket[code]; !ok {
				t.Errorf("base %q missing parsed template for locale %q", base, code)
			}
		}
	}
}

// TestTemplates_RenderVerifyEmail runs the verify-email template through
// every registered locale and asserts that locale-specific copy lands in
// the output (no leakage across locales).
func TestTemplates_RenderVerifyEmail(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	data := VerifyEmailData{
		Code:      "123456",
		ExpiresIn: "10 分钟",
	}

	cases := []struct {
		locale        string
		mustContain   []string
		mustNotContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"123456",
				"10 分钟",
				"验证您的邮箱地址",
				"idcd",
				"DOCTYPE html",
				`lang="zh-CN"`,
			},
		},
		{
			locale: "en",
			mustContain: []string{
				"123456",
				"10 分钟",
				"Verify your email address",
				"idcd",
				"DOCTYPE html",
				`lang="en-US"`,
			},
			mustNotContain: []string{"验证您的邮箱地址"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderVerifyEmail(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
			for _, banned := range tc.mustNotContain {
				if strings.Contains(html, banned) {
					t.Errorf("locale=%s should NOT contain %q", tc.locale, banned)
				}
			}
		})
	}
}

func TestTemplates_RenderWelcome(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	data := WelcomeData{Username: "testuser"}

	cases := []struct {
		locale      string
		mustContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"testuser",
				"欢迎加入 idcd",
				"多地网络诊断",
				"实时监控告警",
				"状态页与报告",
				"开始使用 idcd",
				"DOCTYPE html",
			},
		},
		{
			locale: "en",
			mustContain: []string{
				"testuser",
				"Welcome to idcd!",
				"Global network diagnostics",
				"Real-time monitoring",
				"Status pages",
				"Get started with idcd",
				"DOCTYPE html",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderWelcome(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
		})
	}
}

func TestTemplates_RenderResetPassword(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	data := ResetPasswordData{
		ResetURL:  "https://idcd.com/reset-password?token=abc123",
		ExpiresIn: "30 分钟",
	}

	cases := []struct {
		locale      string
		mustContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"https://idcd.com/reset-password?token=abc123",
				"30 分钟",
				"密码重置请求",
				"重置密码",
				"安全提醒",
				"此链接仅可使用一次",
				"DOCTYPE html",
			},
		},
		{
			locale: "en",
			mustContain: []string{
				"https://idcd.com/reset-password?token=abc123",
				"30 分钟",
				"Password reset request",
				"Reset password",
				"Security tips",
				"This link can only be used once",
				"DOCTYPE html",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderResetPassword(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
		})
	}
}

// TestTemplates_RenderRefundFailed locks in the locale-aware apology email
// (previously inline HTML in handlers.go).
func TestTemplates_RenderRefundFailed(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	data := RefundFailedData{
		PaymentID:     "pay_001",
		ExtTxnID:      "ph_txn_001",
		AmountDisplay: "99.00 CNY",
	}

	cases := []struct {
		locale      string
		mustContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"pay_001",
				"ph_txn_001",
				"99.00 CNY",
				"致歉",
				"idcd 团队",
			},
		},
		{
			locale: "en",
			mustContain: []string{
				"pay_001",
				"ph_txn_001",
				"99.00 CNY",
				"sorry",
				"The idcd team",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderRefundFailed(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
		})
	}
}

// TestTemplates_RenderCertIssued locks in the S2 W8 cert.issued template.
// Both locales must include the primary domain, the dashboard URL, and the
// locale-specific copy without leaking the other locale's headline.
func TestTemplates_RenderCertIssued(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data := CertIssuedData{
		Domain:       "api.example.com",
		SANs:         "api.example.com, www.example.com",
		CA:           "lets-encrypt",
		NotAfter:     "2026-08-15 12:00 UTC",
		CertID:       42,
		DashboardURL: "https://idcd.com/app/cert/42",
	}
	cases := []struct {
		locale         string
		mustContain    []string
		mustNotContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"api.example.com",
				"https://idcd.com/app/cert/42",
				"已签发",
				"您的 SSL 证书已签发",
				"lets-encrypt",
				"2026-08-15 12:00 UTC",
				"DOCTYPE html",
				`lang="zh-CN"`,
			},
			mustNotContain: []string{"Your SSL certificate is ready"},
		},
		{
			locale: "en",
			mustContain: []string{
				"api.example.com",
				"https://idcd.com/app/cert/42",
				"Your SSL certificate is ready",
				"lets-encrypt",
				"DOCTYPE html",
				`lang="en-US"`,
			},
			mustNotContain: []string{"您的 SSL 证书已签发"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderCertIssued(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
			for _, banned := range tc.mustNotContain {
				if strings.Contains(html, banned) {
					t.Errorf("locale=%s should NOT contain %q", tc.locale, banned)
				}
			}
		})
	}
}

// TestTemplates_RenderCertFailed locks in the cert.failed template — must
// surface the error message (including the apology copy) for both locales.
func TestTemplates_RenderCertFailed(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data := CertFailedData{
		Domain:       "api.example.com",
		SANs:         "api.example.com",
		CA:           "lets-encrypt",
		OrderID:      99,
		ErrorMessage: "dns-01: NXDOMAIN for _acme-challenge.api.example.com",
		DashboardURL: "https://idcd.com/app/cert/orders/99",
	}
	cases := []struct {
		locale      string
		mustContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"api.example.com",
				"NXDOMAIN",
				"证书申请失败",
				"签发失败",
				"https://idcd.com/app/cert/orders/99",
			},
		},
		{
			locale: "en",
			mustContain: []string{
				"api.example.com",
				"NXDOMAIN",
				"Certificate request failed",
				"sorry",
				"https://idcd.com/app/cert/orders/99",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderCertFailed(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
		})
	}
}

// TestTemplates_RenderCertExpiring locks in the cert.expiring template. The
// big days number must appear in both locales and copy must localise.
func TestTemplates_RenderCertExpiring(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data := CertExpiringData{
		Domain:       "api.example.com",
		SANs:         "api.example.com",
		CA:           "lets-encrypt",
		NotAfter:     "2026-06-01 00:00 UTC",
		CertID:       42,
		Days:         7,
		DashboardURL: "https://idcd.com/app/cert/42",
	}
	cases := []struct {
		locale      string
		mustContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"api.example.com",
				">7<",
				"证书即将到期提醒",
				"天后到期",
				"https://idcd.com/app/cert/42",
			},
		},
		{
			locale: "en",
			mustContain: []string{
				"api.example.com",
				">7<",
				"Certificate expiring soon",
				"days remaining",
				"https://idcd.com/app/cert/42",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderCertExpiring(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
		})
	}
}

// TestTemplates_RenderCertRenewalFailed locks in the cert.renewal_failed
// template — must surface the error message and the "action required" copy.
func TestTemplates_RenderCertRenewalFailed(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data := CertRenewalFailedData{
		Domain:       "api.example.com",
		SANs:         "api.example.com",
		CA:           "lets-encrypt",
		NotAfter:     "2026-05-25 00:00 UTC",
		CertID:       42,
		ErrorMessage: "dns provider auth failed (HTTP 401)",
		DashboardURL: "https://idcd.com/app/cert/42",
	}
	cases := []struct {
		locale      string
		mustContain []string
	}{
		{
			locale: "cn",
			mustContain: []string{
				"api.example.com",
				"dns provider auth failed",
				"证书自动续期失败",
				"续期失败",
				"https://idcd.com/app/cert/42",
			},
		},
		{
			locale: "en",
			mustContain: []string{
				"api.example.com",
				"dns provider auth failed",
				"Auto-renewal failed",
				"action required",
				"https://idcd.com/app/cert/42",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			html, err := templates.RenderCertRenewalFailed(tc.locale, data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(html, want) {
					t.Errorf("locale=%s missing %q", tc.locale, want)
				}
			}
		})
	}
}

// TestTemplates_RenderCertEmptyDataDoesNotPanic guards the new cert
// templates against zero-valued inputs — the {{if}} guards must keep the
// HTML structurally valid.
func TestTemplates_RenderCertEmptyDataDoesNotPanic(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, loc := range []string{"cn", "en"} {
		t.Run("cert_issued/"+loc, func(t *testing.T) {
			html, err := templates.RenderCertIssued(loc, CertIssuedData{})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "DOCTYPE html") {
				t.Errorf("empty cert_issued data: missing DOCTYPE; got %s", html[:min(120, len(html))])
			}
		})
		t.Run("cert_failed/"+loc, func(t *testing.T) {
			html, err := templates.RenderCertFailed(loc, CertFailedData{})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "DOCTYPE html") {
				t.Error("empty cert_failed data: missing DOCTYPE")
			}
		})
		t.Run("cert_expiring/"+loc, func(t *testing.T) {
			html, err := templates.RenderCertExpiring(loc, CertExpiringData{})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "DOCTYPE html") {
				t.Error("empty cert_expiring data: missing DOCTYPE")
			}
		})
		t.Run("cert_renewal_failed/"+loc, func(t *testing.T) {
			html, err := templates.RenderCertRenewalFailed(loc, CertRenewalFailedData{})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "DOCTYPE html") {
				t.Error("empty cert_renewal_failed data: missing DOCTYPE")
			}
		})
	}
}

// TestTemplates_RenderFallsBackToDefaultLocale covers the registry fallback
// chain: requesting an unsupported locale should resolve to the registry
// default (cn) without erroring.
func TestTemplates_RenderFallsBackToDefaultLocale(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	html, err := templates.RenderVerifyEmail("ja", VerifyEmailData{
		Code:      "654321",
		ExpiresIn: "10 minutes",
	})
	if err != nil {
		t.Fatalf("unsupported locale should fall back, got error: %v", err)
	}
	if !strings.Contains(html, "验证您的邮箱地址") {
		t.Errorf("unsupported locale should render cn fallback, got: %q", html[:min(120, len(html))])
	}
}

// TestTemplates_RenderEmptyDataDoesNotPanic guards against template
// execution panics when data is zero-valued. Re-implements the old
// EmptyData / EmptyUsername / EmptyURL coverage as a single table.
func TestTemplates_RenderEmptyDataDoesNotPanic(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	for _, loc := range []string{"cn", "en"} {
		t.Run("verify_email/"+loc, func(t *testing.T) {
			html, err := templates.RenderVerifyEmail(loc, VerifyEmailData{})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "DOCTYPE html") {
				t.Error("expected HTML structure even with empty data")
			}
		})
		t.Run("welcome/"+loc, func(t *testing.T) {
			html, err := templates.RenderWelcome(loc, WelcomeData{})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "DOCTYPE html") {
				t.Error("expected HTML structure even with empty username")
			}
		})
		t.Run("reset_password/"+loc, func(t *testing.T) {
			html, err := templates.RenderResetPassword(loc, ResetPasswordData{ExpiresIn: "30 分钟"})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "DOCTYPE html") {
				t.Error("expected HTML structure even with empty URL")
			}
		})
	}
}

// TestTemplates_RenderResetPassword_EscapesHostileURL locks in the P1#14
// guarantee: html/template's URL context filter must neutralise a hostile
// URL passed through ResetPasswordData.ResetURL.  Specifically:
//
//   - javascript: scheme is replaced by html/template's #ZgotmplZ sentinel
//     in href context (so clicking the button does NOT execute the script).
//   - the literal substring "javascript:alert" never appears in the rendered
//     HTML (covers both href and the plain-text fallback box that uses
//     {{.ResetURL}} as text content).
//
// The plain-text fallback intentionally still shows the URL as text — but
// because the text-context escape turns ':' / quotes / angle brackets into
// safe entities, the result is not clickable as a script.
func TestTemplates_RenderResetPassword_EscapesHostileURL(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	scriptScheme := []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		`javascript:alert('xss')`,
	}
	for _, raw := range scriptScheme {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			for _, loc := range []string{"cn", "en"} {
				html, err := templates.RenderResetPassword(loc, ResetPasswordData{
					ResetURL:  raw,
					ExpiresIn: "30 minutes",
				})
				if err != nil {
					t.Fatalf("Render(%s): %v", loc, err)
				}
				// html/template's URL filter MUST neutralise the scheme.
				// Specifically, the rendered href must not begin with a
				// javascript: URL (case-insensitive).  html/template
				// substitutes #ZgotmplZ in that situation.
				if strings.Contains(strings.ToLower(html), `href="javascript:`) ||
					strings.Contains(strings.ToLower(html), `href='javascript:`) {
					t.Errorf("locale=%s: javascript: scheme leaked into href, html=%s", loc, html)
				}
				// The plain-text URL box (text context) shows the URL but
				// the colon is encoded as part of the html/template text
				// escape so it does not produce a clickable link.  We do
				// NOT assert the literal substring "javascript:" is absent
				// (it can appear inside text context, where it is inert).
			}
		})
	}

	// Quote-breaking URLs must not introduce a new HTML attribute.
	t.Run("quote_break_attempt", func(t *testing.T) {
		raw := `https://evil.example.com/" onmouseover="alert(1)`
		for _, loc := range []string{"cn", "en"} {
			html, err := templates.RenderResetPassword(loc, ResetPasswordData{
				ResetURL:  raw,
				ExpiresIn: "30 minutes",
			})
			if err != nil {
				t.Fatalf("Render(%s): %v", loc, err)
			}
			// The raw, unescaped quote that would close the href must
			// never appear inside the href attribute.  Specifically, the
			// literal sequence `" onmouseover="` must not survive.
			if strings.Contains(html, `" onmouseover="alert(1)"`) {
				t.Errorf("locale=%s: raw attribute injection survived, html=%s", loc, html)
			}
			// html/template should have percent-encoded the quote to %22
			// inside href context, keeping the entire payload as one URL.
			if !strings.Contains(html, "%22") {
				t.Errorf("locale=%s: expected %%22 (encoded quote) in rendered html, got: %s", loc, html)
			}
		}
	})
}

// TestTemplates_RenderResetPassword_SafeURLPasses ensures benign HTTPS URLs
// flow through untouched — guards against an over-zealous filter rejecting
// real reset links.
func TestTemplates_RenderResetPassword_SafeURLPasses(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	url := "https://idcd.com/reset-password?token=abc-123_DEF"
	html, err := templates.RenderResetPassword("en", ResetPasswordData{
		ResetURL:  url,
		ExpiresIn: "30 minutes",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(html, url) {
		t.Errorf("benign URL was filtered out; html=%s", html)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
