package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newKeyCeremonyRepo(t *testing.T) (*KeyCeremonyLogRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &KeyCeremonyLogRepo{pool: pool}, pool
}

func sampleCeremony() *KeyCeremony {
	notes := "founder + notary"
	return &KeyCeremony{
		ID:     "kc_1",
		Action: "root_gen",
		Actors: []byte(`[{"user_id":"u1","role":"founder"}]`),
		Notes:  &notes,
	}
}

func ceremonyRowColumns() []string {
	return []string{
		"id", "action", "key_id", "key_version",
		"actors", "evidence_url", "notes", "created_at",
	}
}

func sampleCeremonyRow(id, action string) []any {
	now := time.Now().UTC()
	return []any{
		id, action, (*string)(nil), (*int)(nil),
		[]byte(`[{"user_id":"u1","role":"founder"}]`), (*string)(nil), (*string)(nil), now,
	}
}

func TestKeyCeremonyLogRepo_Insert_Success(t *testing.T) {
	r, mock := newKeyCeremonyRepo(t)

	mock.ExpectExec(`INSERT INTO idcd_attest\.key_ceremony_log`).
		WithArgs(anyArgs(8)...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := r.Insert(context.Background(), sampleCeremony())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestKeyCeremonyLogRepo_Insert_Conflict(t *testing.T) {
	r, mock := newKeyCeremonyRepo(t)

	mock.ExpectExec(`INSERT INTO idcd_attest\.key_ceremony_log`).
		WithArgs(anyArgs(8)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	err := r.Insert(context.Background(), sampleCeremony())
	assert.ErrorIs(t, err, ErrConflict)
}

func TestKeyCeremonyLogRepo_Insert_DBError(t *testing.T) {
	r, mock := newKeyCeremonyRepo(t)
	sentinel := errors.New("io")

	mock.ExpectExec(`INSERT INTO idcd_attest\.key_ceremony_log`).
		WithArgs(anyArgs(8)...).
		WillReturnError(sentinel)

	err := r.Insert(context.Background(), sampleCeremony())
	assert.ErrorIs(t, err, sentinel)
}

func TestKeyCeremonyLogRepo_Insert_Nil(t *testing.T) {
	r, _ := newKeyCeremonyRepo(t)
	err := r.Insert(context.Background(), nil)
	require.Error(t, err)
}

func TestKeyCeremonyLogRepo_List_Success(t *testing.T) {
	r, mock := newKeyCeremonyRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.key_ceremony_log\s+ORDER BY created_at DESC`).
		WithArgs(20, 0).
		WillReturnRows(pgxmock.NewRows(ceremonyRowColumns()).
			AddRow(sampleCeremonyRow("kc_1", "root_gen")...).
			AddRow(sampleCeremonyRow("kc_2", "sign_key_rotate")...))

	out, err := r.List(context.Background(), 20, 0)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "kc_1", out[0].ID)
	assert.Equal(t, "sign_key_rotate", out[1].Action)
}

func TestKeyCeremonyLogRepo_List_Empty(t *testing.T) {
	r, mock := newKeyCeremonyRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.key_ceremony_log`).
		WithArgs(10, 5).
		WillReturnRows(pgxmock.NewRows(ceremonyRowColumns()))

	out, err := r.List(context.Background(), 10, 5)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestKeyCeremonyLogRepo_List_QueryError(t *testing.T) {
	r, mock := newKeyCeremonyRepo(t)
	sentinel := errors.New("boom")

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.key_ceremony_log`).
		WithArgs(10, 0).
		WillReturnError(sentinel)

	_, err := r.List(context.Background(), 10, 0)
	assert.ErrorIs(t, err, sentinel)
}
