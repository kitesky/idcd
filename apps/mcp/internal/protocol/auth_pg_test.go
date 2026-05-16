package protocol

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

// fixedNow returns a stable clock for deterministic expiry tests.
func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newPGTestValidator(t *testing.T) (pgxmock.PgxPoolIface, *PGTokenValidator) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	return mock, newPGValidatorWithQuerier(mock)
}

func TestPGTokenValidator_Hit(t *testing.T) {
	mock, v := newPGTestValidator(t)
	defer mock.Close()

	raw := "idcd_pat_" + strings.Repeat("a", 32)
	hash := HashToken(raw)
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	v.SetNow(fixedNow(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)))

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens WHERE token_hash = \$1`).
		WithArgs(hash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("pat_abc", "usr_42", &exp))

	p, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if p == nil {
		t.Fatal("Validate returned nil principal")
	}
	if p.TokenID != "pat_abc" || p.UserID != "usr_42" || p.TokenType != "personal" {
		t.Errorf("principal = %+v, want {TokenID:pat_abc UserID:usr_42 TokenType:personal}", p)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPGTokenValidator_HitNullExpiry(t *testing.T) {
	// v1 PAT may have nullable expires_at. Validator must accept it (treat
	// as "never expires") so existing API-issued PATs keep working. v2-S3
	// will tighten the schema to NOT NULL.
	mock, v := newPGTestValidator(t)
	defer mock.Close()

	raw := "idcd_pat_" + strings.Repeat("b", 32)
	hash := HashToken(raw)

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens WHERE token_hash = \$1`).
		WithArgs(hash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("pat_null_exp", "usr_7", (*time.Time)(nil)))

	p, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if p.TokenID != "pat_null_exp" || p.UserID != "usr_7" {
		t.Errorf("principal = %+v", p)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPGTokenValidator_Miss(t *testing.T) {
	mock, v := newPGTestValidator(t)
	defer mock.Close()

	raw := "idcd_pat_" + strings.Repeat("c", 32)
	hash := HashToken(raw)

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens WHERE token_hash = \$1`).
		WithArgs(hash).
		WillReturnError(pgx.ErrNoRows)

	p, err := v.Validate(context.Background(), raw)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("err = %v, want ErrTokenNotFound", err)
	}
	if p != nil {
		t.Errorf("principal = %+v, want nil", p)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPGTokenValidator_Expired(t *testing.T) {
	mock, v := newPGTestValidator(t)
	defer mock.Close()

	raw := "idcd_pat_" + strings.Repeat("d", 32)
	hash := HashToken(raw)

	// expires_at = 2026-05-01; now = 2026-06-01 ⇒ expired.
	exp := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	v.SetNow(fixedNow(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)))

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens WHERE token_hash = \$1`).
		WithArgs(hash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("pat_old", "usr_old", &exp))

	_, err := v.Validate(context.Background(), raw)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPGTokenValidator_DBError(t *testing.T) {
	mock, v := newPGTestValidator(t)
	defer mock.Close()

	raw := "idcd_pat_" + strings.Repeat("e", 32)
	hash := HashToken(raw)
	boom := errors.New("connection refused")

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens WHERE token_hash = \$1`).
		WithArgs(hash).
		WillReturnError(boom)

	_, err := v.Validate(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// DB errors wrapped — caller turns this into 401 without leaking detail.
	if errors.Is(err, ErrTokenNotFound) || errors.Is(err, ErrTokenExpired) {
		t.Errorf("DB error misclassified as sentinel: %v", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped %q, got %v", boom, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPGTokenValidator_HashesRawToken(t *testing.T) {
	// Defence-in-depth: assert we never send the raw bearer token to Postgres.
	mock, v := newPGTestValidator(t)
	defer mock.Close()

	raw := "idcd_mcp_" + strings.Repeat("f", 32)
	wantHash := HashToken(raw)
	if wantHash == raw {
		t.Fatal("HashToken returned raw input — test invalid")
	}
	exp := time.Now().Add(time.Hour)

	mock.ExpectQuery(`SELECT id, user_id, expires_at FROM personal_access_tokens WHERE token_hash = \$1`).
		WithArgs(wantHash). // strict — pgxmock fails if the SQL got the raw token
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at"}).
			AddRow("pat_h", "usr_h", &exp))

	if _, err := v.Validate(context.Background(), raw); err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestNewPGTokenValidator_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil pool")
		}
	}()
	_ = NewPGTokenValidator(nil)
}
