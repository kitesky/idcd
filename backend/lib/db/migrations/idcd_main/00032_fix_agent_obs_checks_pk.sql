-- +goose Up

-- Change monitor_agent_obs_checks primary key from (id) to composite (id, checked_at)
-- so TimescaleDB can use checked_at as the hypertable partition column while
-- preserving row uniqueness via the full composite key.
-- D1 rule: NO cross-schema FOREIGN KEY REFERENCES.

ALTER TABLE monitor_agent_obs_checks DROP CONSTRAINT IF EXISTS monitor_agent_obs_checks_pkey;
ALTER TABLE monitor_agent_obs_checks ADD PRIMARY KEY (id, checked_at);

-- +goose Down

ALTER TABLE monitor_agent_obs_checks DROP CONSTRAINT IF EXISTS monitor_agent_obs_checks_pkey;
ALTER TABLE monitor_agent_obs_checks ADD PRIMARY KEY (id);
