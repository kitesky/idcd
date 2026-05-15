-- +goose Up
CREATE TABLE node_upgrade_rollouts (
  id            TEXT PRIMARY KEY,
  version       TEXT NOT NULL,
  download_url  TEXT NOT NULL,
  checksum      TEXT NOT NULL,
  rollout_pct   INTEGER NOT NULL DEFAULT 1 CHECK (rollout_pct BETWEEN 1 AND 100),
  status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','completed')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS node_upgrade_rollouts;
