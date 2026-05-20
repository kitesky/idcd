-- +goose Up

-- verdict_order 加 promotion_id snapshot 列，对齐 subscriptions 同名列。
-- 配套 idcd_main/00048 的统一 pricing_items / pricing_promotions 模型。
-- D1: 跨 schema 不写 FK，promotion_id 是 pricing_promotions.id 的弱引用。
ALTER TABLE idcd_attest.verdict_order ADD COLUMN IF NOT EXISTS promotion_id TEXT;

-- +goose Down

ALTER TABLE idcd_attest.verdict_order DROP COLUMN IF EXISTS promotion_id;
