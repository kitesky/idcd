package totp

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateCode_SixDigits(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	code, err := GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %q", code)
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			t.Errorf("non-digit character %q in code %q", ch, code)
		}
	}
}

func TestValidateCode_CurrentCodeTrue(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	code, err := GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	ok, err := ValidateCode(secret, code)
	if err != nil {
		t.Fatalf("ValidateCode: %v", err)
	}
	if !ok {
		t.Errorf("expected current code to validate")
	}
}

func TestValidateCode_WrongCodeFalse(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	ok, err := ValidateCode(secret, "000000")
	if err != nil {
		t.Fatalf("ValidateCode: %v", err)
	}
	real, _ := GenerateCode(secret, time.Now())
	if real == "000000" {
		t.Skip("code happened to be 000000, skipping")
	}
	if ok {
		t.Errorf("expected wrong code to fail validation")
	}
}

func TestOTPAuthURL(t *testing.T) {
	issuer := "idcd"
	account := "user@example.com"
	secret := "TESTSECRET"
	url := OTPAuthURL(issuer, account, secret)
	if !strings.HasPrefix(url, "otpauth://totp/") {
		t.Errorf("expected otpauth:// URL, got %q", url)
	}
	if !strings.Contains(url, issuer) {
		t.Errorf("URL should contain issuer %q, got %q", issuer, url)
	}
	if !strings.Contains(url, account) {
		t.Errorf("URL should contain account %q, got %q", account, url)
	}
	if !strings.Contains(url, secret) {
		t.Errorf("URL should contain secret %q, got %q", secret, url)
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	codes, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if len(codes) != 8 {
		t.Errorf("expected 8 backup codes, got %d", len(codes))
	}
	for _, c := range codes {
		if len(c) != 8 {
			t.Errorf("expected 8-char backup code, got %q", c)
		}
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate backup code %q", c)
		}
		seen[c] = true
	}
}
