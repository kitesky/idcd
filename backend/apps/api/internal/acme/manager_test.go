package acme

import (
	"context"
	"errors"
	"testing"
)

// mockDomainChecker is a test double for DomainChecker.
type mockDomainChecker struct {
	verified map[string]bool
	err      error
}

func (m *mockDomainChecker) IsVerifiedDomain(ctx context.Context, host string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.verified[host], nil
}

func TestHostPolicy_VerifiedDomain(t *testing.T) {
	checker := &mockDomainChecker{
		verified: map[string]bool{
			"status.example.com": true,
		},
	}
	mgr := New(Config{CacheDir: t.TempDir()}, checker)

	if err := mgr.HostPolicy(context.Background(), "status.example.com"); err != nil {
		t.Errorf("expected nil error for verified domain, got: %v", err)
	}
}

func TestHostPolicy_UnverifiedDomain(t *testing.T) {
	checker := &mockDomainChecker{
		verified: map[string]bool{
			"status.example.com": true,
		},
	}
	mgr := New(Config{CacheDir: t.TempDir()}, checker)

	err := mgr.HostPolicy(context.Background(), "other.example.com")
	if err == nil {
		t.Error("expected error for unverified domain, got nil")
	}
}

func TestHostPolicy_NonExistentDomain(t *testing.T) {
	checker := &mockDomainChecker{
		verified: map[string]bool{},
	}
	mgr := New(Config{CacheDir: t.TempDir()}, checker)

	err := mgr.HostPolicy(context.Background(), "notexist.example.com")
	if err == nil {
		t.Error("expected error for non-existent domain, got nil")
	}
}

func TestHostPolicy_DBError(t *testing.T) {
	checker := &mockDomainChecker{
		err: errors.New("db connection error"),
	}
	mgr := New(Config{CacheDir: t.TempDir()}, checker)

	err := mgr.HostPolicy(context.Background(), "status.example.com")
	if err == nil {
		t.Error("expected error when DB returns error, got nil")
	}
}

func TestHostPolicy_EmptyHost(t *testing.T) {
	checker := &mockDomainChecker{verified: map[string]bool{}}
	mgr := New(Config{CacheDir: t.TempDir()}, checker)

	err := mgr.HostPolicy(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty host, got nil")
	}
}

func TestHostPolicy_CaseNormalization(t *testing.T) {
	checker := &mockDomainChecker{
		verified: map[string]bool{
			"status.example.com": true,
		},
	}
	mgr := New(Config{CacheDir: t.TempDir()}, checker)

	// Host with uppercase letters should be normalized.
	if err := mgr.HostPolicy(context.Background(), "STATUS.EXAMPLE.COM"); err != nil {
		t.Errorf("expected nil for uppercase host (normalized to lowercase), got: %v", err)
	}
}

func TestNew_PanicsOnNilChecker(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when checker is nil")
		}
	}()
	New(Config{}, nil)
}
