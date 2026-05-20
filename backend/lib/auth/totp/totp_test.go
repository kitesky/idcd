package totp

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
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
	issuer := "My Issuer"
	account := "user@example.com"
	secret := "TESTSECRET"
	raw := OTPAuthURL(issuer, account, secret)
	if !strings.HasPrefix(raw, "otpauth://totp/") {
		t.Errorf("expected otpauth:// URL, got %q", raw)
	}
	if strings.Contains(raw, " ") {
		t.Errorf("URL must not contain unencoded spaces, got %q", raw)
	}
	// Parse and verify query params round-trip correctly.
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("OTPAuthURL produced unparseable URL: %v", err)
	}
	q := parsed.Query()
	if q.Get("secret") != secret {
		t.Errorf("secret: want %q, got %q", secret, q.Get("secret"))
	}
	if q.Get("issuer") != issuer {
		t.Errorf("issuer: want %q, got %q", issuer, q.Get("issuer"))
	}
}

type memReplay struct {
	mu   sync.Mutex
	seen map[string]struct{}
	fail error
}

func (m *memReplay) Mark(_ context.Context, key string, _ time.Duration) (bool, error) {
	if m.fail != nil {
		return false, m.fail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.seen == nil {
		m.seen = map[string]struct{}{}
	}
	if _, dup := m.seen[key]; dup {
		return false, nil
	}
	m.seen[key] = struct{}{}
	return true, nil
}

func TestValidator_ClockInjection(t *testing.T) {
	secret, _ := GenerateSecret()
	fixed := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	code, _ := GenerateCode(secret, fixed)

	v := &Validator{Now: func() time.Time { return fixed }}
	ok, err := v.Validate(context.Background(), secret, "", code)
	if err != nil || !ok {
		t.Fatalf("expected fixed-clock validate to pass, got ok=%v err=%v", ok, err)
	}

	// Time-shift well outside the ±1 window — should fail.
	v.Now = func() time.Time { return fixed.Add(5 * time.Minute) }
	ok, _ = v.Validate(context.Background(), secret, "", code)
	if ok {
		t.Fatal("expected stale code under shifted clock to fail")
	}
}

func TestValidator_ReplayProtection(t *testing.T) {
	secret, _ := GenerateSecret()
	fixed := time.Now()
	code, _ := GenerateCode(secret, fixed)
	r := &memReplay{}
	v := &Validator{Now: func() time.Time { return fixed }, Replay: r}

	ok, err := v.Validate(context.Background(), secret, "user-1", code)
	if err != nil || !ok {
		t.Fatalf("first validation expected ok, got ok=%v err=%v", ok, err)
	}

	ok, err = v.Validate(context.Background(), secret, "user-1", code)
	if err != nil {
		t.Fatalf("replay attempt unexpected err: %v", err)
	}
	if ok {
		t.Fatal("replay attempt should be rejected")
	}

	// Different user with the same code is allowed (replay is scoped to user).
	ok, err = v.Validate(context.Background(), secret, "user-2", code)
	if err != nil || !ok {
		t.Fatalf("different user with same code expected ok, got ok=%v err=%v", ok, err)
	}
}

func TestValidator_ReplayStoreErrorIsSurfaced(t *testing.T) {
	secret, _ := GenerateSecret()
	fixed := time.Now()
	code, _ := GenerateCode(secret, fixed)
	r := &memReplay{fail: errors.New("redis down")}
	v := &Validator{Now: func() time.Time { return fixed }, Replay: r}

	ok, err := v.Validate(context.Background(), secret, "user-1", code)
	if ok {
		t.Fatal("expected ok=false when replay store errors")
	}
	if err == nil {
		t.Fatal("expected an error to surface from replay store")
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
