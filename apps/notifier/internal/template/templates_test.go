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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
