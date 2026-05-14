-- +goose Up

ALTER TABLE api_key ADD COLUMN key_type TEXT NOT NULL DEFAULT 'production';
CREATE INDEX idx_api_key_owner_keytype ON api_key(owner_id, key_type);

-- +goose Down

DROP INDEX IF EXISTS idx_api_key_owner_keytype;
ALTER TABLE api_key DROP COLUMN IF EXISTS key_type;
