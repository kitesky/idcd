package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// DNSCredentialsRepo handles cert.dns_credentials rows. EncryptedBlob and
// DEKWrapped are pass-through BYTEA columns — this layer never decrypts
// them. List queries deliberately omit those columns so they never get
// serialised into account-listing API responses.
type DNSCredentialsRepo struct {
	pool Pool
}

const dnsCredentialsInsertSQL = `
	INSERT INTO cert.dns_credentials (
		account_id, provider, display_name,
		encrypted_blob, dek_wrapped, kek_key_id, health_status
	) VALUES (
		$1, $2, $3,
		$4, $5, $6, $7
	)
	RETURNING id, created_at
`

// Insert persists a new DNS credential row. The encrypted_blob /
// dek_wrapped bytes are stored verbatim.
func (r *DNSCredentialsRepo) Insert(ctx context.Context, c *DNSCredential) (int64, error) {
	if c.HealthStatus == "" {
		c.HealthStatus = "unknown"
	}
	var (
		id        int64
		createdAt time.Time
	)
	err := r.pool.QueryRow(ctx, dnsCredentialsInsertSQL,
		c.AccountID,
		c.Provider,
		c.DisplayName,
		c.EncryptedBlob,
		c.DEKWrapped,
		c.KEKKeyID,
		c.HealthStatus,
	).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrConflict
		}
		return 0, fmt.Errorf("dns_credentials insert: %w", err)
	}
	c.ID = id
	c.CreatedAt = createdAt
	return id, nil
}

const dnsCredentialsGetByIDSQL = `
	SELECT id, account_id, provider, display_name,
		encrypted_blob, dek_wrapped, kek_key_id,
		health_status, health_checked_at, created_at, revoked_at
	FROM cert.dns_credentials
	WHERE id = $1
`

// GetByID returns a full row including the encrypted blobs — callers
// need them to drive the ACME challenge.
func (r *DNSCredentialsRepo) GetByID(ctx context.Context, id int64) (*DNSCredential, error) {
	var c DNSCredential
	err := r.pool.QueryRow(ctx, dnsCredentialsGetByIDSQL, id).Scan(
		&c.ID,
		&c.AccountID,
		&c.Provider,
		&c.DisplayName,
		&c.EncryptedBlob,
		&c.DEKWrapped,
		&c.KEKKeyID,
		&c.HealthStatus,
		&c.HealthCheckedAt,
		&c.CreatedAt,
		&c.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("dns_credentials get: %w", err)
	}
	return &c, nil
}

const dnsCredentialsListByAccountSQL = `
	SELECT id, account_id, provider, display_name, kek_key_id,
		health_status, health_checked_at, created_at, revoked_at
	FROM cert.dns_credentials
	WHERE account_id = $1
	ORDER BY created_at DESC
`

// ListByAccount returns every DNS credential for an account *without*
// the encrypted blobs — safe to surface in list views.
func (r *DNSCredentialsRepo) ListByAccount(ctx context.Context, accountID string) ([]*DNSCredential, error) {
	rows, err := r.pool.Query(ctx, dnsCredentialsListByAccountSQL, accountID)
	if err != nil {
		return nil, fmt.Errorf("dns_credentials list: %w", err)
	}
	defer rows.Close()

	out := make([]*DNSCredential, 0)
	for rows.Next() {
		var c DNSCredential
		if err := rows.Scan(
			&c.ID,
			&c.AccountID,
			&c.Provider,
			&c.DisplayName,
			&c.KEKKeyID,
			&c.HealthStatus,
			&c.HealthCheckedAt,
			&c.CreatedAt,
			&c.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("dns_credentials list scan: %w", err)
		}
		out = append(out, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dns_credentials list rows: %w", err)
	}
	return out, nil
}

const dnsCredentialsRevokeSQL = `
	UPDATE cert.dns_credentials
	SET revoked_at = $1
	WHERE id = $2 AND revoked_at IS NULL
`

// Revoke sets revoked_at = NOW() on a credential. Idempotent: if the row
// is already revoked we return ErrNotFound so the caller does not assume
// it just did the revoke. Pre-existing revoked rows are common enough
// that the caller should branch on ErrNotFound and treat both "missing"
// and "already revoked" the same.
func (r *DNSCredentialsRepo) Revoke(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, dnsCredentialsRevokeSQL, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("dns_credentials revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const dnsCredentialsUpdateHealthSQL = `
	UPDATE cert.dns_credentials
	SET health_status = $1, health_checked_at = $2
	WHERE id = $3
`

// UpdateHealthStatus records a fresh probe outcome.
func (r *DNSCredentialsRepo) UpdateHealthStatus(ctx context.Context, id int64, status string) error {
	tag, err := r.pool.Exec(ctx, dnsCredentialsUpdateHealthSQL, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("dns_credentials update health: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
