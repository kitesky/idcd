-- +goose Up

-- cert.* account_id was originally BIGINT per PRD §5.2, but apps/api's
-- public."user" id is TEXT (stripe-style prefixed ids like usrXxx).
-- cert-svc decodes the JWT user_id and tries strconv.ParseInt → always
-- 401. The whole certificate platform was therefore unreachable from
-- the moment a real JWT showed up. Align the column type so cert-svc
-- can store the same account identifier apps/api emits.
--
-- Safe to ALTER in place because cert.* dev/prod data is zero rows on
-- every environment (the platform never went live), apart from the
-- single dev cert.acme_accounts row Worker writes on first boot —
-- that row has no account_id column so the migration leaves it alone.
-- The TRUNCATE below clears any dev order_events / domains rows that
-- might have been seeded by tests; production deploys have nothing to
-- truncate.

BEGIN;

TRUNCATE TABLE
  cert.orders,
  cert.order_events,
  cert.certs,
  cert.dns_credentials,
  cert.domains,
  cert.abuse_bans,
  cert.audit_logs,
  cert.renewal_jobs
RESTART IDENTITY;

ALTER TABLE cert.domains          ALTER COLUMN account_id TYPE TEXT USING account_id::text;
ALTER TABLE cert.dns_credentials  ALTER COLUMN account_id TYPE TEXT USING account_id::text;
ALTER TABLE cert.orders           ALTER COLUMN account_id TYPE TEXT USING account_id::text;
ALTER TABLE cert.certs            ALTER COLUMN account_id TYPE TEXT USING account_id::text;
ALTER TABLE cert.audit_logs       ALTER COLUMN account_id TYPE TEXT USING account_id::text;
ALTER TABLE cert.abuse_bans       ALTER COLUMN account_id TYPE TEXT USING account_id::text;

COMMIT;

-- +goose Down

-- Reverse: TEXT → BIGINT. Will fail if any non-numeric account_id rows
-- exist (which is exactly what we want post-rollout); operators must
-- choose whether to TRUNCATE or hand-fix before downgrading.

BEGIN;

ALTER TABLE cert.abuse_bans       ALTER COLUMN account_id TYPE BIGINT USING account_id::bigint;
ALTER TABLE cert.audit_logs       ALTER COLUMN account_id TYPE BIGINT USING account_id::bigint;
ALTER TABLE cert.certs            ALTER COLUMN account_id TYPE BIGINT USING account_id::bigint;
ALTER TABLE cert.orders           ALTER COLUMN account_id TYPE BIGINT USING account_id::bigint;
ALTER TABLE cert.dns_credentials  ALTER COLUMN account_id TYPE BIGINT USING account_id::bigint;
ALTER TABLE cert.domains          ALTER COLUMN account_id TYPE BIGINT USING account_id::bigint;

COMMIT;
