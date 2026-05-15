-- +goose Up
ALTER TABLE probe_task DROP CONSTRAINT IF EXISTS probe_task_type_check;
ALTER TABLE probe_task ADD CONSTRAINT probe_task_type_check
  CHECK (type IN ('http','ping','dns','tcp','traceroute','diagnose','smtp','ntp','mtr'));

-- +goose Down
ALTER TABLE probe_task DROP CONSTRAINT IF EXISTS probe_task_type_check;
ALTER TABLE probe_task ADD CONSTRAINT probe_task_type_check
  CHECK (type IN ('http','ping','dns','tcp','traceroute','diagnose','smtp','ntp'));
