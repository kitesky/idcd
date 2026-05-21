package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// failingOAuthQuerier wraps mockOAuthQuerier so individual writes can be
// induced to fail mid-tx without polluting the shared mock used by happy-path
// tests. Mirrors failingQuerier in auth_register_tx_test.go.
type failingOAuthQuerier struct {
	*mockOAuthQuerier
	failCreateUser           error
	failCreateUserCredential error

	createUserCalls       int
	createCredentialCalls int
}

func (f *failingOAuthQuerier) CreateUser(ctx context.Context, arg idcdmain.CreateUserParams) (idcdmain.User, error) {
	f.createUserCalls++
	if f.failCreateUser != nil {
		return idcdmain.User{}, f.failCreateUser
	}
	return f.mockOAuthQuerier.CreateUser(ctx, arg)
}

func (f *failingOAuthQuerier) CreateUserCredential(ctx context.Context, arg idcdmain.CreateUserCredentialParams) (idcdmain.UserCredential, error) {
	f.createCredentialCalls++
	if f.failCreateUserCredential != nil {
		return idcdmain.UserCredential{}, f.failCreateUserCredential
	}
	return f.mockOAuthQuerier.CreateUserCredential(ctx, arg)
}

func newOAuthTxHandler(t *testing.T, q OAuthQuerier, dingtalkSrv *httptest.Server) (*OAuthHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	h := newTestOAuthHandler(q.(*failingOAuthQuerier).mockOAuthQuerier, newMockStateStore(), dingtalkSrv, nil).
		WithTxPool(pool, func(_ pgx.Tx) OAuthQuerier { return q })
	return h, pool
}

// TestOAuthCallback_Tx_RollbackOnCredentialFailure asserts that when
// CreateUserCredential fails mid-tx, the surrounding CreateUser write rolls
// back too — leaving the users mock empty. This is the P1-10 contract for
// findOrCreateOAuthUser: a half-provisioned OAuth account (user row with no
// credential) would lock the user out permanently because the next callback
// can neither create the user (email/id collision) nor authenticate against
// the missing credential.
func TestOAuthCallback_Tx_RollbackOnCredentialFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"accessToken":"dt_access_tok"}`)
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"openId":"dt_rollback_001","nick":"回滚测试","email":"rb@corp.com"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	inner := newMockOAuthQuerier()
	q := &failingOAuthQuerier{
		mockOAuthQuerier:         inner,
		failCreateUserCredential: errors.New("credential write boom"),
	}

	h, pool := newOAuthTxHandler(t, q, srv)
	// Reset state store to the one used by the handler; reuse the existing
	// store on h.stateStore so the callback can find the state we set.
	stateKey := oauthStateKey(providerDingTalk, "rollback-state")
	_ = h.stateStore.Set(context.Background(), stateKey, "1", oauthStateTTL)

	pool.ExpectBegin()
	pool.ExpectRollback()

	req := httptest.NewRequest(http.MethodGet,
		"/v1/auth/dingtalk/callback?code=code&state=rollback-state", nil)
	rr := httptest.NewRecorder()
	h.DingTalkCallback(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 (provision failure), got %d: %s", rr.Code, rr.Body.String())
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatalf("pgxmock expectations not met: %v", err)
	}
	// Both writes were attempted inside the tx — CreateUser succeeded in the
	// mock but the tx layer rolled it back at the DB.
	if q.createUserCalls != 1 {
		t.Errorf("expected CreateUser to be called exactly once, got %d", q.createUserCalls)
	}
	if q.createCredentialCalls != 1 {
		t.Errorf("expected CreateUserCredential to be called exactly once, got %d", q.createCredentialCalls)
	}
	// The mock's users map captures the in-memory state of CreateUser. The
	// real PG would have undone it on ROLLBACK; the mock can't undo, so we
	// verify the contract via pgxmock's ExpectRollback instead. The point of
	// this assertion is to guarantee the test exercises the failure path: a
	// silent CreateUserCredential success would leave the mock with a credential.
	if len(inner.creds) != 0 {
		t.Errorf("expected no credentials to be persisted on rollback, got %d", len(inner.creds))
	}
}

// TestOAuthCallback_Tx_RollbackOnUserFailure mirrors the above for the case
// where CreateUser itself fails — pgxmock should still see Begin + Rollback
// (we never reach the credential write).
func TestOAuthCallback_Tx_RollbackOnUserFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"accessToken":"tok"}`)
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"openId":"dt_userfail_001","nick":"fail","email":"f@corp.com"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	inner := newMockOAuthQuerier()
	q := &failingOAuthQuerier{
		mockOAuthQuerier: inner,
		failCreateUser:   errors.New("user write boom"),
	}

	h, pool := newOAuthTxHandler(t, q, srv)
	stateKey := oauthStateKey(providerDingTalk, "uf-state")
	_ = h.stateStore.Set(context.Background(), stateKey, "1", oauthStateTTL)

	pool.ExpectBegin()
	pool.ExpectRollback()

	req := httptest.NewRequest(http.MethodGet,
		"/v1/auth/dingtalk/callback?code=c&state=uf-state", nil)
	rr := httptest.NewRecorder()
	h.DingTalkCallback(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "provision") {
		t.Errorf("expected provision failure in body, got %q", rr.Body.String())
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatalf("pgxmock expectations not met: %v", err)
	}
	if q.createCredentialCalls != 0 {
		t.Errorf("expected CreateUserCredential NOT to be called when CreateUser fails, got %d", q.createCredentialCalls)
	}
	if len(inner.users) != 0 {
		t.Errorf("expected no users persisted on rollback, got %d", len(inner.users))
	}
}
