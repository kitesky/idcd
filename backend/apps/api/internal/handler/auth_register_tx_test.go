package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// --- mocks specific to the tx-aware Register path ----------------------------

// failingQuerier wraps mockAuthQuerier so individual methods can be made to
// fail mid-tx without polluting the shared mock. The wrapper captures whether
// each mutating call was attempted (regardless of pgxmock tx outcome) so
// tests can assert "method was called" independent of commit/rollback.
type failingQuerier struct {
	*mockAuthQuerier
	failCreateUser    error
	failCreateUserOTP error

	createUserCalls    int
	createUserOTPCalls int
}

func (f *failingQuerier) CreateUser(ctx context.Context, arg idcdmain.CreateUserParams) (idcdmain.User, error) {
	f.createUserCalls++
	if f.failCreateUser != nil {
		return idcdmain.User{}, f.failCreateUser
	}
	return f.mockAuthQuerier.CreateUser(ctx, arg)
}

func (f *failingQuerier) CreateUserOTP(ctx context.Context, arg idcdmain.CreateUserOTPParams) (idcdmain.UserOtp, error) {
	f.createUserOTPCalls++
	if f.failCreateUserOTP != nil {
		return idcdmain.UserOtp{}, f.failCreateUserOTP
	}
	return f.mockAuthQuerier.CreateUserOTP(ctx, arg)
}

// failingSession is a SessionStorer whose Store can be configured to fail.
// We capture the call count so tests can assert ordering (CreateUser runs
// before Store, so a Store failure proves the rollback path covers the user
// row too).
type failingSession struct {
	failStore  error
	storeCalls int
}

func (f *failingSession) Store(_ context.Context, _, _ string, _ time.Duration) error {
	f.storeCalls++
	return f.failStore
}

func (f *failingSession) Delete(_ context.Context, _ string) error { return nil }

// newTxTestHandler builds an AuthHandler wired with a pgxmock tx pool and a
// querier factory that returns the supplied querier (ignoring the tx — the
// querier mocks the DB layer, the pgxmock pool models the tx lifecycle).
func newTxTestHandler(t *testing.T, q AuthQuerier) (*AuthHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	t.Cleanup(func() { mockPool.Close() })

	h := NewAuthHandler(q, &mockJWT{token: "tok.tx.test"}, &mockSession{}, "test-otp-secret-32bytes-minimum!!").
		WithTxPool(mockPool, func(_ pgx.Tx) AuthQuerier { return q })
	return h, mockPool
}

// --- tests ------------------------------------------------------------------

// TestRegister_Tx_HappyPath_EmailSentAfterCommit asserts the canonical flow:
// CreateUser + OTP + session all run inside a single tx, the tx commits, and
// only after commit does enqueueVerifyEmail fire. This is the contract the
// rollback tests depend on — if the email fires inside the tx, a later
// rollback would silently leak a "your account was created" email.
func TestRegister_Tx_HappyPath_EmailSentAfterCommit(t *testing.T) {
	q := newMockAuthQuerier()
	h, mockPool := newTxTestHandler(t, q)
	eq := &mockEnqueuer{}
	h = h.WithEnqueuer(eq)

	mockPool.ExpectBegin()
	mockPool.ExpectCommit()

	body := `{"email":"tx-happy@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Fatalf("pgxmock expectations: %v", err)
	}
	if _, ok := q.users["tx-happy@example.com"]; !ok {
		t.Errorf("user not stored after happy-path commit")
	}
	if len(q.otps) != 1 {
		t.Errorf("expected exactly 1 OTP after happy-path commit, got %d", len(q.otps))
	}
	if len(eq.tasks) != 1 {
		t.Fatalf("expected 1 enqueued email after commit, got %d", len(eq.tasks))
	}
	if eq.tasks[0].taskType != taskSendVerifyEmail {
		t.Errorf("expected %q, got %q", taskSendVerifyEmail, eq.tasks[0].taskType)
	}
}

// TestRegister_Tx_RollbackOnOTPFailure asserts that when CreateUserOTP fails
// inside the tx, the whole unit of work rolls back and NO verification email
// is enqueued. The pgxmock pool's ExpectBegin → ExpectRollback contract is
// what proves the user row gets rolled back at the DB layer (in a real PG
// session, the user_otp failure causes the entire tx — including the user
// INSERT — to disappear).
func TestRegister_Tx_RollbackOnOTPFailure(t *testing.T) {
	base := newMockAuthQuerier()
	q := &failingQuerier{
		mockAuthQuerier:   base,
		failCreateUserOTP: errors.New("simulated user_otp insert failure"),
	}
	h, mockPool := newTxTestHandler(t, q)
	eq := &mockEnqueuer{}
	h = h.WithEnqueuer(eq)

	mockPool.ExpectBegin()
	mockPool.ExpectRollback()

	body := `{"email":"tx-otp-fail@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code < 500 {
		t.Fatalf("expected 5xx on OTP failure, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mockPool.ExpectationsWereMet(); err != nil {
		// This is the load-bearing assertion: ExpectBegin + ExpectRollback
		// being satisfied means the handler asked the pool for a tx AND told
		// it to roll back. In production that erases the users row.
		t.Fatalf("pgxmock expectations (tx must roll back): %v", err)
	}
	if q.createUserCalls != 1 {
		t.Errorf("expected CreateUser called once, got %d", q.createUserCalls)
	}
	if q.createUserOTPCalls != 1 {
		t.Errorf("expected CreateUserOTP called once, got %d", q.createUserOTPCalls)
	}
	if len(eq.tasks) != 0 {
		t.Errorf("verification email must NOT be enqueued when tx rolls back, got %d tasks", len(eq.tasks))
	}
}

// TestRegister_Tx_SessionFailure_PostCommit pins the outbox-lite contract:
// session.Store (Redis) runs after the pg tx commits, so a Redis blip leaves
// the user row persisted and surfaces as 5xx. The user recovers via /login.
// Email is not enqueued because we don't want "account created" + "login
// broken" arriving together.
func TestRegister_Tx_SessionFailure_PostCommit(t *testing.T) {
	q := newMockAuthQuerier()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	t.Cleanup(func() { mockPool.Close() })

	sess := &failingSession{failStore: errors.New("session store down")}
	h := NewAuthHandler(q, &mockJWT{token: "t"}, sess, "test-otp-secret-32bytes-minimum!!").
		WithTxPool(mockPool, func(_ pgx.Tx) AuthQuerier { return q })
	eq := &mockEnqueuer{}
	h = h.WithEnqueuer(eq)

	mockPool.ExpectBegin()
	mockPool.ExpectCommit()

	body := `{"email":"tx-sess-fail@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code < 500 {
		t.Fatalf("expected 5xx on session-store failure, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Fatalf("pgxmock expectations (tx must commit BEFORE session step): %v", err)
	}
	if sess.storeCalls != 1 {
		t.Errorf("expected sess.Store called once (after commit), got %d", sess.storeCalls)
	}
	if len(eq.tasks) != 0 {
		t.Errorf("verification email must NOT be enqueued when post-commit step fails, got %d tasks", len(eq.tasks))
	}
	// The pg row is committed — that's the explicit contract trade-off. The
	// user can recover by signing in once Redis is healthy.
	if _, ok := q.users["tx-sess-fail@example.com"]; !ok {
		t.Errorf("expected user row to remain committed after Redis-side failure")
	}
}

// TestRegister_Tx_DuplicateEmail_PreservesErrCode asserts the wrap chain
// preserves repository.ErrDuplicate so the handler still emits the dedicated
// AccountEmailTaken errcode (HTTP 409) rather than a generic 500. Without
// this, the tx refactor would silently downgrade an already-correct 409 to
// a 500 — a regression that's easy to ship and hard to spot.
func TestRegister_Tx_DuplicateEmail_PreservesErrCode(t *testing.T) {
	q := newMockAuthQuerier()
	h, mockPool := newTxTestHandler(t, q)

	body := `{"email":"dup@example.com","password":"Password123"}`

	// First registration: happy path.
	mockPool.ExpectBegin()
	mockPool.ExpectCommit()
	req1 := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.Register(rr1, req1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first register: expected 201, got %d: %s", rr1.Code, rr1.Body.String())
	}

	// Second registration with same email: must return 409, NOT 500.
	mockPool.ExpectBegin()
	mockPool.ExpectRollback()
	req2 := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.Register(rr2, req2)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Fatalf("pgxmock expectations: %v", err)
	}
}

// TestRegister_Tx_EnqueueFailureDoesNotFailRequest asserts the outbox-lite
// contract: once the tx commits, an email enqueue failure must NOT roll
// back the account (it's already committed) and must NOT surface a 5xx to
// the user. The account exists; the worst-case is the user has to click
// "resend verification email". Treating an email failure as a hard error
// would be strictly worse for the user.
func TestRegister_Tx_EnqueueFailureDoesNotFailRequest(t *testing.T) {
	q := newMockAuthQuerier()
	h, mockPool := newTxTestHandler(t, q)
	eq := &failingEnqueuer{err: errors.New("queue down")}
	h = h.WithEnqueuer(eq)

	mockPool.ExpectBegin()
	mockPool.ExpectCommit()

	body := `{"email":"enq-fail@example.com","password":"Password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 even when enqueue fails (account is committed), got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Fatalf("pgxmock expectations: %v", err)
	}
	if eq.calls != 1 {
		t.Errorf("expected enqueue attempted once, got %d", eq.calls)
	}
}

// failingEnqueuer counts attempts and returns the configured error so the
// outbox-lite swallow-and-continue path can be observed.
type failingEnqueuer struct {
	err   error
	calls int
}

func (f *failingEnqueuer) EnqueueTask(_ context.Context, _ string, _ []byte, _ string) error {
	f.calls++
	return f.err
}
