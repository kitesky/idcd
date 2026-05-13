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

	if templates.verifyEmail == nil {
		t.Error("Expected verifyEmail template to be parsed")
	}
	if templates.welcome == nil {
		t.Error("Expected welcome template to be parsed")
	}
	if templates.resetPassword == nil {
		t.Error("Expected resetPassword template to be parsed")
	}
}

func TestTemplates_RenderVerifyEmail(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	data := VerifyEmailData{
		Code:      "123456",
		ExpiresIn: "10 分钟",
	}

	html, err := templates.RenderVerifyEmail(data)
	if err != nil {
		t.Fatalf("Failed to render verify email template: %v", err)
	}

	if html == "" {
		t.Error("Expected non-empty HTML output")
	}

	// Check for essential content
	expectedContent := []string{
		"123456",           // verification code
		"10 分钟",           // expiration time
		"验证您的邮箱地址",      // title
		"idcd",             // brand name
		"DOCTYPE html",     // HTML doctype
	}

	for _, content := range expectedContent {
		if !strings.Contains(html, content) {
			t.Errorf("Expected content %q not found in rendered HTML", content)
		}
	}
}

func TestTemplates_RenderWelcome(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	data := WelcomeData{
		Username: "testuser",
	}

	html, err := templates.RenderWelcome(data)
	if err != nil {
		t.Fatalf("Failed to render welcome template: %v", err)
	}

	if html == "" {
		t.Error("Expected non-empty HTML output")
	}

	// Check for essential content
	expectedContent := []string{
		"testuser",         // username
		"欢迎加入 idcd",      // welcome title
		"多地网络诊断",        // feature description
		"实时监控告警",        // feature description
		"状态页与报告",        // feature description
		"开始使用 idcd",      // CTA button
		"DOCTYPE html",     // HTML doctype
	}

	for _, content := range expectedContent {
		if !strings.Contains(html, content) {
			t.Errorf("Expected content %q not found in rendered HTML", content)
		}
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

	html, err := templates.RenderResetPassword(data)
	if err != nil {
		t.Fatalf("Failed to render reset password template: %v", err)
	}

	if html == "" {
		t.Error("Expected non-empty HTML output")
	}

	// Check for essential content
	expectedContent := []string{
		"https://idcd.com/reset-password?token=abc123", // reset URL
		"30 分钟",                                       // expiration time
		"密码重置请求",                                     // title
		"重置密码",                                        // button text
		"安全提醒",                                        // security section
		"此链接仅可使用一次",                                 // security warning
		"DOCTYPE html",                                // HTML doctype
	}

	for _, content := range expectedContent {
		if !strings.Contains(html, content) {
			t.Errorf("Expected content %q not found in rendered HTML", content)
		}
	}
}

func TestTemplates_RenderVerifyEmail_EmptyData(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	// Test with empty data
	data := VerifyEmailData{
		Code:      "",
		ExpiresIn: "",
	}

	html, err := templates.RenderVerifyEmail(data)
	if err != nil {
		t.Fatalf("Failed to render template with empty data: %v", err)
	}

	// Should still render HTML structure even with empty data
	if !strings.Contains(html, "DOCTYPE html") {
		t.Error("Expected HTML structure even with empty data")
	}
}

func TestTemplates_RenderWelcome_EmptyUsername(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	// Test with empty username
	data := WelcomeData{
		Username: "",
	}

	html, err := templates.RenderWelcome(data)
	if err != nil {
		t.Fatalf("Failed to render template with empty username: %v", err)
	}

	// Should still render HTML structure
	if !strings.Contains(html, "DOCTYPE html") {
		t.Error("Expected HTML structure even with empty username")
	}
	if !strings.Contains(html, "欢迎加入 idcd") {
		t.Error("Expected welcome title in rendered HTML")
	}
}

func TestTemplates_RenderResetPassword_EmptyURL(t *testing.T) {
	templates, err := New()
	if err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	// Test with empty reset URL
	data := ResetPasswordData{
		ResetURL:  "",
		ExpiresIn: "30 分钟",
	}

	html, err := templates.RenderResetPassword(data)
	if err != nil {
		t.Fatalf("Failed to render template with empty URL: %v", err)
	}

	// Should still render HTML structure
	if !strings.Contains(html, "DOCTYPE html") {
		t.Error("Expected HTML structure even with empty URL")
	}
	if !strings.Contains(html, "密码重置请求") {
		t.Error("Expected reset password title in rendered HTML")
	}
}