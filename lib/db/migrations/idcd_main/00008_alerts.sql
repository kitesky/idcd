-- +goose Up

-- 00008_alerts.sql — Alert channels, policies, events, and notifications

-- alert_channels: 告警通道
CREATE TABLE IF NOT EXISTS alert_channels (
  id          TEXT PRIMARY KEY,         -- ch_前缀 nanoid
  user_id     TEXT NOT NULL,
  name        TEXT NOT NULL,
  type        TEXT NOT NULL CHECK (type IN ('email','webhook','wecom','dingtalk','feishu','telegram','slack')),
  config      JSONB NOT NULL DEFAULT '{}',   -- 各通道配置（url/token/secret等）
  verified    BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_alert_channels_user ON alert_channels(user_id);

-- alert_policies: 告警策略
CREATE TABLE IF NOT EXISTS alert_policies (
  id            TEXT PRIMARY KEY,       -- pol_前缀 nanoid
  user_id       TEXT NOT NULL,
  monitor_id    TEXT NOT NULL,          -- 应用层 join，无 FK
  channel_ids   TEXT[] NOT NULL DEFAULT '{}',
  name          TEXT NOT NULL,
  delay_s       INTEGER NOT NULL DEFAULT 0,    -- 延迟告警秒数（避免误报）
  recovery_n    INTEGER NOT NULL DEFAULT 3,    -- 连续N次成功才恢复
  mute_start    TIME,                          -- 静音开始时间
  mute_end      TIME,                          -- 静音结束时间
  enabled       BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_alert_policies_monitor ON alert_policies(monitor_id);

-- alert_events: 告警事件
CREATE TABLE IF NOT EXISTS alert_events (
  id            TEXT PRIMARY KEY,       -- evt_前缀 nanoid
  monitor_id    TEXT NOT NULL,
  policy_id     TEXT NOT NULL,
  status        TEXT NOT NULL CHECK (status IN ('firing','resolved','acknowledged')),
  started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at   TIMESTAMPTZ,
  acknowledged_by TEXT,
  acknowledged_at TIMESTAMPTZ,
  metadata      JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_alert_events_monitor ON alert_events(monitor_id, started_at DESC);

-- alert_notifications: 每次实际发出的通知
CREATE TABLE IF NOT EXISTS alert_notifications (
  id            TEXT PRIMARY KEY,       -- ntf_前缀 nanoid
  event_id      TEXT NOT NULL,
  channel_id    TEXT NOT NULL,
  status        TEXT NOT NULL CHECK (status IN ('pending','sent','failed')),
  sent_at       TIMESTAMPTZ,
  error         TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
