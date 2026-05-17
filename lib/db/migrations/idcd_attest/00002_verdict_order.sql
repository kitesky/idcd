-- +goose Up
-- +goose StatementBegin
-- v2 S2 Evidence: verdict_order 订单 / 计费表
-- D1: owner_id 指向 idcd_main.users(id), paddle_order_id 指向 idcd_main 09 order
--     跨 schema 不写 REFERENCES; 由应用层 Repository 校验存在性 + 应用层 join。
-- D5: refund_failed 状态用于 admin 后台 dashboard, 任何此状态订单触发 P0 告警;
--     refund_attempt_count / refund_last_error / refund_apology_sent_at 支撑 5min/30min retry 队列。
CREATE TABLE IF NOT EXISTS idcd_attest.verdict_order (
  id                      TEXT PRIMARY KEY,                        -- v_*
  owner_id                TEXT NOT NULL,                           -- D1: 跨 schema 不写 FK; 应用层 Repository 校验存在
  template                TEXT NOT NULL,                           -- sla|incident|compliance|legal
  target                  TEXT NOT NULL,                           -- domain|url|ip
  time_window_start       TIMESTAMPTZ NOT NULL,
  time_window_end         TIMESTAMPTZ NOT NULL,
  status                  TEXT NOT NULL,                           -- pending|paid|generating|delivered|failed|refunded|refund_failed
  price_cny               NUMERIC(10,2) NOT NULL,
  price_paid_cny          NUMERIC(10,2),                           -- 实收(Paddle 扣费后)
  paddle_order_id         TEXT,                                    -- D1: 关联 idcd_main 09 order, 不写 FK
  refund_reason           TEXT,
  refund_attempt_count    INT NOT NULL DEFAULT 0,                  -- D5: refund retry queue 累计次数
  refund_last_error       TEXT,                                    -- D5: 最后一次 refund 失败原因(Paddle 风控 / 网络 等)
  refund_apology_sent_at  TIMESTAMPTZ,                             -- D5: 道歉邮箱发送时间(30min 失败兜底)
  created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
  paid_at                 TIMESTAMPTZ,
  delivered_at            TIMESTAMPTZ,
  failed_at               TIMESTAMPTZ,
  refunded_at             TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_verdict_order_owner
  ON idcd_attest.verdict_order(owner_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_verdict_order_status
  ON idcd_attest.verdict_order(status)
  WHERE status IN ('paid', 'generating', 'refund_failed');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_verdict_order_status;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_verdict_order_owner;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS idcd_attest.verdict_order;
-- +goose StatementEnd
