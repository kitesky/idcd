// Package template provides email templates with go:embed for static assets.
package template

import (
	"bytes"
	_ "embed"
	"html/template"

	"github.com/kite365/idcd/lib/shared/apperr"
)

//go:embed verify_email.html
var verifyEmailTemplate string

//go:embed welcome.html
var welcomeTemplate string

//go:embed reset_password.html
var resetPasswordTemplate string

// TemplateData holds common template variables.
type TemplateData map[string]any

// VerifyEmailData holds variables for the email verification template.
type VerifyEmailData struct {
	Code      string // 6-digit OTP code
	ExpiresIn string // human-readable duration (e.g., "10 分钟")
}

// WelcomeData holds variables for the welcome email template.
type WelcomeData struct {
	Username string // user's display name or email
}

// ResetPasswordData holds variables for the password reset template.
type ResetPasswordData struct {
	ResetURL  string // password reset URL
	ExpiresIn string // human-readable duration (e.g., "30 分钟")
}

// Templates provides access to all email templates.
type Templates struct {
	verifyEmail   *template.Template
	welcome       *template.Template
	resetPassword *template.Template
}

// New creates a new Templates instance with parsed templates.
func New() (*Templates, error) {
	t := &Templates{}

	var err error

	// Parse verify email template
	t.verifyEmail, err = template.New("verify_email").Parse(verifyEmailTemplate)
	if err != nil {
		return nil, apperr.Internal("解析邮箱验证模板失败", err)
	}

	// Parse welcome template
	t.welcome, err = template.New("welcome").Parse(welcomeTemplate)
	if err != nil {
		return nil, apperr.Internal("解析欢迎邮件模板失败", err)
	}

	// Parse reset password template
	t.resetPassword, err = template.New("reset_password").Parse(resetPasswordTemplate)
	if err != nil {
		return nil, apperr.Internal("解析密码重置模板失败", err)
	}

	return t, nil
}

// RenderVerifyEmail renders the email verification template.
func (t *Templates) RenderVerifyEmail(data VerifyEmailData) (string, error) {
	var buf bytes.Buffer
	if err := t.verifyEmail.Execute(&buf, data); err != nil {
		return "", apperr.Internal("渲染邮箱验证模板失败", err)
	}
	return buf.String(), nil
}

// RenderWelcome renders the welcome email template.
func (t *Templates) RenderWelcome(data WelcomeData) (string, error) {
	var buf bytes.Buffer
	if err := t.welcome.Execute(&buf, data); err != nil {
		return "", apperr.Internal("渲染欢迎邮件模板失败", err)
	}
	return buf.String(), nil
}

// RenderResetPassword renders the password reset email template.
func (t *Templates) RenderResetPassword(data ResetPasswordData) (string, error) {
	var buf bytes.Buffer
	if err := t.resetPassword.Execute(&buf, data); err != nil {
		return "", apperr.Internal("渲染密码重置模板失败", err)
	}
	return buf.String(), nil
}