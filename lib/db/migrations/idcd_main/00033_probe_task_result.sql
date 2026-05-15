-- +goose Up
ALTER TABLE probe_task ADD COLUMN IF NOT EXISTS result JSONB;

-- +goose Down
ALTER TABLE probe_task DROP COLUMN IF EXISTS result;
