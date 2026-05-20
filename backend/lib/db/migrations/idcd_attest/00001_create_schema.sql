-- +goose Up
-- +goose StatementBegin
-- v2 S2 Evidence: 独立 schema 隔离 attestation/verdict 相关数据。
-- D1: 与 idcd_main 跨 schema 不写 FK（应用层 Repository join）。
CREATE SCHEMA IF NOT EXISTS idcd_attest;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS idcd_attest CASCADE;
-- +goose StatementEnd
