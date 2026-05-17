# 14 · 技术架构(v2.0)

> 关联：OVERVIEW.md §5.2 子域、§8、所有模块的技术依赖汇总;DECISIONS.md §K1(三栈 sub-product) / §K2(KMS)
> 关联(v2):18-evidence-and-attestation.md §3、19-ai-agent-observability.md §3
> 阶段：S1 起就需落地，演进式扩展;v2 在 S2 加 attest.idcd.com,S3 加 mcp.idcd.com
> 品牌名占位：`idcd`

---

## 1. 架构原则

1. **务实优先**：不堆技术栈、不追新；用 Go + Next.js + PostgreSQL 这种"成熟到无聊"的组合
2. **单仓多服务**：monorepo 起步，组件解耦但部署集中
3. **可水平扩展**：核心组件无状态，状态都进数据库 / Redis
4. **可观测**：自家产品 dogfood + 标准化日志 / 指标 / Trace
5. **节俭运维**：S1 整体能跑在 3-5 台中等配置 VPS 上
6. **数据独立**：业务库 / 时序库 / 缓存 各司其职
7. **零信任 Agent**：Agent 不在受信网络，所有交互必签名
8. **(v2 NEW) 三栈 sub-product 阵型**:Core / Evidence / MCP 独立子域 + 独立部署 + 独立计量 + 独立 SLA(决策 §K1)
9. **(v2 NEW) 信任根独立**:Verdict 签名密钥架构(KMS + 离线 root)是产品信任根,部署/运维/审计与主业务解耦
10. **(v2 NEW) 主控双 AZ + Agent 本地缓冲**:控制面挂了不影响 Agent 拨测;Agent 本地缓冲 24h 写入,主控恢复后回放

---

## 2. 整体架构图(v2: 三栈 sub-product 阵型)

```
                                Cloudflare（CDN + WAF + DDoS + Turnstile）
                                              │
   ┌──────────────┬──────────────┬───────────┼────────────┬──────────────┬──────────────┐
   │              │              │           │            │              │              │
┌──▼────────┐ ┌───▼────────┐ ┌───▼────────┐ ┌▼──────────┐ ┌▼─────────────┐ ┌▼─────────────┐
│ idcd.com  │ │docs / blog │ │api.idcd.com│ │status.*   │ │attest.idcd   │ │mcp.idcd.com  │
│ Next.js   │ │Next.js SSG │ │API Gateway │ │.idcd.com  │ │.com (v2 NEW) │ │(v2 NEW)      │
│(前端+BFF) │ │(静态/SSG)  │ │(Go)        │ │Next.js SSR│ │Attestation   │ │MCP Server    │
│           │ │             │ │            │ │(用户状态页)│ │API+Worker(Go)│ │(Go/Node)     │
└──┬────────┘ └────────────┘ └───┬────────┘ └─────┬─────┘ └──────┬───────┘ └──────┬───────┘
   │                              │                │              │                │
   └───────────────┬──────────────┴────────────────┴──────────────┴────────────────┘
                   │ HTTP/gRPC (内网, 双 AZ:杭州+法兰克福)
                   ▼
   ┌──────────────────────────────────────────────────────────────────────────┐
   │                            Application Tier (Go)                          │
   │                                                                          │
   │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐ ┌─────────────┐│
   │  │ Account  │ │ Monitor  │ │ Alert    │ │ Status Page  │ │MCP Auth     ││
   │  │ Service  │ │ Service  │ │ Service  │ │ Service      │ │+Dispatcher  ││
   │  │          │ │          │ │          │ │              │ │(v2 NEW)     ││
   │  └──────────┘ └──────────┘ └──────────┘ └──────────────┘ └─────────────┘│
   │                                                                          │
   │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐ ┌─────────────┐│
   │  │ Probe    │ │ Billing  │ │ Report   │ │ Admin        │ │Attestation  ││
   │  │ API      │ │ Service  │ │ Service  │ │ Backend      │ │+Verify(v2)  ││
   │  │          │ │          │ │+LLM 复盘 │ │              │ │             ││
   │  │          │ │          │ │(v2)      │ │              │ │             ││
   │  └──────────┘ └──────────┘ └──────────┘ └──────────────┘ └─────────────┘│
   └────┬───────────┬─────────────┬─────────────┬──────────────┬──────────────┘
        │           │             │             │              │
        ▼           ▼             ▼             ▼              ▼
   ┌──────────┐ ┌──────────┐ ┌────────────┐ ┌──────────┐ ┌────────────────┐
   │Scheduler │ │ Worker   │ │ Notifier   │ │Aggregator│ │Verdict Worker  │
   │  (Go)    │ │ Pool(Go) │ │ (Go)       │ │ (Go)     │ │(v2,签名+TSA)   │
   └────┬─────┘ └────┬─────┘ └─────┬──────┘ └────┬─────┘ └────────┬───────┘
        │            │              │             │                │
        ▼            ▼              ▼             ▼                ▼
   ┌──────────────────────────────────────────────────────────────────────────┐
   │                              Data Tier                                    │
   │  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐                │
   │  │ PostgreSQL  │  │ TimescaleDB  │  │ Redis Cluster    │                │
   │  │ (业务数据)   │  │ (时序数据)    │  │ 队列+缓存+限速    │                │
   │  └─────────────┘  └──────────────┘  └──────────────────┘                │
   │  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐                │
   │  │Object Storage  │ ClickHouse  │  │ Search           │                │
   │  │ R2/OSS/WORM │  │ (S3+ 大流量) │  │ (Meilisearch)    │                │
   │  │(v2:报告归档) │  │              │  │                  │                │
   │  └─────────────┘  └──────────────┘  └──────────────────┘                │
   └──────────────────────────────┬───────────────────────────────────────────┘
                                  │
              ┌───────────────────┼─────────────────────────┐
              │                   │                         │
              ▼                   ▼                         ▼
   ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────────┐
   │  KMS (v2 NEW)    │ │  TSA Client(v2)  │ │  LLM Provider(v2)    │
   │ AWS KMS/阿里KMS  │ │RFC3161 主备三家  │ │ OpenAI/Anthropic/    │
   │ + Shamir Root    │ │DigiCert/GlobalSign│ │ 自家(复盘+解读)      │
   │ 离线(3-of-5)     │ │NTSC(国内,S3)     │ │                      │
   └──────────────────┘ └──────────────────┘ └──────────────────────┘
                                  │
                                  ▼
   ┌──────────────────────────────────────────────────────────────────────────┐
   │              Agent Gateway (WSS + mTLS, 7d cert rotation)                │
   │              + CRL/OCSP 主动撤销路径(v2 NEW)                              │
   └──────────────────────────────────────────────────────────────────────────┘
                                  │
              ┌───────────────────┼─────────────────────────┐
              ▼                   ▼                         ▼
   ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────────┐
   │  Agent x N       │ │  Agent x N       │ │  Community Agent     │
   │  (Tier1 自有,国内)│ │  (Tier1 自有,海外)│ │  (T1/T2/T3 众包,S3)  │
   │  + Anchor 节点    │ │                  │ │                      │
   │  + 24h 本地缓冲   │ │                  │ │  + 反作弊指纹         │
   │  + OTA 3 级灰度   │ │                  │ │                      │
   └──────────────────┘ └──────────────────┘ └──────────────────────┘
```

> **关键约束**:KMS / TSA / LLM 三个外部信任依赖必须有主备容灾;任一长时间不可用要有降级路径(详 §13)。

---

## 3. 技术栈选型

### 3.1 后端(版本锁定 2026-05-13)

> **版本原则**:全部用 2026-05 最新稳定版,避免反工。
> **Go 1.26** / **Next.js 16** / **PostgreSQL 18.3**(云端已用)/ TimescaleDB 2.21+(详 §6.2a 兼容性风险)/ Redis 7.4+ / Tailwind CSS v4 / shadcn/ui latest。

| 组件 | 选型 | 理由 |
|---|---|---|
| 主语言 | **Go 1.26** | 静态二进制、并发模型、社区成熟、节点 Agent 同栈 |
| 微框架 | `chi` 或 `gin` | 轻量、生态好 |
| RPC | gRPC（内部） / REST（对外） | 双协议，内部高效，对外友好 |
| ORM | `sqlc` + `pgx` | 类型安全、性能、避免 ORM 黑盒 |
| 队列 | `river` (Postgres-based) / `asynq` (Redis) | river 业务队列，asynq 高频任务 |
| 任务调度 | 自实现 + Redis Streams | 拨测调度高频高时效 |
| 缓存 | Redis + 进程内 LRU | 双层 |
| 验证 | `go-playground/validator` | 标准 |

### 3.2 前端

| 组件 | 选型 | 理由 |
|---|---|---|
| 框架 | **Next.js 16 (App Router)** | SSR/SSG/CSR 灵活、SEO 友好;React 19 RSC + Turbopack 默认 |
| UI 库 | shadcn/ui + Radix + Tailwind v4 | 工程化、可定制 |
| 状态 | Zustand + TanStack Query | 轻 |
| 图表 | **ECharts** + Recharts 备选 | ECharts 数据量友好；Recharts 简洁场景 |
| 表单 | React Hook Form + zod | 标准 |
| 国际化 | next-intl | App Router 友好 |
| 富文本 | Tiptap | 复盘 / 文档编辑 |
| Markdown | MDX | 文档站 |
| 类型 | TypeScript（strict） | 标配 |
| 包管理 | pnpm | 快 + 节省空间 |

### 3.3 数据存储(版本锁定 2026-05-13)

| 用途 | 选型 |
|---|---|
| 业务关系数据 | **PostgreSQL 18.3**(用户云端已用此版本,锁定) |
| 时序数据（拨测结果、聚合） | TimescaleDB(需 PG 18 兼容版本,详 §6.2a 风险)→ S3+ 大流量切 ClickHouse |
| 缓存 / 限速 / 队列 | Redis 7 Cluster |
| 对象存储（报告 PDF / 导出 / 备份） | Cloudflare R2（首选）/ 阿里云 OSS（备用国内） |
| 全文搜索 | Meilisearch（轻量易部署，中文友好）|
| KMS（密钥） | HashiCorp Vault 或云厂商 KMS |

### 3.4 基础设施

| 用途 | 选型 |
|---|---|
| 主控集群 | Hetzner CCX 系列（欧洲）+ 国内 BGP VPS（杭州/北京）双区 |
| 节点机 | 多家 VPS 分散：Hetzner / Vultr / RackNerd / DMIT / BWG / Akari ... |
| 容器化 | Docker + docker compose（S1 极简）→ K3s 或 Nomad（S3+，按需） |
| 配置管理 | Ansible + Terraform（节点 IaC） |
| CDN/WAF/DDoS | Cloudflare 全套 |
| 邮件 | 自建 SMTP（Postfix + DKIM）+ 备用 SES |
| 域名 / DNS | Cloudflare DNS（境外）+ DNSPod（国内）|

### 3.5 DevOps / 可观测性

| 用途 | 选型 |
|---|---|
| CI/CD | GitHub Actions + 自托管 runner（部署时） |
| 镜像仓库 | GHCR / 阿里云 ACR |
| 日志 | Loki + Promtail |
| 指标 | Prometheus + Grafana |
| Trace | OpenTelemetry + Tempo |
| 错误追踪 | Sentry（自托管）|
| 站点监控 | 自家产品 dogfood + UptimeRobot 兜底 |
| 告警 | 自家产品 + 钉钉 / 邮件 |

---

## 4. 核心组件设计

### 4.1 API Gateway（Go）

**职责**：所有公开 API + 控制台 API 的统一入口。

- 请求路由（按路径分发到具体 service）
- 鉴权（API Key / Session / Webhook 签名）
- 限速（多维度，Redis 实现）
- 黑名单匹配
- 请求 ID 注入
- 调用计量（写 usage_event）
- 错误统一格式
- CORS / 安全头

**性能目标**：单实例 5000 RPS，水平扩展。

### 4.2 Scheduler（调度器）

**职责**：任务调度的大脑。

- 接收新任务（监控触发 / API / 公开工具 / 内部健康检查）
- 节点筛选 + 打分 + 选 N（详见 10 §5.2）
- 任务签名后 push 给 Agent
- 处理 ack / 进度 / 完成
- 超时重试（路由到候选节点）
- 优先级队列管理（P0-P5）
- 错峰偏移

**部署**：
- 主备模式（leader election via Redis / etcd）
- 单 leader 实例 + N 备用
- S3 起按地区分片：CN-Scheduler / Global-Scheduler

### 4.3 Worker Pool

**职责**：后台异步任务执行。

- 大批量计算（聚合、报告生成、PDF 渲染）
- 数据导出
- 邮件 / 短信 / Webhook 发送
- 蜜罐生成
- 数据清理 / 归档

**实现**：
- river（Postgres）+ asynq（Redis）双队列
- river 用于业务异步任务（高可靠、低频）
- asynq 用于通知发送（高频、可丢失）
- 失败重试 + 死信队列

### 4.4 Aggregator（聚合器）

**职责**：拨测结果聚合 + 异常判定 + 触发事件。

- 监听 probe_result 流
- 按 probe_task 收集结果（quorum / consecutive 逻辑）
- 计算 summary
- 更新 monitor 状态
- 触发 alert_event 创建
- 写时序聚合（hour/day）

**部署**：
- 多实例消费 Redis Streams
- 幂等设计（同 task_id 重复处理无副作用）

### 4.5 Notifier（通知派发）

**职责**：所有告警通道的统一派发。

- 接收 alert_event
- 应用 alert_policy（rule 匹配、suppression、escalation）
- 模板渲染
- 调用通道 adapter（10+ 通道）
- 重试 + 死信
- 写 alert_notification 记录

**通道 Adapter**：
- 实现统一接口 `Send(payload) -> Result`
- 每个通道独立 Go module
- 通道故障不影响其他通道

### 4.6 Agent Gateway

**职责**：管理与所有 Agent 的长连接。

- WSS 接入（mTLS）
- 心跳处理
- 任务下发（来自 Scheduler）
- 结果上报（推给 Aggregator）
- 控制消息（drain / upgrade / config）

**部署**：
- 多区部署（中美欧三地）
- 节点就近接入
- 单 gateway 实例承载 5000-10000 Agent
- 横向扩展（节点 → gateway 一致性哈希）

### 4.7 Application Services

按业务域拆 Service（同进程 module 或独立服务，S1 同进程，S3+ 按热点拆分）：

- Account Service：用户 / 团队 / API Key
- Monitor Service：监控 CRUD / 状态机
- Alert Service：策略 / 事件 / 通道
- StatusPage Service：状态页 / 事件 / 订阅
- Probe API：公开工具的 API 实现
- Billing Service：订阅 / 计费 / 支付
- Report Service：报表 / 仪表盘 / SLA
- Admin Backend：管理后台 API

### 4.8 Frontend（Next.js）

#### 主站 idcd.com
- App Router + Server Components
- 工具页 SSG（构建时生成，CDN 缓存）
- 控制台 CSR（登录后）
- BFF：少量服务端代码做聚合 / 鉴权代理

#### 状态页（多租户）
- 一套代码，通过 host + slug 路由到不同租户
- ISR（Incremental Static Regeneration）+ CDN 缓存
- 自定义域名通过 SNI 切换证书

#### 文档站
- Mintlify 或 自建 Nextra
- 纯 SSG

### 4.9 Attestation Service(v2 NEW, 详 18 §3 + D6 Self-Verify 独立)

**职责**:Verdict 报告生成 + 签名 + 时间戳 + 归档 + 公开验签

**关键组件**:
- **API**(`attest.idcd.com`):订单/状态/PDF 下载/分享 token
- **Verdict Generator Worker**(后台异步):多节点交叉验证 + LLM 解读 + PDF 渲染 + KMS 签名 + TSA 时间戳 + S3 归档
  - WAL 化:每 step 完成写 `attestation_record(action, status=success, external_id, idempotency_key)`(详 18 §3.2)
  - KMS sign 调用启用 idempotency token,防止 worker crash 后重试导致重复 sign
- **Self-Verify Worker(v2 D6 独立部署)**:每份报告生成后自检 + 每日抽样 10 份历史报告独立验签
  - **不同进程**:独立 docker container,不与 Generator Worker 共享进程
  - **不同 VPC subnet**:独立 subnet,与 Generator Worker 间仅暴露 attest.idcd.com/verify HTTPS 接口,无内部 RPC
  - **独立 KMS 客户端实例**:不复用 Generator Worker 的 KMS sign 客户端 / 配置 / 缓存
  - 仅调用公开 verify 接口走与外部第三方一致的代码路径
- **Refund Worker(v2 D5)**:失败后 聚合支付 refund retry queue(5min → 30min) + 30min 强制道歉邮箱 + refund_failed 状态入 admin dashboard
- **Public Verify**(`attest.idcd.com/verify`):任意第三方上传 PDF 验签的轻量服务,与主 Worker 解耦;**revoke 期间仍持续可用**(已发报告仍可被验签,详 18 §7.1)

**部署**:
- 独立子域、独立 Docker compose 栈、独立监控
- API stateless,可水平扩展;Generator Worker 单实例起步(M7),S3+ Worker 池
- **Self-Verify Worker 独立 docker compose service** + 独立监控告警 + 独立资源池
- 归档 S3/WORM(对象存储)只增不删,6 年合规

### 4.10 MCP Server(v2 NEW, 详 19 §3 + D13 stateless protocol + stateful connection)

**职责**:把 idcd 所有拨测/监控/Verdict 能力包为 MCP 协议接口,Agent 端可直接 import

**关键组件**:
- **MCP Auth Service**:短期 token 签发 + 撤销 + IP 白名单校验(v2 D2:所有 token 最长 90d 自动 renewal,无永久)
- **MCP Tool Dispatcher**:按 tool 路由到内部 Probe / Monitor / Verdict / Account services 的 RPC
- **MCP Protocol Adapter**:Anthropic MCP spec(JSON-RPC over stdio + HTTP+SSE);S3+ 视情况加 OpenAI Agents Protocol
- **MCP Metering**:每个 tool call 独立计量(v2 D2:与 REST API 配额池**完全独立**,不混合);Free/Pro/Team/Business 都有 MCP units 额度,Agent Pro 是 MCP units 加大独立 SKU

**部署(v2 D13 状态边界明确)**:
- 独立子域 `mcp.idcd.com`
- **业务逻辑 stateless**:可水平扩展,业务状态在 PG + Redis,任一实例可处理任一请求
- **SSE 连接 stateful**:`/v1/agent-obs/events` 是 long-lived SSE 连接
  - **LB sticky session 必要**(基于 token_id 哈希粘性,Cloudflare Load Balancer 支持)
  - heartbeat 机制:30s 内无数据 → 发 keepalive event,断开重连由客户端负责
  - 单实例承载估算:**10k 并发 SSE 连接**(参考 §9.1 Agent Gateway 同档)
  - 横向扩展:多实例 + 一致性哈希(token_id → instance)+ Redis Streams 跨实例事件广播
- 鉴权数据 / 计量数据 进 PostgreSQL + Redis
- 客户端兼容矩阵:Cursor / Claude Code / Codex / 自家 SDK,每发布前 smoke test

### 4.11 KMS / TSA Client(v2 NEW)

**职责**:Verdict 信任根的密钥操作 + 时间戳

**KMS 抽象层(v2 D4 增强:idempotency token)**:
- 接口统一:`Sign(key_id, hash, idempotency_key) -> signature` / `GetPublicKey(key_id, version) -> pem`
- **idempotency token 必传**(v2 D4):防止 worker 重试导致 KMS audit log 重复 sign;AWS KMS / 阿里云 KMS 均支持
- 后端可插拔:AWS KMS / 阿里云 KMS / 自建 Vault / HSM(S4)/ Backup HSM(v2 D11 加速通道,详 18 §3.3)
- 每次调用全审计(key_id + key_version + caller + report_id + idempotency_key)
- Key 版本管理:90 天 sign key 轮换;过期密钥保留只读用于历史验签;revoke 期间 verify 接口仍可用

**LLM Provider 抽象层(v2 D9 + B0a 锁定)**:
- 接口统一:`Generate(prompt_template_id, variables, provider) -> {text, model, prompt_version}`
- **prompt 不保证跨 Provider 一致**(D9 锁定):同一 prompt template 在不同 Provider 输出风格 / schema / 鲁棒性不同
- **baseline = 阿里通义(qwen-max) + DeepSeek**(B0a 锁定):
  - 主选阿里通义(国内主控同地,latency 低,合规);备选 DeepSeek(成本极低)
  - 月成本估算:S2 上线初 ¥300-500/月 vs Claude/GPT $500/月
  - per-Provider 独立 prompt + 独立 eval ≥4.0/5
- **failover 触发条件(D20-关联,新增)**:阿里通义 → DeepSeek 切换触发于:
  - 连续 3 次请求超时(超时阈值 30s)或返回 5xx,且退步后仍失败 → 自动切 DeepSeek
  - DeepSeek 同样失败 → Verdict 报告 LLM 解读步骤跳过(降级输出"LLM 解读暂不可用")+ P1 告警
  - 每次切换写 audit_log(provider, trigger_reason, timestamp)
- 企业用户接入自家 LLM 时:可选 Claude / GPT / 自部署;需自行 prompt 调优 + eval
- 后端可插拔:**阿里通义 / DeepSeek**(主)/ Anthropic Claude / OpenAI GPT / 用户自家(企业版)

**TSA Client**:
- 抽象 RFC3161 协议
- 主备三家:DigiCert / GlobalSign / NTSC(S3 起)
- 主失败 → 5 秒切备 → 备失败 → 切第三 → 全失败 P0 告警 + 报告生成暂停
- 每次调用写 attestation_record(action=tsa_stamped, external_id=tsa_serial)作为 WAL

### 4.12 Agent OTA 3 级灰度(v2 增强, 详 10 模块)

**职责**:Agent 二进制升级,失败自动回滚

**灰度阶段**:
- **L1 (1%)**:随机 1-2 个节点,观察 1 小时
- **L2 (10%)**:扩到 10 节点,观察 4 小时
- **L3 (100%)**:全量推送
- 任何阶段错误率突增 > 基线 2x → 自动回滚 + P1 告警 + 暂停后续灰度

**Kill switch(新增)**:
- `feature_flag.agent_ota_enabled = false` → 立即停止所有灰度推进(不影响已升级节点)
- `feature_flag.agent_ota_force_rollback_version = <version>` → 强制回退全部节点到指定版本(触发全量 L1→L3 回滚)
- Kill switch 操作写 audit_log + P1 告警;详 `docs/RUNBOOKS/agent-mass-rollback.md`

---

## 5. 数据架构

### 5.1 PostgreSQL（业务库）

**用途**：账号、订阅、监控配置、节点元数据、告警事件、状态页配置等

**特性**：
- TimescaleDB 扩展（时序表）
- pg_trgm（模糊搜索）
- pgcrypto（密钥相关）
- 主从复制（一主一从备）
- 自动备份（每日全量 + 每小时增量，异地）

### 5.2 TimescaleDB（时序）

**用途**：
- `monitor_check`（每分钟数百万行）
- `probe_result`（高频，但 90 天后归档）
- `usage_event`（API 调用计量）
- `node_heartbeat`

**优化**：
- Hypertable 自动分区（按 day）
- 自动连续聚合（hour → day）
- 旧分区压缩（10x 节省）
- 超过保留期自动 drop

### 5.3 Redis 用途分层

| Database | 用途 |
|---|---|
| db0 | Session / Auth Cache |
| db1 | 限速（多维度滑动窗口）|
| db2 | 队列（asynq）|
| db3 | 任务调度状态 + Streams |
| db4 | 业务缓存（节点列表、IP 信息 24h）|
| db5 | 实时统计（在线用户、当前 RPS）|

**部署**：S1 单实例 + 持久化，S3 起 Redis Cluster。

**Redis Streams MAXLEN 策略(D18)**:所有 Stream 写入时加 `XADD ... MAXLEN ~ 500000`（近似裁剪，性能友好）。防止 Aggregator / Notifier 停机期间 probe 结果无限堆积导致 OOM。超出后丢弃最旧消息：已平稳消费时不丢数据；维护停机期间接受旧数据丢弃以保护 Redis 内存。

### 5.4 对象存储

- 报告 PDF
- 用户数据导出 ZIP
- 数据库备份
- 节点 Agent 二进制 + 升级包
- 静态资源（用户头像、Logo 等）

### 5.5 ClickHouse（远期 S3+）

仅在 `probe_result` 写入超过 100k RPS 时切换。届时方案：
- 时序原始数据写 ClickHouse
- PostgreSQL 仅保留 monitor 配置 / 事件 / 聚合
- 通过 CDC 同步关键事件

---

## 6. 关键技术决策

### 6.1 为什么 Go 而不是 Node/Rust/Java

- **Go**：静态二进制（Agent 部署简单）、并发模型适合调度、生态成熟、招人不难
- Node：JS 主导前端，后端 BFF 用 Next.js 即可，重业务上 Go
- Rust：性能更好但开发慢，对调度类业务收益不显著
- Java：JVM 重，不适合 Agent

### 6.2a TimescaleDB 与 PostgreSQL 18.3 兼容性(v2 用户决策风险)

> **风险标记**:用户决策"PostgreSQL 18.3"(2025-09 GA,云端已用)。TimescaleDB 官方支持 PG 18 的版本是 **TimescaleDB 2.21+**(2025-11 起);如果云端 PG 18.3 + TimescaleDB 2.21+ 已可用,则无障碍。
> 
> **M1 启动前必验**:在 staging 环境 `CREATE EXTENSION timescaledb` 跑通 + 创建一个 hypertable;若失败,降级路径:
> - 选项 1:回退云端 PG 到 17.x(TimescaleDB 长期 LTS 支持)
> - 选项 2:暂不用 TimescaleDB,monitor_check 用普通分区表(`PARTITION BY RANGE (created_at)`)+ Go 应用层做 chunk 管理 — 牺牲连续聚合便利,但 PG 18 原生分区性能够用
> - 选项 3:直接上 ClickHouse(原本 S3 才切,提前到 S1)
> 
> **17-roadmap M1 增加里程碑**:M1 验证 TimescaleDB 兼容性。

### 6.2 为什么 PostgreSQL + TimescaleDB 而非分裂 ClickHouse(v2 D14 触发指标明确)

- 一套运维（PG 扩展），数据库人才好招
- TimescaleDB 在 10 万节点级别（每秒数万写）足够撑到 S3 中期
- 一旦真到瓶颈再切 ClickHouse，业务库不动

**CK 切换触发指标(v2 D14 锁定)**:

| 指标 | 阈值 | 行动 |
|---|---|---|
| 单日 monitor_check 新增数据量 | > 10 GB(持续 1 周) | 启动 CK 调研 + 7 天内准备 PoC |
| P99 write latency(TimescaleDB) | > 100 ms(持续 1 周) | 启动 CK 调研 + 7 天内准备 PoC |
| 两项均达到 | — | 启动 CK 部署(time 路径写 CK,业务库不动) |

**S3 末必备**:CK 评估报告 + PoC 代码就绪(不能等"到点出事才决")。S3 GA 后任一指标达到 → 启动评估;两项都到 → 启动部署。详 17-roadmap S3 末新增里程碑。

### 6.3 为什么不上 K8s

- S1-S2 体量太小，K8s 运维成本 >> 收益
- docker compose + Ansible 已经能管理 100+ 节点 / 5 个服务
- S3 起按需上 K3s / Nomad，仍不上完整 K8s

### 6.4 为什么不微服务拆解到底

- 早期同进程 module 化（package 边界清晰）
- 真出现部署 / 性能瓶颈再拆
- 避免分布式事务、服务发现、链路追踪复杂度

### 6.5 为什么 Cloudflare 而不是阿里云高防

- CF 全球节点 + 免费层强（足够初创用）
- 阿里云高防贵（万元起）
- 但**国内备案站建议加一层阿里云 CDN**（CF 在大陆性能波动）→ 双 CDN 架构（CF + 阿里）

---

## 7. 部署架构（S1 起步版）

```
        ┌─────────────────────────────────────────────┐
        │             Cloudflare（边缘）              │
        └─────────────────────────────────────────────┘
                            │
        ┌───────────────────┴────────────────────┐
        ▼                                        ▼
┌──────────────────┐                  ┌──────────────────┐
│  控制集群（杭州） │                  │  控制集群（法兰克福）│
│  ┌────────────┐  │                  │  ┌────────────┐  │
│  │ App Server │  │                  │  │ App Server │  │
│  │ (Go)       │  │  ── 跨区复制 ──  │  │ (Go)       │  │
│  ├────────────┤  │                  │  ├────────────┤  │
│  │ PG (主)    │  │                  │  │ PG (热备)  │  │
│  │ Redis      │  │                  │  │ Redis      │  │
│  │ Web (Next) │  │                  │  │ Web (Next) │  │
│  └────────────┘  │                  │  └────────────┘  │
└──────────────────┘                  └──────────────────┘
        │                                        │
        ├────────────────┬───────────────────────┤
        ▼                ▼                       ▼
┌─────────────┐  ┌─────────────┐         ┌─────────────┐
│ Agent x N   │  │ Agent x N   │   ...   │ Agent x N   │
│ (国内多线)   │  │ (海外多区)   │         │ (众包)       │
└─────────────┘  └─────────────┘         └─────────────┘
```

### S1 主控配置

- 杭州主：阿里云 ECS **8C/16G** + ESSD 200GB（~¥450/月，D19：从 4C/8G 升级，全栈估算 4.5-6GB+overhead，8G 过紧）
- 法兰克福备：Hetzner CCX13 2C/4G/80GB（€10/月，轻量热备用途）
- 数据：主 → 备 流式复制（streaming replication）
- 国内 / 海外用户就近接入

### S2 后扩展

- 应用层水平扩展（多实例 + 负载均衡）
- Redis 升 Cluster（3 主 3 从）
- 增加专用调度节点（Scheduler 独立部署）
- 引入 read replica（报表查询走从库）

### S3+ 扩展

- 多区主控（亚太 / 北美 / 欧洲）
- ClickHouse 接入
- K3s / Nomad 容器编排
- 按业务域服务拆分（Probe / Monitor / Notifier 独立部署）

---

## 8. 网络架构

### 8.1 域名 / 子域分布

| 域名 | 用途 | 部署 |
|---|---|---|
| idcd.com | 主站 | Next.js 自建 + Cloudflare |
| api.idcd.com | API | Go API Gateway |
| docs.idcd.com | 文档站 | SSG + CDN(Nextra / Fumadocs) |
| status.idcd.com | 我们自己的状态页 | 自家产品 |
| *.status.idcd.com | 用户状态页 | Next.js 多租户 |
| agent-wss.idcd.com | Agent 长连接入口 | Go Gateway (mTLS) |
| admin.idcd.com | 管理后台 | VPN/堡垒机才可访问 |
| get.idcd.com | Agent 安装脚本 | 纯静态 |
| **attest.idcd.com** (v2 NEW) | Attestation API + 公开验签 + transparency | 独立 Docker compose + S3 归档;Self-Verify Worker 独立 service(D6) |
| **mcp.idcd.com** (v2 NEW) | MCP Server(给 Agent) | Go/Node 独立部署 + JSON-RPC stdio + HTTP+SSE + LB sticky session(D13) |
| **docs.mcp.idcd.com** (v2 NEW, D3) | MCP 文档站(给 Agent 开发者) | Nextra SSG,Cloudflare Pages;`mcp.idcd.com/docs` 走 302 redirect |

### 8.2 内网与防护

- 所有服务监听内网 IP（不公网）
- Cloudflare Tunnel / 自建 WireGuard 做内网穿透
- 数据库 / Redis 仅内网访问
- 仅 Frontend / Gateway 暴露 443

### 8.3 TLS 策略

- 用户域：Cloudflare 终止（Full Strict 模式）
- 用户自定义状态页域：ACME（Let's Encrypt + ZeroSSL 备用）
- Agent ↔ Gateway：mTLS（内部 CA）
- **v2 NEW**:Agent 客户端证书短期 7-30 天轮转(从原长期证书改为短期),自动 renewal
- **v2 NEW**:CRL/OCSP 主动撤销 — 任何节点失窃 → 立即吊销证书 + Gateway 拒绝连接 + 节点池剔除 → 1 小时内完成
- **v2 NEW**:mcp.idcd.com / attest.idcd.com 走与主域同泛域名 cert(Cloudflare 管理),但内部独立部署

---

## 9. 并发与性能

### 9.1 性能基准（S1 单实例）

| 接口 | 目标 | 备注 |
|---|---|---|
| 静态工具页 (CDN) | 10k+ RPS | CF 直接命中 |
| 公开 API（缓存命中） | 5000 RPS | Gateway 内存 / Redis |
| 公开拨测 API（同步） | 200 RPS | 受节点限制 |
| 监控状态查询 | 2000 RPS | DB 走 read replica |
| 节点 ↔ Gateway WSS | 10k 连接 / 实例 | |
| **MCP SSE 连接 (v2 D13)** | **10k 并发 SSE / 实例** | 同 Agent Gateway 容量;Pro 1000 活跃 × 10 concurrent = 10k;LB sticky session 必要;一致性哈希(token_id → instance) |

### 9.2 容量规划

S2 末（5000 监控用户）预估：

- 监控总数：~50,000，平均频率 60s
- 监控拨测 RPS：~830
- 节点拨测 RPS：~2500（3 节点/任务）
- 时序写入：~10,000 行/秒
- 告警事件：~1000/天
- API 调用：~500 RPS（峰值）

### 9.3 扩容指南

| 瓶颈 | 解决 |
|---|---|
| Gateway CPU | 多实例 + LB |
| Postgres 写入 | 分表 / 时序数据下沉 TimescaleDB |
| Redis 内存 | Cluster |
| Scheduler 单点 | 分片（按 monitor_id hash） |
| Agent Gateway | 多区 + 一致性哈希 |

---

## 10. 异步任务与事件流

### 10.1 关键异步任务

| 任务 | 队列 | 频率 |
|---|---|---|
| 监控定时拨测 | Scheduler 直接调度 | 高频持续 |
| 告警通知发送 | asynq | 突发 |
| 报告 PDF 生成 | river | 低频 |
| 数据导出 | river | 低频 |
| 邮件发送 | asynq | 中频 |
| 每日 SLA 计算 | cron | 每日 |
| 时序数据压缩 | cron | 每日 |
| 节点健康打分 | cron | 每日 |
| ICP 备案抓取 | cron | 每周 |
| GitHub Token 扫描 | cron | 每小时 |

### 10.2 事件总线

不引入 Kafka 等重型，使用 Redis Streams：

```
streams:
  probe.results       → Aggregator
  monitor.events      → Notifier + StatusPage
  alert.events        → Notifier + Audit
  billing.events      → Billing + Email
```

S3+ 业务复杂时可升级到 NATS / NSQ。

---

## 11. 安全架构

### 11.1 边界
- Cloudflare WAF 规则
- 国家级 IP 屏蔽（按需）
- Bot Score
- Turnstile

### 11.2 应用层
- 所有 endpoint 经过 Gateway（统一鉴权 / 限速 / 黑名单）
- SSRF 防护（解析→白名单→连接）
- 输入验证 + 输出转义
- CSP / Same-Site Cookie / CSRF Token

### 11.3 数据层
- PG / Redis 仅内网
- 静态加密（磁盘 + 备份）
- 密钥 KMS 管理
- 数据库 query 审计（敏感表）

### 11.4 Agent 层
- mTLS
- 任务签名校验
- 硬编码任务白名单
- 上报水印
- **v2 NEW**:短期客户端证书(7-30 天)+ 自动 renewal
- **v2 NEW**:CRL/OCSP 主动撤销路径,失窃节点 1 小时内完全踢出
- **v2 NEW**:Anchor 偏差实时检测 → 偏差节点结果不计入 Verdict 报告

### 11.5 (v2 NEW) Verdict 信任根层

- KMS:云 KMS(AWS/阿里云)起步,HSM S4 可升
- Root key:Shamir 3-of-5 离线 quorum,任何在线密钥失窃**不影响 root**
- Sign key:90 天轮换,过期密钥保留只读用于历史验签
- 调用全审计(key_id + key_version + caller + report_id 写 PG)
- 应急撤销 SOP:怀疑泄露 → revoke + rotate + 通知所有历史报告持有者验签自检 + transparency 公开

### 11.6 (v2 NEW) MCP 凭证安全

- 短期 token(1h-90d),不签发永久 token
- token hash 存储,前端展示一次后只显示后 4 位
- Service account token 强制 IP 白名单
- 异常突增告警(可能被滥用)
- 用户控制台一键撤销 + 操作审计

详见 12-compliance-and-abuse.md(v2 新增 §11 权威测评白名单 / §12 KMS 应急 SOP)。

---

## 12. 可观测性（Dogfood）

### 12.1 自家产品监控自家
- 主站 / API / Gateway 都加监控
- Status Page 公开
- 告警走自家通道

### 12.2 三大支柱

| 类型 | 工具 | 用途 |
|---|---|---|
| Metrics | Prometheus + Grafana | RPS / 延迟 / 错误率 / 资源 |
| Logs | Loki + Promtail | 业务日志 / 审计 |
| Traces | OpenTelemetry + Tempo | 端到端追踪 |

### 12.3 SLI / SLO

| 服务 | SLI | SLO |
|---|---|---|
| API Gateway | 可用性 / P95 延迟 | 99.9% / 200ms |
| 公开工具页 | 可用性 | 99.95% |
| 监控调度 | 任务延迟 P95 | 5s |
| 告警送达 | 30s 内送达率 | 99% |
| 数据库主库 | 可用性 | 99.95% |

错误预算：超出预算冻结发布。

---

## 13. CI/CD

### 13.1 仓库结构

```
idcd/
├── apps/
│   ├── web/           Next.js 前端
│   ├── api/           Go API
│   ├── scheduler/     Go 调度
│   ├── notifier/      Go 通知
│   ├── aggregator/    Go 聚合
│   ├── gateway/       Go Agent Gateway
│   └── agent/         Go Agent
├── packages/
│   ├── db/            sqlc 生成的 Go 代码
│   ├── api-spec/      OpenAPI yaml
│   ├── ui/            shadcn 组件
│   └── sdk-js/        JS SDK
├── infra/
│   ├── terraform/     IaC 节点 / VPS
│   ├── ansible/       部署 playbook
│   └── docker/        compose 文件
└── docs/              本文档
```

### 13.2 流程

```
开发 → PR → CI（lint / test / build / 安全扫描）
  → review + approve
  → merge → 自动构建镜像 → 推 GHCR
  → 触发部署（staging）
  → smoke 测试 → 手动确认（或自动 if 通过）
  → 部署 prod（蓝绿 / 金丝雀）
  → 自动验证关键 SLI
  → 失败自动回滚
```

### 13.3 Agent 发布(v2 增强:3 级灰度)

- 独立的 release 流程
- **v2 修订: 灰度三级 → L1 (1%) 观察 1h → L2 (10%) 观察 4h → L3 (100%)**
- 任何阶段错误率突增 > 基线 2x → 自动回滚 + P1 告警 + 暂停后续灰度
- 失败自动回滚（节点保留前版本）
- 详见 10 §8.3

### 13.4 (v2 NEW) Attestation / MCP 发布

- 独立 release pipeline,不与 Core service 耦合
- Attestation 发布前必须验证 KMS sign + TSA stamp + Self-verify 全链路
- MCP server 发布前必须跑 Cursor / Claude Code 兼容性 smoke test
- Verdict 签名密钥首次生产部署需走"密钥仪式" SOP(详 18 §3.3)

---

## 14. 备份与容灾

### 14.1 备份策略

| 数据 | 策略 |
|---|---|
| PostgreSQL | 每日全量 + WAL 每 5 分钟 + 异地 R2/OSS |
| Redis | 每日 RDB + AOF | （非关键，重启可重建） |
| 对象存储 | 跨区域冗余（R2 默认） |
| 配置 / 密钥 | KMS + 离线副本（保险柜物理） |

恢复演练：每季度一次。

### 14.2 故障级别

| 级别 | 场景 | RTO | RPO |
|---|---|---|---|
| L1 | 单实例挂 | 自动切换 | < 1s |
| L2 | 单可用区挂 | 跨可用区切换 | < 30s |
| L3 | 主区域挂 | 跨区切换（半自动） | < 5 min |
| L4 | 数据库灾难 | 异地恢复 | < 5 min |

---

## 15. 配置管理

### 15.1 环境层级
- local（开发机）
- staging（预生产）
- prod（生产）

### 15.2 配置项分类
- 环境差异（DB URL / API Key）→ env vars + Vault
- 业务参数（限速阈值 / 配额）→ 后台运营配置
- 功能开关 → Feature Flag 服务

### 15.3 秘密管理
- 不入 git
- KMS / Vault 集中托管
- 应用通过 IAM 临时凭证获取

---

## 16. 与各模块的技术依赖

| 模块 | 主要技术依赖 |
|---|---|
| 02 公开工具 | Next.js SSG + Scheduler + Probe API |
| 03 账号 | Account Service + Redis Session + **MCP token store(v2)** |
| 04 监控 | Monitor Service + Scheduler + TimescaleDB + **Agent obs M21-M23(v2)** |
| 05 告警 | Alert Service + Notifier + asynq |
| 06 状态页 | StatusPage Service + Next.js 多租户 + ACME + **LLM 起草 Worker(v2)** |
| 07 报表 | Report Service + TimescaleDB 聚合 + PDF Worker + **LLM 复盘(v2)** |
| 08 API | Gateway + 所有 Service |
| 09 计费 | Billing Service + 支付 SDK + KMS + **Verdict / Compliance 独立计量(v2)** |
| 10 节点 | Agent Gateway + Scheduler + mTLS CA + **CRL/OCSP 撤销(v2) + Anchor 偏差告警(v2)** |
| 11 后台 | Admin Backend + 内网 + Audit Log + **KMS 仪式后台 + Verdict 工单(v2)** |
| 12 合规 | Gateway 限速 + CF + Audit + **权威测评白名单 + KMS 应急 SOP(v2)** |
| 13 SEO | Next.js SSG + sitemap + **/leaderboard 生成器(v2)** |
| **18 Evidence (v2 NEW)** | attest.idcd.com 独立部署 + KMS + TSA Client + S3/WORM 归档 |
| **19 AI/MCP (v2 NEW)** | mcp.idcd.com 独立部署 + MCP Auth + Tool Dispatcher + LLM Provider 抽象 |

---

## 17. 阶段交付清单

### S1（0–4 月）必须落地
- 仓库结构 + monorepo
- Go API Gateway + Scheduler + Agent Gateway
- Next.js 主站（工具页 + 首页）
- PostgreSQL + TimescaleDB + Redis
- Cloudflare 全套
- mTLS CA + Agent enrollment
- Loki + Prometheus + Grafana
- CI/CD 基础
- 100+ 节点 IaC

### S2（4–8 月）
- 控制台前端（监控 / 告警 / 状态页 / 计费）
- Application Services 拆分（同进程 module）
- Worker Pool 完善
- Read Replica
- Sentry 错误追踪
- 状态页多租户 + ACME
- 容器化部署完善
- **v2 NEW: attest.idcd.com 独立子域 + Attestation Service + Worker + Self-Verify**
- **v2 NEW: KMS 选型 + 首次 Root key 仪式 + 90 天 sign key 自动轮换**
- **v2 NEW: TSA Client(DigiCert + GlobalSign 双家)**
- **v2 NEW: Agent 客户端证书短期化(7-30d) + CRL/OCSP 撤销**
- **v2 NEW: LLM Provider 抽象层(用于故障复盘自动起草)**
- **v2 NEW: Agent OTA 3 级灰度(1%/10%/100%)**

### S3（8–14 月）
- 多区主控
- Redis Cluster
- Scheduler 分片
- ClickHouse 评估 / 接入
- K3s / Nomad 评估
- 服务拆分（按热点）
- 性能优化
- **v2 NEW: mcp.idcd.com 独立子域 + MCP Server alpha(M9)→ GA(M12)**
- **v2 NEW: MCP Auth + Tool Dispatcher + 独立计量**
- **v2 NEW: 第三家 TSA(NTSC)接入**
- **v2 NEW: Agent obs 子系统(M21/M22/M23 监控类型对应的 Probe 实现)**
- **v2 NEW: 区块链锚定 alpha(可选 add-on)**

### S4（14+ 月）
- 多区域容灾
- **v2 NEW: HSM 硬件密钥升级**
- **v2 NEW: 白标 Attestation API / MCP server**
- **v2 NEW: M24 Agent Output Quality 监控**
- 私有部署版（On-Premises）打包
- 企业级备份策略
- 高级安全审计

---

## 18. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 调度系统单点 | Leader election + 多备 + 数据库持久化任务 |
| PostgreSQL 写入瓶颈 | TimescaleDB hypertable + 早期识别 |
| Cloudflare 在大陆访问慢 | 国内站走阿里 CDN + 智能 DNS |
| Agent 升级失败大面积掉线 | 灰度 + 自动回滚 + kill switch |
| Redis 内存爆掉 | 监控 + LRU + 限速键 TTL |
| 多区数据一致性 | 主写单点 + 异步复制（接受短暂滞后） |

---

## 19. 决策记录（已锁定，见 DECISIONS.md）

### v1.0
- ✅ **B2** 主站前端：**自建（Docker + Cloudflare）**
- ✅ **B1** 国内主控：**阿里云**（ECS / RDS / OSS / SMS 一条龙）
- ✅ **B3** 文档站：**自建 Nextra / Fumadocs**
- ✅ **B4** DDoS 防护：**Cloudflare Pro / Business**
- ✅ **G4** VPN：**自建 WireGuard**

### v2.0 (K 节, 2026-05-12)
- ✅ **K1** 三栈 sub-product:Core / Evidence / MCP 独立子域 + 独立部署 + 独立计量 + 独立 SLA
- ✅ **K2** 签名密钥架构:云 KMS + Shamir 3-of-5 离线 root + 90 天 sign key 轮换;HSM S4 评估
- ✅ **K-架构 mTLS 撤销**:短期证书(7-30d) + 自动 renewal + CRL/OCSP 主动撤销 + 1 小时内全节点完全踢出
- ✅ **K-架构 Agent OTA 3 级灰度**:L1(1%) → L2(10%) → L3(100%),失败自动回滚
- ✅ **K-架构 TSA 主备**:DigiCert + GlobalSign 起步;NTSC S3 加入;失败自动切换

### 已采用 PRD 默认

- Agent ↔ Gateway：**WSS + mTLS**（穿透性好）
- 日志：Loki + Promtail
- 队列：river（PG）+ asynq（Redis）双轨

### 待定（不紧迫）

- [ ] Nats / NSQ 事件总线：S3 评估
- [ ] CF Workers 承担 Edge API：S3 评估
- [ ] **v2 NEW** OpenAI Agents Protocol 是否在 mcp.idcd.com 加 adapter:S3 末根据市场份额评估
- [ ] **v2 NEW** PAdES 签名等级(B-B / B-T / B-LT):S2 实施时定
- [ ] **v2 NEW** 区块链锚定具体链(Ethereum / Polygon / Arweave):S3 评估
