-- +goose Up

-- probe_result: 节点上报的探测原始结果（TimescaleDB hypertable）
-- 与 probe_task 关联但跨 schema 不写 FK（D1）；任务侧的 probe_task.result 是 last-write 摘要，
-- 这里保留每次节点上报的完整 raw + summary，供 attest / 排障 / 时序回放使用。
--
-- 主键含 created_at 是 TimescaleDB hypertable 的硬性要求（分区列必须在所有唯一索引内）。
-- 应用层去重靠 (task_id, node_id) 业务键 + aggregator 的 Redis dedup。
CREATE TABLE IF NOT EXISTS probe_result (
  id           TEXT        NOT NULL,
  task_id      TEXT        NOT NULL,
  node_id      TEXT        NOT NULL,
  raw          JSONB,
  summary      JSONB,
  duration_ms  INTEGER,
  success      BOOLEAN,
  error        TEXT,
  signature    TEXT        NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (task_id, node_id, created_at)
);

SELECT create_hypertable('probe_result', 'created_at', chunk_time_interval => INTERVAL '1 day', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_probe_result_task    ON probe_result (task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_probe_result_node    ON probe_result (node_id, created_at DESC);
-- 排障查询只关心失败样本；partial index 把 success=true 的（占绝大多数）剔除，
-- 显著降低写放大与索引体积。
CREATE INDEX IF NOT EXISTS idx_probe_result_failure ON probe_result (created_at DESC) WHERE success = FALSE;

-- +goose Down

DROP TABLE IF EXISTS probe_result;
