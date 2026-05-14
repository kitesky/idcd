CREATE TABLE status_page_subscriptions (
  id             TEXT PRIMARY KEY,
  status_page_id TEXT NOT NULL,
  channel_type   TEXT NOT NULL,
  endpoint       TEXT NOT NULL,
  verified       BOOLEAN NOT NULL DEFAULT FALSE,
  verify_token   TEXT UNIQUE,
  events         TEXT[] NOT NULL DEFAULT ARRAY['incident','recovery','maintenance'],
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON status_page_subscriptions(status_page_id);
CREATE INDEX ON status_page_subscriptions(verify_token) WHERE verify_token IS NOT NULL;
