// server_jwt_blocklist_test.go — smoke tests for JWT JTI blocklist wiring.
//
// P1#11 added jwt.WithBlocklist + RedisBlocklist, but the production server
// kept calling the legacy NewService(cfg) which silently skipped revocation.
// That meant refresh / logout could not actually kill a leaked token across
// API replicas — Verify would still accept it until natural exp.
//
// These tests pin the wiring at the Server boundary so a future refactor
// can't quietly revert to the legacy code path.
//
// We don't drive HTTP end-to-end; the contract under test is purely
// "buildJWTService returns a Service whose blocklist is actually used".
// That contract is observable via behavior: RevokeToken is a no-op when
// no blocklist is wired (returns nil and the token still verifies), but
// when one IS wired the same token verifies as ErrTokenRevoked afterwards.

package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/config"
)

// newJWTTestServer builds a Server with only the fields buildJWTService
// touches — no router, no DB, no pgx pool. Pass nil to skip the redis
// wiring (covers the legacy "no blocklist" branch).
func newJWTTestServer(t *testing.T, rdb redis.UniversalClient) *Server {
	t.Helper()
	return &Server{
		redis:  rdb,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		config: &config.Config{
			JWT: config.JWTConfig{
				// 32+ chars; jwt.NewServiceWithOptions rejects shorter secrets.
				Secret: "test-secret-aaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
	}
}

// TestBuildJWTService_WiresRedisBlocklist is the load-bearing assertion:
// when Redis is configured, the returned Service must consult the
// blocklist on Verify. We prove that by issuing a token, revoking it via
// the Service's own RevokeToken (which is a no-op when no blocklist is
// wired), and asserting Verify now returns Unauthorized "token revoked".
func TestBuildJWTService_WiresRedisBlocklist(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis start: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	srv := newJWTTestServer(t, rdb)

	svc, err := srv.buildJWTService()
	if err != nil {
		t.Fatalf("buildJWTService: %v", err)
	}
	if svc == nil {
		t.Fatal("buildJWTService returned nil Service")
	}

	// Sign a normal access token. Use a non-trivial expiry so blocklistTTL
	// stays positive after RevokeToken parses it.
	tok, err := svc.Sign("user-1", "session-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Sanity: token verifies before revocation.
	if _, err := svc.Verify(tok); err != nil {
		t.Fatalf("pre-revoke verify failed: %v", err)
	}

	// Revoke. If buildJWTService failed to wire a blocklist this is a
	// silent no-op and the post-revoke verify below would still pass —
	// which is exactly the regression we're guarding against.
	if err := svc.RevokeToken(context.Background(), tok); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Post-revoke: must be rejected.
	_, err = svc.Verify(tok)
	if err == nil {
		t.Fatal("expected post-revoke Verify to fail, got nil — blocklist NOT wired")
	}
	var apErr *apperr.Error
	if !errors.As(err, &apErr) {
		t.Fatalf("expected *apperr.Error, got %T: %v", err, err)
	}
	if apErr.Code != apperr.CodeUnauthorized {
		t.Fatalf("expected CodeUnauthorized, got %s (%s)", apErr.Code, apErr.Message)
	}
	if apErr.Message != "token revoked" {
		t.Fatalf("expected message %q, got %q", "token revoked", apErr.Message)
	}
}

// TestBuildJWTService_NoRedisIsLegacyBehavior covers the fallback path
// used by minimal test harnesses / dev configs without Redis: no blocklist
// is attached and RevokeToken is a no-op (matches lib/auth/jwt contract).
func TestBuildJWTService_NoRedisIsLegacyBehavior(t *testing.T) {
	srv := newJWTTestServer(t, nil)

	svc, err := srv.buildJWTService()
	if err != nil {
		t.Fatalf("buildJWTService: %v", err)
	}
	if svc == nil {
		t.Fatal("buildJWTService returned nil Service")
	}

	tok, err := svc.Sign("user-1", "session-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := svc.RevokeToken(context.Background(), tok); err != nil {
		t.Fatalf("revoke (no-blocklist path): %v", err)
	}
	// Without a blocklist Verify still succeeds — that's the documented
	// behavior of the legacy NewService path that we explicitly preserve.
	if _, err := svc.Verify(tok); err != nil {
		t.Fatalf("post-revoke verify (no-blocklist) failed: %v", err)
	}
}

// TestBuildJWTService_RejectsShortSecret pins the Validation error path so
// a future change that swaps NewServiceWithOptions for a permissive
// constructor would surface in CI rather than at first boot.
func TestBuildJWTService_RejectsShortSecret(t *testing.T) {
	srv := newJWTTestServer(t, nil)
	srv.config.JWT.Secret = "too-short" // <32 chars

	_, err := srv.buildJWTService()
	if err == nil {
		t.Fatal("expected validation error for short secret, got nil")
	}
}
