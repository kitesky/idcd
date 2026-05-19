// Package repo holds the Postgres data-access layer for cert.* tables.
//
// Per CLAUDE.md D1, this layer never writes cross-schema FKs; the service
// layer joins cert.* rows to account.* / billing.* rows in application code.
// Methods are intentionally narrow — CRUD plus the atomic state-machine
// helpers consumed by the ACME worker — and contain no business logic.
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the minimal pgx surface every repo needs. Both *pgxpool.Pool and
// pgxmock.PgxPoolIface satisfy it, so production wiring and unit tests
// share the same concrete repos.
type Pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repos aggregates one repository per cert.* table. Construct once at
// startup with New and pass into the service layer.
type Repos struct {
	Orders         *OrdersRepo
	OrderEvents    *OrderEventsRepo
	Certs          *CertsRepo
	DNSCredentials *DNSCredentialsRepo
	ACMEAccounts   *ACMEAccountsRepo
	RenewalJobs    *RenewalJobsRepo
	AuditLogs      *AuditLogsRepo
	Domains        *DomainsRepo
	AbuseBans      *AbuseBansRepo
}

// New wires every per-table repo over the given pgx pool.
func New(pool *pgxpool.Pool) *Repos {
	return NewWithPool(pool)
}

// NewWithPool builds a Repos from any Pool implementation — used by tests
// that pass a pgxmock pool instead of *pgxpool.Pool.
func NewWithPool(pool Pool) *Repos {
	return &Repos{
		Orders:         &OrdersRepo{pool: pool},
		OrderEvents:    &OrderEventsRepo{pool: pool},
		Certs:          &CertsRepo{pool: pool},
		DNSCredentials: &DNSCredentialsRepo{pool: pool},
		ACMEAccounts:   &ACMEAccountsRepo{pool: pool},
		RenewalJobs:    &RenewalJobsRepo{pool: pool},
		AuditLogs:      &AuditLogsRepo{pool: pool},
		Domains:        &DomainsRepo{pool: pool},
		AbuseBans:      &AbuseBansRepo{pool: pool},
	}
}

// Shared errors. Callers branch on these via errors.Is.
var (
	// ErrNotFound is returned when a single-row lookup (GetByID, Get…)
	// matches zero rows.
	ErrNotFound = errors.New("repo: not found")
	// ErrConflict is returned when an INSERT trips a UNIQUE constraint —
	// most commonly orders.idempotency_key or order_events
	// (order_id, action_seq).
	ErrConflict = errors.New("repo: unique constraint violated")
	// ErrInvalidStatus is returned by optimistic-locked status updates
	// when zero rows match, meaning the caller's "from" status no longer
	// reflects the row in the DB.
	ErrInvalidStatus = errors.New("repo: invalid status transition")
)

// pgUniqueViolation is the SQLSTATE for "unique_violation". Used by every
// Insert path to translate constraint conflicts into ErrConflict.
const pgUniqueViolation = "23505"

// isUniqueViolation returns true when err wraps a pgx unique-violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}
	return false
}

// OrderStatus mirrors the cert.orders.status enum (text column with a fixed
// set of allowed values, see PRD §20 / STATE-MACHINES.md).
type OrderStatus string

const (
	OrderStatusDraft                 OrderStatus = "draft"
	OrderStatusValidating            OrderStatus = "validating"
	OrderStatusAwaitingOrgValidation OrderStatus = "awaiting_org_validation"
	OrderStatusIssuing               OrderStatus = "issuing"
	OrderStatusIssued                OrderStatus = "issued"
	OrderStatusFailed                OrderStatus = "failed"
	OrderStatusRevoking              OrderStatus = "revoking"
	OrderStatusRevoked               OrderStatus = "revoked"
)

// CertStatus mirrors cert.certs.status.
type CertStatus string

const (
	CertStatusIssued  CertStatus = "issued"
	CertStatusRevoked CertStatus = "revoked"
	CertStatusExpired CertStatus = "expired"
)

// Order is the in-memory projection of one cert.orders row.
type Order struct {
	ID               int64
	AccountID        string
	SANs             []string
	SANsUnicode      []string
	CommonName       *string
	Tier             string
	CA               string
	ResellerChannel  *string
	ResellerOrderRef *string
	OrganizationID   *int64
	ValidityDays     int
	ChallengeType    string
	DNSCredentialID  *int64
	Status           OrderStatus
	CSRPEM           *string
	CertID           *int64
	BillingInvoiceID *string
	RetryCount       int
	LastError        *string
	IdempotencyKey   *string
	CreatedAt        time.Time
	FinalizedAt      *time.Time
}

// OrderEvent is a single WAL entry for an order. action_seq is monotonic
// per order_id; the (order_id, action_seq) UNIQUE constraint guarantees
// the worker can replay deterministically.
type OrderEvent struct {
	ID         int64
	OrderID    int64
	ActionSeq  int
	Action     string
	Payload    []byte // JSON; nil = no payload
	OccurredAt time.Time
}

// Cert is the in-memory projection of one cert.certs row.
type Cert struct {
	ID                int64
	OrderID           int64
	AccountID         string
	SANs              []string
	Issuer            string
	SerialHex         string
	FingerprintSHA256 string
	LeafPEM           string
	ChainPEM          string
	KeyKMSHandle      string
	NotBefore         time.Time
	NotAfter          time.Time
	Status            CertStatus
	RevokedAt         *time.Time
	RevokeReason      *string
	CreatedAt         time.Time
}

// DNSCredential is the in-memory projection of one cert.dns_credentials
// row. EncryptedBlob and DEKWrapped are KMS-wrapped opaque bytes; this
// layer does not decrypt them. List queries omit them so they never
// surface in account-listing API responses.
type DNSCredential struct {
	ID              int64
	AccountID       string
	Provider        string
	DisplayName     string
	EncryptedBlob   []byte // nil on list queries
	DEKWrapped      []byte // nil on list queries
	KEKKeyID        string
	HealthStatus    string
	HealthCheckedAt *time.Time
	CreatedAt       time.Time
	RevokedAt       *time.Time
}

// ACMEAccount is one platform ACME registration (one row per CA × env).
type ACMEAccount struct {
	ID               int64
	CA               string
	Env              string
	AccountURL       string
	KeyKMSHandle     string
	EABKID           *string
	EABHMACKMSHandle *string
	CreatedAt        time.Time
}

// RenewalJob is one queued renewal attempt for an issued cert.
type RenewalJob struct {
	ID           int64
	CertID       int64
	ScheduledAt  time.Time
	AttemptCount int
	LastError    *string
	Status       string
	NewOrderID   *int64
	CreatedAt    time.Time
}

// AuditLog is one append-only entry on the cert.audit_logs table.
type AuditLog struct {
	ID         int64
	AccountID  *string
	Actor      string
	Action     string
	TargetKind *string
	TargetID   *int64
	Payload    []byte // JSON; nil = no payload
	OccurredAt time.Time
}

// Domain is one cert.domains row — per-account FQDN registry + CAA cache.
type Domain struct {
	ID           int64
	AccountID    string
	FQDN         string
	CAAStatus    *string
	CAACheckedAt *time.Time
	CreatedAt    time.Time
}
