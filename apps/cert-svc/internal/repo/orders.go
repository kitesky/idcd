package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/lib/shared/pagination"
)

// OrdersRepo is the cert.orders data-access surface used by the service
// layer and the ACME worker. Status updates are optimistic-locked on the
// caller-supplied "from" status so concurrent workers cannot stomp each
// other.
type OrdersRepo struct {
	pool Pool
}

const ordersColumns = `id, account_id, sans, sans_unicode, common_name, tier, ca,
	reseller_channel, reseller_order_ref, organization_id, validity_days,
	challenge_type, dns_credential_id, status, csr_pem, cert_id,
	billing_invoice_id, retry_count, last_error, idempotency_key,
	created_at, finalized_at`

const ordersInsertSQL = `
	INSERT INTO cert.orders (
		account_id, sans, sans_unicode, common_name, tier, ca,
		reseller_channel, reseller_order_ref, organization_id, validity_days,
		challenge_type, dns_credential_id, status, csr_pem,
		billing_invoice_id, idempotency_key
	) VALUES (
		$1, $2, $3, $4, $5, $6,
		$7, $8, $9, $10,
		$11, $12, $13, $14,
		$15, $16
	)
	RETURNING id, created_at
`

// Insert persists a new order. On UNIQUE (account_id, idempotency_key)
// conflict we look up the existing row id and return it alongside
// ErrConflict so the caller can replay the original outcome instead of
// double-billing or double-issuing.
func (r *OrdersRepo) Insert(ctx context.Context, o *Order) (int64, error) {
	var (
		id        int64
		createdAt time.Time
	)
	err := r.pool.QueryRow(ctx, ordersInsertSQL,
		o.AccountID,
		o.SANs,
		o.SANsUnicode,
		o.CommonName,
		o.Tier,
		o.CA,
		o.ResellerChannel,
		o.ResellerOrderRef,
		o.OrganizationID,
		o.ValidityDays,
		o.ChallengeType,
		o.DNSCredentialID,
		string(o.Status),
		o.CSRPEM,
		o.BillingInvoiceID,
		o.IdempotencyKey,
	).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) && o.IdempotencyKey != nil {
			existingID, lookupErr := r.lookupByIdempotencyKey(ctx, o.AccountID, *o.IdempotencyKey)
			if lookupErr == nil {
				return existingID, ErrConflict
			}
			return 0, fmt.Errorf("orders insert conflict lookup: %w", lookupErr)
		}
		if isUniqueViolation(err) {
			return 0, ErrConflict
		}
		return 0, fmt.Errorf("orders insert: %w", err)
	}
	o.ID = id
	o.CreatedAt = createdAt
	return id, nil
}

const ordersLookupIdempotencySQL = `
	SELECT id
	FROM cert.orders
	WHERE account_id = $1 AND idempotency_key = $2
`

func (r *OrdersRepo) lookupByIdempotencyKey(ctx context.Context, accountID string, key string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, ordersLookupIdempotencySQL, accountID, key).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return id, nil
}

const ordersGetByIDSQL = `
	SELECT ` + ordersColumns + `
	FROM cert.orders
	WHERE id = $1
`

// GetByID returns the order with the given primary-key id, or ErrNotFound.
func (r *OrdersRepo) GetByID(ctx context.Context, id int64) (*Order, error) {
	row := r.pool.QueryRow(ctx, ordersGetByIDSQL, id)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("orders get: %w", err)
	}
	return o, nil
}

const ordersCountByAccountSinceSQL = `
	SELECT COUNT(*)::int
	FROM cert.orders
	WHERE account_id = $1 AND created_at >= $2
`

// CountByAccountSince returns the number of orders an account has created
// since the supplied cutoff, scoped to a single COUNT(*) query.
func (r *OrdersRepo) CountByAccountSince(ctx context.Context, accountID string, since time.Time) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, ordersCountByAccountSinceSQL, accountID, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("orders count by account since: %w", err)
	}
	return n, nil
}

const ordersListByAccountSQL = `
	SELECT ` + ordersColumns + `
	FROM cert.orders
	WHERE account_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3
`

const ordersListByAccountStatusSQL = `
	SELECT ` + ordersColumns + `
	FROM cert.orders
	WHERE account_id = $1 AND status = $2
	ORDER BY created_at DESC
	LIMIT $3 OFFSET $4
`

// ListByAccount returns orders for one account, optionally filtered by a
// single status. Pass status == nil to list all statuses. Ordered by
// created_at DESC (newest first).
func (r *OrdersRepo) ListByAccount(ctx context.Context, accountID string, status *OrderStatus, limit, offset int) ([]*Order, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status != nil {
		rows, err = r.pool.Query(ctx, ordersListByAccountStatusSQL, accountID, string(*status), limit, offset)
	} else {
		rows, err = r.pool.Query(ctx, ordersListByAccountSQL, accountID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("orders list: %w", err)
	}
	defer rows.Close()

	out := make([]*Order, 0)
	for rows.Next() {
		o, scanErr := scanOrder(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("orders list scan: %w", scanErr)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders list rows: %w", err)
	}
	return out, nil
}

const ordersUpdateStatusSQL = `
	UPDATE cert.orders
	SET status = $1, last_error = $2
	WHERE id = $3 AND status = $4
`

// UpdateStatus moves an order from fromStatus to toStatus, optimistically
// locked: if the current status no longer matches fromStatus the UPDATE
// affects zero rows and we return ErrInvalidStatus. lastError is written
// regardless (nil clears the column).
func (r *OrdersRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus OrderStatus, lastError *string) error {
	tag, err := r.pool.Exec(ctx, ordersUpdateStatusSQL, string(toStatus), lastError, id, string(fromStatus))
	if err != nil {
		return fmt.Errorf("orders update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidStatus
	}
	return nil
}

const ordersIncrementRetrySQL = `
	UPDATE cert.orders
	SET retry_count = retry_count + 1
	WHERE id = $1
`

// IncrementRetryCount bumps retry_count by 1. Returns ErrNotFound when
// the order id does not exist.
func (r *OrdersRepo) IncrementRetryCount(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, ordersIncrementRetrySQL, id)
	if err != nil {
		return fmt.Errorf("orders increment retry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const ordersSetCertIDSQL = `
	UPDATE cert.orders
	SET cert_id = $1
	WHERE id = $2
`

// SetCertID attaches an issued cert id to the order row.
func (r *OrdersRepo) SetCertID(ctx context.Context, orderID, certID int64) error {
	tag, err := r.pool.Exec(ctx, ordersSetCertIDSQL, certID, orderID)
	if err != nil {
		return fmt.Errorf("orders set cert_id: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const ordersSetFinalizedAtSQL = `
	UPDATE cert.orders
	SET finalized_at = $1
	WHERE id = $2
`

// SetFinalizedAt records the wall-clock time the order reached a terminal
// status (issued / failed / revoked).
func (r *OrdersRepo) SetFinalizedAt(ctx context.Context, id int64, t time.Time) error {
	tag, err := r.pool.Exec(ctx, ordersSetFinalizedAtSQL, t, id)
	if err != nil {
		return fmt.Errorf("orders set finalized_at: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const ordersListPickableSQL = `
	SELECT ` + ordersColumns + `
	FROM cert.orders
	WHERE status = ANY($1)
	ORDER BY created_at ASC
	LIMIT $2
`

// ListPickable returns the oldest orders whose status is in the given set
// — used by the ACME worker poll loop. Returned in created_at ASC so
// long-stuck rows do not starve.
func (r *OrdersRepo) ListPickable(ctx context.Context, statuses []OrderStatus, limit int) ([]*Order, error) {
	if len(statuses) == 0 {
		return []*Order{}, nil
	}
	strs := make([]string, len(statuses))
	for i, s := range statuses {
		strs[i] = string(s)
	}
	rows, err := r.pool.Query(ctx, ordersListPickableSQL, strs, limit)
	if err != nil {
		return nil, fmt.Errorf("orders list pickable: %w", err)
	}
	defer rows.Close()

	out := make([]*Order, 0)
	for rows.Next() {
		o, scanErr := scanOrder(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("orders list pickable scan: %w", scanErr)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders list pickable rows: %w", err)
	}
	return out, nil
}

// rowScanner is the slice of pgx.Row and pgx.Rows we use for scanning —
// pgx.Row.Scan and pgx.Rows.Scan share this signature.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrder(r rowScanner) (*Order, error) {
	var (
		o          Order
		statusText string
	)
	if err := r.Scan(
		&o.ID,
		&o.AccountID,
		&o.SANs,
		&o.SANsUnicode,
		&o.CommonName,
		&o.Tier,
		&o.CA,
		&o.ResellerChannel,
		&o.ResellerOrderRef,
		&o.OrganizationID,
		&o.ValidityDays,
		&o.ChallengeType,
		&o.DNSCredentialID,
		&statusText,
		&o.CSRPEM,
		&o.CertID,
		&o.BillingInvoiceID,
		&o.RetryCount,
		&o.LastError,
		&o.IdempotencyKey,
		&o.CreatedAt,
		&o.FinalizedAt,
	); err != nil {
		return nil, err
	}
	o.Status = OrderStatus(statusText)
	return &o, nil
}

// AdminOrderFilter is the filter set the admin handler passes to
// AdminListOrders. Each pointer-typed field is treated as optional:
// a nil pointer means "do not filter on this dimension". The result
// set is ordered by created_at DESC (newest first) with a hard cap
// applied by the caller via Limit / Offset.
type AdminOrderFilter struct {
	Status    *OrderStatus
	AccountID  *string
	CA        *string
	Limit     int
	Offset    int
}

// AdminListOrders is the cross-account variant of ListByAccount used by
// /v1/admin/cert/orders. All filters are optional. The query is composed
// dynamically (positional pgx args) because the cross-product of optional
// filters is small (8 combinations) and keeping the predicate explicit
// keeps the EXPLAIN trivially readable for an ops engineer.
func (r *OrdersRepo) AdminListOrders(ctx context.Context, f AdminOrderFilter) ([]*Order, error) {
	// Repo 层使用 RepoMaxPageSize 作为硬上限——handler 应已先 Clamp 到
	// MaxPageSize；这层只是内部脚本 / migration 误传超大值时的兜底。
	limit := pagination.ClampWith(f.Limit, pagination.AdminDefaultPageSize, pagination.RepoMaxPageSize)
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	sql := "SELECT " + ordersColumns + " FROM cert.orders"
	args := make([]any, 0, 5)
	clauses := make([]string, 0, 3)
	if f.Status != nil {
		args = append(args, string(*f.Status))
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.AccountID != nil {
		args = append(args, *f.AccountID)
		clauses = append(clauses, fmt.Sprintf("account_id = $%d", len(args)))
	}
	if f.CA != nil {
		args = append(args, *f.CA)
		clauses = append(clauses, fmt.Sprintf("ca = $%d", len(args)))
	}
	if len(clauses) > 0 {
		sql += " WHERE " + joinAnd(clauses)
	}
	args = append(args, limit)
	args = append(args, offset)
	sql += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("orders admin list: %w", err)
	}
	defer rows.Close()

	out := make([]*Order, 0)
	for rows.Next() {
		o, scanErr := scanOrder(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("orders admin list scan: %w", scanErr)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders admin list rows: %w", err)
	}
	return out, nil
}

// joinAnd concatenates SQL predicate fragments with " AND ". Defined here
// rather than imported from strings to keep the file's dependency surface
// minimal (the only other strings.* use is in scanOrder).
func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

const ordersCountByCASinceSQL = `
	SELECT count(*) FROM cert.orders
	WHERE ca = $1 AND created_at >= $2
`

// CountByCASince returns the number of cert.orders rows for a given CA
// created since the supplied timestamp. Empty caName matches the orders
// that left the CA field blank (legacy / default Router branch). Used
// by the Router's QuotaChecker for the per-account-3h dimension (PRD
// §8.1, LE 300 newOrder / account / 3h).
func (r *OrdersRepo) CountByCASince(ctx context.Context, caName string, since time.Time) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, ordersCountByCASinceSQL, caName, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("orders count by ca since: %w", err)
	}
	return n, nil
}
