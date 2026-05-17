# 15 · 数据模型(v2)（数据库表结构）

> 关联：所有模块的数据需求汇总；14-tech-architecture.md
> 关联(v2):18-evidence-and-attestation.md §5、19-ai-agent-observability.md §4
> 阶段：S1 起逐步建立，每阶段增量;**v2 新增 9 张表(Verdict / Attestation / TSA / Key ceremony / MCP session/tool_call/token / Agent obs / LLM 复盘扩展)**
> 品牌名占位：`idcd`

---

## 1. 设计原则

1. **关系数据 PG + 时序数据 TimescaleDB（PG 扩展），一套部署**
2. **ID 用人类可读前缀 + nanoid**（`u_xxx` `m_xxx`），便于排查
3. **软删除 + 时间戳三件套**：`created_at` / `updated_at` / `deleted_at`
4. **审计 + 日志单独库**：避免污染业务库
5. **索引按查询模式建**，宁可读慢也不堆冗余索引
6. **JSONB 用于真正动态字段**，关键字段抽出列
7. **不存敏感**（密码哈希 / 信用卡 / 私钥）

---

## 2. 命名规范

| 类型 | 规范 |
|---|---|
| 库名 | `idcd_main` / `idcd_timeseries` / `idcd_audit` |
| 表名 | snake_case 单数 → `user`, `monitor`, `alert_event` |
| 字段 | snake_case，时间字段 `_at` 结尾 |
| 主键 | `id` （类型 `text`，前缀 + nanoid） |
| 外键 | `<entity>_id` |
| 索引 | `idx_<table>_<columns>` |
| 唯一索引 | `uq_<table>_<columns>` |
| 软删除 | `deleted_at timestamptz NULL` |

### ID 前缀表

| 实体 | 前缀 | 示例 |
|---|---|---|
| user | `u_` | `u_aBcDeFg123` |
| team | `t_` | `t_xyz789` |
| api_key | `ak_` | `ak_abc` |
| api_secret | `idc_live_` | `idc_live_abcdefgh...` |
| monitor | `m_` | `m_xxx` |
| monitor_check | `mc_` | |
| alert_event | `ae_` | |
| alert_policy | `ap_` | |
| channel | `ch_` | |
| status_page | `sp_` | |
| status_component | `sc_` | |
| status_incident | `inc_` | |
| node | `nd_` | 含语义：`nd_jp_tk_01_vultr` |
| probe_task | `pt_` | |
| report | `r_` | |
| order | `ord_` | |
| invoice | `inv_` | |
| subscription | `sub_` | |
| payment_method | `pm_` | |
| refund | `rf_` | |
| coupon | `cpn_` | |
| ticket | `tk_` | |
| dashboard | `db_` | |
| audit_log | `al_` | |
| webhook_endpoint | `we_` | |
| event | `evt_` | |
| **verdict_order (v2)** | `v_` | `v_abc123` |
| **verdict_report (v2)** | `vr_` | `vr_xyz` |
| **attestation_record (v2)** | `att_` | `att_def` |
| **tsa_response (v2)** | `tsa_` | `tsa_ghi` |
| **key_ceremony_log (v2)** | `kc_` | `kc_jkl` |
| **mcp_session (v2)** | `mcps_` | `mcps_mno` |
| **mcp_tool_call (v2)** | `mctc_` | `mctc_pqr` |
| **mcp_token (v2)** | `mcpt_` | `mcpt_stu` |
| **agent_obs_monitor (v2)** | `aom_` | `aom_vwx` |
| **agent_obs_event (v2)** | `aoe_` | `aoe_yz0` |
| **compliance_subscription (v2)** | `cs_` | `cs_123` |
| **leaderboard_report (v2)** | `lb_` | `lb_2026_05` |

---

## 3. 整体 ER 概览（文字版）

```
user ─┬─< user_credential
      ├─< user_session
      ├─< api_key
      ├─< team_member >─ team ─< subscription ─< order ─< invoice
      ├─< monitor (owner=user/team) ─< monitor_check
      │                              ─< alert_event >─ alert_policy
      ├─< alert_event ─< alert_notification >─ channel
      ├─< status_page ─< status_component ─< status_incident
      ├─< report
      └─< audit_log

probe_task ─< probe_result >─ node
                              ─< node_heartbeat
                              ─< node_health_metric_hour

community_node_application ─ user
community_node_observation ─ node
community_node_status_event ─ node
community_node_appeal ─ user

ticket >─ user
admin_audit_log >─ admin_user
```

---

## 4. 核心实体 DDL（PostgreSQL）

### 4.1 用户与认证

```sql
-- 用户
CREATE TABLE "user" (
  id              text PRIMARY KEY,                -- u_xxx
  email           citext UNIQUE NOT NULL,
  email_verified_at timestamptz,
  phone           text,
  phone_verified_at timestamptz,
  username        citext UNIQUE,
  display_name    text,
  avatar_url      text,
  bio             text,
  locale          text DEFAULT 'zh-CN',
  timezone        text DEFAULT 'Asia/Shanghai',
  password_hash   text,
  password_changed_at timestamptz,
  status          text NOT NULL DEFAULT 'active'   -- active|locked|pending_deletion|deleted
                  CHECK (status IN ('active','locked','pending_deletion','deleted')),
  pending_deletion_at timestamptz,
  email_marketing_opted_in boolean DEFAULT true,
  last_login_at   timestamptz,
  last_login_ip   inet,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX idx_user_status ON "user"(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_user_email_lower ON "user"(lower(email));

-- 多登录凭证
CREATE TABLE user_credential (
  id              text PRIMARY KEY,
  user_id         text NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  type            text NOT NULL,                   -- password|wechat|github|google|phone
  external_id     text,                            -- OAuth provider 的 user id
  metadata        jsonb DEFAULT '{}'::jsonb,
  linked_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_user_credential_type_external ON user_credential(type, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX idx_user_credential_user ON user_credential(user_id);

-- 2FA
CREATE TABLE user_2fa (
  user_id         text PRIMARY KEY REFERENCES "user"(id) ON DELETE CASCADE,
  type            text NOT NULL,                   -- totp|webauthn
  secret_encrypted bytea,
  backup_codes_encrypted bytea,
  enabled_at      timestamptz NOT NULL DEFAULT now()
);

-- 会话
CREATE TABLE user_session (
  id              text PRIMARY KEY,
  user_id         text NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  refresh_token_hash text NOT NULL,
  device          text,
  client_ip       inet,
  user_agent      text,
  workspace_id    text,                            -- 当前工作区
  created_at      timestamptz NOT NULL DEFAULT now(),
  expires_at      timestamptz NOT NULL,
  revoked_at      timestamptz
);
CREATE INDEX idx_user_session_user ON user_session(user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_user_session_token_hash ON user_session(refresh_token_hash);
```

### 4.2 团队

```sql
CREATE TABLE team (
  id              text PRIMARY KEY,
  name            text NOT NULL,
  slug            citext UNIQUE NOT NULL,
  owner_id        text NOT NULL REFERENCES "user"(id),
  plan_id         text,                            -- 关联 subscription 现行 plan
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);

CREATE TABLE team_member (
  team_id         text NOT NULL REFERENCES team(id) ON DELETE CASCADE,
  user_id         text NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  role            text NOT NULL CHECK (role IN ('owner','admin','member','viewer','billing')),
  joined_at       timestamptz NOT NULL DEFAULT now(),
  invited_by      text REFERENCES "user"(id),
  left_at         timestamptz,
  PRIMARY KEY (team_id, user_id)
);

CREATE TABLE team_invitation (
  id              text PRIMARY KEY,
  team_id         text NOT NULL REFERENCES team(id) ON DELETE CASCADE,
  email           citext NOT NULL,
  role            text NOT NULL,
  token_hash      text NOT NULL,
  invited_by      text NOT NULL REFERENCES "user"(id),
  expires_at      timestamptz NOT NULL,
  accepted_at     timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_team_invitation_email ON team_invitation(email);
```

### 4.3 API Key

```sql
CREATE TABLE api_key (
  id              text PRIMARY KEY,
  owner_type      text NOT NULL CHECK (owner_type IN ('user','team')),
  owner_id        text NOT NULL,
  name            text NOT NULL,
  prefix          text NOT NULL,                   -- idc_live_abcdefgh
  secret_hash     text NOT NULL,                   -- SHA-256 of full secret
  scopes          text[] NOT NULL DEFAULT '{}',
  rate_limit_override jsonb,
  allowed_ips     cidr[],
  allowed_origins text[],
  expires_at      timestamptz,
  last_used_at    timestamptz,
  last_used_ip    inet,
  usage_total     bigint NOT NULL DEFAULT 0,
  status          text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','revoked','expired')),
  created_by      text NOT NULL REFERENCES "user"(id),
  created_at      timestamptz NOT NULL DEFAULT now(),
  revoked_at      timestamptz
);
CREATE UNIQUE INDEX uq_api_key_prefix ON api_key(prefix);
CREATE INDEX idx_api_key_owner ON api_key(owner_type, owner_id) WHERE status = 'active';
```

### 4.4 监控

```sql
CREATE TABLE monitor (
  id              text PRIMARY KEY,
  owner_type      text NOT NULL CHECK (owner_type IN ('user','team')),
  owner_id        text NOT NULL,
  group_id        text,
  name            text NOT NULL,
  type            text NOT NULL,                   -- http|ping|tcping|dns|ssl|domain|icp|keyword|json|heartbeat|browser|tx
  target          text NOT NULL,
  params          jsonb NOT NULL DEFAULT '{}',
  interval_sec    integer NOT NULL,
  node_selection  jsonb NOT NULL DEFAULT '{}',     -- {mode, size, regions, isps, tags}
  assertions      jsonb NOT NULL DEFAULT '[]',     -- 多条断言
  trigger_rule    jsonb NOT NULL DEFAULT '{"consecutive_fail":2,"fail_node_quorum":"2/3"}',
  alert_policy_id text REFERENCES alert_policy(id),
  tags            text[] DEFAULT '{}',
  status          text NOT NULL DEFAULT 'up'
                  CHECK (status IN ('up','down','degraded','paused','maintenance','unknown')),
  current_streak_count integer DEFAULT 0,
  current_streak_start_at timestamptz,
  last_check_at   timestamptz,
  last_result_id  text,
  paused_at       timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX idx_monitor_owner_status ON monitor(owner_type, owner_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_monitor_tags ON monitor USING GIN(tags);
CREATE INDEX idx_monitor_target ON monitor(target);

CREATE TABLE monitor_group (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  name            text NOT NULL,
  parent_id       text REFERENCES monitor_group(id),
  created_at      timestamptz DEFAULT now()
);

-- 维护窗口
CREATE TABLE maintenance_window (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  name            text NOT NULL,
  scope_type      text NOT NULL CHECK (scope_type IN ('monitor','tag','all')),
  scope_value     jsonb NOT NULL,                  -- monitor_ids[] 或 tags[]
  start_at        timestamptz,
  end_at          timestamptz,
  cron_expr       text,                            -- 周期场景
  timezone        text DEFAULT 'UTC',
  created_at      timestamptz DEFAULT now()
);
```

### 4.5 监控结果（时序）

```sql
-- TimescaleDB Hypertable
CREATE TABLE monitor_check (
  id              text NOT NULL,
  monitor_id      text NOT NULL REFERENCES monitor(id) ON DELETE CASCADE,
  started_at      timestamptz NOT NULL,
  finished_at     timestamptz NOT NULL,
  result          text NOT NULL CHECK (result IN ('up','degraded','down','error')),
  node_results    jsonb NOT NULL,                  -- 多节点详细
  summary         jsonb NOT NULL,                  -- 聚合摘要
  triggered_event_id text,
  PRIMARY KEY (monitor_id, started_at, id)
);
SELECT create_hypertable('monitor_check', 'started_at', chunk_time_interval => INTERVAL '1 day');
SELECT add_compression_policy('monitor_check', INTERVAL '30 days');
SELECT add_retention_policy('monitor_check', INTERVAL '180 days');  -- Business 档；按用户级别另议

CREATE INDEX idx_monitor_check_monitor_time ON monitor_check(monitor_id, started_at DESC);

-- 小时聚合（Continuous Aggregate）
CREATE MATERIALIZED VIEW monitor_check_hourly
  WITH (timescaledb.continuous) AS
  SELECT monitor_id,
         time_bucket('1 hour', started_at) AS bucket_at,
         count(*) AS total,
         count(*) FILTER (WHERE result='up') AS up_count,
         count(*) FILTER (WHERE result='down') AS down_count,
         count(*) FILTER (WHERE result='degraded') AS degraded_count,
         avg((summary->>'avg_response_ms')::float) AS avg_response_ms,
         percentile_cont(0.95) WITHIN GROUP (ORDER BY (summary->>'avg_response_ms')::float) AS p95_response_ms
  FROM monitor_check
  GROUP BY monitor_id, bucket_at;

SELECT add_continuous_aggregate_policy('monitor_check_hourly',
  start_offset => INTERVAL '7 days', end_offset => INTERVAL '1 hour',
  schedule_interval => INTERVAL '1 hour');

-- 日聚合类似
CREATE MATERIALIZED VIEW monitor_check_daily ... ;
```

### 4.6 拨测任务（一次性）

```sql
CREATE TABLE probe_task (
  id              text PRIMARY KEY,
  type            text NOT NULL,                   -- http|ping|dns|...
  target          text NOT NULL,
  target_normalized text NOT NULL,                 -- 规范化后的 target，索引用
  params          jsonb NOT NULL DEFAULT '{}',
  initiated_by    text,                            -- user_id 或 NULL（匿名）
  api_key_id      text,
  client_ip       inet,
  user_agent      text,
  node_selection  jsonb NOT NULL,
  status          text NOT NULL CHECK (status IN ('queued','running','completed','failed','cancelled')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  started_at      timestamptz,
  completed_at    timestamptz
);
CREATE INDEX idx_probe_task_target ON probe_task(target_normalized, created_at DESC);
CREATE INDEX idx_probe_task_user_time ON probe_task(initiated_by, created_at DESC) WHERE initiated_by IS NOT NULL;

CREATE TABLE probe_result (
  id              text NOT NULL,
  task_id         text NOT NULL,
  node_id         text NOT NULL,
  raw             jsonb,
  summary         jsonb,
  duration_ms     integer,
  success         boolean,
  error           text,
  signature       text NOT NULL,                   -- 节点 Ed25519 签名
  created_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (task_id, node_id)
);
SELECT create_hypertable('probe_result', 'created_at', chunk_time_interval => INTERVAL '1 day');
SELECT add_retention_policy('probe_result', INTERVAL '90 days');
```

### 4.7 告警策略 / 通道 / 事件

```sql
CREATE TABLE alert_policy (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  name            text NOT NULL,
  rules           jsonb NOT NULL DEFAULT '[]',
  escalation      jsonb DEFAULT '{}',
  suppression     jsonb DEFAULT '{}',
  mute            jsonb DEFAULT '{}',
  on_recovery     jsonb DEFAULT '{}',
  is_default      boolean DEFAULT false,
  created_at      timestamptz DEFAULT now(),
  updated_at      timestamptz DEFAULT now()
);

CREATE TABLE channel (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  type            text NOT NULL,                   -- email|webhook|wecom_robot|...
  name            text NOT NULL,
  config_encrypted bytea NOT NULL,                 -- 敏感字段（webhook url, secret）加密
  health          text DEFAULT 'ok' CHECK (health IN ('ok','fail','paused')),
  last_test_at    timestamptz,
  last_test_result text,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE alert_event (
  id              text PRIMARY KEY,
  monitor_id      text NOT NULL REFERENCES monitor(id),
  owner_id        text NOT NULL,
  type            text NOT NULL CHECK (type IN ('down','up','degraded')),
  severity        text NOT NULL CHECK (severity IN ('critical','warning','info')),
  started_at      timestamptz NOT NULL,
  ended_at        timestamptz,
  duration_sec    integer,
  reason          text,
  affected_nodes  jsonb,
  acknowledged_by text REFERENCES "user"(id),
  acked_at        timestamptz,
  resolved_by     text REFERENCES "user"(id),
  resolved_at     timestamptz,
  resolved_kind   text CHECK (resolved_kind IN ('auto','manual')),
  is_false_positive boolean DEFAULT false,
  notes           text
);
CREATE INDEX idx_alert_event_monitor ON alert_event(monitor_id, started_at DESC);
CREATE INDEX idx_alert_event_owner_open ON alert_event(owner_id, started_at DESC) WHERE ended_at IS NULL;

CREATE TABLE alert_notification (
  id              text PRIMARY KEY,
  event_id        text NOT NULL REFERENCES alert_event(id) ON DELETE CASCADE,
  channel_id      text NOT NULL REFERENCES channel(id),
  channel_type    text NOT NULL,
  payload         jsonb,
  sent_at         timestamptz,
  delivery_status text NOT NULL DEFAULT 'queued'
                  CHECK (delivery_status IN ('queued','sent','failed','retrying')),
  attempts        integer DEFAULT 0,
  last_error      text,
  latency_ms      integer
);

CREATE TABLE alert_comment (
  id              text PRIMARY KEY,
  event_id        text NOT NULL REFERENCES alert_event(id) ON DELETE CASCADE,
  user_id         text NOT NULL REFERENCES "user"(id),
  body            text NOT NULL,
  mentioned_users text[],
  created_at      timestamptz DEFAULT now()
);
```

### 4.8 状态页

```sql
CREATE TABLE status_page (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  slug            citext UNIQUE NOT NULL,
  name            text NOT NULL,
  description     text,
  default_domain  text NOT NULL,                   -- <slug>.status.idcd.com
  custom_domain   text UNIQUE,
  custom_domain_verified_at timestamptz,
  cert_status     text,
  cert_expires_at timestamptz,
  visibility      text DEFAULT 'public' CHECK (visibility IN ('public','password','private')),
  password_hash   text,
  design          jsonb DEFAULT '{}',
  watermark_enabled boolean DEFAULT true,
  i18n            jsonb DEFAULT '{}',
  created_at      timestamptz DEFAULT now(),
  updated_at      timestamptz DEFAULT now(),
  deleted_at      timestamptz
);

CREATE TABLE status_section (
  id              text PRIMARY KEY,
  status_page_id  text NOT NULL REFERENCES status_page(id) ON DELETE CASCADE,
  name            text NOT NULL,
  position        integer DEFAULT 0
);

CREATE TABLE status_component (
  id              text PRIMARY KEY,
  status_page_id  text NOT NULL REFERENCES status_page(id) ON DELETE CASCADE,
  section_id      text REFERENCES status_section(id),
  name            text NOT NULL,
  description     text,
  position        integer DEFAULT 0,
  source_type     text NOT NULL CHECK (source_type IN ('monitor','manual','api')),
  monitor_ids     text[] DEFAULT '{}',
  aggregation_rule text DEFAULT 'any_down',
  current_status  text DEFAULT 'operational',
  last_changed_at timestamptz
);

CREATE TABLE status_incident (
  id              text PRIMARY KEY,
  status_page_id  text NOT NULL REFERENCES status_page(id) ON DELETE CASCADE,
  title           text NOT NULL,
  status          text NOT NULL CHECK (status IN ('investigating','identified','monitoring','resolved')),
  impact          text NOT NULL CHECK (impact IN ('minor','major','critical','maintenance')),
  affected_components text[],
  visibility      text DEFAULT 'public',
  notify_subscribers boolean DEFAULT true,
  auto_close_on_recovery boolean DEFAULT true,
  source          text DEFAULT 'manual' CHECK (source IN ('auto','manual','api')),
  related_alert_event_id text,
  started_at      timestamptz NOT NULL DEFAULT now(),
  resolved_at     timestamptz,
  postmortem_published_at timestamptz
);

CREATE TABLE status_incident_update (
  id              text PRIMARY KEY,
  incident_id     text NOT NULL REFERENCES status_incident(id) ON DELETE CASCADE,
  status          text NOT NULL,
  body            text NOT NULL,
  posted_by       text,
  posted_at       timestamptz DEFAULT now()
);

CREATE TABLE status_subscriber (
  id              text PRIMARY KEY,
  status_page_id  text NOT NULL REFERENCES status_page(id) ON DELETE CASCADE,
  channel         text NOT NULL,
  contact         text NOT NULL,
  verified_at     timestamptz,
  subscribed_at   timestamptz DEFAULT now(),
  notify_on_minor boolean DEFAULT false,
  notify_on_maintenance boolean DEFAULT false
);
CREATE INDEX idx_status_subscriber_page ON status_subscriber(status_page_id);
```

### 4.9 节点

```sql
CREATE TABLE node (
  id              text PRIMARY KEY,                -- nd_jp_tk_01_vultr
  type            text NOT NULL CHECK (type IN ('owned_idc','owned_cloud','anchor','community','dedicated','private')),
  status          text NOT NULL CHECK (status IN ('provisioning','enrolling','observing','active','drained','disabled','banned','retired','offline')),
  tier            integer CHECK (tier IN (1,2,3)),
  roles           text[] DEFAULT '{}',
  country         text NOT NULL,
  region          text,
  city            text,
  latitude        double precision,
  longitude       double precision,
  timezone        text,
  ipv4            inet,
  ipv6            inet,
  asn             integer,
  asn_org         text,
  isp_category    text,
  cpu_cores       integer,
  memory_mb       integer,
  bandwidth_mbps_in integer,
  bandwidth_mbps_out integer,
  max_concurrent_tasks integer DEFAULT 50,
  max_rps         integer DEFAULT 30,
  capabilities    jsonb DEFAULT '{}',
  agent_version   text,
  os              text,
  kernel          text,
  provider        text,
  owner_user_id   text REFERENCES "user"(id),     -- 众包贡献者；自有为 NULL
  trust_level     integer DEFAULT 3,
  is_anchor       boolean DEFAULT false,
  enrolled_by     text,
  deployed_at     timestamptz,
  last_seen_at    timestamptz,
  contact         text,
  notes           text,
  created_at      timestamptz DEFAULT now(),
  updated_at      timestamptz DEFAULT now()
);
CREATE INDEX idx_node_status_country ON node(status, country);
CREATE INDEX idx_node_asn ON node(asn);
CREATE INDEX idx_node_owner ON node(owner_user_id) WHERE owner_user_id IS NOT NULL;

CREATE TABLE node_heartbeat (
  node_id         text NOT NULL,
  ts              timestamptz NOT NULL,
  load_metrics    jsonb,
  in_progress_tasks integer,
  PRIMARY KEY (node_id, ts)
);
SELECT create_hypertable('node_heartbeat', 'ts', chunk_time_interval => INTERVAL '1 day');
SELECT add_retention_policy('node_heartbeat', INTERVAL '7 days');

CREATE TABLE node_health_metric_hour (
  node_id         text NOT NULL,
  bucket_at       timestamptz NOT NULL,
  total_tasks     integer,
  succ_tasks      integer,
  fail_tasks      integer,
  avg_latency_ms  double precision,
  p95_latency_ms  double precision,
  uptime_seconds  integer,
  PRIMARY KEY (node_id, bucket_at)
);

CREATE TABLE node_enrollment_token (
  token_hash      text PRIMARY KEY,
  owner_user_id   text NOT NULL,
  used_at         timestamptz,
  expires_at      timestamptz NOT NULL,
  target_node_id  text,
  created_at      timestamptz DEFAULT now()
);
```

### 4.10 众包节点扩展

```sql
CREATE TABLE community_node_application (
  id              text PRIMARY KEY,
  user_id         text NOT NULL REFERENCES "user"(id),
  requested_at    timestamptz DEFAULT now(),
  enrollment_token_hash text,
  token_used_at   timestamptz,
  approved_at     timestamptz,
  rejected_at     timestamptz,
  rejection_reason text,
  resulting_node_id text REFERENCES node(id)
);

CREATE TABLE community_node_observation (
  id              text PRIMARY KEY,
  node_id         text NOT NULL REFERENCES node(id),
  started_at      timestamptz NOT NULL,
  ended_at        timestamptz,
  honey_total     integer DEFAULT 0,
  honey_passed    integer DEFAULT 0,
  echo_total      integer DEFAULT 0,
  echo_consistent integer DEFAULT 0,
  baseline_score  double precision,
  decision        text CHECK (decision IN ('passed','rejected','extended'))
);

CREATE TABLE community_node_status_event (
  id              text PRIMARY KEY,
  node_id         text NOT NULL REFERENCES node(id),
  from_state      text NOT NULL,
  to_state        text NOT NULL,
  triggered_by    text NOT NULL,                   -- 'auto' 或 admin_user_id
  signal_type     text,
  signal_details  jsonb,
  occurred_at     timestamptz DEFAULT now()
);
CREATE INDEX idx_community_status_event_node ON community_node_status_event(node_id, occurred_at DESC);

CREATE TABLE community_node_appeal (
  id              text PRIMARY KEY,
  user_id         text NOT NULL,
  node_id         text NOT NULL REFERENCES node(id),
  related_event_id text,
  statement       text NOT NULL,
  evidence        jsonb,
  status          text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','upheld','reversed','partial')),
  reviewed_by     text,
  reviewed_at     timestamptz,
  decision_note   text,
  user_satisfied  boolean,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE community_node_fingerprint (
  node_id         text PRIMARY KEY REFERENCES node(id),
  fingerprint_hash text NOT NULL,
  components      jsonb,
  confidence      double precision,
  duplicate_of_node_id text REFERENCES node(id)
);
CREATE INDEX idx_community_fingerprint_hash ON community_node_fingerprint(fingerprint_hash);

CREATE TABLE community_node_points (
  node_id         text PRIMARY KEY REFERENCES node(id),
  user_id         text NOT NULL,
  total_points    bigint DEFAULT 0,
  daily_bonus     bigint DEFAULT 0,
  task_bonus      bigint DEFAULT 0,
  penalty         bigint DEFAULT 0,
  last_calc_at    timestamptz
);

CREATE TABLE honey_task_template (
  id              text PRIMARY KEY,
  target          text NOT NULL,
  expected_result_summary jsonb NOT NULL,
  task_type       text NOT NULL,
  params          jsonb,
  enabled         boolean DEFAULT true,
  detection_threshold jsonb,
  created_at      timestamptz DEFAULT now()
);
```

### 4.11 报告与诊断

```sql
CREATE TABLE report (
  id              text PRIMARY KEY,                -- r_xxx
  diagnosis_id    text,
  task_ids        text[] DEFAULT '{}',
  target_domain   text,
  owner_id        text,                            -- nullable for anonymous
  visibility      text DEFAULT 'public' CHECK (visibility IN ('public','private','password')),
  password_hash   text,
  expires_at      timestamptz,
  summary         jsonb,
  score           integer,
  status          text DEFAULT 'running' CHECK (status IN ('running','done','failed')),
  created_at      timestamptz DEFAULT now()
);
CREATE INDEX idx_report_target ON report(target_domain, created_at DESC);

CREATE TABLE sla_report (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  period          text NOT NULL CHECK (period IN ('month','quarter','year')),
  period_start    date NOT NULL,
  period_end      date NOT NULL,
  uptime_overall  double precision,
  mtta_avg        integer,
  mttr_avg        integer,
  events_count    integer,
  critical_events_count integer,
  monitors_breakdown jsonb,
  file_url        text,
  generated_at    timestamptz DEFAULT now()
);

CREATE TABLE dashboard (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  name            text NOT NULL,
  layout          jsonb DEFAULT '{}',
  variables       jsonb DEFAULT '{}',
  shared_token    text,
  created_at      timestamptz DEFAULT now(),
  updated_at      timestamptz DEFAULT now()
);
```

### 4.12 商业化

```sql
CREATE TABLE plan (
  id              text PRIMARY KEY,
  code            text UNIQUE NOT NULL,            -- free|pro|team|business
  name            text NOT NULL,
  description     text,
  prices          jsonb NOT NULL,
  limits          jsonb NOT NULL,
  visible         boolean DEFAULT true,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE subscription (
  id              text PRIMARY KEY,
  owner_type      text NOT NULL CHECK (owner_type IN ('user','team')),
  owner_id        text NOT NULL,
  plan_id         text NOT NULL REFERENCES plan(id),
  status          text NOT NULL CHECK (status IN ('trial','active','past_due','canceled','expired')),
  period          text NOT NULL CHECK (period IN ('monthly','yearly')),
  currency        text NOT NULL DEFAULT 'CNY',
  current_period_start timestamptz NOT NULL,
  current_period_end   timestamptz NOT NULL,
  trial_ends_at   timestamptz,
  cancel_at_period_end boolean DEFAULT false,
  payment_method_id text,
  created_at      timestamptz DEFAULT now(),
  canceled_at     timestamptz
);
CREATE INDEX idx_subscription_owner ON subscription(owner_type, owner_id) WHERE status IN ('active','trial','past_due');
CREATE INDEX idx_subscription_renewal ON subscription(current_period_end) WHERE status = 'active' AND cancel_at_period_end = false;

CREATE TABLE payment_method (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  type            text NOT NULL CHECK (type IN ('wechat','alipay','paddle','stripe','bank')),
  external_id     text,
  display         text,
  is_default      boolean DEFAULT false,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE "order" (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  type            text NOT NULL CHECK (type IN ('subscription','addon','topup','invoice','prorate')),
  plan_id         text,
  amount          numeric(12,2) NOT NULL,
  currency        text NOT NULL,
  tax             numeric(12,2) DEFAULT 0,
  status          text NOT NULL CHECK (status IN ('pending','paid','failed','canceled','refunded')),
  payment_method_id text,
  payment_channel text,
  external_txn_id text,
  created_at      timestamptz DEFAULT now(),
  paid_at         timestamptz,
  refunded_at     timestamptz
);
CREATE INDEX idx_order_owner ON "order"(owner_id, created_at DESC);
CREATE INDEX idx_order_status ON "order"(status, created_at) WHERE status = 'pending';

CREATE TABLE refund (
  id              text PRIMARY KEY,
  order_id        text NOT NULL REFERENCES "order"(id),
  amount          numeric(12,2) NOT NULL,
  reason          text,
  status          text NOT NULL CHECK (status IN ('pending','approved','rejected','completed','failed')),
  requested_by    text,
  approved_by     text,
  refunded_at     timestamptz,
  external_refund_id text
);

CREATE TABLE invoice (
  id              text PRIMARY KEY,
  order_id        text NOT NULL REFERENCES "order"(id),
  owner_id        text NOT NULL,
  title           text NOT NULL,                   -- 抬头
  tax_id          text,
  type            text NOT NULL CHECK (type IN ('individual','company','special','foreign')),
  amount          numeric(12,2) NOT NULL,
  status          text NOT NULL,
  file_url        text,
  issued_at       timestamptz
);

CREATE TABLE credit_ledger (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  change          numeric(12,2) NOT NULL,
  balance_after   numeric(12,2) NOT NULL,
  source          text NOT NULL,
  reference_id    text,
  created_at      timestamptz DEFAULT now()
);
CREATE INDEX idx_credit_ledger_owner ON credit_ledger(owner_id, created_at DESC);

CREATE TABLE coupon (
  code            text PRIMARY KEY,
  description     text,
  type            text NOT NULL CHECK (type IN ('percent','amount')),
  value           numeric(12,2) NOT NULL,
  valid_from      timestamptz,
  valid_until     timestamptz,
  max_uses        integer,
  used_count      integer DEFAULT 0,
  applicable_plans text[],
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE referral (
  inviter_user_id text NOT NULL,
  invitee_user_id text NOT NULL,
  registered_at   timestamptz NOT NULL,
  first_paid_order_id text,
  commission_amount numeric(12,2),
  paid_to_inviter_at timestamptz,
  PRIMARY KEY (inviter_user_id, invitee_user_id)
);
```

### 4.13 用量与计费事件

```sql
CREATE TABLE usage_event (
  id              text NOT NULL,
  owner_id        text NOT NULL,
  api_key_id      text,
  dimension       text NOT NULL,                   -- api_call|sms|voice|monitor_check|...
  endpoint        text,
  method          text,
  weight          integer DEFAULT 1,
  status_code     integer,
  duration_ms     integer,
  response_bytes  integer,
  request_id      text,
  client_ip       inet,
  occurred_at     timestamptz NOT NULL,
  PRIMARY KEY (owner_id, occurred_at, id)
);
SELECT create_hypertable('usage_event', 'occurred_at', chunk_time_interval => INTERVAL '1 day');
SELECT add_retention_policy('usage_event', INTERVAL '180 days');

CREATE TABLE api_quota_usage (
  owner_id        text NOT NULL,
  period          text NOT NULL,                   -- day|month
  bucket_at       timestamptz NOT NULL,
  weighted_total  bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (owner_id, period, bucket_at)
);
```

### 4.14 Webhook

```sql
CREATE TABLE webhook_endpoint (
  id              text PRIMARY KEY,
  owner_id        text NOT NULL,
  name            text NOT NULL,
  url             text NOT NULL,
  secret_hash     text NOT NULL,
  events          text[] DEFAULT '{}',
  is_active       boolean DEFAULT true,
  last_delivery_at timestamptz,
  last_status     text,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE webhook_delivery (
  id              text PRIMARY KEY,
  endpoint_id     text NOT NULL REFERENCES webhook_endpoint(id),
  event_id        text NOT NULL,
  event_type      text NOT NULL,
  attempt         integer DEFAULT 1,
  request_payload jsonb,
  response_status integer,
  response_body   text,
  latency_ms      integer,
  next_retry_at   timestamptz,
  delivered_at    timestamptz,
  failed_at       timestamptz
);
CREATE INDEX idx_webhook_delivery_retry ON webhook_delivery(next_retry_at) WHERE delivered_at IS NULL AND failed_at IS NULL;
```

### 4.15 工单 / 审计 / 后台

```sql
CREATE TABLE ticket (
  id              text PRIMARY KEY,
  type            text NOT NULL CHECK (type IN ('support','abuse','security','billing','refund')),
  user_id         text,
  subject         text NOT NULL,
  body            text,
  status          text NOT NULL CHECK (status IN ('open','waiting_user','resolved','closed')),
  assignee_admin_id text,
  priority        text CHECK (priority IN ('low','normal','high','urgent')),
  sla_due_at      timestamptz,
  satisfaction_score integer,
  metadata        jsonb,
  created_at      timestamptz DEFAULT now(),
  resolved_at     timestamptz
);

CREATE TABLE ticket_message (
  id              text PRIMARY KEY,
  ticket_id       text NOT NULL REFERENCES ticket(id) ON DELETE CASCADE,
  author_type     text NOT NULL CHECK (author_type IN ('user','admin')),
  author_id       text,
  body            text NOT NULL,
  is_internal_note boolean DEFAULT false,
  attachments     jsonb,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE audit_log (
  id              text PRIMARY KEY,
  owner_id        text,
  ts              timestamptz NOT NULL DEFAULT now(),
  actor_user_id   text,
  action          text NOT NULL,
  resource_type   text,
  resource_id     text,
  client_ip       inet,
  user_agent      text,
  location        text,
  result          text CHECK (result IN ('ok','fail')),
  error_reason    text,
  metadata        jsonb
);
-- 这张表写入大，独立到 idcd_audit 库或时序库
SELECT create_hypertable('audit_log', 'ts', chunk_time_interval => INTERVAL '7 days');
SELECT add_retention_policy('audit_log', INTERVAL '180 days');

CREATE TABLE admin_user (
  id              text PRIMARY KEY,
  email           citext UNIQUE NOT NULL,
  role            text NOT NULL,
  status          text DEFAULT 'active',
  password_hash   text NOT NULL,
  totp_secret_encrypted bytea,
  last_login_at   timestamptz,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE admin_audit_log (
  id              text PRIMARY KEY,
  ts              timestamptz DEFAULT now(),
  admin_user_id   text NOT NULL,
  action          text NOT NULL,
  resource_type   text,
  resource_id     text,
  before          jsonb,
  after           jsonb,
  client_ip       inet,
  user_agent      text,
  reason          text,
  ticket_ref      text
);

CREATE TABLE approval (
  id              text PRIMARY KEY,
  action_type     text NOT NULL,
  requested_by    text NOT NULL,
  target          jsonb,
  reason          text,
  status          text DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  approver_admin_id text,
  approved_at     timestamptz,
  executed_at     timestamptz,
  original_payload jsonb,
  created_at      timestamptz DEFAULT now()
);
```

### 4.16 反滥用

```sql
CREATE TABLE denylist (
  id              text PRIMARY KEY,
  category        text NOT NULL CHECK (category IN ('tech','sensitive','dynamic','user_reported')),
  pattern_type    text NOT NULL CHECK (pattern_type IN ('cidr','domain','regex')),
  pattern         text NOT NULL,
  reason          text,
  added_by        text,
  expires_at      timestamptz,
  created_at      timestamptz DEFAULT now()
);
CREATE INDEX idx_denylist_active ON denylist(category) WHERE expires_at IS NULL OR expires_at > now();

CREATE TABLE rate_limit_override (
  id              text PRIMARY KEY,
  scope_type      text NOT NULL CHECK (scope_type IN ('user','target','api_key','global')),
  scope_value     text NOT NULL,
  dimension       text NOT NULL,
  override_value  jsonb,
  reason          text,
  created_at      timestamptz DEFAULT now()
);

CREATE TABLE abuse_report (
  id              text PRIMARY KEY,
  type            text NOT NULL,
  reporter_email  text,
  reporter_ip     inet,
  target          text,
  related_resource_type text,
  related_resource_id text,
  status          text DEFAULT 'open',
  assigned_admin_id text,
  evidence        jsonb,
  resolution      text,
  created_at      timestamptz DEFAULT now(),
  resolved_at     timestamptz
);

CREATE TABLE user_risk_score (
  user_id         text PRIMARY KEY REFERENCES "user"(id),
  score           integer NOT NULL DEFAULT 50,    -- 0=最高风险，100=最低风险
  factors         jsonb,
  last_calc_at    timestamptz DEFAULT now()
);
```

### 4.17 缓存表（数据缓存，不是 Redis）

```sql
-- 长期 IP / WHOIS / ICP / SSL 缓存（可被 API 复用）
CREATE TABLE ip_info_cache (
  ip              inet PRIMARY KEY,
  asn             integer,
  asn_org         text,
  isp             text,
  country         text,
  region          text,
  city            text,
  raw             jsonb,
  source          text,
  fetched_at      timestamptz DEFAULT now(),
  ttl_until       timestamptz
);
CREATE INDEX idx_ip_info_cache_ttl ON ip_info_cache(ttl_until);

CREATE TABLE whois_cache (
  key             text PRIMARY KEY,                -- domain or ip
  raw             text,
  parsed          jsonb,
  fetched_at      timestamptz DEFAULT now(),
  ttl_until       timestamptz
);

CREATE TABLE icp_cache (
  domain          text PRIMARY KEY,
  parsed          jsonb,
  fetched_at      timestamptz DEFAULT now(),
  ttl_until       timestamptz
);

CREATE TABLE ssl_cache (
  key             text PRIMARY KEY,                -- host:port
  parsed          jsonb,
  fetched_at      timestamptz DEFAULT now(),
  ttl_until       timestamptz
);
```

---

## 4.X v2 新增表(Evidence / MCP / Agent obs / Compliance / Leaderboard)

> 决策 §K1 三栈 sub-product 阵型下,Evidence(attest.idcd.com)/ MCP(mcp.idcd.com)/ 业务(idcd_main)以 schema 隔离,**架构上保留独立 PostgreSQL cluster 部署能力**(S2 起步同 cluster,S4 企业版可拆为 idcd_main_db / idcd_attest_db / idcd_mcp_db 物理隔离)。
> **跨 schema 不用 FK**(包括到 `user(id)` / `team(id)` / `verdict_report(id)` 等的引用):eng review D1 锁定 — 当前 cluster 阶段也走应用层 Repository join,**不在 DDL 中写 cross-schema REFERENCES**,以保留 S4 物理拆分时迁移成本为零。
> 同 schema 内 FK 保留(如 `verdict_report.order_id → verdict_order.id` 在 idcd_attest schema 内)。
> 应用层 join 由 Repository 抽象层提供,避免 service 代码到处拼接;owner_id / team_id / report_id 等跨 schema 引用走 Repository.GetByOwnerId() / GetReport() 类接口。

### 4.X.1 verdict_order(v2 Evidence)

```sql
CREATE TABLE verdict_order (
  id              text PRIMARY KEY,                        -- v_*
  owner_id        text NOT NULL,                            -- 跨 schema 不写 FK;应用层 Repository 校验存在
  template        text NOT NULL,                            -- sla|incident|compliance|legal
  target          text NOT NULL,                            -- domain|url|ip
  time_window_start  timestamptz NOT NULL,
  time_window_end    timestamptz NOT NULL,
  status          text NOT NULL,                            -- pending|paid|generating|delivered|failed|refunded|refund_failed
  price_cny       numeric(10,2) NOT NULL,
  price_paid_cny  numeric(10,2),                            -- 实收(Paddle 扣费后)
  paddle_order_id text,                                     -- 关联 09 order
  refund_reason   text,
  refund_attempt_count int NOT NULL DEFAULT 0,              -- v2 D5: refund retry queue 累计次数
  refund_last_error text,                                   -- v2 D5: 最后一次 refund 失败原因(Paddle 风控 / 网络 等)
  refund_apology_sent_at timestamptz,                       -- v2 D5: 道歉邮箱发送时间(30min 失败兜底)
  created_at      timestamptz NOT NULL DEFAULT now(),
  paid_at         timestamptz,
  delivered_at    timestamptz,
  failed_at       timestamptz,
  refunded_at     timestamptz
);
CREATE INDEX idx_verdict_order_owner ON verdict_order(owner_id, created_at DESC);
CREATE INDEX idx_verdict_order_status ON verdict_order(status) WHERE status IN ('paid','generating','refund_failed');
-- v2 D5: refund_failed 状态用于 admin 后台 dashboard,任何此状态订单触发 P0 告警
```

### 4.X.2 verdict_report(v2 Evidence)

```sql
CREATE TABLE verdict_report (
  id              text PRIMARY KEY,                        -- vr_*
  order_id        text NOT NULL UNIQUE REFERENCES verdict_order(id),  -- 同 idcd_attest schema FK 保留
  pdf_url         text NOT NULL,                            -- S3 path
  pdf_size_bytes  bigint,
  content_hash    text NOT NULL,                            -- sha256(pdf bytes)
  signature       bytea NOT NULL,                           -- KMS sign output
  signature_key_id      text NOT NULL,
  signature_key_version int NOT NULL,
  tsa_provider    text NOT NULL,                            -- digicert|globalsign|ntsc
  tsa_response_blob     bytea,
  tsa_time        timestamptz NOT NULL,
  blockchain_anchor     jsonb,                              -- {chain: polygon, tx_hash: ...} OPTIONAL
  nodes_used      jsonb NOT NULL,                           -- [node_id, ...]
  node_consistency_pct  numeric(5,2),                       -- 多节点一致性 0-100
  llm_used        boolean DEFAULT false,
  llm_model       text,
  llm_prompt_version text,
  self_verify_status    text,                               -- pass|fail|pending
  self_verify_at  timestamptz,
  confidence_label      text,                               -- high|medium|low
  report_type     text NOT NULL DEFAULT 'observation_only', -- v2 D-Concern1: observation_only(默认);保留枚举供 S4 司法鉴定所合作通道升级
  archived_url    text,                                     -- 永久归档 S3 WORM
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_verdict_report_order ON verdict_report(order_id);
CREATE INDEX idx_verdict_report_key_version ON verdict_report(signature_key_id, signature_key_version);
-- v2 D-Concern1: report_type 在公开 verify 接口返回,避免被误用为鉴定结论
```

### 4.X.3 attestation_record(v2 Evidence,审计跟踪)

```sql
-- v2 D4: attestation_record 充当 Verdict 生成流程的 WAL(Write-Ahead Log)。
-- 每 step 完成后 worker 写一条 (report_id, action, status, external_id, result=success)。
-- Worker crash 后续跑时:先查 attestation_record 中此 report_id 已成功 step,跳过续跑下一 step。
-- UNIQUE(report_id, action) 防止重复签名 / 重复时间戳 / 重复归档 — 严格 step-level idempotency。
CREATE TABLE attestation_record (
  id              text PRIMARY KEY,                        -- att_*
  report_id       text NOT NULL REFERENCES verdict_report(id),  -- 同 schema FK 保留
  action          text NOT NULL,                            -- signed|tsa_stamped|anchored|s3_archived|self_verified|revoked
  status          text NOT NULL DEFAULT 'pending',          -- v2 D4: pending|success|failure
  external_id     text,                                     -- TSA serial / chain tx hash / KMS req id / S3 ETag
  idempotency_key text,                                     -- v2 D4: 提供给外部 API(如 AWS KMS)的 idempotency token
  payload_hash    text,
  result          text NOT NULL,                            -- success|failure
  error_detail    text,
  retry_count     int NOT NULL DEFAULT 0,                   -- v2 D4: step 重试次数(<=3)
  created_at      timestamptz NOT NULL DEFAULT now(),
  completed_at    timestamptz,                              -- v2 D4: success 时记录
  CONSTRAINT uq_attestation_report_action UNIQUE (report_id, action)  -- v2 D4: step-level idempotency
);
CREATE INDEX idx_attestation_report ON attestation_record(report_id, created_at);
CREATE INDEX idx_attestation_pending ON attestation_record(status) WHERE status = 'pending';
-- v2 D4: WAL replay 查询 — SELECT action FROM attestation_record WHERE report_id=$1 AND status='success'
```

### 4.X.4 tsa_response(v2 Evidence,TSA 服务调用记录)

```sql
CREATE TABLE tsa_response (
  id              text PRIMARY KEY,                        -- tsa_*
  provider        text NOT NULL,                            -- digicert|globalsign|ntsc
  request_hash    text NOT NULL,
  response_blob   bytea,
  serial_number   text,
  issued_at       timestamptz,
  valid_until     timestamptz,
  status          text NOT NULL,                            -- success|failure|timeout
  latency_ms      int,
  used_by_report_id text REFERENCES verdict_report(id),     -- 同 idcd_attest schema FK 保留
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_tsa_response_provider_time ON tsa_response(provider, created_at DESC);
CREATE INDEX idx_tsa_response_status ON tsa_response(status) WHERE status != 'success';
```

### 4.X.5 key_ceremony_log(v2 Evidence,密钥仪式审计)

```sql
CREATE TABLE key_ceremony_log (
  id              text PRIMARY KEY,                        -- kc_*
  action          text NOT NULL,                            -- root_gen|root_split|sign_key_rotate|emergency_revoke
  key_id          text,
  key_version     int,
  actors          jsonb NOT NULL,                           -- [{user_id|external_id, role}, ...]
  evidence_url    text,                                     -- 录像 / 公证 PDF
  notes           text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
-- 此表只增不删,且需要双人审批的应用层写入控制(详 11-admin)
```

### 4.X.6 mcp_session(v2 MCP)

```sql
CREATE TABLE mcp_session (
  id              text PRIMARY KEY,                        -- mcps_*
  token_id        text NOT NULL REFERENCES mcp_token(id),   -- 同 idcd_mcp schema FK 保留
  owner_id        text NOT NULL,                            -- 跨 schema 不写 FK;Repository 校验
  client_id       text,                                     -- Cursor|ClaudeCode|Codex|sdk-py|sdk-ts|other
  client_version  text,
  client_ip       inet,
  started_at      timestamptz NOT NULL DEFAULT now(),
  last_activity_at timestamptz NOT NULL DEFAULT now(),
  ended_at        timestamptz,
  total_tool_calls int DEFAULT 0,
  total_units     int DEFAULT 0
);
CREATE INDEX idx_mcp_session_owner_active ON mcp_session(owner_id, last_activity_at DESC) WHERE ended_at IS NULL;
```

### 4.X.7 mcp_tool_call(v2 MCP)

```sql
CREATE TABLE mcp_tool_call (
  id              text PRIMARY KEY,                        -- mctc_*
  session_id      text NOT NULL REFERENCES mcp_session(id), -- 同 idcd_mcp schema FK 保留
  owner_id        text NOT NULL,                            -- 跨 schema 不写 FK
  tool_name       text NOT NULL,                            -- idcd_ping|idcd_http_probe|...
  request_payload_hash  text,                               -- 默认哈希存储
  response_payload_hash text,
  request_payload_raw   jsonb,                              -- v2 D7: 失败 case + 用户授权下临时存原文 7 天,自动清理 cron job
  response_payload_raw  jsonb,                              -- v2 D7: 同上
  payload_retain_until  timestamptz,                        -- v2 D7: created_at + 7d,过期 cron 清理
  units_charged   int NOT NULL,
  status          text NOT NULL,                            -- success|failure|timeout|rate_limited
  latency_ms      int,
  error_class     text,
  error_detail    text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
-- 高频写入,TimescaleDB Hypertable
SELECT create_hypertable('mcp_tool_call', 'created_at', chunk_time_interval => interval '1 day');
CREATE INDEX idx_mcp_tool_call_owner_time ON mcp_tool_call(owner_id, created_at DESC);
CREATE INDEX idx_mcp_tool_call_session_time ON mcp_tool_call(session_id, created_at DESC);  -- v2 D7: 排障 Cursor/会话级查询
-- v2 D7: 用户在 /app/mcp/settings 可选择 "失败 case 临时存 7 天原 payload"(默认关);开启后 status=failure 的 calls 写入 *_raw 字段;过期自动清理
```

### 4.X.8 mcp_token(v2 MCP)

```sql
CREATE TABLE mcp_token (
  id              text PRIMARY KEY,                        -- mcpt_*
  owner_id        text NOT NULL,                            -- 跨 schema 不写 FK
  type            text NOT NULL,                            -- personal|workspace|service
  token_hash      text NOT NULL UNIQUE,                     -- 哈希存储,前端展示一次
  token_display   text,                                     -- 后 4 位 + 前缀(展示用)
  name            text,                                     -- 用户起的名字
  scope           jsonb NOT NULL,                           -- {tools: [...], regions: [...]}
  ip_whitelist    jsonb,                                    -- ["1.2.3.4/32", ...] service 必填
  revoked         boolean DEFAULT false,
  revoke_reason   text,
  expires_at      timestamptz NOT NULL,                     -- v2 D2: 所有 token 必有过期日,最长 90 天;personal 24h, workspace 90d, service 90d 全自动 renewal
  auto_renew      boolean NOT NULL DEFAULT true,            -- v2 D2: 自动 renewal(workspace/service 默认开)
  last_renewed_at timestamptz,                              -- v2 D2: 最后一次自动 renewal 时间
  created_at      timestamptz NOT NULL DEFAULT now(),
  last_used_at    timestamptz
);
CREATE INDEX idx_mcp_token_owner_active ON mcp_token(owner_id) WHERE NOT revoked;
-- v2 D2: 严格"无永久 token"原则;expires_at NOT NULL;auto_renew=true 自动续期
```

### 4.X.9 agent_obs_monitor(v2 Agent obs, M21-M23)

```sql
CREATE TABLE agent_obs_monitor (
  id              text PRIMARY KEY,                        -- aom_*
  owner_id        text NOT NULL,                            -- 跨 schema 不写 FK
  agent_name      text NOT NULL,                            -- 用户给 Agent 起的名
  step_name       text,                                     -- Agent 工作流的 step name(可选)
  type            text NOT NULL,                            -- llm|tool|rag|other (对应 M21|M22|M23)
  endpoint_url    text NOT NULL,
  endpoint_config jsonb NOT NULL,                           -- 不同 type 不同字段(详 04 §3.15-3.17)
  frequency_seconds int NOT NULL,
  budget_per_check_usd  numeric(10,4),                      -- M21 必填,防爆账单
  failure_threshold jsonb NOT NULL,
  notification_channels jsonb,
  status          text NOT NULL DEFAULT 'active',           -- active|paused|exceeded_budget
  total_cost_this_month_usd numeric(10,4) NOT NULL DEFAULT 0,  -- v2 D7: 必须用原子 UPDATE 累加
  created_at      timestamptz NOT NULL DEFAULT now(),
  paused_at       timestamptz
);
CREATE INDEX idx_agent_obs_monitor_owner ON agent_obs_monitor(owner_id, type);
-- v2 D7: total_cost_this_month_usd 累加必须用原子 UPDATE:
-- UPDATE agent_obs_monitor SET total_cost_this_month_usd = total_cost_this_month_usd + $1
--   WHERE id = $2 AND total_cost_this_month_usd + $1 <= budget_per_check_usd * (期内剩余 check 数)
-- 同 transaction 内做 budget 检查 + status 更新到 exceeded_budget(若超额)
```

### 4.X.10 agent_obs_event(v2 Agent obs 时序事件)

```sql
CREATE TABLE agent_obs_event (
  id              text PRIMARY KEY,                        -- aoe_*
  monitor_id      text NOT NULL REFERENCES agent_obs_monitor(id),  -- 同 idcd_main schema FK 保留
  owner_id        text NOT NULL,                            -- 同 idcd_main schema 内,FK 保留
  region          text,                                     -- 哪个节点验证
  occurred_at     timestamptz NOT NULL,
  latency_ms      int,
  success         boolean NOT NULL,
  failure_class   text,                                     -- timeout|4xx|5xx|malformed|semantic|budget
  failure_detail  jsonb,
  cost_usd        numeric(10,6),                            -- M21 实际花费
  trace_id        text                                      -- 可选 Agent 端 trace 关联
);
SELECT create_hypertable('agent_obs_event', 'occurred_at', chunk_time_interval => interval '1 day');
CREATE INDEX idx_agent_obs_event_monitor_time ON agent_obs_event(monitor_id, occurred_at DESC);
```

### 4.X.11 compliance_subscription(v2 Evidence,Compliance 年订)

```sql
CREATE TABLE compliance_subscription (
  id              text PRIMARY KEY,                        -- cs_*
  owner_id        text NOT NULL,                            -- 跨 schema 不写 FK(user / team 在不同 schema)
  tier            text NOT NULL,                            -- starter|pro|enterprise
  status          text NOT NULL,                            -- active|past_due|canceled|expired
  period_start    timestamptz NOT NULL,
  period_end      timestamptz NOT NULL,
  monitors_quota  int NOT NULL,
  reports_frequency text NOT NULL,                          -- monthly|weekly|custom
  history_retention_months int NOT NULL,
  free_verdict_count int DEFAULT 0,                         -- Pro 含 5 份 / Enterprise 不限
  free_verdict_used  int DEFAULT 0,
  price_cny       numeric(10,2),
  paddle_subscription_id text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  canceled_at     timestamptz
);
CREATE INDEX idx_compliance_subscription_owner ON compliance_subscription(owner_id, status);
```

### 4.X.12 leaderboard_report(v2 内容矩阵)

```sql
CREATE TABLE leaderboard_report (
  id              text PRIMARY KEY,                        -- lb_2026_05
  period_year     int NOT NULL,
  period_month    int NOT NULL,
  data            jsonb NOT NULL,                           -- 完整排行数据(各厂商/区域/指标)
  methodology_version text NOT NULL,
  excluded_vendors jsonb,                                   -- 已申请退出的厂商
  pdf_url         text,                                     -- 配套 Verdict 报告
  verdict_report_id text,                                   -- 跨 schema 不写 FK(verdict_report 在 idcd_attest)
  status          text NOT NULL,                            -- draft|reviewing|published|archived
  reviewer_id     text,                                     -- 跨 schema 不写 FK
  errata_pdf_url  text,                                     -- v2 D-Concern3: 已发布后修订 用"勘误公告"PDF,不删除原报告
  errata_reason   text,                                     -- v2 D-Concern3: 勘误原因(厂商申诉 / 内部修订)
  published_at    timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_leaderboard_period ON leaderboard_report(period_year, period_month);
```

### 4.X.13 leaderboard_optout_request(v2 厂商退出申请)

```sql
CREATE TABLE leaderboard_optout_request (
  id              text PRIMARY KEY,
  vendor_name     text NOT NULL,
  applicant_email text NOT NULL,
  applicant_organization text,
  reason          text,
  contact_phone   text,
  identity_proof_url text,                                  -- 主体认证材料
  status          text NOT NULL,                            -- pending|approved|rejected
  reviewer_id     text,                                     -- 跨 schema 不写 FK(admin_user 可能独立)
  decision_reason text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  decided_at      timestamptz
);
```

### 4.X.14 postmortem(v2 扩展, 详 07 §6 LLM 起草工作流)

> 07 模块中的 postmortem 表 v2 字段扩展(已在 07 §12 中标注):
> - `generated_by` 改为 enum: ai|user|hybrid
> - 新增 `status` enum: ai_drafting|ai_drafted|under_review|published|rejected
> - 新增 `llm_model` / `llm_prompt_version` / `ai_draft_at` / `reviewer_id` / `review_completed_at`
> - 新增 `ai_segments_accepted_count` / `ai_segments_rewritten_count`(反馈循环)

---

### 4.X.15 cert.* schema(v2 免费证书模块, S2 上线)

> 完整 DDL 见 `lib/db/migrations/idcd_main/00042_cert_init.sql`。详 [`20-free-cert.md §5`](./20-free-cert.md#5-领域模型与数据表)。
>
> **D1 跨 schema 不写 FK 说明**:`cert.orders.account_id` / `cert.dns_credentials.account_id` / `cert.certs.account_id` 等列指向 `account.users.id` 但**不**声明 FK,走 Repository 应用层 join(`apps/cert-svc/internal/repo/*.go`)。`cert.orders.cert_id` / `cert.renewal_jobs.cert_id` / `cert.renewal_jobs.new_order_id` 这类同 schema 引用也保持无 FK 风格,与全库迁移耦合策略一致。
>
> **付费档兼容**:`cert.orders` 的 `tier` / `sans_unicode` / `common_name` / `validity_days` / `reseller_channel` / `reseller_order_ref` / `organization_id` / `billing_invoice_id` 字段从 S1 day-1 就建好,S3 付费 CA 接入零 schema 改动(详 20-free-cert §20)。

```sql
CREATE SCHEMA IF NOT EXISTS cert;

-- 4.X.15.1 cert.domains — 域名注册表 + CAA 缓存,按账号 dedup
CREATE TABLE cert.domains (
  id             BIGSERIAL    PRIMARY KEY,
  account_id     BIGINT       NOT NULL,   -- → account.users.id (无 FK)
  fqdn           TEXT         NOT NULL,
  caa_status     TEXT,
  caa_checked_at TIMESTAMPTZ,
  created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (account_id, fqdn)
);

-- 4.X.15.2 cert.dns_credentials — DNS Provider API 凭据,KMS 信封加密
CREATE TABLE cert.dns_credentials (
  id                BIGSERIAL    PRIMARY KEY,
  account_id        BIGINT       NOT NULL,   -- → account.users.id (无 FK)
  provider          TEXT         NOT NULL,   -- cloudflare|aliyun|dnspod|route53|gcloud|manual
  display_name      TEXT         NOT NULL,
  encrypted_blob    BYTEA        NOT NULL,
  dek_wrapped       BYTEA        NOT NULL,
  kek_key_id        TEXT         NOT NULL,
  health_status     TEXT         NOT NULL DEFAULT 'unknown',
  health_checked_at TIMESTAMPTZ,
  created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  revoked_at        TIMESTAMPTZ
);

-- 4.X.15.3 cert.acme_accounts — 平台级 ACME 账号(每 CA × env 一条)
CREATE TABLE cert.acme_accounts (
  id                  BIGSERIAL    PRIMARY KEY,
  ca                  TEXT         NOT NULL,   -- lets-encrypt|zerossl|buypass
  env                 TEXT         NOT NULL,   -- production|staging
  account_url         TEXT         NOT NULL,
  key_kms_handle      TEXT         NOT NULL,
  eab_kid             TEXT,
  eab_hmac_kms_handle TEXT,
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (ca, env)
);

-- 4.X.15.4 cert.orders — 签发订单。付费档字段(tier/sans_unicode/common_name/...) day-1 就建好
CREATE TABLE cert.orders (
  id                  BIGSERIAL    PRIMARY KEY,
  account_id          BIGINT       NOT NULL,
  sans                TEXT[]       NOT NULL,
  sans_unicode        TEXT[],
  common_name         TEXT,
  tier                TEXT         NOT NULL DEFAULT 'free-dv',
  ca                  TEXT         NOT NULL,
  reseller_channel    TEXT,
  reseller_order_ref  TEXT,
  organization_id     BIGINT,
  validity_days       INT          NOT NULL DEFAULT 90,
  challenge_type      TEXT         NOT NULL,
  dns_credential_id   BIGINT,
  status              TEXT         NOT NULL,
  csr_pem             TEXT,
  cert_id             BIGINT,
  billing_invoice_id  TEXT,
  retry_count         INT          NOT NULL DEFAULT 0,
  last_error          TEXT,
  idempotency_key     TEXT,
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  finalized_at        TIMESTAMPTZ,
  UNIQUE (account_id, idempotency_key),
  UNIQUE (billing_invoice_id)
);
CREATE INDEX ON cert.orders (account_id, status);
CREATE INDEX ON cert.orders (status) WHERE status IN ('validating','issuing');

-- 4.X.15.5 cert.order_events — 状态机 WAL(每 order action_seq 单调递增)
CREATE TABLE cert.order_events (
  id            BIGSERIAL    PRIMARY KEY,
  order_id      BIGINT       NOT NULL,
  action_seq    INT          NOT NULL,
  action        TEXT         NOT NULL,
  payload_jsonb JSONB,
  occurred_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (order_id, action_seq)
);

-- 4.X.15.6 cert.certs — 已签发证书
CREATE TABLE cert.certs (
  id                 BIGSERIAL    PRIMARY KEY,
  order_id           BIGINT       NOT NULL,
  account_id         BIGINT       NOT NULL,
  sans               TEXT[]       NOT NULL,
  issuer             TEXT         NOT NULL,
  serial_hex         TEXT         NOT NULL,
  fingerprint_sha256 TEXT         NOT NULL,
  leaf_pem           TEXT         NOT NULL,
  chain_pem          TEXT         NOT NULL,
  key_kms_handle     TEXT         NOT NULL,
  not_before         TIMESTAMPTZ  NOT NULL,
  not_after          TIMESTAMPTZ  NOT NULL,
  status             TEXT         NOT NULL,   -- issued|revoked|expired
  revoked_at         TIMESTAMPTZ,
  revoke_reason      TEXT,
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX ON cert.certs (account_id, status);
CREATE INDEX ON cert.certs (not_after) WHERE status='issued';

-- 4.X.15.7 cert.renewal_jobs — 续期任务(到期前 30 天调度)
CREATE TABLE cert.renewal_jobs (
  id            BIGSERIAL    PRIMARY KEY,
  cert_id       BIGINT       NOT NULL,
  scheduled_at  TIMESTAMPTZ  NOT NULL,
  attempt_count INT          NOT NULL DEFAULT 0,
  last_error    TEXT,
  status        TEXT         NOT NULL,   -- scheduled|running|succeeded|failed|aborted
  new_order_id  BIGINT,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 4.X.15.8 cert.audit_logs — append-only 审计
CREATE TABLE cert.audit_logs (
  id            BIGSERIAL    PRIMARY KEY,
  account_id    BIGINT,
  actor         TEXT         NOT NULL,
  action        TEXT         NOT NULL,
  target_kind   TEXT,
  target_id     BIGINT,
  payload_jsonb JSONB,
  occurred_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

---

## 5. 关键策略

### 5.1 分区与归档

| 表 | 分区策略 | 归档 |
|---|---|---|
| monitor_check | TimescaleDB Hypertable，按天 | 按订阅档保留 7-180 天 |
| probe_result | 按天 | 保留 90 天 |
| node_heartbeat | 按天 | 保留 7 天 |
| usage_event | 按天 | 保留 180 天 |
| audit_log | 按周 | 保留 180 天 |
| webhook_delivery | 按月（普通 PG） | 保留 30 天 |

超出保留期：旧分区自动压缩 → drop。重要事件做归档（到 R2 / OSS）。

### 5.2 索引策略

- 主查询模式建索引
- `WHERE deleted_at IS NULL` 用部分索引
- 时序表用 `(monitor_id, started_at DESC)` 等组合
- jsonb 字段查询用 GIN 索引
- 多列索引顺序按选择性排

### 5.3 字段加密

| 字段 | 算法 | 用法 |
|---|---|---|
| password_hash | Argon2id | 用户密码 |
| api_key.secret_hash | SHA-256 | 不可逆 |
| user_2fa.secret_encrypted | AES-256-GCM | 应用层加密 |
| channel.config_encrypted | AES-256-GCM | webhook URL / 第三方 token |
| user_2fa.backup_codes_encrypted | AES-256-GCM | 备份码 |
| admin_user.totp_secret_encrypted | AES-256-GCM | TOTP 密钥 |

主密钥由 KMS 托管，应用启动时拿一次 DEK（数据加密密钥）解密。

### 5.4 软删除

- 关键实体（user / monitor / status_page / dashboard）用 `deleted_at`
- 业务查询自动加 `WHERE deleted_at IS NULL`
- 真正物理删除走异步任务（30 天后）

### 5.5 时间字段

- 全部 `timestamptz`（含时区）
- 应用层统一 UTC 存储
- 显示层按用户时区转换
- 不存 `date` 类型（特殊场景如 sla_report.period_start 例外）

### 5.6 字符串与本地化

- 主键 / slug / 标识符：text（强制 ASCII 规则）
- 用户输入：text 不限长（业务层校验）
- 邮箱 / 用户名：`citext`（不区分大小写）
- 多语言内容：jsonb `{ zh: "...", en: "..." }`

### 5.7 关系完整性

- 用户软删除后保留 PII 外键（保持关系，但 PII 脱敏）
- 强关联（teamMember → user）用 ON DELETE CASCADE
- 弱关联（monitor → alert_policy）用 ON DELETE SET NULL

### 5.8 迁移策略

- 使用 `golang-migrate` 或 `sqlc` 配套工具
- 所有 schema 变更走版本化迁移
- 破坏性变更走两阶段（新字段并存 → 旧字段下线）
- 大表 ALTER 用 `pg_repack` / pt-online-schema-change

---

## 6. 数据生命周期

```
用户行为
  ↓
[实时事件层]  → Redis Streams → 消费者实时处理
  ↓
[写入业务库 / 时序库]
  ↓
[聚合层]
  ├── 分钟聚合 → 实时仪表盘
  ├── 小时聚合 → 短期查询
  └── 日聚合 → 长期查询 + SLA 报告
  ↓
[归档层]
  ├── TimescaleDB 压缩
  ├── 冷数据 → R2/OSS 归档
  └── 超期 → 删除（合规）
```

---

## 7. 性能与扩容

### 7.1 单表大小预估（S3 末）

| 表 | 估计行数 | 估计磁盘 |
|---|---|---|
| user | 50,000 | < 100 MB |
| monitor | 80,000 | 200 MB |
| monitor_check | 200 亿+ | 2-10 TB（压缩后 1-2 TB）|
| probe_result | 50 亿+ | 1-3 TB |
| usage_event | 10 亿+ | 100 GB |
| alert_event | 1000 万 | 5 GB |
| node | 10,000 | 50 MB |

### 7.2 切 ClickHouse 的触发条件

任一满足：
- `monitor_check` 单日新增 > 10 GB
- TimescaleDB 写入 P99 > 100ms
- 聚合查询超过 5 秒
- 表 chunk > 5000 个

### 7.3 读写分离
- 主库写
- 1-2 个 read replica 给后台查询 + 报表

### 7.4 连接池
- pgbouncer / 自家
- 应用层连接池：单实例 10-50 连接
- 防止连接风暴：动态限制

---

## 8. 数据治理

### 8.1 数据所有权
- 业务数据归属 `owner_type / owner_id`
- 用户注销 → 所有 owner_id = 用户的资源做 PII 匿名化
- 团队解散 → 资源转 Owner 个人 或匿名化

### 8.2 数据导出
- 用户可一键导出所有自己的数据（PIPL）
- 后台异步任务，邮件下载链接

### 8.3 数据备份
- 物理备份：每日全量 + WAL 持续
- 逻辑备份：每周 pg_dump
- 异地：R2 / OSS 跨区

### 8.4 数据恢复演练
- 每季度从备份完整恢复一次到 staging
- 验证 RTO / RPO 达标

---

## 9. 阶段交付清单

### S1
- 用户 / 凭证 / 会话 / API Key
- 节点 / 节点心跳 / 健康
- probe_task / probe_result
- report（基础）
- denylist / rate_limit_override
- audit_log
- ip_info_cache / whois_cache / icp_cache / ssl_cache

### S2
- monitor / monitor_check / 聚合
- alert_policy / channel / alert_event / alert_notification
- status_page / status_section / status_component / status_incident
- subscription / order / refund / invoice / payment_method
- credit_ledger / coupon / referral
- usage_event / api_quota_usage
- webhook_endpoint / webhook_delivery
- ticket / abuse_report
- **v2 NEW**:
  - verdict_order / verdict_report / attestation_record / tsa_response / key_ceremony_log
  - compliance_subscription
  - postmortem 扩展字段(LLM 起草支持)
  - leaderboard_report / leaderboard_optout_request

### S3
- community_node_* 全套
- dashboard / sla_report
- user_risk_score
- admin_user / admin_audit_log / approval
- **v2 NEW**:
  - mcp_session / mcp_tool_call / mcp_token
  - agent_obs_monitor / agent_obs_event(Hypertable)

### S4
- Enterprise / SSO 相关
- 私有部署适配（schema 多租户隔离）
- **v2 NEW**:
  - 白标 Attestation 多租户隔离(`tenant_id` 列)
  - HSM 密钥指针(key_ceremony_log 增 hsm_serial 字段)
  - M24 agent_output_quality_score 表

---

## 10. 决策记录（已锁定，见 DECISIONS.md）

### v1.0
- ✅ **D5** 主键：**text（prefix + nanoid）**，如 `u_xxx` `m_xxx`

### v2.0 (K 节, 2026-05-12)
- ✅ **K1** 三栈虽独立子域,但共用 PG 集群,通过 schema 隔离:`idcd_main` / `idcd_attest` / `idcd_mcp` / `idcd_audit`
- ✅ **K-数据 跨 schema 不用 FK**:应用层 join,避免迁移耦合
- ✅ **K-数据 ID 前缀全表统一**:13 个新前缀加入 §2 ID 前缀表
- ✅ **K-数据 高频时序表用 Hypertable**:mcp_tool_call + agent_obs_event 用 TimescaleDB
- ✅ **K-数据 敏感数据哈希存储**:mcp_tool_call.request_payload_hash 不存原文(用户 prompt 隐私)
- ✅ **K-数据 信任根审计**:key_ceremony_log 只增不删,双人审批写入

### 已采用 PRD 默认

- 软删除：`deleted_at`（部分索引 `WHERE deleted_at IS NULL`）
- 加密：应用层 AES-256-GCM（KMS 托管主密钥）
- 多语言：jsonb `{ zh: "...", en: "..." }`
- audit_log：独立逻辑分区 + 6 个月起保留
- **v2 NEW** verdict_report 归档表存 6 年(合规);archived_url 走 WORM 对象存储

### 待定（不紧迫）

- [ ] 时序数据 S3 切 ClickHouse 还是继续 TimescaleDB：视数据量发展决定（监控目标：单日新增 > 10 GB / P99 > 100ms 触发评估）
- [ ] **v2 NEW** mcp_tool_call 高频率下是否走 ClickHouse 而非 Hypertable(S3 评估)
- [ ] **v2 NEW** verdict_report 是否引入 `verdict_dispute` 申诉表(详 12 §19 / 18 §7.4):S2 末根据投诉量决
