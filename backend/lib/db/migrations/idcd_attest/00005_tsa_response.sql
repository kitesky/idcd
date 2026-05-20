-- +goose Up
-- +goose StatementBegin
-- v2 S2 Evidence: tsa_response TSA 服务调用记录表
-- 同 schema FK: used_by_report_id -> idcd_attest.verdict_report(id) 保留 REFERENCES
CREATE TABLE IF NOT EXISTS idcd_attest.tsa_response (
  id                 TEXT PRIMARY KEY,                        -- tsa_*
  provider           TEXT NOT NULL,                           -- digicert|globalsign|ntsc
  request_hash       TEXT NOT NULL,
  response_blob      BYTEA,
  serial_number      TEXT,
  issued_at          TIMESTAMPTZ,
  valid_until        TIMESTAMPTZ,
  status             TEXT NOT NULL,                           -- success|failure|timeout
  latency_ms         INT,
  used_by_report_id  TEXT
                     REFERENCES idcd_attest.verdict_report(id), -- 同 schema FK 保留
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_tsa_response_provider_time
  ON idcd_attest.tsa_response(provider, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_tsa_response_status
  ON idcd_attest.tsa_response(status)
  WHERE status != 'success';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_tsa_response_status;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idcd_attest.idx_tsa_response_provider_time;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS idcd_attest.tsa_response;
-- +goose StatementEnd
