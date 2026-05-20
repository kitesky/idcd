-- +goose Up

-- 统一定价模型：所有付费项（plan 月费 + verdict 单次报告 + 未来证书/API 加量包）
-- 共用一张 pricing_items 表，按 (kind, item_key) 索引；促销规则同样可跨品类。
-- 业务方需要按时段/优惠码/品类做促销，不重新发版改价。

-- subscriptions.plan CHECK 顺手补 'agent_pro'（provider.go 已定义 PlanAgentPro
-- 但 00009 的 CHECK 漏了，订阅该档会被 23514 拒）。
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_plan_check;
ALTER TABLE subscriptions
  ADD CONSTRAINT subscriptions_plan_check
  CHECK (plan IN ('free','pro','agent_pro','team','business'));

-- subscriptions 价格 snapshot 三件套：实付金额 / 币种 / 命中的 promo。
-- 后期改基础价或促销结束都不影响历史订阅的续费金额。NULL = 历史行无 snapshot。
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS amount_cents BIGINT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'CNY';
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS promotion_id TEXT;

-- pricing_items: 所有可定价商品 / 档位的基础价（admin 可改）。
-- kind 是品类（plan / verdict_template / 未来扩 cert / api_pack 等），item_key 是该品类内的具体 key。
-- (kind, item_key) 复合 PK 保证全局唯一。
CREATE TABLE IF NOT EXISTS pricing_items (
  kind         TEXT NOT NULL CHECK (kind IN ('plan','verdict_template')),
  item_key     TEXT NOT NULL,
  price_cents  BIGINT NOT NULL CHECK (price_cents >= 0),
  currency     TEXT   NOT NULL DEFAULT 'CNY',
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by   TEXT,
  PRIMARY KEY (kind, item_key)
);

-- Seed: plan 5 档 + verdict_template 4 档。
-- ON CONFLICT 保证 migration 幂等且不会覆盖运维改过的价格。
INSERT INTO pricing_items (kind, item_key, price_cents, currency) VALUES
  ('plan', 'free',      0,     'CNY'),
  ('plan', 'pro',       9900,  'CNY'),
  ('plan', 'agent_pro', 29900, 'CNY'),
  ('plan', 'team',      29900, 'CNY'),
  ('plan', 'business',  99900, 'CNY'),
  ('verdict_template', 'sla',        29900, 'CNY'),
  ('verdict_template', 'incident',   19900, 'CNY'),
  ('verdict_template', 'compliance', 49900, 'CNY'),
  ('verdict_template', 'legal',      99900, 'CNY')
ON CONFLICT (kind, item_key) DO NOTHING;

-- pricing_promotions: 促销规则（时段折扣，可选 coupon code）
-- 适用范围三层（精确 → 泛化）：
--   applies_kind=X, applies_key=Y  仅 (X,Y) 该单品
--   applies_kind=X, applies_key=NULL  该品类全场（如 plan 全场 8 折）
--   applies_kind=NULL, applies_key=NULL  全平台所有付费品
-- discount_type:
--   percent  — value=1..100，按百分比打折
--   amount   — value=减免分数（基础价 - value，>=0）
--   override — value=直接定为最终价（分），如"限时 ¥9 = 900"
-- coupon_code NULL = 自动应用（活动期间所有用户享受，无需输码）
CREATE TABLE IF NOT EXISTS pricing_promotions (
  id            TEXT PRIMARY KEY,
  applies_kind  TEXT,
  applies_key   TEXT,
  discount_type TEXT NOT NULL CHECK (discount_type IN ('percent','amount','override')),
  value         BIGINT NOT NULL CHECK (value > 0),
  coupon_code   TEXT,
  start_at      TIMESTAMPTZ NOT NULL,
  end_at        TIMESTAMPTZ NOT NULL,
  max_uses      INTEGER,
  used_count    INTEGER NOT NULL DEFAULT 0,
  active        BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT pricing_promotions_time_check CHECK (start_at < end_at),
  CONSTRAINT pricing_promotions_percent_range
    CHECK (discount_type != 'percent' OR (value > 0 AND value <= 100)),
  -- 不允许 applies_kind=NULL 但 applies_key=非 NULL（语义无效）
  CONSTRAINT pricing_promotions_scope_check
    CHECK (applies_kind IS NOT NULL OR applies_key IS NULL)
);
CREATE INDEX IF NOT EXISTS idx_pricing_promotions_active_window
  ON pricing_promotions(active, start_at, end_at) WHERE active = TRUE;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_pricing_promotions_coupon
  ON pricing_promotions(coupon_code) WHERE coupon_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_pricing_promotions_scope
  ON pricing_promotions(applies_kind, applies_key);

-- +goose Down

DROP TABLE IF EXISTS pricing_promotions;
DROP TABLE IF EXISTS pricing_items;

ALTER TABLE subscriptions DROP COLUMN IF EXISTS promotion_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS currency;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS amount_cents;

ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_plan_check;
ALTER TABLE subscriptions
  ADD CONSTRAINT subscriptions_plan_check
  CHECK (plan IN ('free','pro','team','business'));
