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

-- PK 必须含 checked_at（TimescaleDB hypertable 分区列要求）。
-- 历史 00032 是对老版本 PK 的补丁修复，新建库直接合并到这里。
CREATE TABLE monitor_agent_obs_checks (
  id               TEXT NOT NULL,
  monitor_id       TEXT NOT NULL,
  obs_type         TEXT NOT NULL,
  status           TEXT NOT NULL,
  latency_ms       DOUBLE PRECISION,
  tokens_used      INTEGER,
  error_code       TEXT,
  response_preview TEXT,
  checked_at       TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (id, checked_at)
);
SELECT create_hypertable('monitor_agent_obs_checks', 'checked_at');
CREATE INDEX ON monitor_agent_obs_checks(monitor_id, checked_at DESC);

-- +goose Down

DROP TABLE IF EXISTS monitor_agent_obs_checks;
DROP TABLE IF EXISTS monitor_agent_obs_configs;
