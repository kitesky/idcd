-- +goose Up

CREATE TABLE incident_postmortems (
  id             TEXT PRIMARY KEY,
  alert_event_id TEXT NOT NULL UNIQUE,
  monitor_id     TEXT NOT NULL,
  user_id        TEXT NOT NULL,
  title          TEXT NOT NULL,
  status         TEXT NOT NULL DEFAULT 'draft',
  severity       TEXT NOT NULL DEFAULT 'low',
  impact         TEXT NOT NULL DEFAULT '',
  timeline       JSONB NOT NULL DEFAULT '[]',
  root_cause     TEXT NOT NULL DEFAULT '',
  resolution     TEXT NOT NULL DEFAULT '',
  action_items   JSONB NOT NULL DEFAULT '[]',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON incident_postmortems(user_id);
CREATE INDEX ON incident_postmortems(alert_event_id);
