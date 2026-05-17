package repo

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockPool returns a fresh pgxmock pool wired to t.Cleanup. Mirrors
// the helper in apps/cert-svc/internal/repo so the test style is
// portable across services.
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

func TestNew_WiresEveryRepo(t *testing.T) {
	pool := newMockPool(t)
	repos := New(pool)
	assert.NotNil(t, repos.Orders)
	assert.NotNil(t, repos.Reports)
	assert.NotNil(t, repos.AttestationRecords)
	assert.NotNil(t, repos.TSAResponses)
	assert.NotNil(t, repos.KeyCeremonyLog)
}

func TestIsUniqueViolation(t *testing.T) {
	t.Run("matches pg unique violation", func(t *testing.T) {
		err := &pgconn.PgError{Code: pgUniqueViolation}
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
	// errors.Is plumbing — guard against future refactors collapsing
	// them.
	assert.NotErrorIs(t, ErrNotFound, ErrConflict)
	assert.NotErrorIs(t, ErrConflict, ErrInvalidStatus)
	assert.NotErrorIs(t, ErrInvalidStatus, ErrNotFound)
}

func TestOrderStatusConstants(t *testing.T) {
	// Lock the enum strings so accidental rename is caught at test
	// time — values are persisted to disk via verdict_order.status.
	assert.Equal(t, "pending", OrderStatusPending)
	assert.Equal(t, "paid", OrderStatusPaid)
	assert.Equal(t, "generating", OrderStatusGenerating)
	assert.Equal(t, "delivered", OrderStatusDelivered)
	assert.Equal(t, "failed", OrderStatusFailed)
	assert.Equal(t, "refunded", OrderStatusRefunded)
	assert.Equal(t, "refund_failed", OrderStatusRefundFailed)
}
