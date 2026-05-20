-- +goose Up

-- Cert platform schema (see docs/prd/20-free-cert.md §5.2).
-- All cross-schema references (e.g. account_id → account.users) are
-- intentionally NOT enforced as FKs, per CLAUDE.md D1.

CREATE SCHEMA IF NOT EXISTS cert;

-- Domain registry: dedup + CAA cache per account.
CREATE TABLE cert.domains (
  id             BIGSERIAL    PRIMARY KEY,
  account_id     BIGINT       NOT NULL,
  fqdn           TEXT         NOT NULL,
  caa_status     TEXT,
  caa_checked_at TIMESTAMPTZ,
  created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (account_id, fqdn)
);

-- DNS API credentials, KMS-enveloped.
CREATE TABLE cert.dns_credentials (
  id                BIGSERIAL    PRIMARY KEY,
  account_id        BIGINT       NOT NULL,
  provider          TEXT         NOT NULL,
  display_name      TEXT         NOT NULL,
  encrypted_blob    BYTEA        NOT NULL,
  dek_wrapped       BYTEA        NOT NULL,
  kek_key_id        TEXT         NOT NULL,
  health_status     TEXT         NOT NULL DEFAULT 'unknown',
  health_checked_at TIMESTAMPTZ,
  created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  revoked_at        TIMESTAMPTZ
);

-- Platform ACME accounts, one row per CA × environment.
CREATE TABLE cert.acme_accounts (
  id                  BIGSERIAL    PRIMARY KEY,
  ca                  TEXT         NOT NULL,
  env                 TEXT         NOT NULL,
  account_url         TEXT         NOT NULL,
  key_kms_handle      TEXT         NOT NULL,
  eab_kid             TEXT,
  eab_hmac_kms_handle TEXT,
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (ca, env)
);

-- Issuance orders. Paid-tier columns (tier / sans_unicode / common_name /
-- validity_days / reseller_channel / reseller_order_ref / organization_id /
-- billing_invoice_id) live here from day one so the S3 reseller flow does
-- not require a schema change. See PRD §20.
CREATE TABLE cert.orders (
  id                  BIGSERIAL    PRIMARY KEY,
  account_id          BIGINT       NOT NULL,
  sans                TEXT[]       NOT NULL,
  sans_unicode        TEXT[],
  common_name         TEXT,
  tier                TEXT         NOT NULL DEFAULT 'free-dv',
  ca                  TEXT         NOT NULL,
  reseller_channel    TEXT,
  reseller_order_ref  TEXT,
  organization_id     BIGINT,
  validity_days       INT          NOT NULL DEFAULT 90,
  challenge_type      TEXT         NOT NULL,
  dns_credential_id   BIGINT,
  status              TEXT         NOT NULL,
  csr_pem             TEXT,
  cert_id             BIGINT,
  billing_invoice_id  TEXT,
  retry_count         INT          NOT NULL DEFAULT 0,
  last_error          TEXT,
  idempotency_key     TEXT,
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  finalized_at        TIMESTAMPTZ,
  UNIQUE (account_id, idempotency_key),
  UNIQUE (billing_invoice_id)
);
CREATE INDEX ON cert.orders (account_id, status);
CREATE INDEX ON cert.orders (status) WHERE status IN ('validating','issuing');

-- Per-order event WAL (action_seq is monotonic per order, see PRD §6).
CREATE TABLE cert.order_events (
  id            BIGSERIAL    PRIMARY KEY,
  order_id      BIGINT       NOT NULL,
  action_seq    INT          NOT NULL,
  action        TEXT         NOT NULL,
  payload_jsonb JSONB,
  occurred_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (order_id, action_seq)
);

-- Issued certificates.
CREATE TABLE cert.certs (
  id                 BIGSERIAL    PRIMARY KEY,
  order_id           BIGINT       NOT NULL,
  account_id         BIGINT       NOT NULL,
  sans               TEXT[]       NOT NULL,
  issuer             TEXT         NOT NULL,
  serial_hex         TEXT         NOT NULL,
  fingerprint_sha256 TEXT         NOT NULL,
  leaf_pem           TEXT         NOT NULL,
  chain_pem          TEXT         NOT NULL,
  key_kms_handle     TEXT         NOT NULL,
  not_before         TIMESTAMPTZ  NOT NULL,
  not_after          TIMESTAMPTZ  NOT NULL,
  status             TEXT         NOT NULL,
  revoked_at         TIMESTAMPTZ,
  revoke_reason      TEXT,
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX ON cert.certs (account_id, status);
CREATE INDEX ON cert.certs (not_after) WHERE status='issued';

-- Renewal jobs.
CREATE TABLE cert.renewal_jobs (
  id            BIGSERIAL    PRIMARY KEY,
  cert_id       BIGINT       NOT NULL,
  scheduled_at  TIMESTAMPTZ  NOT NULL,
  attempt_count INT          NOT NULL DEFAULT 0,
  last_error    TEXT,
  status        TEXT         NOT NULL,
  new_order_id  BIGINT,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Audit log, append-only.
CREATE TABLE cert.audit_logs (
  id            BIGSERIAL    PRIMARY KEY,
  account_id    BIGINT,
  actor         TEXT         NOT NULL,
  action        TEXT         NOT NULL,
  target_kind   TEXT,
  target_id     BIGINT,
  payload_jsonb JSONB,
  occurred_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- +goose Down

DROP TABLE IF EXISTS cert.audit_logs;
DROP TABLE IF EXISTS cert.renewal_jobs;
DROP TABLE IF EXISTS cert.certs;
DROP TABLE IF EXISTS cert.order_events;
DROP TABLE IF EXISTS cert.orders;
DROP TABLE IF EXISTS cert.acme_accounts;
DROP TABLE IF EXISTS cert.dns_credentials;
DROP TABLE IF EXISTS cert.domains;
DROP SCHEMA IF EXISTS cert;
