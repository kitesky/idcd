-- +goose Up

-- team_invitations.token → token_hash (SHA-256, hex).
--
-- Why: the raw URL token doubles as a bearer credential (anyone with the
-- value can POST /v1/teams/accept-invitation). Storing it plaintext means a
-- read-only DB leak (replica snapshot, audit query, support tool) hands the
-- attacker a fleet of valid accept tokens. Mirrors how we already store
-- PATs and team API keys (handler/pat_handler.go,
-- handler/apikey_handler.go) — hex(sha256(secret)).
--
-- Strategy: add hash column nullable, backfill with pgcrypto digest() to
-- match the Go handler's hex(sha256()) format byte-for-byte, then promote
-- to NOT NULL + UNIQUE and drop the plaintext column. pgcrypto ships with
-- modern Postgres; CREATE EXTENSION IF NOT EXISTS is a no-op when it's
-- already installed (and is idempotent in goose's transaction wrapper).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE team_invitations ADD COLUMN token_hash TEXT;

UPDATE team_invitations
SET token_hash = encode(digest(token, 'sha256'), 'hex')
WHERE token_hash IS NULL AND token IS NOT NULL;

ALTER TABLE team_invitations ALTER COLUMN token_hash SET NOT NULL;
CREATE UNIQUE INDEX team_invitations_token_hash_uniq ON team_invitations(token_hash);

-- DROP COLUMN cascades the anonymous (token) index added in 00018.
ALTER TABLE team_invitations DROP COLUMN token;

-- +goose Down

-- Hashes are one-way. Re-add the plaintext column for schema parity but
-- mark every pending row expired — the rolled-back handler looks rows up
-- by `token`, which is now NULL for all historical invitations and would
-- match any caller. Better to fail closed: operators reissue after rollback.
ALTER TABLE team_invitations ADD COLUMN token TEXT;
UPDATE team_invitations SET status = 'expired' WHERE status = 'pending';
ALTER TABLE team_invitations DROP COLUMN token_hash;
