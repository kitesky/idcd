-- +goose Up

-- Add fingerprint tracking to enrolled_nodes
ALTER TABLE enrolled_nodes
    ADD COLUMN IF NOT EXISTS fingerprint          JSONB,
    ADD COLUMN IF NOT EXISTS fingerprint_updated_at TIMESTAMPTZ;

-- Command queue for OTA upgrades and config hot-reload
CREATE TABLE node_commands (
    id          TEXT        PRIMARY KEY,
    node_id     TEXT        NOT NULL,
    command     TEXT        NOT NULL CHECK (command IN ('upgrade', 'reload_config')),
    payload     JSONB       NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'sent', 'acked', 'failed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at     TIMESTAMPTZ,
    acked_at    TIMESTAMPTZ
);

-- fast lookup of pending commands per node (used on every heartbeat)
CREATE INDEX idx_node_commands_pending ON node_commands (node_id, status)
    WHERE status = 'pending';

-- +goose Down

DROP INDEX IF EXISTS idx_node_commands_pending;
DROP TABLE  IF EXISTS node_commands;
ALTER TABLE enrolled_nodes
    DROP COLUMN IF EXISTS fingerprint_updated_at,
    DROP COLUMN IF EXISTS fingerprint;
