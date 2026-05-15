-- +goose Up
ALTER TABLE probe_task
  ADD COLUMN IF NOT EXISTS speed_download_mbps NUMERIC(10,2),
  ADD COLUMN IF NOT EXISTS speed_upload_mbps   NUMERIC(10,2);

-- +goose Down
ALTER TABLE probe_task
  DROP COLUMN IF EXISTS speed_download_mbps,
  DROP COLUMN IF EXISTS speed_upload_mbps;
