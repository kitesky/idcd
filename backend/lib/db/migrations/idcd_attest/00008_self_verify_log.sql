-- +goose Up
-- +goose StatementBegin
-- self_verify_log: 独立 attest-verify 服务 (D6) 的验证结果日志表
-- 与 verdict_report.self_verify_status 独立：
--   - 该表由 apps/attest-verify 独立写入，apps/attest 不读不写
--   - record_id 指向 attestation_record.id，但无 REFERENCES（D1 跨路径不写 FK）
--   - id 前缀 svl_，与 att_（attestation_record）、vr_（verdict_report）区分
--   - record_id 上的 UNIQUE 约束：每条记录最多一行日志。配合 INSERT 的
--     ON CONFLICT (record_id) DO NOTHING，防止 worker 重启 / 重投递时
--     被 status=error 行永久锁出（listPendingSQL 用 LEFT JOIN NULL 去重）。
CREATE TABLE IF NOT EXISTS idcd_attest.self_verify_log (
    id           TEXT PRIMARY KEY,              -- svl_* (由 apps/attest-verify 生成)
    record_id    TEXT NOT NULL UNIQUE,          -- attestation_record.id 弱引用 (D1: 无 FK)
    verified_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status       TEXT NOT NULL
                 CHECK (status IN ('pass', 'fail', 'error')),
    latency_ms   BIGINT,                        -- 从发起请求到收到响应的毫秒数；NULL 表示请求前失败
    error        TEXT,                          -- 失败/错误原因；pass 时为 NULL
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- status + created_at 索引：用于监控/告警查询（最近 N 条 fail/error）
-- record_id 的查询索引由 UNIQUE 约束自动提供，无需额外 CREATE INDEX。
CREATE INDEX IF NOT EXISTS idx_self_verify_log_status_time
    ON idcd_attest.self_verify_log(status, created_at DESC);
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_self_verify_log_status_time;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS idcd_attest.self_verify_log;
-- +goose StatementEnd
