-- +goose Up

ALTER TABLE api_key ADD COLUMN team_id TEXT;

CREATE INDEX ON api_key(team_id) WHERE team_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS api_key_team_id_idx;
ALTER TABLE api_key DROP COLUMN IF EXISTS team_id;
