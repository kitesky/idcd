-- +goose Up

-- monitors: 监控项
CREATE TABLE IF NOT EXISTS monitors (
  id          TEXT PRIMARY KEY,                    -- mon_前缀 nanoid
  user_id     TEXT NOT NULL,                       -- 应用层 join，无 FK（D1规则）
  name        TEXT NOT NULL,
  type        TEXT NOT NULL CHECK (type IN ('http','https','ping','tcp','dns','ssl_expiry','domain_expiry','icp_change','keyword')),
  target      TEXT NOT NULL,
  config      JSONB NOT NULL DEFAULT '{}',         -- 类型相关配置（断言/关键字/端口等）
  interval_s  INTEGER NOT NULL DEFAULT 300,        -- 检测频率（秒）：60/300/1800
  node_count  INTEGER NOT NULL DEFAULT 3,          -- 并发节点数
  status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','maintenance','archived')),
  last_check_at TIMESTAMPTZ,
  next_check_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_monitors_user_id ON monitors(user_id);
CREATE INDEX IF NOT EXISTS idx_monitors_status_next ON monitors(status, next_check_at) WHERE status = 'active';

-- monitor_checks: 每次检测结果（TimescaleDB hypertable）
CREATE TABLE IF NOT EXISTS monitor_checks (
  check_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  monitor_id  TEXT NOT NULL,
  node_id     TEXT NOT NULL,
  status      TEXT NOT NULL CHECK (status IN ('up','down','degraded')),
  latency_ms  INTEGER,
  error       TEXT,
  metadata    JSONB NOT NULL DEFAULT '{}'
);
SELECT create_hypertable('monitor_checks', 'check_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_monitor_checks_monitor ON monitor_checks(monitor_id, check_at DESC);

-- +goose Down

DROP TABLE IF EXISTS monitor_checks;
DROP TABLE IF EXISTS monitors;
