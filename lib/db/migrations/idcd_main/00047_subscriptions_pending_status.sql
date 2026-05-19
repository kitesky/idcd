-- +goose Up

-- handler 在 Subscribe 调用 PaymentHub 后立即 INSERT status='pending'，
-- 等 webhook 的 payment.succeeded 才翻成 'active'。
-- 原 00009 的 CHECK 漏了 'pending'，导致 POST /v1/billing/subscribe 500。

ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_status_check;
ALTER TABLE subscriptions
  ADD CONSTRAINT subscriptions_status_check
  CHECK (status IN ('pending','active','cancelled','past_due','paused'));

-- +goose Down

ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_status_check;
ALTER TABLE subscriptions
  ADD CONSTRAINT subscriptions_status_check
  CHECK (status IN ('active','cancelled','past_due','paused'));
