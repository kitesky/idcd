-- +goose Up

CREATE TABLE user_session (
  id                  text        PRIMARY KEY,
  user_id             text        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  refresh_token_hash  text        NOT NULL,
  device              text,
  client_ip           inet,
  user_agent          text,
  workspace_id        text,
  created_at          timestamptz NOT NULL DEFAULT now(),
  expires_at          timestamptz NOT NULL,
  revoked_at          timestamptz
);

CREATE INDEX idx_user_session_user
  ON user_session(user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_user_session_token
  ON user_session(refresh_token_hash);
CREATE INDEX idx_user_session_expires
  ON user_session(expires_at) WHERE revoked_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS user_session;
