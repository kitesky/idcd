-- +goose Up

CREATE TABLE alert_silences (
  id            TEXT PRIMARY KEY,
  user_id       TEXT NOT NULL,
  monitor_id    TEXT,
  reason        TEXT NOT NULL,
  starts_at     TIMESTAMPTZ NOT NULL,
  ends_at       TIMESTAMPTZ NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON alert_silences(user_id);
CREATE INDEX ON alert_silences(monitor_id) WHERE monitor_id IS NOT NULL;
CREATE INDEX ON alert_silences(ends_at);

CREATE TABLE alert_groups (
  id            TEXT PRIMARY KEY,
  user_id       TEXT NOT NULL,
  name          TEXT NOT NULL,
  group_by      TEXT NOT NULL,
  group_value   TEXT NOT NULL,
  wait_seconds  INTEGER NOT NULL DEFAULT 60,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE alert_noise_stats (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL,
  date            DATE NOT NULL,
  monitor_id      TEXT NOT NULL,
  total_firings   INTEGER NOT NULL DEFAULT 0,
  total_resolved  INTEGER NOT NULL DEFAULT 0,
  avg_duration_s  DOUBLE PRECISION,
  flap_count      INTEGER NOT NULL DEFAULT 0,
  UNIQUE(user_id, date, monitor_id)
);
CREATE INDEX ON alert_noise_stats(user_id, date DESC);
