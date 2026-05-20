-- +goose Up

-- idcd 自家健康度状态页 (idcd.com/status) 数据底座。
-- 不依赖 Prometheus 90d retention（核心 retention 仅 30d 且 prom 是 ephemeral）,
-- 自己持久化两张粒度表：5min 细查 7d / 天聚合 90d (GitHub Status 风格 bar 用)。
-- 第三张 status_incidents 是人工录入的事件时间线（v1 手动，v2 接 alertmanager 自动写入）。

-- service_key 命名约定（应用层枚举，DB 不约束）:
--   后端 service:  'api' | 'cert-svc' | 'gateway' | 'aggregator' | 'notifier'
--   前端 web:      'web'
--   节点:          'node:<node_id>'  (e.g. 'node:nd_psYypXZS9uCw')
--   节点组（汇总）: 暂不引入；前端按 detail.country_code GROUP BY 即可

-- status 编码（与前端 ServiceStatus enum 对齐）：
--   1 = operational, 2 = degraded, 3 = outage, 4 = maintenance

-- status_uptime_5min: 5 分钟粒度，仅留 7 天。
-- detail JSONB 给节点行存 ip / country_code / latency_ms 等动态字段，
-- 给 service 行存 prom 抓取结果或探活耗时。
CREATE TABLE IF NOT EXISTS status_uptime_5min (
  service_key TEXT        NOT NULL,
  bucket_at   TIMESTAMPTZ NOT NULL,
  status      SMALLINT    NOT NULL CHECK (status IN (1,2,3,4)),
  detail      JSONB       NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (service_key, bucket_at)
);
-- 按时间倒序快速取最新一桶
CREATE INDEX IF NOT EXISTS idx_status_uptime_5min_recent
  ON status_uptime_5min(bucket_at DESC);

-- status_uptime_daily: 天粒度，留 90 天。
-- uptime_pct 由当天 288 个 5min 桶中 status=1 的占比计算（DB 不算，应用算后落表）。
-- worst_status 取当天最差状态用于决定 bar 颜色。
-- incident_ids 关联同日发生的事件（hover tooltip 用）。
CREATE TABLE IF NOT EXISTS status_uptime_daily (
  service_key  TEXT          NOT NULL,
  day          DATE          NOT NULL,
  uptime_pct   NUMERIC(5,2)  NOT NULL CHECK (uptime_pct >= 0 AND uptime_pct <= 100),
  worst_status SMALLINT      NOT NULL CHECK (worst_status IN (1,2,3,4)),
  incident_ids BIGINT[]      NOT NULL DEFAULT ARRAY[]::BIGINT[],
  PRIMARY KEY (service_key, day)
);
-- 取某 service 最近 90 天 bar 数据时按 day DESC 限 90 行
CREATE INDEX IF NOT EXISTS idx_status_uptime_daily_service_day
  ON status_uptime_daily(service_key, day DESC);

-- status_incidents: 事件时间线（人工录入 + 未来告警自动写）。
-- severity 与 GitHub Status 对齐：degradation / partial_outage / outage / maintenance。
-- related 数组存关联链接（如根因 PR、postmortem 文档 URL）。
CREATE TABLE IF NOT EXISTS status_incidents (
  id          BIGSERIAL    PRIMARY KEY,
  service_key TEXT         NOT NULL,
  started_at  TIMESTAMPTZ  NOT NULL,
  ended_at    TIMESTAMPTZ,
  severity    TEXT         NOT NULL CHECK (severity IN ('degradation','partial_outage','outage','maintenance')),
  title       TEXT         NOT NULL,
  summary     TEXT,
  related     TEXT[]       NOT NULL DEFAULT ARRAY[]::TEXT[],
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  CONSTRAINT status_incidents_time_check CHECK (ended_at IS NULL OR ended_at >= started_at)
);
-- 状态页首屏取最近 30 天 incident 按时间倒序
CREATE INDEX IF NOT EXISTS idx_status_incidents_recent
  ON status_incidents(started_at DESC);
-- 按 service 过滤 + 按时间倒序（service 卡片 hover 显示当天 incident 用）
CREATE INDEX IF NOT EXISTS idx_status_incidents_service_time
  ON status_incidents(service_key, started_at DESC);

-- +goose Down

DROP TABLE IF EXISTS status_incidents;
DROP TABLE IF EXISTS status_uptime_daily;
DROP TABLE IF EXISTS status_uptime_5min;
