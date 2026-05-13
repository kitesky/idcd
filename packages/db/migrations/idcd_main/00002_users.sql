-- +goose Up

CREATE TABLE "user" (
  id                        text        PRIMARY KEY,
  email                     citext      UNIQUE NOT NULL,
  email_verified_at         timestamptz,
  phone                     text,
  phone_verified_at         timestamptz,
  username                  citext      UNIQUE,
  display_name              text,
  avatar_url                text,
  bio                       text,
  locale                    text        NOT NULL DEFAULT 'zh-CN',
  timezone                  text        NOT NULL DEFAULT 'Asia/Shanghai',
  password_hash             text,
  password_changed_at       timestamptz,
  status                    text        NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active','locked','pending_deletion','deleted')),
  pending_deletion_at       timestamptz,
  email_marketing_opted_in  boolean     NOT NULL DEFAULT true,
  last_login_at             timestamptz,
  last_login_ip             inet,
  created_at                timestamptz NOT NULL DEFAULT now(),
  updated_at                timestamptz NOT NULL DEFAULT now(),
  deleted_at                timestamptz
);

CREATE INDEX idx_user_status    ON "user"(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_user_email_lc  ON "user"(lower(email::text));
CREATE INDEX idx_user_deleted   ON "user"(deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TRIGGER trg_user_updated_at
  BEFORE UPDATE ON "user"
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 多登录凭证（password/wechat/github/google/phone）
CREATE TABLE user_credential (
  id          text        PRIMARY KEY,
  user_id     text        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  type        text        NOT NULL,
  external_id text,
  metadata    jsonb       NOT NULL DEFAULT '{}',
  linked_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_user_credential_type_ext
  ON user_credential(type, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX idx_user_credential_user ON user_credential(user_id);

-- 2FA（TOTP / WebAuthn）
CREATE TABLE user_2fa (
  user_id                  text    PRIMARY KEY REFERENCES "user"(id) ON DELETE CASCADE,
  type                     text    NOT NULL,
  secret_encrypted         bytea,
  backup_codes_encrypted   bytea,
  enabled_at               timestamptz NOT NULL DEFAULT now()
);

-- OTP 临时存储（邮箱验证 / 密码重置）
CREATE TABLE user_otp (
  id           text        PRIMARY KEY,
  user_id      text        NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  type         text        NOT NULL CHECK (type IN ('email_verify','password_reset','login')),
  code_hash    text        NOT NULL,
  attempts     integer     NOT NULL DEFAULT 0,
  expires_at   timestamptz NOT NULL,
  used_at      timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_otp_user_type ON user_otp(user_id, type) WHERE used_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS user_otp;
DROP TABLE IF EXISTS user_2fa;
DROP TABLE IF EXISTS user_credential;
DROP TABLE IF EXISTS "user";
