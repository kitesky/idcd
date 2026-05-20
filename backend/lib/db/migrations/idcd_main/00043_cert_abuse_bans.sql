-- +goose Up

-- Account-level abuse bans for the cert platform. An "active" ban is one
-- with lifted_at IS NULL; once lifted, the row is retained for audit but
-- no longer blocks new orders.
--
-- Compared to the audit_logs trail (which records every admin action
-- including bans/unbans), this table is the canonical source of truth
-- the AbuseDetector reads on every order create. Audit_logs still gets
-- an append per admin action; the two are independently consistent.
CREATE TABLE cert.abuse_bans (
    id              BIGSERIAL    PRIMARY KEY,
    account_id      BIGINT       NOT NULL,
    reason          TEXT         NOT NULL,
    banned_by       TEXT         NOT NULL DEFAULT 'admin',
    banned_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    lifted_at       TIMESTAMPTZ,
    lifted_by       TEXT,
    lifted_reason   TEXT
);

-- Partial unique index — at most one active ban per account. Re-banning
-- a previously-lifted account inserts a new row; re-banning an already
-- active account is an idempotent no-op (caller catches the unique
-- violation and treats as "already banned").
CREATE UNIQUE INDEX cert_abuse_bans_active_uniq
    ON cert.abuse_bans (account_id)
    WHERE lifted_at IS NULL;

-- Fast "is this account active-banned?" lookup. The partial index above
-- already covers this, but Postgres won't always use it for a NULL
-- predicate; a plain btree on (account_id) keeps the hot path cheap.
CREATE INDEX cert_abuse_bans_account_idx ON cert.abuse_bans (account_id);

-- +goose Down
DROP TABLE IF EXISTS cert.abuse_bans;
