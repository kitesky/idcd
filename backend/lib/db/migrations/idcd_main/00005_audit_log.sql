-- +goose Up

-- TimescaleDB hypertable — 复合主键必须含时间列
CREATE TABLE audit_log (
  id            text        NOT NULL,
  ts            timestamptz NOT NULL DEFAULT now(),
  owner_id      text,
  actor_user_id text,
  action        text        NOT NULL,
  resource_type text,
  resource_id   text,
  client_ip     inet,
  user_agent    text,
  location      text,
  result        text        CHECK (result IN ('ok','fail')),
  error_reason  text,
  metadata      jsonb,
  PRIMARY KEY (id, ts)
);

SELECT create_hypertable('audit_log', 'ts', chunk_time_interval => INTERVAL '7 days');
SELECT add_retention_policy('audit_log', INTERVAL '180 days');

CREATE INDEX idx_audit_log_owner    ON audit_log(owner_id, ts DESC) WHERE owner_id IS NOT NULL;
CREATE INDEX idx_audit_log_actor    ON audit_log(actor_user_id, ts DESC) WHERE actor_user_id IS NOT NULL;
CREATE INDEX idx_audit_log_resource ON audit_log(resource_type, resource_id, ts DESC);

-- +goose Down

DROP TABLE IF EXISTS audit_log;
