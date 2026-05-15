-- +goose Up

-- migrate:up

CREATE TABLE beta_invitations (
  id           TEXT PRIMARY KEY,
  code         TEXT UNIQUE NOT NULL,
  email        TEXT,
  status       TEXT NOT NULL DEFAULT 'pending',
  requested_by TEXT,
  approved_by  TEXT,
  used_by      TEXT,
  used_at      TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON beta_invitations(status);
CREATE INDEX ON beta_invitations(email);
CREATE INDEX ON beta_invitations(requested_by);

-- migrate:down

DROP TABLE IF EXISTS beta_invitations;
