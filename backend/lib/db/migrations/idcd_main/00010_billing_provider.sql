-- +goose Up

-- 00010_billing_provider.sql
-- payment_providers 配置表（每个支付通道一行：stub / wepay / alipay / stripe）。
-- subscriptions / invoices / payments 的 provider + ext_* 列已在 00009 一次成型，
-- 本迁移仅补出 provider 配置表。

CREATE TABLE IF NOT EXISTS payment_providers (
  id          TEXT PRIMARY KEY DEFAULT 'default',
  provider    TEXT NOT NULL DEFAULT 'stub',  -- 'stub' | 'wepay' | 'alipay' | 'stripe'
  enabled     BOOLEAN NOT NULL DEFAULT TRUE,
  config      JSONB NOT NULL DEFAULT '{}',   -- 加密配置，app 层解密
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO payment_providers (id, provider) VALUES ('default', 'stub') ON CONFLICT DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS payment_providers;
