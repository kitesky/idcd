-- +goose Up

CREATE TABLE monitor_baselines (
  id            TEXT PRIMARY KEY,
  monitor_id    TEXT NOT NULL UNIQUE,
  p50_latency   DOUBLE PRECISION,
  p95_latency   DOUBLE PRECISION,
  p99_latency   DOUBLE PRECISION,
  success_rate  DOUBLE PRECISION,
  sample_count  INTEGER NOT NULL DEFAULT 0,
  computed_at   TIMESTAMPTZ NOT NULL,
  window_hours  INTEGER NOT NULL DEFAULT 168,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON monitor_baselines(monitor_id);

CREATE TABLE anchor_deviations (
  id             TEXT PRIMARY KEY,
  monitor_id     TEXT NOT NULL,
  baseline_id    TEXT NOT NULL,
  deviation_type TEXT NOT NULL,
  current_value  DOUBLE PRECISION NOT NULL,
  baseline_value DOUBLE PRECISION NOT NULL,
  deviation_pct  DOUBLE PRECISION NOT NULL,
  severity       TEXT NOT NULL,
  status         TEXT NOT NULL DEFAULT 'open',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at    TIMESTAMPTZ
);
CREATE INDEX ON anchor_deviations(monitor_id, status);

-- +goose Down

DROP TABLE IF EXISTS anchor_deviations;
DROP TABLE IF EXISTS monitor_baselines;
