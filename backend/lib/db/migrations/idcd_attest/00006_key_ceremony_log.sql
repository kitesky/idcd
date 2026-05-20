-- +goose Up
-- +goose StatementBegin
-- v2 S2 Evidence: key_ceremony_log 密钥仪式审计表
-- 此表只增不删,且需要双人审批的应用层写入控制(详 11-admin)。
-- actors 字段记录参与仪式的所有人员(创始人 / 外部公证 / Shamir share holder 等)。
CREATE TABLE IF NOT EXISTS idcd_attest.key_ceremony_log (
  id            TEXT PRIMARY KEY,                            -- kc_*
  action        TEXT NOT NULL,                               -- root_gen|root_split|sign_key_rotate|emergency_revoke
  key_id        TEXT,
  key_version   INT,
  actors        JSONB NOT NULL,                              -- [{user_id|external_id, role}, ...]
  evidence_url  TEXT,                                        -- 录像 / 公证 PDF
  notes         TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS idcd_attest.key_ceremony_log;
-- +goose StatementEnd
