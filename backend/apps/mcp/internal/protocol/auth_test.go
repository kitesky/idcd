package protocol

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHashToken_Stable(t *testing.T) {
	a := HashToken("idcd_mcp_abc123")
	b := HashToken("idcd_mcp_abc123")
	if a != b {
		t.Fatalf("HashToken non-deterministic: %q vs %q", a, b)
	}
	if len(a) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars (sha256)", len(a))
	}
	c := HashToken("idcd_mcp_abc124")
	if a == c {
		t.Error("HashToken collided for distinct inputs")
	}
}

func TestExtractBearerToken_Variants(t *testing.T) {
	good := "idcd_mcp_" + strings.Repeat("a", 32)
	cases := []struct {
		name   string
		header string
		want   string
		err    error
	}{
		{"missing", "", "", ErrTokenMissing},
		{"no Bearer prefix", good, "", ErrTokenMalformed},
		{"empty Bearer", "Bearer ", "", ErrTokenMalformed},
		{"wrong prefix", "Bearer foo_bar_" + strings.Repeat("a", 32), "", ErrTokenMalformed},
		{"too short", "Bearer idcd_mcp_abc", "", ErrTokenMalformed},
		{"pat fallback", "Bearer idcd_pat_" + strings.Repeat("b", 32), "idcd_pat_" + strings.Repeat("b", 32), nil},
		{"mcp ok", "Bearer " + good, good, nil},
		{"trailing whitespace tolerated", "Bearer " + good + "   ", good, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/messages", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			got, err := extractBearerToken(req)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if got != tc.want {
				t.Errorf("token = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStaticTokenValidator_HappyPath(t *testing.T) {
	raw := "idcd_mcp_" + strings.Repeat("c", 32)
	v := NewStaticTokenValidator(StaticTokenRecord{
		RawToken:  raw,
		TokenID:   "mcpt_1",
		UserID:    "usr_1",
		Workspace: "wks_1",
		Type:      "workspace",
		Scopes:    []string{"tools:call", "tools:list"},
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})

	p, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("Validate err: %v", err)
	}
	if p.UserID != "usr_1" || p.WorkspaceID != "wks_1" || p.TokenType != "workspace" {
		t.Errorf("principal = %+v", p)
	}
	if len(p.Scopes) != 2 {
		t.Errorf("scopes = %v, want 2 entries", p.Scopes)
	}
}

func TestStaticTokenValidator_NotFound(t *testing.T) {
	v := NewStaticTokenValidator()
	_, err := v.Validate(context.Background(), "idcd_mcp_"+strings.Repeat("z", 32))
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("err = %v, want ErrTokenNotFound", err)
	}
}

func TestStaticTokenValidator_Expired(t *testing.T) {
	raw := "idcd_mcp_" + strings.Repeat("d", 32)
	v := NewStaticTokenValidator(StaticTokenRecord{
		RawToken:  raw,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	_, err := v.Validate(context.Background(), raw)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestStaticTokenValidator_Revoked(t *testing.T) {
	raw := "idcd_mcp_" + strings.Repeat("e", 32)
	v := NewStaticTokenValidator(StaticTokenRecord{
		RawToken:  raw,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Revoked:   true,
	})
	_, err := v.Validate(context.Background(), raw)
	if !errors.Is(err, ErrTokenRevoked) {
		t.Errorf("err = %v, want ErrTokenRevoked", err)
	}
}

func TestStaticTokenValidator_FixedClock(t *testing.T) {
	raw := "idcd_mcp_" + strings.Repeat("f", 32)
	v := NewStaticTokenValidator(StaticTokenRecord{
		RawToken:  raw,
		ExpiresAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})

	// Before expiry → ok.
	v.SetNow(func() time.Time { return time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC) })
	if _, err := v.Validate(context.Background(), raw); err != nil {
		t.Errorf("before expiry: err = %v, want nil", err)
	}

	// At expiry boundary → expired (strict before).
	v.SetNow(func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) })
	if _, err := v.Validate(context.Background(), raw); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("at expiry: err = %v, want ErrTokenExpired", err)
	}
}

func TestPrincipalFromContext_AbsentReturnsNil(t *testing.T) {
	if p := PrincipalFromContext(context.Background()); p != nil {
		t.Errorf("got %+v, want nil", p)
	}
}

func TestPrincipalFromContext_RoundTrip(t *testing.T) {
	want := &Principal{UserID: "u", TokenID: "t"}
	ctx := withPrincipal(context.Background(), want)
	if got := PrincipalFromContext(ctx); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
