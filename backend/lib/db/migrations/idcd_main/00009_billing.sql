-- +goose Up

-- 00009_billing.sql
-- S2 billing tables: subscriptions, invoices, payments, status_pages
-- D1 rule: NO cross-schema FOREIGN KEY REFERENCES; all joins done at application layer.
-- 支付走聚合支付（微信 / 支付宝 / Stripe），所有外部 ID 列统一命名 ext_*。

-- subscriptions: 订阅
CREATE TABLE IF NOT EXISTS subscriptions (
  id              TEXT PRIMARY KEY,          -- sub_前缀 nanoid
  user_id         TEXT NOT NULL,             -- 应用层 join，无 FK
  plan            TEXT NOT NULL CHECK (plan IN ('free','pro','team','business')),
  status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','cancelled','past_due','paused')),
  provider        TEXT NOT NULL DEFAULT 'stub',  -- 'stub' | 'wepay' | 'alipay' | 'stripe'
  ext_sub_id      TEXT UNIQUE,               -- 支付方订阅 ID（接入后填充）
  current_period_start TIMESTAMPTZ,
  current_period_end   TIMESTAMPTZ,
  cancel_at       TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions(user_id);

-- invoices: 发票记录
CREATE TABLE IF NOT EXISTS invoices (
  id              TEXT PRIMARY KEY,          -- inv_前缀 nanoid
  user_id         TEXT NOT NULL,
  subscription_id TEXT,                      -- 应用层 join，无 FK
  provider        TEXT NOT NULL DEFAULT 'stub',
  ext_invoice_id  TEXT,
  amount_cents    INTEGER NOT NULL,
  currency        TEXT NOT NULL DEFAULT 'CNY',
  status          TEXT NOT NULL CHECK (status IN ('paid','pending','refunded','failed')),
  paid_at         TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_invoices_user ON invoices(user_id, created_at DESC);

-- payments: 支付流水
CREATE TABLE IF NOT EXISTS payments (
  id              TEXT PRIMARY KEY,          -- pay_前缀 nanoid
  user_id         TEXT NOT NULL,
  invoice_id      TEXT,                      -- 应用层 join，无 FK
  provider        TEXT NOT NULL DEFAULT 'stub',
  ext_txn_id      TEXT UNIQUE,
  amount_cents    INTEGER NOT NULL,
  currency        TEXT NOT NULL DEFAULT 'CNY',
  status          TEXT NOT NULL CHECK (status IN ('succeeded','failed','refunded','refund_failed')),
  refund_retry_count INTEGER NOT NULL DEFAULT 0,
  refund_failed_at TIMESTAMPTZ,
  metadata        JSONB NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_payments_user ON payments(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_refund_failed ON payments(status, refund_failed_at) WHERE status = 'refund_failed';

-- status_pages: 状态页
CREATE TABLE IF NOT EXISTS status_pages (
  id              TEXT PRIMARY KEY,          -- sp_前缀 nanoid
  user_id         TEXT NOT NULL,
  slug            TEXT NOT NULL UNIQUE,      -- URL slug: <slug>.status.idcd.com
  name            TEXT NOT NULL,
  description     TEXT,
  custom_domain   TEXT,
  branding        BOOLEAN NOT NULL DEFAULT TRUE,  -- TRUE=显示水印
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_status_pages_user ON status_pages(user_id);
