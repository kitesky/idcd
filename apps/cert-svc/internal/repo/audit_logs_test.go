package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuditRepo(t *testing.T) (*AuditLogsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &AuditLogsRepo{pool: pool}, pool
}

func TestAuditLogsRepo_Append_Success(t *testing.T) {
	r, mock := newAuditRepo(t)
	now := time.Now().UTC()
	acct := int64(42)
	tkind := "order"
	tid := int64(101)
	l := &AuditLog{
		AccountID:  &acct,
		Actor:      "user:u_1",
		Action:     "order.create",
		TargetKind: &tkind,
		TargetID:   &tid,
		Payload:    []byte(`{"foo":"bar"}`),
	}

	mock.ExpectQuery(`INSERT INTO cert\.audit_logs`).
		WithArgs(&acct, "user:u_1", "order.create", &tkind, &tid, []byte(`{"foo":"bar"}`)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).AddRow(int64(900), now))

	require.NoError(t, r.Append(context.Background(), l))
	assert.Equal(t, int64(900), l.ID)
}

func TestAuditLogsRepo_Append_NilPayload(t *testing.T) {
	r, mock := newAuditRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.audit_logs`).
		WithArgs((*int64)(nil), "system", "kms.rotate", (*string)(nil), (*int64)(nil), nil).
		WillReturnRows(pgxmock.NewRows([]string{"id", "occurred_at"}).AddRow(int64(1), time.Now()))
	require.NoError(t, r.Append(context.Background(), &AuditLog{Actor: "system", Action: "kms.rotate"}))
}

func TestAuditLogsRepo_Append_DBError(t *testing.T) {
	r, mock := newAuditRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.audit_logs`).
		WithArgs((*int64)(nil), "a", "b", (*string)(nil), (*int64)(nil), nil).
		WillReturnError(errors.New("io"))
	err := r.Append(context.Background(), &AuditLog{Actor: "a", Action: "b"})
	require.Error(t, err)
}
