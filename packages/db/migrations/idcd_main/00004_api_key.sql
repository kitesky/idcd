-- +goose Up

-- API Key（idc_live_xxxx 前缀格式）
CREATE TABLE api_key (
  id                    text        PRIMARY KEY,
  owner_type            text        NOT NULL CHECK (owner_type IN ('user','team')),
  owner_id              text        NOT NULL,
  name                  text        NOT NULL,
  prefix                text        NOT NULL,           -- idc_live_xxxxxxxx（8 字符可见部分）
  secret_hash           text        NOT NULL,           -- SHA-256(full_secret)，不可逆
  scopes                text[]      NOT NULL DEFAULT '{}',
  rate_limit_override   jsonb,
  allowed_ips           cidr[],
  allowed_origins       text[],
  expires_at            timestamptz,
  last_used_at          timestamptz,
  last_used_ip          inet,
  usage_total           bigint      NOT NULL DEFAULT 0,
  status                text        NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','revoked','expired')),
  created_by            text        NOT NULL REFERENCES "user"(id),
  created_at            timestamptz NOT NULL DEFAULT now(),
  revoked_at            timestamptz
);

CREATE UNIQUE INDEX uq_api_key_prefix   ON api_key(prefix);
CREATE INDEX idx_api_key_owner          ON api_key(owner_type, owner_id) WHERE status = 'active';
CREATE INDEX idx_api_key_created_by     ON api_key(created_by);

-- +goose Down

DROP TABLE IF EXISTS api_key;
