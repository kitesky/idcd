-- +goose Up
-- +goose StatementBegin
-- v2 S2 Evidence: verdict_report 已生成报告表
-- 同 schema FK: order_id -> idcd_attest.verdict_order(id) 保留 REFERENCES
-- D-Concern1: report_type 默认 observation_only; 公开 verify 接口返回,避免误用为鉴定结论;
--             保留枚举供 S4 司法鉴定所合作通道升级。
CREATE TABLE IF NOT EXISTS idcd_attest.verdict_report (
  id                     TEXT PRIMARY KEY,                         -- vr_*
  order_id               TEXT NOT NULL UNIQUE
                         REFERENCES idcd_attest.verdict_order(id), -- 同 schema FK 保留
  pdf_url                TEXT NOT NULL,                            -- S3 path
  pdf_size_bytes         BIGINT,
  content_hash           TEXT NOT NULL,                            -- sha256(pdf bytes)
  signature              BYTEA NOT NULL,                           -- KMS sign output
  signature_key_id       TEXT NOT NULL,
  signature_key_version  INT NOT NULL,
  tsa_provider           TEXT NOT NULL,                            -- digicert|globalsign|ntsc
  tsa_response_blob      BYTEA,
  tsa_time               TIMESTAMPTZ NOT NULL,
  blockchain_anchor      JSONB,                                    -- {chain: polygon, tx_hash: ...} OPTIONAL
  nodes_used             JSONB NOT NULL,                           -- [node_id, ...]
  node_consistency_pct   NUMERIC(5,2),                             -- 多节点一致性 0-100
  llm_used               BOOLEAN DEFAULT FALSE,
  llm_model              TEXT,
  llm_prompt_version     TEXT,
  self_verify_status     TEXT,                                     -- pass|fail|pending
  self_verify_at         TIMESTAMPTZ,
  confidence_label       TEXT,                                     -- high|medium|low
  report_type            TEXT NOT NULL DEFAULT 'observation_only', -- D-Concern1: observation_only 默认
  archived_url           TEXT,                                     -- 永久归档 S3 WORM
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_verdict_report_order
  ON idcd_attest.verdict_report(order_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_verdict_report_key_version
  ON idcd_attest.verdict_report(signature_key_id, signature_key_version);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_verdict_report_key_version;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_verdict_report_order;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS idcd_attest.verdict_report;
-- +goose StatementEnd
