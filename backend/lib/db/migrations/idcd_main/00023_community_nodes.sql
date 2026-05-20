-- +goose Up

CREATE TABLE node_applications (
  id                   TEXT PRIMARY KEY,
  user_id              TEXT NOT NULL,
  hostname             TEXT NOT NULL,
  ip_address           TEXT NOT NULL,
  country              TEXT NOT NULL,
  city                 TEXT,
  isp                  TEXT,
  bandwidth_mbps       INTEGER,
  os_info              TEXT,
  motivation           TEXT,
  status               TEXT NOT NULL DEFAULT 'pending',
  reviewed_by          TEXT,
  review_note          TEXT,
  probation_started_at TIMESTAMPTZ,
  activated_at         TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON node_applications(user_id);
CREATE INDEX ON node_applications(status);

CREATE TABLE node_points (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL,
  amount     INTEGER NOT NULL,
  balance    INTEGER NOT NULL,
  reason     TEXT NOT NULL,
  ref_id     TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON node_points(user_id);

CREATE TABLE point_redemptions (
  id            TEXT PRIMARY KEY,
  user_id       TEXT NOT NULL,
  points_spent  INTEGER NOT NULL,
  reward_type   TEXT NOT NULL,
  reward_amount INTEGER NOT NULL,
  status        TEXT NOT NULL DEFAULT 'pending',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
