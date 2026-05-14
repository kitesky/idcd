-- +goose Up

CREATE TABLE monitor_agent_obs_configs (
  monitor_id          TEXT PRIMARY KEY,
  obs_type            TEXT NOT NULL,
  endpoint_url        TEXT NOT NULL,
  model_name          TEXT,
  expected_tokens_max INTEGER,
  latency_sla_ms      INTEGER,
  payload_template    JSONB,
  check_interval_s    INTEGER NOT NULL DEFAULT 60,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE monitor_agent_obs_checks (
  id               TEXT PRIMARY KEY,
  monitor_id       TEXT NOT NULL,
  obs_type         TEXT NOT NULL,
  status           TEXT NOT NULL,
  latency_ms       DOUBLE PRECISION,
  tokens_used      INTEGER,
  error_code       TEXT,
  response_preview TEXT,
  checked_at       TIMESTAMPTZ NOT NULL
);
SELECT create_hypertable('monitor_agent_obs_checks', 'checked_at');
CREATE INDEX ON monitor_agent_obs_checks(monitor_id, checked_at DESC);

-- +goose Down

DROP TABLE IF EXISTS monitor_agent_obs_checks;
DROP TABLE IF EXISTS monitor_agent_obs_configs;
