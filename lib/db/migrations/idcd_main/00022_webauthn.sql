-- +goose Up

CREATE TABLE webauthn_credentials (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL,
  credential_id   TEXT NOT NULL UNIQUE,
  public_key      TEXT NOT NULL,
  aaguid          TEXT NOT NULL DEFAULT '',
  sign_count      BIGINT NOT NULL DEFAULT 0,
  device_name     TEXT NOT NULL DEFAULT '',
  transports      TEXT[] NOT NULL DEFAULT '{}',
  backed_up       BOOLEAN NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at    TIMESTAMPTZ
);
CREATE INDEX ON webauthn_credentials(user_id);

CREATE TABLE webauthn_challenges (
  challenge       TEXT PRIMARY KEY,
  user_id         TEXT,
  purpose         TEXT NOT NULL,
  expires_at      TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down

DROP TABLE IF EXISTS webauthn_challenges;
DROP TABLE IF EXISTS webauthn_credentials;
