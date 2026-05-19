# 20 · SSL 证书在线申请（Cert Platform — Free + Paid Ready）

> 关联：OVERVIEW.md（新增工具模块）、14-tech-architecture、15-data-model、DECISIONS.md（本模块决策以 D-FC 前缀登记）
> 阶段：S1 MVP 免费（Let's Encrypt + Cloudflare DNS + 手动模式）→ S2 多 CA + 多 DNS provider + 自动续期 → S3 商业化档位 + **付费 CA 渠道（reseller）接入** → S4 OV / EV / 团队席位
> 是否登录可用：**必须登录**（私钥下载 / 续期 / 撤销均需账号绑定，配额与反滥用要求）
> 模块定位：**独立工具模块，不并入 02-public-tools**。前端独立路由组 `/app/cert/*`，后端独立 service `apps/cert-svc`。
> **付费扩展性**：本模块从第一天就按"DV 免费 + DV/OV/EV 付费"双轨设计；S1 仅实现免费分支，但 CA 适配层、状态机、订单模型已经为 reseller 协议预留接口（详 §20）。

---

## 1. 模块定位与边界

### 1.1 一句话定位

让用户在 idcd 后台**一键申请 / 自动续期 / 安全下载**符合 CA/Browser Forum 标准的免费 DV 证书（含通配符），覆盖 Let's Encrypt / ZeroSSL / Buypass / Google Trust Services 四大 ACME 免费 CA。

### 1.2 与既有模块的边界

| 模块 | 关系 |
|---|---|
| 02-public-tools | **无耦合**。免费证书不是"公开匿名工具"，不出现在 `/tools/*` 入口。后台 `/app/cert/*` 独立第一级菜单 |
| 03-account-system | 复用账号、登录、配额计量框架；新增 `cert.*` 配额维度 |
| 09-billing | S1 完全免费；S3 增量档（API / 多账号 / 私钥托管增值服务）走既有聚合支付 / 阿里云市场链路 |
| 11-admin | 复用 admin 框架，新增"证书订单"、"CA 配额"、"DNS provider 健康度"三块面板 |
| 12-compliance-and-abuse | 共用反滥用底盘；新增"短时间内大量不同根域名签发"的风控规则 |
| 18-evidence | **不复用 KMS 签发链路**。Verdict 用 idcd 自己的 KMS 签名做证据签名；本模块的"证书私钥"用独立 KMS keyring（D-FC-04） |
| 19-ai-agent | 无关联（本模块不暴露 MCP tool；远期 S4 可考虑） |

### 1.3 不做什么（S1-S2 范围）

- 不自建 CA（我们永远是 ACME 客户端 + reseller 客户端，不签发自家根）
- 不做代码签名 / S/MIME 证书
- 不做反向 CA（用户上传 CSR 让我们签）
- 不做证书托管自动部署（不主动登录用户服务器 / Nginx / CDN）—— S3 可选支持 webhook 推送
- 不做证书透明度（CT）日志查询工具（与本模块无关，归到 02-public-tools 远期 SEO 工具）
- 不做 mTLS / 客户端证书（暂不在 scope）
- **S1-S2 不做 OV / EV**（不做企业身份审核）；**S3 起通过付费 reseller 渠道接入**（详 §20）

---

## 2. 目标用户与场景

### 2.1 用户画像

| 画像 | 痛点 | 在 idcd 申请的理由 |
|---|---|---|
| 个人独立开发者 | 多个小站点 / 副业项目，自己跑 acme.sh 容易掉链 | 一处登录管全部域名，到期自动提醒 |
| 中小企业运维 | 域名分散在多家 DNS 商，手动加 TXT 烦 | 一次授权 DNS API，所有 CA 自动续期 |
| 国内开发者 | 阿里云 / 腾讯云免费证书每年 20 张限额、续期繁琐 | 不限额（受 CA 自身 rate limit 制约）、统一中文界面、内网可达 |
| SaaS 多租户产品方 | 给客户自定义域名签证书（白标） | 远期 v3 提供 API + Webhook，按张数计费 |

### 2.2 核心场景

1. **首次申请**：登录 → 输入域名 / SAN → 选 CA → 选 challenge → 授权 DNS API（或手动模式）→ 等待签发 → 下载证书包
2. **自动续期**：到期前 30 天自动触发，DNS-01 走原 provider 自动签发，结果邮件 + 站内消息
3. **撤销**：误签 / 私钥泄露场景，一键 Revoke，CA 端 + 本地状态同步
4. **多 SAN 单证书**：`a.com` + `*.a.com` + `b.com` 打在一张证书内（受 CA 限制 100 SAN）
5. **CAA 排错**：申请前预检 CAA 记录，给出明确"为何被拒"提示

### 2.3 北极星指标

| 指标 | S1 目标（MVP 上线 + 1 月） | S2 目标 | S3 目标 |
|---|---|---|---|
| DAU（进入 /app/cert 的登录用户） | 30 | 300 | 3,000 |
| 累计签发张数 | 500 | 10,000 | 100,000 |
| 端到端签发 P95 耗时（提交 → 可下载） | ≤ 180s | ≤ 90s | ≤ 60s |
| 续期成功率 | — | ≥ 98% | ≥ 99.5% |
| CA quota 健康度（任一 CA 周用量 / 上限） | ≤ 60% | ≤ 50% | 多 CA 轮询 ≤ 40% |
| 滥用率（被风控拦截订单 / 总订单） | < 1% | < 0.5% | < 0.3% |

---

## 3. 功能范围（按阶段）

### 3.1 S1 MVP（4 周，可单人完成）

**目标**：能让一个登录用户在 Cloudflare DNS 下成功签发并下载一张 Let's Encrypt 单域名 / 多 SAN 证书。

- 后台 `/app/cert` 一级菜单 + 三个子页面（订单列表 / 新建订单 / DNS 凭据管理）
- 支持 CA：**仅 Let's Encrypt**（ACMEv2，免 EAB）
- 支持 challenge：**DNS-01 自动（Cloudflare）+ DNS-01 手动**（用户自己加 TXT，平台轮询 dig）
- 支持 SAN：≤ 10 个域名 / 单证书；支持通配符
- 私钥：本地生成 ECDSA P-256，AES-GCM 加密落库（DEK 由进程内 master key 加密；S2 切真 KMS）
- 下载：PEM 单包（`fullchain.pem` + `privkey.pem`），下载链接 5 分钟过期
- 配额：单账号 5 张/月、5 张/同一根域名/周（独立于 LE 上游 rate limit）
- 邮件：签发成功 / 失败两封模板
- **不做**：自动续期、撤销、多 CA、Pkcs#12 导出、Webhook

### 3.2 S2（再 4 周）

- 多 CA：ZeroSSL（EAB）+ Buypass；CA 路由策略（默认 LE，配额 > 70% 切 ZeroSSL）
- 多 DNS provider：阿里云 DNS、DNSPod、Route53、Google Cloud DNS（用 `go-acme/lego` 内置 provider）
- 自动续期：到期前 30 天 cron 调度 + retry queue（参考 D5 refund retry 风格）
- 撤销：用户主动 / admin 强制
- PKCS#12 (.pfx) 导出
- KMS 真接入（阿里云 KMS 或自托管 HashiCorp Vault；遵循 D-FC-04 决策）
- CAA 预检 + 明确错误码
- abuse-detection 规则（短时多根域名风控）

### 3.3 S3（4-6 周，商业化窗口）

- Google Trust Services（需 GCP 账号 + EAB）
- API 接入（OpenAPI 3.1，与 idcd 主 API 同体系，独立 `/v1/cert/*` 命名空间）
- Webhook：签发完成 / 续期完成 / 即将到期 三类事件
- 自定义命名规则、批量导入域名
- 商业化档：免费 50 张 / 月，超出按张计费（¥1-5/张），API 接入年订
- 多账号 / RBAC（团队席位）

### 3.4 不在范围

- ACME 服务端（我们是 ACME 客户端的封装，不是 CA）
- 自动部署到 Nginx / Apache / 负载均衡器
- 证书发现 / 资产清点（远期独立模块）

---

## 4. 系统架构

### 4.1 模块清单（落到 idcd 既有 monorepo）

| 模块 | 路径 | 是否新增 | 职责 |
|---|---|---|---|
| `cert-svc` | `apps/cert-svc`（新增 go.work entry） | 新 | 订单领域服务 + HTTP handler，承载所有 `/api/v1/cert/*` |
| `cert-worker` | `apps/cert-svc/cmd/worker` 子命令 | 新 | ACME orchestrator，消费 Redis Stream 跑状态机 |
| `cert-renewer` | `apps/cert-svc/cmd/renewer` 子命令 | 新 | cron 调度续期，复用 worker 队列 |
| `lib/cert-ca` | `lib/cert/ca` | 新 | CA 适配层（封装 `go-acme/lego`） |
| `lib/cert-dns` | `lib/cert/dns` | 新 | DNS provider 适配（复用 lego 的 provider 集） |
| `lib/cert-vault` | `lib/cert/vault` | 新 | 私钥 / DNS 凭据加密存取，对接 KMS |
| `apps/web` | `apps/web/app/app/cert/*` | 扩展 | 前端路由组，复用 `(app)` Sidebar 布局 |
| `apps/gateway` | 现有 | 扩展 | 加 `/v1/cert/*` 路由转发到 `cert-svc` |
| `apps/notifier` | 现有 | 扩展 | 加证书事件模板（签发成功 / 失败 / 即将到期） |
| `apps/api` | 现有 | 不动 | 与本模块解耦 |
| `apps/scheduler` | 现有 | 复用 | `cert-renewer` 注册为一个 schedule entry（与 monitor cron 同框架） |
| `apps/admin`（在 `apps/web/app/admin/cert/*`） | 扩展 | — | 管理面板 |

> **跨 schema 不写 FK**（遵循 D1）：cert 模块新建独立 schema `cert.*`，与 `account.*` / `billing.*` 走应用层 join。

### 4.2 部署拓扑

```
        ┌─────────────┐
client →│  gateway    │── /v1/cert/* ──→ cert-svc (HTTP)
        └─────────────┘                   │
                                          │ enqueue (Redis Stream)
                                          ▼
                                   ┌────────────┐    ┌──────────┐
                                   │ cert-worker│←──→│ KMS / DB │
                                   │ (N 副本)   │    └──────────┘
                                   └─────┬──────┘
                                         │
                          ┌──────────────┼──────────────┐
                          ▼              ▼              ▼
                       CA APIs       DNS APIs       cert-store
                       (LE/ZSSL...)  (CF/Ali...)    (Postgres)

        scheduler ──cron── cert-renewer ──enqueue──→ Redis Stream
```

**单实例上限**：cert-worker 每实例并发 50 个 in-flight order（一个 order 平均 60-180s，CPU/网络都不是瓶颈，瓶颈是 DNS 传播等待）；S1 单实例足够。

### 4.3 与 idcd 现有基础设施的复用

| 复用项 | 来源 |
|---|---|
| 鉴权 / Session | `lib/auth`（账号系统） |
| DB 连接池 / 迁移 | `lib/db`（Postgres pgx） |
| 限流 | `lib/ratelimit` |
| 日志 / Metric / Trace | `lib/shared/obs` |
| 邮件发送 | `apps/notifier` |
| 配置中心 | `lib/shared/config` |

---

## 5. 领域模型与数据表

### 5.1 实体关系（cert schema）

```
account (account.users)              ← 跨 schema 引用，应用层 join
   ▲
   │
cert_domains ─────┐
                  │
cert_dns_creds    │
   ▲              │
   │              │
   └──── cert_orders ──┬── cert_order_events (WAL)
                       │
                       └── certs ──── cert_renewal_jobs

cert_acme_accounts  ← 平台向各 CA 注册的账号（按 CA × 环境一行）
cert_audit_logs     ← 所有写操作不可变追加
```

### 5.2 表 DDL（草图，最终以 15-data-model.md §4.X 收口）

```sql
CREATE SCHEMA IF NOT EXISTS cert;

-- 域名登记（可选，仅做去重 / CAA 缓存）
CREATE TABLE cert.domains (
  id            BIGSERIAL PRIMARY KEY,
  account_id    BIGINT       NOT NULL,        -- 应用层 join account.users
  fqdn          TEXT         NOT NULL,
  caa_status    TEXT,                          -- 'ok' | 'forbid_le' | 'forbid_all' | 'unknown'
  caa_checked_at TIMESTAMPTZ,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (account_id, fqdn)
);

-- DNS API 凭据（KMS 信封加密）
CREATE TABLE cert.dns_credentials (
  id              BIGSERIAL PRIMARY KEY,
  account_id      BIGINT      NOT NULL,
  provider        TEXT        NOT NULL,       -- 'cloudflare' | 'aliyun' | 'dnspod' | 'route53' | ...
  display_name    TEXT        NOT NULL,
  encrypted_blob  BYTEA       NOT NULL,       -- AES-GCM(DEK, payload)
  dek_wrapped     BYTEA       NOT NULL,       -- KMS Wrap(DEK)
  kek_key_id      TEXT        NOT NULL,
  health_status   TEXT        NOT NULL DEFAULT 'unknown',
  health_checked_at TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at      TIMESTAMPTZ
);

-- 平台 ACME 账号（按 CA × env）
CREATE TABLE cert.acme_accounts (
  id              BIGSERIAL PRIMARY KEY,
  ca              TEXT        NOT NULL,       -- 'letsencrypt' | 'zerossl' | 'buypass' | 'gts'
  env             TEXT        NOT NULL,       -- 'prod' | 'staging'
  account_url     TEXT        NOT NULL,
  key_kms_handle  TEXT        NOT NULL,
  eab_kid         TEXT,
  eab_hmac_kms_handle TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (ca, env)
);

-- 签发订单（含付费扩展位，S1 不使用 tier 之外字段）
CREATE TABLE cert.orders (
  id                  BIGSERIAL PRIMARY KEY,
  account_id          BIGINT      NOT NULL,
  sans                TEXT[]      NOT NULL,            -- 已 Punycode 化的 ASCII 表示
  sans_unicode        TEXT[],                          -- 原始 Unicode 输入（用于展示）
  common_name         TEXT,                            -- 通常 = sans[0]；可为空（仅依赖 SAN）
  tier                TEXT        NOT NULL DEFAULT 'free-dv',  -- 'free-dv'|'paid-dv'|'paid-ov'|'paid-ev'
  ca                  TEXT        NOT NULL,            -- 'letsencrypt'|'zerossl'|'buypass'|'gts'|reseller channel
  reseller_channel    TEXT,                            -- S3 起：'digicert'|'sectigo'|'gogetssl' 等；免费分支 NULL
  reseller_order_ref  TEXT,                            -- S3 起：reseller 端订单 ID
  organization_id     BIGINT,                          -- S3 起：cert.organizations 应用层 join
  validity_days       INT         NOT NULL DEFAULT 90,
  challenge_type      TEXT        NOT NULL,            -- 'dns-01' | 'http-01' | 'email' (S3 OV/EV)
  dns_credential_id   BIGINT,                          -- NULL 表示手动模式
  status              TEXT        NOT NULL,            -- draft|validating|awaiting_org_validation|issuing|issued|failed|revoking|revoked
  csr_pem             TEXT,
  cert_id             BIGINT,
  billing_invoice_id  TEXT,                            -- S3 起：09-billing 关联（UNIQUE 防重复扣款）
  retry_count         INT         NOT NULL DEFAULT 0,
  last_error          TEXT,
  idempotency_key     TEXT,                            -- 防重复点击
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finalized_at        TIMESTAMPTZ,
  UNIQUE (account_id, idempotency_key),
  UNIQUE (billing_invoice_id)
);
CREATE INDEX ON cert.orders (account_id, status);
CREATE INDEX ON cert.orders (status) WHERE status IN ('validating','issuing');

-- 订单事件（WAL，参考 D4 attestation_record 风格）
CREATE TABLE cert.order_events (
  id              BIGSERIAL PRIMARY KEY,
  order_id        BIGINT      NOT NULL,
  action_seq      INT         NOT NULL,
  action          TEXT        NOT NULL,       -- 'new_order'|'authz_pending'|'dns_present'|'dns_propagated'|'authz_valid'|'finalize'|'download'|'failed'|...
  payload_jsonb   JSONB,
  occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (order_id, action_seq)
);

-- 已签发证书
CREATE TABLE cert.certs (
  id                BIGSERIAL PRIMARY KEY,
  order_id          BIGINT      NOT NULL,
  account_id        BIGINT      NOT NULL,
  sans              TEXT[]      NOT NULL,
  issuer            TEXT        NOT NULL,
  serial_hex        TEXT        NOT NULL,
  fingerprint_sha256 TEXT       NOT NULL,
  leaf_pem          TEXT        NOT NULL,
  chain_pem         TEXT        NOT NULL,
  key_kms_handle    TEXT        NOT NULL,     -- KMS 句柄；S1 可临时是 'aesgcm:base64-blob'
  not_before        TIMESTAMPTZ NOT NULL,
  not_after         TIMESTAMPTZ NOT NULL,
  status            TEXT        NOT NULL,     -- 'issued'|'revoked'
  revoked_at        TIMESTAMPTZ,
  revoke_reason     TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON cert.certs (account_id, status);
CREATE INDEX ON cert.certs (not_after) WHERE status='issued';

-- 续期任务
CREATE TABLE cert.renewal_jobs (
  id              BIGSERIAL PRIMARY KEY,
  cert_id         BIGINT      NOT NULL,
  scheduled_at    TIMESTAMPTZ NOT NULL,
  attempt_count   INT         NOT NULL DEFAULT 0,
  last_error      TEXT,
  status          TEXT        NOT NULL,       -- 'queued'|'running'|'succeeded'|'failed'|'abandoned'
  new_order_id    BIGINT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 审计
CREATE TABLE cert.audit_logs (
  id            BIGSERIAL PRIMARY KEY,
  account_id    BIGINT,
  actor         TEXT        NOT NULL,         -- 'user:123' | 'system' | 'admin:5'
  action        TEXT        NOT NULL,
  target_kind   TEXT,
  target_id     BIGINT,
  payload_jsonb JSONB,
  occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 5.3 字段约束

- `cert.certs.leaf_pem` 不超过 8KB；`chain_pem` 不超过 16KB（防恶意巨型链）
- `cert.orders.sans` 长度 ≤ 100，每个元素小写归一化，禁止 IP / localhost / 内网域名
- `cert.dns_credentials.encrypted_blob` 不超过 4KB
- `cert.order_events` 单订单事件数 ≤ 64（超过即异常订单，告警）

---

## 6. 状态机

### 6.1 CertOrder

```
                ┌──────────────────────────────────────┐
                │                                      ▼
draft ──submit──► validating ──authz_valid──► issuing ──cert_ready──► issued
                     │                          │
                     │ caa_fail / authz_invalid │ finalize_fail
                     ▼                          ▼
                  failed ◀──retry_exceeded── failed
                     │
                     │ user_retry
                     ▼
                 validating

issued ──user_revoke──► revoking ──ca_ack──► revoked
issued ──renew_due ──► (renewal_job 创建新 order，不改原 order 状态；新订单走上方主路径)
```

**说明**
- `validating`：CAA 检查 + ACME newOrder + Authz 阶段
- `issuing`：DNS-01 challenge 已 present、等 CA 验证 + finalize + download
- `failed`：可恢复错误（DNS 写入失败 / CA 5xx）保留 retry 入口；不可恢复（CAA forbid / 域名无效）置为 terminal

### 6.2 RenewalJob

```
queued ──worker_pick──► running ──ok──► succeeded
                          │
                          │ fail
                          ▼
                       queued (next backoff: D+1 → D+3 → D+7)
                          │
                          │ attempt >= 3 && days_to_expire < 7
                          ▼
                       abandoned (告警用户 + admin)
```

> 详细状态机最终合入 STATE-MACHINES.md。

---

## 7. 关键流程时序

### 7.1 新建签发（DNS-01 自动模式）

```
user        gateway       cert-svc       cert-worker       CA            DNS-API       KMS
 │ POST orders  │            │                │             │              │            │
 ├─────────────►│ authn/quota│                │             │              │            │
 │              ├───────────►│ idemp check    │             │              │            │
 │              │            ├── CAA dig (cache 5min) ───────────────────────────────────►
 │              │            ├── write order(draft) + event(create)         │            │
 │              │            ├── enqueue stream                              │            │
 │              │◄───────────┤ 202 + orderId                                 │            │
 │◄─────────────┤            │                │             │              │            │
 │              │            │                │ pick task   │              │            │
 │              │            │                ├── KMS GenerateKey ────────────────────────►
 │              │            │                ├── build CSR                 │            │
 │              │            │                ├── ACME newOrder ───────────►│            │
 │              │            │                │◄────authz + challenge ──────┤            │
 │              │            │                ├── dns_credentials decrypt ───────────────►
 │              │            │                ├── DNS Present (TXT) ──────────────────────►
 │              │            │                ├── propagation wait (dig poll, 30-300s)
 │              │            │                ├── notify CA ───────────────►│            │
 │              │            │                ├── poll authz ──────────────►│            │
 │              │            │                │◄────valid───────────────────┤            │
 │              │            │                ├── finalize ────────────────►│            │
 │              │            │                │◄────cert URL────────────────┤            │
 │              │            │                ├── download leaf+chain ─────►│            │
 │              │            │                ├── DNS CleanUp ────────────────────────────►
 │              │            │                ├── write certs + order(issued) + event(done)
 │              │            │                ├── enqueue renewal job (T-30d)
 │              │            │                ├── notifier "签发成功"
 │ GET orders/id│            │                │             │              │            │
 ├─────────────►│            │                │             │              │            │
 │              │◄── status=issued, cert_id ──┤             │              │            │
 │ GET certs/id/download                      │             │              │            │
 ├─────────────►│            │                │             │              │            │
 │              │◄── one-time signed URL (5min) ─           │              │            │
```

### 7.2 手动模式分支

第 ② 步在 worker 拿到 challenge 后**不写 DNS**，而是把 TXT 记录写入 `order_events.payload_jsonb`，订单状态停在 `validating`；前端展示"请加 TXT 记录"指引；worker 后台 dig 轮询（每 30s 一次，最多 30min），用户加完 TXT 后自动继续后续步骤。

### 7.3 自动续期

`cert-renewer`（每小时 cron）：
```sql
SELECT id FROM cert.certs
WHERE status='issued'
  AND not_after < NOW() + INTERVAL '30 days'
  AND NOT EXISTS (
    SELECT 1 FROM cert.renewal_jobs
    WHERE cert_id = certs.id AND status IN ('queued','running','succeeded')
  );
```
为每条创建 `renewal_jobs(queued)` → worker 复用 7.1 流程创建新 order；签发成功后 `cert.certs` 老记录保留（审计），订单页"当前有效证书"指向新记录。

### 7.4 撤销

用户点撤销 → cert-svc 调 `CA.Revoke(cert, reason='unspecified')` → 成功后 `certs.status=revoked` + cancel 未来 renewal_job。Revoke 用 ACME 账号 key 签名（也可用 cert 私钥；我们统一用账号 key，避免要从 KMS 解密用户私钥）。

### 7.5 跨流程不变量

- **IDN 归一化**：所有入口域名先做 `idna.Lookup.ToASCII`（Punycode），SAN 列表存 ASCII，UI 展示存 Unicode；防止用户输入 `阿里巴巴.com` 与 `xn--mgbx4cd0ab.com` 被当成两个不同域名
- **TXT 必须 deferred cleanup**：DNS Present 后立即在事件队列登记 cleanup 任务，无论 order 成功 / 失败 / worker 崩溃，cleanup 都执行；避免 zone 长期残留 `_acme-challenge.x` 污染
- **CN vs SAN 策略**：CSR 中 `CommonName` 留空或填 `sans[0]`（仅为兼容老 Windows / Java 客户端），现代浏览器只看 SAN。固定策略：`CN = sans[0]`，避免遗漏
- **私钥轮换**：每次续期生成新私钥（不复用旧 key），降低长期暴露风险
- **订单状态下发**：前端用**短轮询**（订单状态接口，间隔 3s，订单 issued/failed 后停止），不引入 SSE / WebSocket；S2 评估是否升 SSE
- **ACME 账号 key 备份**：账号 key 丢失 = 无法 revoke 历史证书；KMS 必须开启自动备份 + 跨 region 复制 + admin 季度演练 restore

---

## 8. CA 与 DNS Provider 矩阵

### 8.1 支持的 CA

**免费（ACME 协议）**

| CA | 阶段 | ACME endpoint | EAB | 备注 |
|---|---|---|---|---|
| Let's Encrypt | S1 | `acme-v02.api.letsencrypt.org` | 无 | 主力。**多重 rate limit 都共享平台账号**：50 张/Registered Domain/周、300 newOrder/账号/3h、5 失败 validation/账号/域/小时。需在 §13 监控 |
| ZeroSSL | S2 | `acme.zerossl.com/v2/DV90` | 必须 | 注册时换 EAB；Web 注册免费档每账号 3 张/月**仅限 web 流程**，ACME 走 EAB 不受此限（但有 newOrder 总体节流） |
| Buypass | S2 | `api.buypass.com/acme` | 无 | 180 天有效期；**支持通配符**（2023 起开放） |
| Google Trust Services | S3 | `dv.acme-v02.api.pki.goog` | 必须（GCP 账号） | 通配符额度紧 |

> **MPIC 影响**：自 2025 起 LE / GTS 等主流 CA 都已启用 Multi-Perspective Issuance Corroboration，从多个 vantage point 各自查询 DNS 验证 challenge。这意味着我们写完 TXT 必须等到**全球权威 NS 都生效**，而不仅本地 resolver 看到。`lib/cert/dns` 的传播检查必须直接 dig 域名的 authoritative NS（不查 8.8.8.8 这类公共 resolver 缓存）。

**付费（Reseller / CertCentral 协议，S3 起接入；候选见 §20）**

| 渠道 | 协议 | 等级 | 接入难度 | 备注 |
|---|---|---|---|---|
| DigiCert CertCentral | REST | DV / OV / EV / 通配符 / 多域 | 中 | 直签，定价偏高，品牌信任最强 |
| Sectigo (Comodo) Reseller | REST | DV / OV / EV | 中 | 国内 reseller 多，价格低 |
| GoGetSSL | REST | DV / OV / EV | 低 | 文档清晰，常被中小 SaaS 用 |
| NameCheap Reseller | REST | DV / OV | 低 | 个人开发者熟，API 限于 reseller 账号 |
| 阿里云 / 腾讯云 SSL 市场 | 各自私有 | DV / OV / EV | 高（每家私有协议） | 国内主体合规需要，S4 可选 |

> **有效期 trend**：CA/Browser Forum 已表决（SC-081）将公网 TLS 证书最长有效期从 397 天逐步降至：2026-03 → 200 天、2027-03 → 100 天、2029-03 → 47 天。`cert.orders.validity_days` 必须支持动态，不要在代码里硬编码 397。

**CA 路由策略**（cert-svc 在 order 创建时执行）：
1. 用户显式指定 → 直接用
2. 通配符 → 优先 LE → ZeroSSL → GTS
3. 普通 → 看 LE 周配额，若 > 70% 切 ZeroSSL
4. 失败 retry → 自动换 CA（最多换 1 次）

### 8.2 支持的 DNS Provider（S2 全开）

直接复用 [`go-acme/lego/providers/dns/*`](https://github.com/go-acme/lego)。首批接入：

| Provider | 凭据 | 区域 |
|---|---|---|
| Cloudflare | API Token (Zone:DNS:Edit) | 全球，S1 |
| 阿里云 DNS | AccessKey | 国内主力，S2 |
| DNSPod / 腾讯云 DNS | API Token | 国内主力，S2 |
| AWS Route53 | AKSK | 海外，S2 |
| Google Cloud DNS | Service Account JSON | 海外，S2 |
| GoDaddy | API Key | 长尾，S3 |
| Namesilo / Namecheap | API Key | 长尾，S3 |
| 手动模式 | — | S1 兜底 |

> 不自己实现 provider；只在 lego provider 外面包薄薄一层做 (a) 错误归一化、(b) propagation timeout 收敛到统一区间 30-300s、(c) audit log 记录 present/cleanup。

---

## 9. 私钥与 KMS 方案（D-FC-04 决策点）

### 9.1 关键 tradeoff

| 方案 | 私钥可下载 | 安全等级 | 实施成本 |
|---|---|---|---|
| A. KMS 内生成，CSR 走 KMS Sign | ❌ 用户不能拿走 | 高 | 中 |
| B. 本地生成，KMS 加密落库 | ✅ | 中 | 低 |
| C. 客户端生成 CSR 上传 | ✅ 私钥不出本地 | 最高 | UX 差 |

**初步建议**：S1 选 **B**（必须能下载，否则证书没法部署到用户服务器）；S3 增量提供 **C**（高级模式，给安全敏感客户）。**S1 不上 A**。

### 9.2 加密细节（方案 B）

- 私钥生成：ECDSA P-256（默认）/ RSA 2048（兼容旧服务器，opt-in）
- 加密：AES-256-GCM，DEK 每条记录独立，nonce 12 字节随机
- DEK 由 KMS Wrap：
  - S1：进程内 master key（`MASTER_KEY` 环境变量，base64）；缺陷已知（不抗主机失窃），但允许 MVP 上线
  - S2：阿里云 KMS（国内）+ AWS KMS（海外）双路径；遵循 D-FC-04
- 解密路径：仅 cert-svc / cert-worker 进程 + 用户主动下载时
- 下载链接：HMAC 签名，5 分钟过期，单次使用（Redis SETNX 标记），全程 HTTPS

### 9.3 ACME 账号私钥

- 平台向每个 CA 注册一次，account key 走 KMS 托管
- account key rollover 计划：每年一次（admin 手动触发，遵循 ACME §7.3.5）

---

## 10. API 设计

### 10.1 用户侧（前端调用）

```
POST   /api/v1/cert/orders
       body: {sans:[...], challenge:"dns-01", dns_credential_id?, ca?, idempotency_key}
       → 202 {order_id, status}

GET    /api/v1/cert/orders                 list (paged)
GET    /api/v1/cert/orders/{id}            含 challenge 指示（手动模式回 TXT 内容）
POST   /api/v1/cert/orders/{id}/retry      重试失败订单

POST   /api/v1/cert/dns-credentials        {provider, display_name, secrets:{...}}
GET    /api/v1/cert/dns-credentials
DELETE /api/v1/cert/dns-credentials/{id}
POST   /api/v1/cert/dns-credentials/{id}/health-check

GET    /api/v1/cert/certs                  list
GET    /api/v1/cert/certs/{id}             metadata
POST   /api/v1/cert/certs/{id}/download    body:{format:"pem"|"pfx"|"nginx"}
                                           → {download_url, expires_at}
POST   /api/v1/cert/certs/{id}/revoke      {reason?}
```

### 10.2 Admin 侧

```
GET    /api/v1/admin/cert/orders            过滤 status / account / ca
POST   /api/v1/admin/cert/orders/{id}/force-fail
GET    /api/v1/admin/cert/ca-quota          各 CA 用量
GET    /api/v1/admin/cert/dns-health        各 provider 健康度
POST   /api/v1/admin/cert/accounts/{id}/ban 反滥用临时封禁
```

### 10.3 错误码（统一前缀 `CERT_`）

| Code | HTTP | 含义 |
|---|---|---|
| `CERT_QUOTA_EXCEEDED` | 429 | 账号月配额已用完 |
| `CERT_DOMAIN_INVALID` | 400 | 域名格式 / IP / localhost / 黑名单 |
| `CERT_CAA_FORBID` | 422 | CAA 不允许目标 CA |
| `CERT_DNS_PROVIDER_FAIL` | 502 | DNS API 写入失败 |
| `CERT_DNS_PROPAGATION_TIMEOUT` | 504 | TXT 全球传播超时 |
| `CERT_CA_RATE_LIMITED` | 429 | 上游 CA 限流（建议切 CA） |
| `CERT_CA_AUTHZ_INVALID` | 422 | CA 验证失败（CAA / DNS） |
| `CERT_PRIVATE_KEY_LOST` | 500 | 私钥解密失败（KMS 故障） |
| `CERT_ABUSE_BLOCKED` | 403 | 自动反滥用规则拦截（黑名单 / 短时多根域名 / 单根域名爆发） |
| `CERT_ACCOUNT_BANNED` | 403 | 管理员封禁（`cert.abuse_bans` 有效记录） |

> 完整列表纳入 16-api-spec.yaml 增量。

---

## 11. 前端页面与路由

### 11.1 路由（apps/web）

| 路径 | 描述 | 布局 |
|---|---|---|
| `/app/cert` | 总览（最近订单 + 即将到期 + 配额） | `(app)` Sidebar |
| `/app/cert/new` | 申请向导（4 步：域名 → CA → challenge → 确认） | 同上 |
| `/app/cert/orders` | 订单列表 + 详情抽屉 | 同上 |
| `/app/cert/orders/[id]` | 订单详情（手动模式显示 TXT 引导） | 同上 |
| `/app/cert/certs` | 已签发证书列表 | 同上 |
| `/app/cert/certs/[id]` | 证书详情（含下载 / 撤销） | 同上 |
| `/app/cert/dns-credentials` | DNS 凭据管理 | 同上 |
| `/admin/cert/*` | 管理面板 | `/admin` 布局 |

### 11.2 关键组件（全部 shadcn/ui，禁止裸 div + className 拼 UI）

- 申请向导：`Card` + `Steps`（自建 composition）+ `Form` + `Input` / `Select` + `Button`
- 订单列表：`Table` + `Badge`（状态） + `DropdownMenu`（操作）
- 手动 TXT 引导：`Alert` + `Code` block + "已添加，开始验证" `Button`
- 凭据表单：`Sheet`（侧边抽屉）+ `Form` + 提供商对应的 `Input` 字段集（不同 provider 字段不同）
- 倒计时 / 续期提醒：`Badge` + `Tooltip`
- 下载弹窗：`Dialog` + `RadioGroup`（格式选择）
- Toast：`Sonner`

### 11.3 文案

中英双语，所有 user-facing 字符串走 i18n 文件（遵循 I18N-PLAN.md）。

---

## 12. 反滥用与配额（与 12-compliance-and-abuse 协同）

### 12.1 静态配额

| 维度 | 限额 | 时窗 |
|---|---|---|
| 单账号订单 | 20 | 1 天 |
| 单账号已签发有效证书 | 100 | 总量 |
| 单根域名签发 | 10 | 7 天（独立于 LE 自身 50/7d 限） |
| 单 IP 注册 → 30 分钟内签发 | 2 | 防注册即薅 |
| DNS provider 凭据数 | 20 | 总量 |

### 12.2 动态风控

- 短时多根域名：1 小时内 ≥ 5 个不同根域名 → 人工审核 hold
- 域名疑似钓鱼：对照 Google Safe Browsing / 国家反诈名单（远期 S2 接入）
- 同账号反复 retry 失败（>10 次 / 天）→ 限流 + 通知

### 12.3 黑名单

- 域名硬黑名单（gov.cn 子域、知名银行、知名互联网公司核心域）→ 拒签 + admin alert
- 必须人工确认才能为这些域名签发（防被诱导签恶意子域）

---

## 13. 可观测与测试

### 13.1 Metrics（Prometheus）

| 指标 | 类型 | 维度 |
|---|---|---|
| `cert_orders_total` | counter | ca, status |
| `cert_order_duration_seconds` | histogram | ca, challenge |
| `cert_ca_quota_usage_ratio` | gauge | ca |
| `cert_dns_propagation_seconds` | histogram | provider |
| `cert_renewal_jobs_total` | counter | result |
| `cert_active_certs` | gauge | ca |
| `cert_kms_op_total` | counter | op, result |

### 13.2 Trace

OpenTelemetry，每个 order 一条端到端 trace（cert-svc → cert-worker → CA → DNS）。

### 13.3 告警

- CA 周配额 > 80% → P2
- DNS provider 错误率 > 10%（5min 窗口）→ P1
- 续期失败率 > 5%（24h 窗口）→ P1
- KMS 调用错误 > 0（连续 5 次）→ P0

### 13.4 测试门禁（遵循 CLAUDE.md）

- `lib/cert/*` 行覆盖 ≥ 90%；纯算函数 100%
- `apps/cert-svc` handler 用 httptest 全路径覆盖
- ACME 集成测试用 **Pebble**（LE 官方 ACME mock CA）跑在 CI
- DNS provider 测试用 **lego 自带 mock provider**
- KMS 测试用本地 AES-GCM stub
- 前端 utility 用 Vitest；申请向导用 Testing Library 测关键路径

### 13.5 端到端冒烟（pre-prod）

- 用 `*.staging.idcd.com` 子域，对 LE staging 环境每小时跑一次 end-to-end
- 失败立即 P1

---

## 14. 部署与运维

### 14.1 配置

```
CERT_DB_DSN
CERT_REDIS_URL
CERT_KMS_PROVIDER       # 'env' (S1) | 'aliyun' | 'aws' | 'vault'
CERT_MASTER_KEY         # S1: base64 32B; S2 起删除
CERT_LE_ACCOUNT_EMAIL   # 平台 ACME 账号联系邮箱
CERT_LE_ENV             # 'staging' | 'production'
CERT_ZSSL_EAB_KID / HMAC
CERT_GTS_EAB_KID / HMAC
CERT_DEFAULT_PROPAGATION_TIMEOUT=300s
CERT_DOWNLOAD_LINK_TTL=300s
```

### 14.2 数据库迁移

走 idcd 现有 migration 工具（`lib/db/migrate`），新建 `migrations/cert/*.sql`。lint：跨 schema FK 检测（scripts/lint-cross-schema-fk.sh 已存在）。

### 14.3 灾备

- DB 全量 + WAL，与 idcd 主库同策略
- 私钥加密 blob 已脱密 KMS → 即便 DB 泄露也不可解
- KMS keyring 跨 region 复制（S2）

### 14.4 退役 / 数据导出

用户注销账号：所有有效证书 force-revoke（CA 端 + 本地），私钥销毁（KMS DestroyKey），保留 audit log 6 年（合规）。

---

## 15. 实施路线图

### 15.1 S1（4 周）

| 周 | 交付 |
|---|---|
| W1 | DB schema + migration；`apps/cert-svc` 骨架（HTTP + worker 子命令）；`lib/cert/ca` Let's Encrypt 适配；`lib/cert/vault` env-master-key 实现 |
| W2 | ACME 完整状态机 + WAL；CSR 生成；Pebble 集成测试 |
| W3 | `lib/cert/dns` Cloudflare provider + 手动模式；DNS 凭据加解密 + UI |
| W4 | 前端申请向导 + 订单页 + 下载弹窗；邮件通知；端到端冒烟；上 staging |

**S1 验收**：登录用户能在 5 分钟内完成 Cloudflare 模式签发一张 `*.example.com` 证书并下载 fullchain/privkey 部署到 Nginx 验证 HTTPS 通。

### 15.2 S2（4 周）

| 周 | 交付 |
|---|---|
| W5 | ZeroSSL + Buypass adapter；CA 路由策略；CAA 预检 |
| W6 | 阿里云 / DNSPod / R53 / GCP DNS provider；KMS 真接入（阿里云 KMS） |
| W7 | 自动续期 cron + retry 队列；撤销；PKCS#12 导出 |
| W8 | abuse-detection；admin 面板；监控告警 |

### 15.3 S3（4-6 周）

API + Webhook + GTS + 商业化档 + 团队席位。

---

## 16. 决策清单（D-FC 前缀，待 DECISIONS.md §N 收口）

| ID | 决策 | 选项 | 当前倾向 |
|---|---|---|---|
| **D-FC-01** | S1 是否仅 Let's Encrypt | A) 仅 LE / B) LE + ZSSL 同步 | A（降低集成复杂度） |
| **D-FC-02** | 私钥下载策略 | A) KMS 不可导出 / B) 加密落库可下载 / C) 客户端 CSR | B（产品可用性优先） |
| **D-FC-03** | DNS provider 实现 | A) 自研所有 / B) 全用 lego / C) 混合 | B（社区维护，最快） |
| **D-FC-04** | KMS 选型 | A) 阿里云 KMS / B) AWS KMS / C) 自托管 Vault | A（国内主体）+ B（海外灾备）双路径 S2 完成 |
| **D-FC-05** | 是否做 HTTP-01 challenge | A) 做 / B) 不做（仅 DNS-01） | B（用户不需要在服务器装 agent，所有场景 DNS-01 覆盖足够） |
| **D-FC-06** | 续期失败兜底 | A) 反复 retry 直到过期 / B) 3 次后停 + 强告警 | B（避免雪崩） |
| **D-FC-07** | 域名黑名单维护 | A) 内置静态表 / B) 配置中心动态拉 / C) 第三方 API | B（运营可改不发版） |
| **D-FC-08** | 是否做自动部署 | A) S3 做 / B) 永不做 | B（合规风险 + 维护成本，让用户手动部署） |
| **D-FC-09** | 商业化时点 | A) S2 开始限免费档 / B) S3 / C) 永久全免费 | B（先建生态，S3 商业化） |
| **D-FC-10** | 是否暴露 MCP tool | A) S3 做 / B) 永不 | A（与 19-ai-agent 协同，但优先级低） |
| **D-FC-11** | 付费 CA 渠道首选 | A) DigiCert 直签 / B) Sectigo reseller / C) GoGetSSL / D) 多渠道并行 | C 单渠道试水 → D 长期；详 §20 |
| **D-FC-12** | OV / EV 流程归属 | A) 本模块扩展 / B) 单独 enterprise 模块 | A（订单 / 状态机 / 通知 / 凭据全部复用，独立模块成本太高） |
| **D-FC-13** | 付费证书私钥策略 | A) 同免费策略本地生成 / B) 强制 KMS 不可导出（高端档） | A + B 两档（基础档 A，企业档 B） |

> 推进 MVP 前需先锁 D-FC-01 / -02 / -04 / -05 四项。

---

## 17. 风险与开放问题

### 17.1 已知风险

| 风险 | 缓解 |
|---|---|
| 上游 CA rate limit 集中触发（LE 周配额耗尽） | 多 CA 路由 + 提前告警 + S2 ZSSL 备份 |
| DNS provider API key 失窃 → 用户域名被劫持 | KMS 信封加密 + 凭据使用即时审计 + 用户侧凭据健康度监控 + 一键吊销 |
| 私钥泄露（DB 被脱库） | KMS 隔离 KEK；S1 master key 是已知短板，S2 必须切真 KMS |
| 平台被用于钓鱼站签证书 | abuse-detection + 黑名单 + admin 抽审 |
| 用户大量手动模式订单卡 30 分钟 → worker 资源占满 | 手动模式订单数量上限 + 独立队列 |
| CT 日志暴露用户域名 / 子域 | 文档明确告知，提供"不申请通配符就不暴露所有子域"建议 |

### 17.2 开放问题

- 通配符 + 多 SAN 同证书的边界（如 `*.a.com` + `a.com` + `b.com`），UI 如何引导用户合理拆分
- S1 是否需要"用户自带 CSR" 流程（D-FC-02 选 B 后此选项 S3 才上）
- 是否提供 `acme-dns` 模式（用户把 `_acme-challenge.x.com` CNAME 到我们的子域，平台代管 TXT）？S2 决策
- 退款 / 信用通道复用 09-billing 还是独立？（S3 商业化时再决）
- 是否提供 "证书部署测试"（签完后调 idcd 既有 SSL 检测工具测一遍）？S2 加，复用 02-public-tools 的 SSL 检测 API

---

## 18. 与现有文档的同步清单（实施前 PR）

- [ ] OVERVIEW.md §4 新增"免费证书工具"小节，链接本文件
- [ ] DECISIONS.md §N 新增 D-FC-01 ~ -10 条目（先列后定）
- [ ] 15-data-model.md §4.X 增 `cert.*` schema 完整 DDL
- [ ] STATE-MACHINES.md 增 CertOrder / RenewalJob 状态图
- [ ] 16-api-spec.yaml 增 `/v1/cert/*` 端点
- [ ] ER-DIAGRAM.md 增 cert 实体关系
- [ ] 17-roadmap.md 增 S1/S2/S3 cert 里程碑
- [ ] CLAUDE.md SSOT 表加一行指向本文件
- [ ] DESIGN.md 无需改动（完全沿用 shadcn/ui zinc + OKLCH）

---

## 19. 下一步

1. **PM/技术 lead** 审本文 → 锁定 D-FC-01/-02/-04/-05/-11 → 进 DECISIONS.md
2. 派单：W1 任务可拆 3 个并行 worktree
   - WT-A：DB migration + `apps/cert-svc` HTTP 骨架
   - WT-B：`lib/cert/ca` LE adapter + Pebble 测试（**接口设计必须考虑付费 CA 适配，详 §20.3**）
   - WT-C：`lib/cert/vault` env-master-key 实现 + 单元测试
3. W1 末尾合流 → 进入 W2 ACME 状态机

---

## 20. 付费证书扩展性设计（S3 起接入，但 S1 架构就要兼容）

### 20.1 为什么从一开始就考虑

付费 CA 不是另起一个模块——它和免费走同一套订单 / 状态机 / DNS / KMS / 证书库 / 前端。**S1 不实现，但接口缝必须留对**，否则 S3 推 reseller 时要回头改 `cert.orders` schema、状态机、CA 接口、前端表单字段，工作量翻倍。

### 20.2 共享 vs 新增

| 能力 | 免费（ACME） | 付费（Reseller） | 设计 |
|---|---|---|---|
| 订单状态机 | ✅ | ✅ 大致相同 | 复用 §6 主状态机，新增 `awaiting_org_validation`（仅 OV/EV） |
| 域名所有权验证 (DCV) | ACME challenge | DNS-01 / HTTP-01 / Email | 共用 `lib/cert/dns` 写 TXT；Email DCV 是新增 |
| DNS provider 适配 | ✅ | ✅ | 100% 复用 |
| 私钥 / CSR / KMS | ✅ | ✅ | 100% 复用 |
| 证书存储 / 下载 / 格式 | ✅ | ✅ | 100% 复用 |
| 续期调度 | T-30d | T-30d（397 天证书） | 复用 `cert.renewal_jobs`，仅参数不同 |
| 通知 / 审计 | ✅ | ✅ | 复用 |
| 前端订单列表 | ✅ | ✅ | 复用，加 "tier" 列（free/paid-dv/paid-ov/paid-ev） |
| **CA 协议** | ACME RFC 8555 | Reseller 各家私有 REST | **新增 `ResellerCA` 实现，与 `AcmeCA` 并列同 interface** |
| **组织信息（OV/EV）** | ❌ | 公司名 / 地址 / 电话 / 营业执照 | **新增 `cert.organizations` 表 + 表单 + 上传** |
| **回调验证（OV/EV）** | ❌ | 电话回访 / 邮件回复 | **新增 `awaiting_org_validation` 子状态 + 人工干预接口** |
| **计费 / 退款** | 不涉及 | 聚合支付 / 阿里云 | **复用 09-billing；新增 `cert.orders.billing_invoice_id`** |
| **库存 / 预付费券** | ❌ | reseller 通常按张预付 / 按订单结算 | **新增 `cert.sku_credits`（按渠道按规格预存）** |

### 20.3 `CA` 接口必须从 S1 就这么设计

```go
// lib/cert/ca/ca.go (S1 创建)
type Tier string
const (
    TierFreeDV Tier = "free-dv"
    TierPaidDV Tier = "paid-dv"
    TierPaidOV Tier = "paid-ov"
    TierPaidEV Tier = "paid-ev"
)

type CA interface {
    Name() string
    Tier() Tier
    SupportsWildcard() bool
    ValidityDays() int          // 90 (ACME) | 397 (paid)
    SupportedChallenges() []ChallengeType  // dns-01/http-01/email
}

// 免费分支
type AcmeCA interface {
    CA
    NewOrder(ctx, OrderRequest) (AcmeOrder, error)
    Authorize(ctx, authzURL) (Authz, error)
    NotifyChallenge(ctx, chalURL) error
    PollAuthz(ctx, authzURL) (AuthzStatus, error)
    Finalize(ctx, orderURL, csr) (certURL string, err error)
    Download(ctx, certURL) (leaf, chain []byte, err error)
    Revoke(ctx, cert, reason) error
}

// 付费分支（S3 加，但 S1 接口就在）
type ResellerCA interface {
    CA
    CreateOrder(ctx, ResellerOrderRequest) (orderRef string, dcv DCVInstruction, err error)
    SubmitOrgInfo(ctx, orderRef, OrgInfo) error          // 仅 OV/EV
    PollOrder(ctx, orderRef) (ResellerOrderStatus, error)
    FetchCert(ctx, orderRef) (leaf, chain []byte, err error)
    Revoke(ctx, orderRef, reason) error
}
```

`cert-worker` 在状态机里按订单的 `tier` 字段分发到 `AcmeCA` 或 `ResellerCA` 的执行器，主状态机保持一致。

### 20.4 `cert.orders` 表需要从 S1 就预留的字段

**已合并进 §5.2 初始 DDL**：`tier / validity_days / organization_id / billing_invoice_id / reseller_order_ref / reseller_channel / common_name / sans_unicode`。S3 接 reseller 时只新增 `cert.organizations` 表 + `awaiting_org_validation` 状态，不动 `cert.orders` schema。

### 20.5 状态机扩展（S3）

```
draft → validating (DCV) → [tier∈OV,EV ? awaiting_org_validation → ] issuing → issued
                              ↑ 人工 / 客户上传 / 回访完成
```

`awaiting_org_validation` 是付费独有，免费不会进入这个状态。worker 在该状态下只轮询 reseller，不做 ACME 操作。

### 20.6 商业化与结算（S3）

- 用户下单付费证书 → 走 09-billing 创单 → 支付成功后 `cert.orders.billing_invoice_id` 写入 → worker 开始 reseller 流程
- 退款：支付后未提交 reseller → 全额退；已提交但 CA 未签发 → 调 reseller refund API；已签发 → 不退
- 接口幂等保证：`billing_invoice_id` UNIQUE，防止重复扣款
- SKU 模型：`(channel, tier, validity, wildcard, san_count)` 五元组 → 价格表

### 20.7 风险

- Reseller 渠道 API 不稳定 / 私有协议变更：用多渠道（D-FC-11）路由 + 抽象层
- OV/EV 流程 SLA 难控（依赖客户提供材料 + CA 人工审核，可能拖 1-5 天）：明确 UX 时间预期 + 工单跟踪
- 跨境合规：付费证书涉及增票 / 跨境结算，可能需独立法律实体（与 09-billing 同步）

### 20.8 验收（S1 必须满足，为 S3 铺路）

- [ ] `lib/cert/ca` 接口分为 `CA` 基接口 + `AcmeCA` 子接口
- [ ] `cert.orders` schema 已含 `tier / validity_days / organization_id / billing_invoice_id / reseller_order_ref / reseller_channel`
- [ ] cert-worker 状态机按 `tier` 字段分发执行器（即便 S1 只有一个 `AcmeExecutor`）
- [ ] 前端订单创建表单留 `tier` 字段位置（S1 锁定 `free-dv` 不显示）
- [ ] DECISIONS.md 已为 D-FC-11/-12/-13 留位（具体决策可 S2 末尾再锁）
