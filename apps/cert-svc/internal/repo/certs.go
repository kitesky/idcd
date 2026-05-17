package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CertsRepo is the cert.certs data-access surface. Lifecycle transitions
// (issued → revoked / expired) are optimistic-locked on the caller's
// "from" status to keep the renewer worker and admin revoke path from
// racing each other.
type CertsRepo struct {
	pool Pool
}

const certsColumns = `id, order_id, account_id, sans, issuer, serial_hex,
	fingerprint_sha256, leaf_pem, chain_pem, key_kms_handle,
	not_before, not_after, status, revoked_at, revoke_reason, created_at`

const certsInsertSQL = `
	INSERT INTO cert.certs (
		order_id, account_id, sans, issuer, serial_hex,
		fingerprint_sha256, leaf_pem, chain_pem, key_kms_handle,
		not_before, not_after, status
	) VALUES (
		$1, $2, $3, $4, $5,
		$6, $7, $8, $9,
		$10, $11, $12
	)
	RETURNING id, created_at
`

// Insert persists a newly-issued cert and returns its id.
func (r *CertsRepo) Insert(ctx context.Context, c *Cert) (int64, error) {
	var (
		id        int64
		createdAt time.Time
	)
	err := r.pool.QueryRow(ctx, certsInsertSQL,
		c.OrderID,
		c.AccountID,
		c.SANs,
		c.Issuer,
		c.SerialHex,
		c.FingerprintSHA256,
		c.LeafPEM,
		c.ChainPEM,
		c.KeyKMSHandle,
		c.NotBefore,
		c.NotAfter,
		string(c.Status),
	).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrConflict
		}
		return 0, fmt.Errorf("certs insert: %w", err)
	}
	c.ID = id
	c.CreatedAt = createdAt
	return id, nil
}

const certsGetByIDSQL = `
	SELECT ` + certsColumns + `
	FROM cert.certs
	WHERE id = $1
`

// GetByID returns the cert with the given id, or ErrNotFound.
func (r *CertsRepo) GetByID(ctx context.Context, id int64) (*Cert, error) {
	row := r.pool.QueryRow(ctx, certsGetByIDSQL, id)
	c, err := scanCert(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("certs get: %w", err)
	}
	return c, nil
}

const certsListByAccountSQL = `
	SELECT ` + certsColumns + `
	FROM cert.certs
	WHERE account_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3
`

const certsListByAccountStatusSQL = `
	SELECT ` + certsColumns + `
	FROM cert.certs
	WHERE account_id = $1 AND status = $2
	ORDER BY created_at DESC
	LIMIT $3 OFFSET $4
`

// ListByAccount returns certs for one account, optionally filtered by
// status. Pass status == nil to skip the filter.
func (r *CertsRepo) ListByAccount(ctx context.Context, accountID int64, status *CertStatus, limit, offset int) ([]*Cert, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status != nil {
		rows, err = r.pool.Query(ctx, certsListByAccountStatusSQL, accountID, string(*status), limit, offset)
	} else {
		rows, err = r.pool.Query(ctx, certsListByAccountSQL, accountID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("certs list: %w", err)
	}
	defer rows.Close()

	out := make([]*Cert, 0)
	for rows.Next() {
		c, scanErr := scanCert(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("certs list scan: %w", scanErr)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("certs list rows: %w", err)
	}
	return out, nil
}

const certsListExpiringBeforeSQL = `
	SELECT ` + certsColumns + `
	FROM cert.certs
	WHERE status = 'issued' AND not_after < $1
	ORDER BY not_after ASC
	LIMIT $2
`

// ListExpiringBefore returns issued certs whose not_after is earlier than
// t — used by the renewer to enqueue jobs.
func (r *CertsRepo) ListExpiringBefore(ctx context.Context, t time.Time, limit int) ([]*Cert, error) {
	rows, err := r.pool.Query(ctx, certsListExpiringBeforeSQL, t, limit)
	if err != nil {
		return nil, fmt.Errorf("certs list expiring: %w", err)
	}
	defer rows.Close()

	out := make([]*Cert, 0)
	for rows.Next() {
		c, scanErr := scanCert(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("certs list expiring scan: %w", scanErr)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("certs list expiring rows: %w", err)
	}
	return out, nil
}

const certsUpdateStatusSQL = `
	UPDATE cert.certs
	SET status = $1, revoked_at = $2, revoke_reason = $3
	WHERE id = $4 AND status = $5
`

// UpdateStatus transitions a cert from fromStatus to toStatus with
// optimistic locking. Pass revokedAt and reason only when transitioning
// to revoked; nil otherwise.
func (r *CertsRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus CertStatus, revokedAt *time.Time, reason *string) error {
	tag, err := r.pool.Exec(ctx, certsUpdateStatusSQL, string(toStatus), revokedAt, reason, id, string(fromStatus))
	if err != nil {
		return fmt.Errorf("certs update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidStatus
	}
	return nil
}

func scanCert(r rowScanner) (*Cert, error) {
	var (
		c          Cert
		statusText string
	)
	if err := r.Scan(
		&c.ID,
		&c.OrderID,
		&c.AccountID,
		&c.SANs,
		&c.Issuer,
		&c.SerialHex,
		&c.FingerprintSHA256,
		&c.LeafPEM,
		&c.ChainPEM,
		&c.KeyKMSHandle,
		&c.NotBefore,
		&c.NotAfter,
		&statusText,
		&c.RevokedAt,
		&c.RevokeReason,
		&c.CreatedAt,
	); err != nil {
		return nil, err
	}
	c.Status = CertStatus(statusText)
	return &c, nil
}

// Registered-domain peak query. Group by the last two labels of the
// first SAN to approximate Let's Encrypt's "Registered Domain" bucket
// (PRD §8.1 — 50 certs / RD / week). This is a 2-label heuristic that
// undercounts ccTLDs like .co.uk (treats "co.uk" as the RD), which
// makes the Router *more* conservative there — acceptable for a
// fallover signal.
const certsMaxPerRDSinceSQL = `
	SELECT COALESCE(MAX(c), 0) FROM (
		SELECT count(*) AS c
		FROM cert.certs
		WHERE issuer = $1 AND created_at >= $2 AND array_length(sans, 1) >= 1
		GROUP BY regexp_replace(sans[1], '^.*\.([^.]+\.[^.]+)$', '\1')
	) g
`

// MaxCertsPerRegisteredDomainSince returns the peak per-RD issuance
// count for issuer in the supplied window. Used by the Router's
// QuotaChecker (LE: 50 certs / RD / week).
func (r *CertsRepo) MaxCertsPerRegisteredDomainSince(ctx context.Context, issuer string, since time.Time) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, certsMaxPerRDSinceSQL, issuer, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("certs max per registered domain since: %w", err)
	}
	return n, nil
}
