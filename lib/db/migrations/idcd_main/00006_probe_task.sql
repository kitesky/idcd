-- +goose Up

CREATE TABLE probe_task (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('http', 'ping', 'dns', 'tcp', 'traceroute', 'diagnose')),
    target TEXT NOT NULL,
    target_normalized TEXT NOT NULL,
    params JSONB DEFAULT '{}'::jsonb,
    initiated_by TEXT, -- user_id or NULL for anonymous
    api_key_id TEXT,
    client_ip INET,
    user_agent TEXT,
    node_selection JSONB DEFAULT '[]'::jsonb,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_probe_task_status ON probe_task(status);
CREATE INDEX idx_probe_task_initiated_by ON probe_task(initiated_by) WHERE initiated_by IS NOT NULL;
CREATE INDEX idx_probe_task_created_at ON probe_task(created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS probe_task;
