-- +goose Up
-- +goose StatementBegin
-- v2 S2 Evidence: attestation_record 审计跟踪表 (D4 WAL)
-- D4: attestation_record 充当 Verdict 生成流程的 WAL(Write-Ahead Log)。
--     每 step 完成后 worker 写一条 (report_id, action, status, external_id, result=success)。
--     Worker crash 后续跑时: 先查 attestation_record 中此 report_id 已成功 step,
--     跳过续跑下一 step。
-- D4: UNIQUE(report_id, action) 防止重复签名 / 重复时间戳 / 重复归档 —
--     严格 step-level idempotency。KMS sign 必传 idempotency_key。
-- 同 schema FK: report_id -> idcd_attest.verdict_report(id) 保留 REFERENCES
CREATE TABLE IF NOT EXISTS idcd_attest.attestation_record (
  id               TEXT PRIMARY KEY,                         -- att_*
  report_id        TEXT NOT NULL
                   REFERENCES idcd_attest.verdict_report(id),-- 同 schema FK 保留
  action           TEXT NOT NULL,                            -- signed|tsa_stamped|anchored|s3_archived|self_verified|revoked
  status           TEXT NOT NULL DEFAULT 'pending',          -- D4: pending|success|failure
  external_id      TEXT,                                     -- TSA serial / chain tx hash / KMS req id / S3 ETag
  idempotency_key  TEXT,                                     -- D4: 外部 API(如 AWS KMS) idempotency token
  payload_hash     TEXT,
  result           TEXT NOT NULL,                            -- success|failure
  error_detail     TEXT,
  retry_count      INT NOT NULL DEFAULT 0,                   -- D4: step 重试次数(<=3)
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at     TIMESTAMPTZ,                              -- D4: success 时记录
  CONSTRAINT uq_attestation_report_action UNIQUE (report_id, action) -- D4: step-level idempotency
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_attestation_report
  ON idcd_attest.attestation_record(report_id, created_at);
-- +goose StatementEnd

-- +goose StatementBegin
-- D4: WAL replay 查询 — SELECT action FROM attestation_record WHERE report_id=$1 AND status='success'
CREATE INDEX IF NOT EXISTS idx_attestation_pending
  ON idcd_attest.attestation_record(status)
  WHERE status = 'pending';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_attestation_pending;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_attestation_report;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS idcd_attest.attestation_record;
-- +goose StatementEnd
