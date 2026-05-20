// Package db — transaction helpers.
//
// WithTx wraps multi-step DB operations in a transaction boundary so half-
// committed writes (e.g. user.Create + audit_log.Create where the second
// fails) can never persist. See REVIEW-FINDINGS-2026-05-16.md P2#22.
//
// Design notes (decisions encoded in this file):
//
//  1. The public entry point accepts *pgxpool.Pool to match call sites that
//     already hold one. An exported TxBeginner interface lets tests and
//     callers that hold a narrower interface (e.g. pgxmock.PgxPoolIface)
//     reuse the same machinery via WithTxBeginner.
//
//  2. TxOptions are passed as a variadic to avoid a second signature: callers
//     supply zero or one pgx.TxOptions value. More than one is a programming
//     error and panics — that's fine because it's caught at the first test
//     run, never at runtime in prod.
//
//  3. Nested calls (fn calls WithTx again with the same ctx) use SAVEPOINTs
//     via pgx.Tx.Begin(). The outer tx commits/rolls back as one unit; a
//     failed inner savepoint rolls back only its own statements, matching
//     standard "subtransaction" semantics. This avoids the surprise of
//     callers accidentally opening a second physical tx on a different
//     connection.
//
//  4. context.Canceled / DeadlineExceeded mid-fn: we rollback using a
//     fallback context (Background with a 5s timeout) so the connection is
//     returned to the pool cleanly even though the caller's ctx is dead.
//     The original ctx error is returned to the caller untouched.
//
//  5. Panics inside fn are caught, the tx is rolled back, and the panic is
//     re-raised. The connection is not leaked.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxBeginner is the minimal interface needed to start a transaction.
// *pgxpool.Pool, pgx.Tx, and pgxmock.PgxPoolIface all satisfy it.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// txBeginnerWithOpts is satisfied by *pgxpool.Pool (and pgxmock pools) for
// BeginTx with isolation options. pgx.Tx itself does NOT have BeginTx — its
// nested savepoints are created via Begin() — so this is checked optionally.
type txBeginnerWithOpts interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// rollbackTimeout bounds how long we wait when rolling back after the caller's
// context was canceled. Long enough for the round-trip, short enough that a
// dead Postgres doesn't pin a worker.
const rollbackTimeout = 5 * time.Second

// txCtxKey is the context key under which the active *pgx.Tx is stored. This
// lets nested WithTx calls discover an outer tx and open a savepoint instead
// of a second physical transaction.
type txCtxKey struct{}

// FromContext returns the active transaction stored in ctx by WithTx, or nil.
// Useful for repositories that want to participate in an outer transaction
// when present and otherwise execute against the pool directly. Most callers
// should NOT use this — prefer passing pgx.Tx to repo methods explicitly.
func FromContext(ctx context.Context) pgx.Tx {
	if v, ok := ctx.Value(txCtxKey{}).(pgx.Tx); ok {
		return v
	}
	return nil
}

// ContextWithTx returns a new context carrying tx so that a nested WithTx
// call discovers it and opens a SAVEPOINT instead of a second physical
// transaction. Repositories that want to participate in an outer transaction
// when one is present can call this to stash the tx for downstream helpers.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

// withTxValue is an internal alias kept for readability inside this file.
func withTxValue(ctx context.Context, tx pgx.Tx) context.Context {
	return ContextWithTx(ctx, tx)
}

// WithTx runs fn inside a database transaction.
//
//   - On nil error, Commit is called. If Commit fails, that error is returned.
//   - On non-nil error from fn, Rollback is called and fn's error is returned.
//   - If fn panics, Rollback is called and the panic is re-raised.
//   - If ctx is canceled / deadline-exceeded mid-fn, Rollback runs with a
//     fallback context and the original ctx error is returned.
//   - If ctx already carries a pgx.Tx (nested call), a SAVEPOINT is opened
//     instead of a second physical transaction. The savepoint releases on
//     success or rolls back to the savepoint on error.
//
// At most one TxOptions may be supplied. Passing more than one panics.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error, opts ...pgx.TxOptions) error {
	if pool == nil {
		return errors.New("db.WithTx: pool is nil")
	}
	return WithTxBeginner(ctx, pool, fn, opts...)
}

// WithTxBeginner is identical to WithTx but accepts the narrower TxBeginner
// interface. This is the seam tests use: pass a pgxmock pool here. Production
// code should prefer WithTx.
func WithTxBeginner(ctx context.Context, b TxBeginner, fn func(pgx.Tx) error, opts ...pgx.TxOptions) (err error) {
	if b == nil {
		return errors.New("db.WithTxBeginner: beginner is nil")
	}
	if fn == nil {
		return errors.New("db.WithTxBeginner: fn is nil")
	}
	if len(opts) > 1 {
		panic("db.WithTxBeginner: at most one pgx.TxOptions may be supplied")
	}

	// Nested-call detection: if ctx already has an active tx, use a savepoint.
	if outer := FromContext(ctx); outer != nil {
		return withSavepoint(ctx, outer, fn)
	}

	tx, err := beginTx(ctx, b, opts)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}

	// Bind tx to ctx so nested WithTx calls find it.
	ctx = withTxValue(ctx, tx)

	committed := false
	defer func() {
		if r := recover(); r != nil {
			rollback(ctx, tx)
			panic(r)
		}
		if committed {
			return
		}
		// fn returned a non-nil error OR Commit was never reached.
		rollback(ctx, tx)
	}()

	if err = fn(tx); err != nil {
		return err
	}

	// If ctx died after fn returned but before commit, prefer to surface that.
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}

	if err = tx.Commit(ctx); err != nil {
		// pgx returns ErrTxClosed if the tx was rolled back inside fn (e.g. a
		// query error rolled it back internally). Surface that verbatim — the
		// rollback in defer is a no-op in that case.
		return fmt.Errorf("db: commit tx: %w", err)
	}
	committed = true
	return nil
}

// beginTx starts a transaction, honoring TxOptions when supplied.
func beginTx(ctx context.Context, b TxBeginner, opts []pgx.TxOptions) (pgx.Tx, error) {
	if len(opts) == 1 {
		// BeginTx is on *pgxpool.Pool (and pgxmock) but not on pgx.Tx. If the
		// caller passed a TxBeginner that doesn't support options, that's a
		// programming error — surface it loudly.
		bo, ok := b.(txBeginnerWithOpts)
		if !ok {
			return nil, fmt.Errorf("db: TxBeginner %T does not support TxOptions", b)
		}
		return bo.BeginTx(ctx, opts[0])
	}
	return b.Begin(ctx)
}

// rollback attempts to release the transaction. If ctx is already canceled,
// we use a short-lived fallback context so the connection is released to the
// pool cleanly. ErrTxClosed (tx already committed or rolled back internally)
// is swallowed because it's not actionable.
func rollback(ctx context.Context, tx pgx.Tx) {
	rbCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		rbCtx, cancel = context.WithTimeout(context.Background(), rollbackTimeout)
		defer cancel()
	}
	if err := tx.Rollback(rbCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		// Log path would go here. We don't have a logger in lib/db — and we
		// don't want to take a logger dep just for this. The rollback error
		// is intentionally swallowed: the caller already has fn's error or a
		// panic to propagate, and a rollback failure on an already-broken
		// connection doesn't add actionable info.
		_ = err
	}
}

// withSavepoint runs fn inside a SAVEPOINT on an existing outer transaction.
// pgx.Tx.Begin() creates a savepoint when called on an active tx.
func withSavepoint(ctx context.Context, outer pgx.Tx, fn func(pgx.Tx) error) (err error) {
	sp, err := outer.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin savepoint: %w", err)
	}

	// Rebind ctx so a deeper nested call gets *this* savepoint, not the outer.
	ctx = withTxValue(ctx, sp)

	released := false
	defer func() {
		if r := recover(); r != nil {
			_ = sp.Rollback(rollbackCtx(ctx))
			panic(r)
		}
		if released {
			return
		}
		_ = sp.Rollback(rollbackCtx(ctx))
	}()

	if err = fn(sp); err != nil {
		return err
	}
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	if err = sp.Commit(ctx); err != nil {
		return fmt.Errorf("db: release savepoint: %w", err)
	}
	released = true
	return nil
}

// rollbackCtx returns a context safe to use for rollback. If the live ctx is
// dead we fall back to Background with a bounded timeout. The cancel func is
// not exposed to callers — the deferred timeout context is short-lived
// enough that goroutine leak is not a concern.
func rollbackCtx(ctx context.Context) context.Context {
	if ctx.Err() == nil {
		return ctx
	}
	rb, _ := context.WithTimeout(context.Background(), rollbackTimeout) //nolint:govet // intentionally not deferring cancel; bounded by timeout
	return rb
}
