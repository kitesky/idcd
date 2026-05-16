-- +goose Up

-- 00040_i18n_status_page_default_locale.sql — i18n Phase 2c (DB migrations) ②
--
-- 状态页 owner 可指定访客默认语言（D5）。访客 locale 决策链：
--   1. ?lang= query 参数
--   2. status_pages.default_locale（本字段，owner 配置）
--   3. Accept-Language negotiation
--   4. registry.default ('cn')
--
-- 详 docs/prd/I18N-PLAN.md §2.6 / §4.2。

ALTER TABLE status_pages
  ADD COLUMN IF NOT EXISTS default_locale TEXT NOT NULL DEFAULT 'cn';

-- +goose Down

ALTER TABLE status_pages
  DROP COLUMN IF EXISTS default_locale;
