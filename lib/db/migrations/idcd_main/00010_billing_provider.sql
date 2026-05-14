-- 00010_billing_provider.sql
-- Rename Paddle-specific columns to generic provider columns
-- D1 rule: NO cross-schema FOREIGN KEY REFERENCES; all joins done at application layer.

-- subscriptions: paddle_sub_id → ext_sub_id，新增 provider 列
ALTER TABLE subscriptions
  ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'stub',
  ADD COLUMN IF NOT EXISTS ext_sub_id TEXT;

-- 如果 paddle_sub_id 列存在则迁移数据并删除旧列
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name='subscriptions' AND column_name='paddle_sub_id') THEN
    UPDATE subscriptions SET ext_sub_id = paddle_sub_id WHERE paddle_sub_id IS NOT NULL;
    ALTER TABLE subscriptions DROP COLUMN paddle_sub_id;
  END IF;
END $$;

-- invoices: paddle_invoice_id → ext_invoice_id
ALTER TABLE invoices
  ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'stub',
  ADD COLUMN IF NOT EXISTS ext_invoice_id TEXT;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name='invoices' AND column_name='paddle_invoice_id') THEN
    UPDATE invoices SET ext_invoice_id = paddle_invoice_id WHERE paddle_invoice_id IS NOT NULL;
    ALTER TABLE invoices DROP COLUMN paddle_invoice_id;
  END IF;
END $$;

-- payments: paddle_txn_id → ext_txn_id
ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'stub',
  ADD COLUMN IF NOT EXISTS ext_txn_id TEXT;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name='payments' AND column_name='paddle_txn_id') THEN
    UPDATE payments SET ext_txn_id = paddle_txn_id WHERE paddle_txn_id IS NOT NULL;
    ALTER TABLE payments DROP COLUMN paddle_txn_id;
  END IF;
END $$;

-- 支付提供商配置表（每个 provider 的配置存这里）
CREATE TABLE IF NOT EXISTS payment_providers (
  id          TEXT PRIMARY KEY DEFAULT 'default',
  provider    TEXT NOT NULL DEFAULT 'stub',  -- 'stub' | 'wepay' | 'alipay' | 'stripe'
  enabled     BOOLEAN NOT NULL DEFAULT TRUE,
  config      JSONB NOT NULL DEFAULT '{}',   -- 加密配置，app层解密
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO payment_providers (id, provider) VALUES ('default', 'stub') ON CONFLICT DO NOTHING;
