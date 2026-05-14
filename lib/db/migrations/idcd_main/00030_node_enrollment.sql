-- +goose Up

-- one-time enrollment tokens (token itself never stored — only SHA-256 hash)
CREATE TABLE node_enrollment_tokens (
    id          TEXT        PRIMARY KEY,
    token_hash  TEXT        NOT NULL UNIQUE,
    label       TEXT,                              -- human label, e.g. "jp-tokyo-01"
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '24 hours',
    used_at     TIMESTAMPTZ,
    used_by     TEXT,                              -- node_id that consumed this token
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- enrolled agent nodes registered via one-time token
CREATE TABLE enrolled_nodes (
    id             TEXT        PRIMARY KEY,
    node_id        TEXT        NOT NULL UNIQUE,    -- public ID, e.g. nd_k7mNpQr2xZ9T
    secret_hash    TEXT        NOT NULL UNIQUE,    -- SHA-256(secret_key) for gateway auth
    hostname       TEXT,
    arch           TEXT,                           -- amd64 | arm64
    os             TEXT,                           -- linux
    kernel         TEXT,
    ip_address     TEXT,
    agent_version  TEXT,
    status         TEXT        NOT NULL DEFAULT 'pending'
                               CHECK (status IN ('pending','active','drained','offline','disabled')),
    enrolled_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at   TIMESTAMPTZ,
    metadata       JSONB       NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_enrolled_nodes_status     ON enrolled_nodes (status);
CREATE INDEX idx_enrolled_nodes_last_seen  ON enrolled_nodes (last_seen_at);

-- +goose Down

DROP TABLE IF EXISTS enrolled_nodes;
DROP TABLE IF EXISTS node_enrollment_tokens;
