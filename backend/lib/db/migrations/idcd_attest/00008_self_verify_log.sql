-- +goose Up
-- +goose StatementBegin
-- self_verify_log: 独立 attest-verify 服务 (D6) 的验证结果日志表
-- 与 verdict_report.self_verify_status 独立：
--   - 该表由 apps/attest-verify 独立写入，apps/attest 不读不写
--   - record_id 指向 attestation_record.id，但无 REFERENCES（D1 跨路径不写 FK）
--   - id 前缀 svl_，与 att_（attestation_record）、vr_（verdict_report）区分
CREATE TABLE IF NOT EXISTS idcd_attest.self_verify_log (
    id           TEXT PRIMARY KEY,              -- svl_* (由 apps/attest-verify 生成)
    record_id    TEXT NOT NULL,                 -- attestation_record.id 弱引用 (D1: 无 FK)
    verified_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status       TEXT NOT NULL,                 -- pass | fail | error
    latency_ms   BIGINT,                        -- 从发起请求到收到响应的毫秒数
    error        TEXT,                          -- 失败/错误原因；pass 时为 NULL
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- record_id 索引：用于去重查询（LEFT JOIN ON svl.record_id = ar.id）
CREATE INDEX IF NOT EXISTS idx_self_verify_log_record
    ON idcd_attest.self_verify_log(record_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- status + created_at 索引：用于监控/告警查询（最近 N 条 fail/error）
CREATE INDEX IF NOT EXISTS idx_self_verify_log_status_time
    ON idcd_attest.self_verify_log(status, created_at DESC);
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_self_verify_log_status_time;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_self_verify_log_record;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS idcd_attest.self_verify_log;
-- +goose StatementEnd
