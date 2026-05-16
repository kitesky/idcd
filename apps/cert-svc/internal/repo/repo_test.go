package repo

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockPool(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

// anyArgs returns a slice of n pgxmock.AnyArg matchers — convenience for
// "we don't care about the args, only the error outcome" tests.
func anyArgs(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = pgxmock.AnyArg()
	}
	return out
}

func TestNewWithPool_WiresEveryRepo(t *testing.T) {
	pool := newMockPool(t)
	repos := NewWithPool(pool)
	assert.NotNil(t, repos.Orders)
	assert.NotNil(t, repos.OrderEvents)
	assert.NotNil(t, repos.Certs)
	assert.NotNil(t, repos.DNSCredentials)
	assert.NotNil(t, repos.ACMEAccounts)
	assert.NotNil(t, repos.RenewalJobs)
	assert.NotNil(t, repos.AuditLogs)
	assert.NotNil(t, repos.Domains)
}

func TestIsUniqueViolation(t *testing.T) {
	t.Run("matches pg unique violation", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505"}
		assert.True(t, isUniqueViolation(err))
	})
	t.Run("ignores other pg codes", func(t *testing.T) {
		err := &pgconn.PgError{Code: "42P01"}
		assert.False(t, isUniqueViolation(err))
	})
	t.Run("ignores non-pg errors", func(t *testing.T) {
		assert.False(t, isUniqueViolation(errors.New("conn refused")))
	})
	t.Run("nil is not a violation", func(t *testing.T) {
		assert.False(t, isUniqueViolation(nil))
	})
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	// errors.Is plumbing — guard against future refactors collapsing them.
	assert.NotErrorIs(t, ErrNotFound, ErrConflict)
	assert.NotErrorIs(t, ErrConflict, ErrInvalidStatus)
	assert.NotErrorIs(t, ErrInvalidStatus, ErrNotFound)
}
