-- +goose Up

-- 00041_i18n_subscription_locale.sql — i18n Phase 2c (DB migrations) ③
--
-- 状态页订阅者邮件按订阅时提交的 locale 发送（详 docs/prd/I18N-PLAN.md §4.2）。
--
-- 现有表名是 status_page_subscriptions（00027_status_subscriptions.sql 创建）。
-- I18N-PLAN.md 中以 email_subscriptions 指代的就是此表 —— 当前实现把所有渠道
-- （email/webhook/...）合并存一张表，channel_type 区分。locale 字段对所有渠道
-- 都有意义（webhook 也可能想本地化 payload），所以加在表级。

ALTER TABLE status_page_subscriptions
  ADD COLUMN IF NOT EXISTS locale TEXT NOT NULL DEFAULT 'cn';

-- +goose Down

ALTER TABLE status_page_subscriptions
  DROP COLUMN IF EXISTS locale;
