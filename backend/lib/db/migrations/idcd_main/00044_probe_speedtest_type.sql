-- 修复 00037_probe_speedtest 遗漏：00037 只加了 speed_*_mbps 列，
-- 没扩 probe_task_type_check 约束，导致 POST /v1/probe/speedtest 始终 500
-- (SQLSTATE 23514)。E2E 测试发现，补上 'speedtest'。

-- +goose Up
ALTER TABLE probe_task DROP CONSTRAINT IF EXISTS probe_task_type_check;
ALTER TABLE probe_task ADD CONSTRAINT probe_task_type_check
  CHECK (type IN ('http','ping','dns','tcp','traceroute','diagnose','smtp','ntp','mtr','speedtest'));

-- +goose Down
ALTER TABLE probe_task DROP CONSTRAINT IF EXISTS probe_task_type_check;
ALTER TABLE probe_task ADD CONSTRAINT probe_task_type_check
  CHECK (type IN ('http','ping','dns','tcp','traceroute','diagnose','smtp','ntp','mtr'));
