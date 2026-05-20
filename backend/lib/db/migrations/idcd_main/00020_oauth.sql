-- +goose Up

-- OAuth provider credentials are stored in user_credential table (00002_users.sql).
-- The user_credential table already provides:
--   type        TEXT  -- OAuth provider name (e.g. 'dingtalk', 'feishu')
--   external_id TEXT  -- Provider-assigned user ID (openId / open_id)
-- with a unique index: uq_user_credential_type_ext ON user_credential(type, external_id)
-- No additional columns are required.

-- +goose Down
