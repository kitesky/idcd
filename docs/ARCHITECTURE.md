# idcd.com 项目架构(实施级)

> 版本:**v2.0**(对应 PRD v2 / Eng Review 2026-05-13 锁定)
> 受众:工程实施者(创始人 + AI 协同 + 未来加入的工程师)
> 这是项目落地架构,**不是 PRD 决策**。
>   - PRD 决策汇总:`docs/prd/DECISIONS.md`
>   - PRD 技术决策:`docs/prd/14-tech-architecture.md`
>   - Eng Review 锁定决策(D1-D14):`docs/prd/ENG-REVIEW-REPORT.md`
>   - 实施前 TODO 清单:`docs/prd/ENG-REVIEW-TODOS.md`
> 本文档定位:**让开发者打开 monorepo 后,5 分钟看懂结构、30 分钟跑起来、1 天能动手改第一个 feature。**

---

## 0. 用本文档前先读什么

| 角色 | 阅读顺序 |
|---|---|
| **创始人 / 实施负责人** | OVERVIEW §1-2 → 本文 §1-§6 → ENG-REVIEW-REPORT(14 项 D 决策)→ ENG-REVIEW-TODOS |
| **新加入的工程师** | OVERVIEW §1-2 → 本文全篇 → 自己关心的模块 PRD(02-19) |
| **AI 协同 agent** | 本文 §1(monorepo 结构)→ §3(关键决策实施)→ 任务相关模块 PRD |
| **企业 due diligence** | 本文 §3 + §4 + §11 安全 → 18-evidence + DECISIONS §K2/§M(D6/D11)|

---

## 1. Monorepo 结构

```text
idcd/
├── apps/                          # 8 Go services + 4 Workers + 4 Next.js apps
│   ├── web/                       # Next.js 主站(idcd.com)— SSG 工具页 + CSR 控制台
│   ├── status/                    # Next.js 状态页(*.status.idcd.com)— 多租户 SSR
│   ├── docs/                      # Nextra 主文档站(docs.idcd.com)— 纯 SSG
│   ├── docs-mcp/                  # MCP 文档站(docs.mcp.idcd.com,D3 独立子域)
│   │
│   ├── api/                       # Go API Gateway(api.idcd.com)
│   ├── scheduler/                 # Go 调度器(主备 leader election)
│   ├── aggregator/                # Go 聚合器
│   ├── notifier/                  # Go 告警派发
│   ├── gateway/                   # Go Agent Gateway(WSS + mTLS)
│   ├── agent/                     # Go Agent 节点二进制(部署到 100+ 节点)
│   ├── admin/                     # Go Admin Backend(admin.idcd.com,仅 VPN)
│   │
│   ├── attest-api/                # Go Attestation API(attest.idcd.com)
│   ├── attest-worker/             # Go Verdict Generator Worker(异步,WAL on attestation_record)
│   ├── attest-verify/             # Go Self-Verify Worker(D6 独立部署,独立 subnet + 独立 KMS 实例)
│   ├── attest-refund/             # Go Refund Worker(D5 retry queue + 道歉邮箱兜底)
│   │
│   └── mcp-server/                # Go MCP Server(mcp.idcd.com,业务 stateless + SSE stateful)
│
├── packages/                      # 跨 app 共享(Go module + npm workspace)
│   ├── db/                        # sqlc 生成代码 + Repository 抽象(D1 跨 schema join)
│   │   ├── migrations/            # golang-migrate 顺序迁移(三 schema:idcd_main / idcd_attest / idcd_mcp)
│   │   ├── queries/               # *.sql 文件,sqlc 生成
│   │   └── repository/            # 应用层 join 抽象(GetByOwnerId / GetReport ...)
│   ├── api-spec/                  # OpenAPI yaml + 自动生成 Go server / TS client(详 16-api-spec.md)
│   ├── ui/                        # shadcn/ui 组件 + 设计系统 token + ECharts presets
│   ├── sdk-js/                    # idcd-js SDK(npm 发布,公开 API)
│   ├── sdk-mcp-py/                # idcd-mcp-py(pypi,S3 GA)
│   ├── sdk-mcp-ts/                # idcd-mcp-ts(npm,S3 GA)
│   ├── llm/                       # LLM Provider 抽象层(D9 per-Provider prompt)
│   │   ├── anthropic/
│   │   ├── openai/
│   │   ├── custom/                # 企业用户自家 LLM
│   │   ├── prompts/               # 按 (provider, version) 二元 key 存储的 prompt 模板
│   │   └── eval/                  # per-Provider eval pipeline
│   ├── kms/                       # KMS 抽象层(D4 idempotency token 必传)
│   │   ├── aws/
│   │   ├── aliyun/
│   │   ├── vault/                 # 自建 Vault(S3 可选)
│   │   └── hsm/                   # Backup HSM(D11 SoftHSM/YubiHSM2 客户端)
│   ├── tsa/                       # RFC3161 TSA client(主备三家:DigiCert / GlobalSign / NTSC)
│   ├── mcp-protocol/              # MCP spec adapter(Anthropic JSON-RPC over stdio + HTTP+SSE)
│   ├── auth/                      # JWT / session / API key / MCP token 共享逻辑
│   ├── ratelimit/                 # Redis 滑动窗口 + 多维度限速
│   ├── audit/                     # 审计日志统一接口
│   └── shared/                    # 共享类型 / 工具函数 / errors / IDs(prefix+nanoid)
│
├── infra/
│   ├── terraform/                 # IaC:节点 / VPS / DNS / Cloudflare(详 10-nodes-and-agents)
│   ├── ansible/                   # 部署 playbook(主控集群 + Agent OTA)
│   ├── docker/                    # docker-compose.yml(S1-S2 主部署形态)
│   │   ├── docker-compose.core.yml          # api/scheduler/gateway/...
│   │   ├── docker-compose.attest.yml        # attest-api/attest-worker(独立栈)
│   │   ├── docker-compose.attest-verify.yml # D6 独立 service + 独立 subnet
│   │   ├── docker-compose.mcp.yml           # mcp-server + docs-mcp
│   │   └── docker-compose.observability.yml # Loki/Prometheus/Grafana/Tempo/Sentry
│   └── k3s/                       # K3s 配置(S3+ 评估,详 §6.3)
│
├── docs/
│   ├── prd/                       # PRD(20 模块 + DECISIONS + ENG-REVIEW + ENG-REVIEW-TODOS)
│   ├── ARCHITECTURE.md            # 本文件
│   ├── CONTRIBUTING.md            # 开发流程 / commit 规范 / review 规则
│   ├── DEPLOYMENT.md              # 详细部署 SOP(各 stage + 灰度 + 回滚)
│   ├── TROUBLESHOOTING.md         # 故障排查 SOP(指向 12 §20-22 / 11 §15.4-15.5)
│   └── RUNBOOKS/                  # 应急 SOP:KMS / TSA / Agent / refund_failed / ...
│
├── scripts/                       # 一次性脚本 / dev tools / seed
│   ├── seed.go                    # dev 数据 seed(50 用户 + 测试 token + 模拟 monitor)
│   ├── chaos/                     # 故障注入(toxiproxy + KMS/TSA/Paddle 50% 失败率,D4/D5 演示)
│   └── keymony/                   # 密钥仪式辅助脚本(air-gap 笔记本用)
│
├── tests/
│   ├── integration/               # 单 service + DB,go test 触发
│   ├── e2e/                       # Playwright(web) + 自家 SDK 模拟 MCP 客户端
│   ├── load/                      # k6 + ghz(gRPC),性能基线持续验证
│   └── mcp-compat/                # MCP 兼容性 smoke test(Cursor / Claude Code / Codex,TODO-6)
│
├── .github/
│   └── workflows/                 # CI/CD:lint / test / build / deploy / agent-gradual-rollout
│
├── package.json                   # pnpm workspace
├── pnpm-workspace.yaml
├── go.work                        # Go workspace(8 apps + packages/* 各自 module)
├── Makefile                       # make dev-up / make seed / make test 单一入口
├── VERSION
├── CHANGELOG.md
└── README.md
```

**关键设计决定**:

1. **monorepo + 多 module**:用 `go.work` + `pnpm-workspace.yaml`,所有 service 同一 git 仓但 build / deploy 独立。S1-S2 阶段同进程 module 化即可,真出现部署瓶颈才拆(详 14 §6.4)。
2. **`apps/` 与 `packages/` 物理分离**:`apps/` 是可部署单元(有 Dockerfile + main.go);`packages/` 是被 import 的(无 main)。
3. **`attest-*` / `mcp-server` 独立 docker compose 栈**:不与 core 共栈,K1 三栈 sub-product 阵型的实施落地。
4. **三 schema 在同一 PG cluster 起步**:`idcd_main` / `idcd_attest` / `idcd_mcp` schema 隔离;DDL 中**跨 schema 不写 FK**(D1);S4 企业 due diligence 时可拆三个独立 cluster,代码层无需改动。

---

## 2. 服务清单

### 2.1 后端 Go services(8 个)

| Service | 子域 / 端口 | 部署形态 | 主要依赖 | 备注 |
|---|---|---|---|---|
| **api-gateway** | api.idcd.com :8080 | 多实例水平扩展 | PG + Redis | 5000 RPS 单实例;统一鉴权 / 限速 / 计量入口(详 14 §4.1) |
| **scheduler** | (内部) :8081 | 主备 leader election(Redis/etcd) | PG + Redis Streams | S1 单 leader;S3 按地区分片(CN-Scheduler / Global-Scheduler) |
| **aggregator** | (内部) :8082 | 多实例消费 Redis Streams | PG + TimescaleDB | 幂等设计 — 同 task_id 重复处理无副作用 |
| **notifier** | (内部) :8083 | 多实例 + asynq 队列 | Redis(asynq)+ 10+ 通道 adapter | 通道故障不影响其他通道 |
| **gateway** (Agent Gateway) | agent-wss.idcd.com :8084 | 多区部署(中/美/欧) | mTLS CA + Redis | 10k 连接 / 实例;Agent → gateway 一致性哈希 |
| **admin** | admin.idcd.com :8085 | 单实例 + 内网/堡垒机 | PG + audit_log | VPN/WireGuard 才可访问 |
| **attest-api** | attest.idcd.com :8086 | 多实例 stateless | PG(idcd_attest schema)+ S3 | 订单 / 状态 / PDF 下载 / 分享 token / **公开验签**(revoke 期间持续可用) |
| **mcp-server** | mcp.idcd.com :8087 | 多实例 + **LB sticky session**(D13)| PG + Redis Pub-Sub | 业务 stateless,SSE stateful;10k 并发 SSE / 实例 |

### 2.2 Worker(异步,4 个)

| Worker | 类型 | 队列 | 备注 |
|---|---|---|---|
| **core-worker** | river(PG)+ asynq(Redis)双轨 | river:报告 / 导出 / PDF;asynq:邮件 / 通知 | 业务异步任务 |
| **attest-worker** (Verdict Generator) | 独立 docker container | Redis Stream `verdict_generation_queue` | WAL on `attestation_record`(D4);每 step 前查 success 跳过;KMS sign + idempotency token |
| **attest-verify** (Self-Verify Worker) | **独立 docker container + 独立 VPC subnet + 独立 KMS 客户端实例**(D6) | 内部 cron(每报告 1 次 + 每日抽样 10 份) | 仅调用 attest.idcd.com/verify 公开接口,**不复用** attest-worker 任何代码 / 配置 / 缓存 |
| **attest-refund** (Refund Worker) | 独立 docker container | retry queue(5min → 30min) | D5:30min 失败强制道歉邮箱 + `verdict_order.status=refund_failed` + P0 告警 + admin dashboard 待处理 |

### 2.3 前端 Next.js apps(4 个)

| App | 域名 | 渲染策略 | 备注 |
|---|---|---|---|
| **web** | idcd.com | 工具页 SSG + 控制台 CSR + 少量 BFF | shadcn/ui + Tailwind v4 + ECharts |
| **status** | *.status.idcd.com | ISR + CDN 缓存 | 多租户:host + slug 路由;自定义域名 SNI 切证书 + ACME |
| **docs** | docs.idcd.com | 纯 SSG(Nextra) | Cloudflare Pages |
| **docs-mcp** | docs.mcp.idcd.com | 纯 SSG(Nextra) | Cloudflare Pages;`mcp.idcd.com/docs` 走 **302 redirect** 到此(D3) |

### 2.4 Agent 节点二进制

- 路径:`apps/agent/`,Go 静态二进制,通过 systemd 启动
- 部署:Ansible + Terraform,初版 100+ 节点(Tier1 自有 + 海外 VPS)
- 升级:**OTA 3 级灰度**(K-架构 + 14 §13.3):L1 (1%) 1h → L2 (10%) 4h → L3 (100%)
- 任何阶段错误率 > 基线 2x → 自动回滚 + P1 + 暂停后续灰度
- 24h 本地缓冲:主控挂了 Agent 仍持续拨测,主控恢复回放
- mTLS 客户端证书 **7-30 天短期** + 自动 renewal + CRL/OCSP 撤销(失窃节点 1h 内完全踢出)

### 2.5 服务总计

> **8 Go services + 4 Workers + 4 Next.js apps + 1 Agent 二进制 = 17 个可部署单元**。
> 加上 `packages/*` 14 个共享包,monorepo 总计 **~30 个一级目录**。

---

## 3. 关键架构决策的实施落地

> 本节是本文档的"灵魂":Eng Review 锁定的 14 项 D 决策(+ DECISIONS §M)如何在代码 / 部署上落地。**每项 D 都说明"怎么实施" + "在哪个目录" + "如何验证"**。

### 3.1 schema 隔离 + 跨 schema 不写 FK(D1 / K-数据)

**为什么**:三栈 sub-product 阵型(K1)要求 attest / mcp / main 可独立 cluster 部署;若 DDL 写跨 schema FK,S4 拆分时必须改代码 + 数据迁移 nightmare。

**实施**:

| 项 | 实施细节 |
|---|---|
| **DDL** | `packages/db/migrations/` 三个 schema 各自 migration 目录(`idcd_main/` / `idcd_attest/` / `idcd_mcp/`)。**所有 v2 表的 `owner_id` / `verdict_report_id` 等跨 schema 列不写 `REFERENCES`**;同 schema FK(如 `verdict_report.order_id → verdict_order.id`)保留。详 15 §4.X 序言。 |
| **应用层 join** | `packages/db/repository/` 提供 `GetByOwnerId(ctx, id)` / `GetReport(ctx, reportId)` 类抽象;**service 代码不直接 SQL join 跨 schema**,统一走 Repository。 |
| **DBA 权限** | PostgreSQL role 按 schema 分级:`idcd_main_rw` / `idcd_attest_rw` / `idcd_mcp_rw`;attest-* worker 仅授予 idcd_attest 权限,bug 不会跨写。 |
| **S4 升级路径** | 拆分时:导出 schema → 导入独立 cluster → 改 service 连接串(`DATABASE_URL_ATTEST` 替换);**代码 0 行修改**。 |

**验证**:CI 跑 `scripts/lint-cross-schema-fk.sh`,扫描所有 migration 文件,任何 `REFERENCES idcd_main.*` 或类似跨 schema 引用 → fail。

---

### 3.2 Verdict WAL 状态机 + KMS idempotency(D4 / CRITICAL GAP 8.1)

**为什么**:Verdict 生成是 10 个 step 的长流程(拉数据 → 多节点交叉 → LLM → PDF → 哈希 → **KMS sign** → **TSA stamp** → 嵌入 PAdES → 归档 → 自检)。Worker crash 重试可能导致 KMS audit log 重复签名 = 违反信任根可审计性。

**实施**:

| 项 | 实施细节 |
|---|---|
| **`attestation_record` 充 WAL** | DDL:`UNIQUE(report_id, action)` + `status` enum (`pending` / `success` / `failure`)。每 step 完成后 worker 写一条 `(report_id, action, status=success, external_id, idempotency_key)`。详 15 §4.X.3。 |
| **Worker 续跑逻辑** | `apps/attest-worker/internal/wal/`:进入每 step 前先 `SELECT action FROM attestation_record WHERE report_id=$1 AND status='success'`;已成功的 step 跳过并复用 external_id。代码模式参考 `packages/db/repository/attestation.go` 提供的 `IsStepDone(reportId, action) bool` 接口。 |
| **KMS idempotency** | `packages/kms/`:`Sign(keyId, hash, idempotencyKey)` 必传 token(AWS KMS / 阿里云 KMS 均支持);token 写入 `attestation_record.idempotency_key` 持久化。 |
| **失败上限** | `retry_count` 字段 ≤ 3,超出转 DLQ;DLQ 告警 5 分钟内必到。 |

**验证**(S2 上线前**必演示**):staging 注入 KMS / TSA / S3 / Self-verify 各 step 失败率 50%(toxiproxy + 自家 chaos script),验证:
- worker crash 重启后续跑无重复 sign(KMS audit log 不重复)
- DLQ 监控告警 5 分钟内触发
- 30min 内用户收到道歉邮箱(D5)

→ 落地脚本:`scripts/chaos/verdict-failure-injection.sh`

---

### 3.3 Refund retry queue + 道歉邮箱兜底(D5 / CRITICAL GAP 8.1)

**为什么**:Paddle refund API 本身可能失败(网络 / 风控)。用户付 ¥299 → 拿不到报告 → 拿不到退款 = 品牌死亡。

**实施**:

| 项 | 实施细节 |
|---|---|
| **`attest-refund` 独立 Worker** | 路径:`apps/attest-refund/`。监听 verdict 失败事件,调 Paddle refund。 |
| **retry queue 策略** | 5min retry → 30min retry → 仍失败 → `verdict_order.status=refund_failed` + P0 告警(创始人本人手机)。 |
| **30min 强制道歉邮箱** | **无论 refund API 是否成功,30min 内必发**:"由于 [类别] 无法生成报告,已发起全额退款 ¥XXX。若 1-3 工作日内未到账,请回复此邮件,我们手动处理。" → 写 `verdict_order.refund_apology_sent_at`。 |
| **admin dashboard** | `apps/admin/`:`/admin/refund-failed` 页面,可查 / 手动处理所有 refund_failed 订单。详 11 §15.4。 |

**验证**:staging 注入 Paddle 50% refund 失败率,验证 30min 内用户收到道歉邮箱 + admin dashboard 出现该订单。

---

### 3.4 Self-Verify Worker 独立部署(D6 / CRITICAL GAP)

**为什么**:若 Self-verify 与 Generator 共用代码 / 配置 / KMS 客户端,bug 互盖 = 自检失效 = 信任根失守。

**实施**(物理隔离 4 层):

| 隔离层 | 落地 |
|---|---|
| **不同进程** | `apps/attest-verify/` 独立 main.go,独立 docker container,独立 docker compose service(见 `infra/docker/docker-compose.attest-verify.yml`)。 |
| **不同 VPC subnet** | Terraform 配置中 `attest-verify` 部署在独立 subnet,与 `attest-worker` 之间**仅暴露 attest.idcd.com/verify HTTPS 接口**;内部 RPC 防火墙阻断。 |
| **独立 KMS 客户端实例** | `attest-verify` 用自己的 `packages/kms/` 实例化(独立 config / IAM role / 客户端缓存);**不复用** `attest-worker` 的 sign 客户端。 |
| **公开 verify 路径** | `attest-verify` 通过 `https://attest.idcd.com/verify` 公开接口验签,**与外部第三方走完全一致的代码路径**。 |

**验证**:部署后展示
- `docker ps` 显示两个 container ID 不同
- 网络拓扑展示两个 subnet
- 运行时 strace / OpenTelemetry trace 显示 attest-verify 仅调 verify HTTP 接口,无内部 RPC 调用

---

### 3.5 Backup HSM 独立加速通道(D11 / S4 启用)

> ⚠️ **实施阶段:S4**。Pre-4 决策推迟采购 Backup HSM，S2 仅走 12h Shamir 单路径，接受 SLA 偶尔滑至 24h+ 的现实风险。
> 本节规约 S4 时的实施细节，供架构预留参考。

**为什么**:6h SLA 假设 5 个 Shamir 持有人周日凌晨 1h 内全联系上 — 不现实。Backup HSM 是独立加速路径。

**实施(S4 启用)**:

| 项 | 实施细节 |
|---|---|
| **硬件** | SoftHSM(¥1000+)或 YubiHSM2(¥3000+),冷启动笔记本(air-gap)+ USB HSM;**物理放离线保险柜**。 |
| **重组方式** | **1-of-1**(创始人物理获取 + 解锁密码);密码与 Shamir 切片**独立保管**(不同保险柜)。 |
| **代码** | `packages/kms/hsm/`:与云 KMS 同一接口(`Sign / GetPublicKey`),仅 backend 不同。 |
| **触发条件** | 主路径 12h SOP 已启动 + ≥3 Shamir 持有人 4h 内仍无法联系 → 启用 Backup HSM。 |
| **限制** | 仅一次性紧急 sign key 轮换,不参与日常签名;启用后 **7 天内必须补做完整 Shamir 仪式**。 |
| **演练** | **S4 启用前必须演练 1 次**,记录耗时基线。 |

**SLA 重调**:12h 主路径 + 4h Backup HSM 加速。详 11 §15.5。

---

### 3.6 MCP SSE LB sticky + 状态边界(D13)

**为什么**:`mcp-server` 的业务逻辑 stateless(可水平扩展),但 `/v1/agent-obs/events` 是 long-lived SSE 连接(stateful)。

**实施**:

| 项 | 实施细节 |
|---|---|
| **Cloudflare Load Balancer** | 配置 sticky session 基于 token_id 哈希;同一 token 的 SSE 连接始终落在同一 instance。 |
| **业务状态隔离** | 鉴权 / 计量 / config 状态全在 PG + Redis;任一 mcp-server 实例可处理任一**非 SSE**请求。 |
| **SSE 单实例容量** | **10k 并发 SSE / 实例**(参考 Agent Gateway 同档,详 14 §9.1)。Pro 用户 1000 活跃 × 10 concurrent = 10k。 |
| **跨实例广播** | Redis Pub-Sub:token revoke 时跨所有 instance broadcast → 立即断开该 token 的所有 active SSE。 |
| **heartbeat** | 30s 内无数据 → 发 keepalive event;断开重连由客户端负责。 |
| **横向扩展** | 一致性哈希(token_id → instance);新增实例 / 实例下线时 minimal connection migration。 |

→ 落地代码:`apps/mcp-server/internal/sse/` + `packages/mcp-protocol/sse.go`。

---

### 3.7 LLM Provider 抽象 + per-Provider prompt(D9)

**为什么**:Claude / GPT / 自家 LLM 对同一 prompt 输出风格 / schema / 鲁棒性不同。统一 prompt = 部分 Provider 输出质量崩盘。

**实施**:

| 项 | 实施细节 |
|---|---|
| **`packages/llm/`** | `anthropic/` / `openai/` / `custom/` 各自 client;统一接口 `Generate(promptTemplateId, vars, provider) -> {text, model, prompt_version}`。 |
| **Prompt 存储** | `packages/llm/prompts/{provider}/{template_id}/{version}.tmpl`;按 (provider, version) 二元 key。 |
| **baseline 范围** | 仅 **Claude + GPT** 维护 prompt + eval 数据集 + 每月 ≥4.0/5 才 ship。 |
| **企业 / 自家 LLM** | 用户在 `/app/llm/settings` 接入自家 endpoint;**需自行 prompt 调优 + 自行 eval**(企业版功能);≥4.0/5 才允许该 prompt 在 Verdict / postmortem 中使用。 |
| **eval 数据集** | `packages/llm/eval/datasets/`:bootstrap 50 条(30 公开事故 + 20 内部 dogfood,TODO-5)。 |

→ 详 07 §6.3 + 14 §4.11。

---

### 3.8 MCP 鉴权:无永久 token + 三形态(D2)

**实施**:

| 形态 | 有效期 | renewal 机制 | IP 白名单 |
|---|---|---|---|
| `personal` | **24h** | OAuth-like flow 自动 refresh(每天首次使用) | 可选 |
| `workspace` | **90d** | `auto_renew=true`:过期前 24h 自动续期;**超 30 天未用不续期**(等于自动撤销) | 可选 |
| `service` | **90d** | 同 workspace | **强制**(无白名单不签发) |

代码层:
- `packages/auth/mcp_token.go`:`Validate(tokenHash) (ownerId, scope, expiresAt, err)` + `Renew(tokenId) error`
- `apps/mcp-server/internal/auth/`:每次 tool call 进入前查 `expires_at`;< 24h 触发 renewal
- `mcp_token` 表:`expires_at NOT NULL`(无永久);`auto_renew bool`;`last_renewed_at timestamptz`
- **配额池完全独立**:`MCP units/day` 与 `API calls/day` 是两条独立 progress bar(详 19 §3.5)

---

### 3.9 其他 D 决策快速索引

| D | 概要 | 实施位置 |
|---|---|---|
| D3 | `docs.mcp.idcd.com` 独立子域 + `mcp.idcd.com/docs` 302 | `apps/docs-mcp/` + Cloudflare Pages 配置 |
| D7 | `agent_obs_monitor.total_cost_this_month_usd` 原子 UPDATE + `mcp_tool_call(session_id)` 索引 + 失败 case 7d 原 payload | `packages/db/repository/agent_obs.go` 原子事务 + DDL 索引 |
| D8 | LLM 复盘 eval bootstrap 50 条 | `packages/llm/eval/datasets/bootstrap-2026/`(TODO-5) |
| D10 | Anchor 阈值 placeholder + S2 前 30 天 baseline 校准 | `apps/scheduler/internal/anchor/`;TODO-4 |
| D12 | 3 档工单 SLA:纯自动 / 1h 仅 P0(KMS/节点失窃)/ 24h 常规 | `apps/admin/internal/ticket/sla.go`;详 11 §15.4 |
| D14 | TimescaleDB → ClickHouse 触发指标:>10GB/d 或 P99>100ms(持续 1 周)| Prometheus alert rule;TODO 在 17-roadmap S3 末 |

---

## 4. 部署架构

### 4.1 S1 部署(M1-M4,极简)

```text
┌──────────────────────────────┐    ┌──────────────────────────────┐
│  阿里云 ECS 4C/8G(杭州)      │    │  Hetzner CCX13(法兰克福)    │
│  ¥300/月 + ESSD 200GB        │    │  €13/月 80GB                 │
│  docker-compose 启全栈        │    │  docker-compose 启全栈        │
│  ├─ api / scheduler / agg... │    │  ├─ api / scheduler / agg... │
│  ├─ PostgreSQL(主)+ Redis    │◀──▶│  ├─ PostgreSQL(热备)+ Redis  │
│  ├─ Loki / Prometheus        │    │  ├─ Loki / Prometheus        │
│  └─ Next.js(web/docs)         │    │  └─ Next.js(web/docs)         │
└──────────────────────────────┘    └──────────────────────────────┘
              │                                     │
              └─────────── Streaming replication ───┘
              │
              ▼
    100+ 节点(Tier1 自有 IDC + 海外低配 VPS,Terraform 管)
```

**关键点**:
- 所有 core service 同机 docker compose(`infra/docker/docker-compose.core.yml`)
- 国内/海外用户走 Cloudflare 就近接入
- 主→备 流式复制(streaming replication,< 30s lag)
- 100+ 节点纯 Ansible + Terraform

### 4.2 S2 部署(M5-M8,Evidence 上线)

新增:
- **attest.idcd.com 独立 docker compose 栈**(`infra/docker/docker-compose.attest.yml` + `docker-compose.attest-verify.yml`)
  - attest-api(多实例,LB)
  - attest-worker(单实例起步,S3+ 改 Worker 池)
  - **attest-verify 独立 subnet**(D6)
  - attest-refund(独立 container)
- **KMS**:云 KMS(AWS / 阿里云,根据收款主体地区)
- **TSA Client**:DigiCert + GlobalSign 双家(`packages/tsa/`)
- **应用层多实例水平扩展**:api / aggregator / notifier 各 2+ 实例
- **Redis Cluster 起步**(3 主 3 从)
- **read replica**:报表查询走从库

### 4.3 S3 部署(M9-M14,MCP 上线 + 多区)

新增:
- **mcp.idcd.com 独立部署**(`infra/docker/docker-compose.mcp.yml`)
  - mcp-server 多实例 + Cloudflare LB sticky session
  - 兼容性 smoke test(Cursor / Claude Code / Codex)发布前必跑
- **多区主控**:亚太 / 北美 / 欧洲
- **Scheduler 分片**:CN-Scheduler / Global-Scheduler
- **ClickHouse 评估 / 接入**(若 D14 触发指标命中)
- **K3s / Nomad 评估**(按需,可不上;详 14 §6.3)
- **第三家 TSA(NTSC)接入**

### 4.4 S4 部署(M15+,企业化)

新增:
- **HSM 硬件密钥升级**(从云 KMS → HSM,企业 due diligence)
- **白标 Attestation API / MCP server**:`tenant_id` 列加入 + 多租户隔离
- **私有部署版(On-Premises)**:打包 docker compose + 配置模板
- **多区域容灾**:跨大洲主备
- **高级安全审计** + 企业级备份策略

---

## 5. CI/CD Pipeline

### 5.1 PR 流程

| 阶段 | 工具 / 命令 | 通过条件 |
|---|---|---|
| **lint** | `golangci-lint run` + `eslint` + `ruff` + `sqlfluff` | 0 error |
| **lint cross-schema FK** | `scripts/lint-cross-schema-fk.sh`(D1) | 0 跨 schema REFERENCES |
| **lint 鉴定词** | `scripts/lint-attestation-words.sh`(D-Concern1) | PRD / 营销 / 控制台 / 邮件模板中"鉴定 / 认定 / 判定"字样 → fail |
| **type-check** | `tsc --noEmit` + `go vet` | 0 error |
| **unit test** | `go test ./...` + `vitest` + `pytest` | 覆盖率 ≥ baseline |
| **integration test** | `tests/integration/`(spin up PG + Redis container) | 全通 |
| **安全扫描** | `govulncheck` + `npm audit --production` + GitGuardian secrets | 0 high |
| **review** | 1 人 + AI 协同:`/codex review` 提供独立 second opinion | 创始人 final approve |

→ branch protection: required checks 全绿才能 merge

### 5.2 部署流程

```text
merge to main
    │
    ▼
GitHub Actions: build docker images → push GHCR / 阿里云 ACR
    │
    ▼
auto deploy to staging
    │
    ▼
smoke test(健康检查 + 关键 API 路径)
    │
    ▼
手动 confirm(or auto if smoke pass) → prod 蓝绿 / canary
    │
    ▼
关键 SLI 自动验证(P95 latency / error rate / refund_failed 累积)
    │
    └─ 失败自动回滚到上一版本
```

### 5.3 Agent OTA 3 级灰度(K-架构)

独立 release pipeline(`.github/workflows/agent-release.yml`):

| 阶段 | 节点比例 | 观察窗 | 失败阈值 |
|---|---|---|---|
| L1 | 1%(随机 1-2 节点)| 1 hour | 错误率 > 基线 ×2 → 自动回滚 + P1 + 暂停 |
| L2 | 10%(扩到 10 节点)| 4 hours | 同上 |
| L3 | 100% | — | 失败自动回滚(节点保留前版本)|

→ 详 10 §8.3 + 14 §13.3

### 5.4 v2 Attestation / MCP 独立 pipeline

- **`attest-*` services 独立 release**(不与 core 耦合):attest-api / attest-worker / attest-verify / attest-refund 共享 release tag(同步部署)
  - **发布前必须验证**:KMS sign + TSA stamp + Self-verify 全链路 staging 通过(详 TODO-1)
- **`mcp-server` 独立 release**
  - **发布前必须跑 Cursor / Claude Code / Codex 兼容性 smoke test**(TODO-6:S3 alpha 前研究 CI 友好方案,如各家都不提供 headless,自家 SDK 作 mock client)

### 5.5 Verdict 签名密钥首次生产部署("密钥仪式"SOP)

`docs/RUNBOOKS/key-ceremony.md` 详细 SOP。要点:
1. air-gap 笔记本生成 root key
2. Shamir 切分 5 份 → 5 个不同人员 / 律所 / 可信第三方
3. 全过程录像 + 公证 + 上传 `idcd.com/transparency/key-ceremony`(DNSSEC root ceremony 范式)
4. Backup HSM 同步初始化 + 演练 1 次(TODO-2)
5. 写入 `key_ceremony_log` 表(只增不删 + 双人审批写入)

---

## 6. 开发者 Onboard

### 6.1 本地开发(目标 15 分钟跑起来)

**前置依赖**(2026-05-13 锁定 latest):
- **Go 1.26**(用户本地已最新)
- **Node 22+**(Next.js 16 要求)
- **pnpm 10+**
- **Docker 27+**
- **PostgreSQL 18.3**(云端已用)
- **TimescaleDB 2.21+**(兼容 PG 18,M1 验)
- **Redis 7.4+**
- make

```bash
git clone https://github.com/<org>/idcd
cd idcd

# 一键安装依赖 + 启 PG/Redis docker
make dev-setup

# 启全部 service(docker compose)
make dev-up

# seed dev 数据(50 用户 / 测试 MCP token / 模拟 monitor)
make seed

# 跑测试
make test                # 所有
make test-integration    # 仅集成
make test-mcp-compat     # MCP 兼容性(需 Cursor / Claude Code 本地装)

# 单 service 热重载
make dev-api             # 重启 api-gateway(air / reflex)
make dev-web             # next dev
```

### 6.2 关键代码路径速查

| 我想做什么 | 入口 |
|---|---|
| 新增公开 API 端点 | `apps/api/internal/handlers/` + 同步更新 `packages/api-spec/openapi.yaml` |
| 新增 MCP tool | `apps/mcp-server/internal/tools/` + 注册到 `tools/registry.go` + 计费配置 `mcp_tool_call.units_charged` |
| 新增监控类型(M21-M24) | `apps/scheduler/internal/probes/` + `04-monitoring §3` + DDL `agent_obs_monitor.endpoint_config jsonb` |
| 新增 Verdict 模板(SLA/Incident/Compliance/Legal) | `apps/attest-worker/internal/templates/` + 加 unit test 验证 PDF 渲染 + 法律边界硬编码段落 |
| 新增告警通道(钉钉/飞书/Webhook/Email/SMS/...) | `packages/notifier/channels/` 实现 `Send(payload) -> Result` 接口 |
| 新增 LLM Provider | `packages/llm/{provider}/` + 注册 `packages/llm/registry.go` + 添加 per-Provider prompt + eval 数据集 |
| 新增 schema 迁移 | `packages/db/migrations/{schema}/NNN_xxx.up.sql` + `.down.sql`;**严禁跨 schema REFERENCES** |
| 新增数据库查询 | `packages/db/queries/*.sql` → `sqlc generate` → 在 `packages/db/repository/` 包装 |
| 新增 prod 配置 | `infra/ansible/group_vars/prod/` + secrets 走 Vault(不入 git) |
| 修 Agent 二进制 | `apps/agent/` + 走 OTA 3 级灰度;dev 模式可 `make agent-local` 单机跑 |
| Hot fix prod 单 service | `git tag v{x.y.z}-{service}` → CI 触发该 service 单 build & deploy(不触发 monorepo 全量) |

### 6.3 第一周新人 checklist

- [ ] 跑通 `make dev-up` + 浏览 idcd.com / docs.idcd.com / status.idcd.com(本地)
- [ ] 通读 OVERVIEW.md + 本文档 §1-§3
- [ ] 完成第一个 PR:加一个 trivial endpoint(如 `/api/health/deep`)
- [ ] 跑通 attest-worker 完整链路(用 staging 测试 KMS,看到 PDF 输出)
- [ ] 跑通 mcp-server 本地(Claude Code 连接 `http://localhost:8087/mcp`)
- [ ] 阅读 ENG-REVIEW-REPORT 全文 + ENG-REVIEW-TODOS 全文

---

## 7. 监控与可观测(Dogfood)

### 7.1 三大支柱

| 类型 | 工具 | 用途 |
|---|---|---|
| **Metrics** | Prometheus + Grafana | RPS / 延迟 / 错误率 / 资源 / SLI |
| **Logs** | Loki + Promtail | 业务日志 / 审计;每 service 都注入 `request_id` |
| **Traces** | OpenTelemetry + Tempo | 端到端追踪;`attest-worker` 每 step 必加 span |
| **Errors** | Sentry(自托管)| Go + JS 统一 |
| **Site monitoring** | **自家产品 dogfood** + UptimeRobot 兜底 | idcd.com/status 公开 |
| **Alert** | 自家通道 + 钉钉 + 邮件 + 手机(P0)| 创始人手机告警仅限 P0(D12)|

### 7.2 SLI / SLO

| 服务 | SLI | SLO |
|---|---|---|
| API Gateway | 可用性 / P95 延迟 | 99.9% / 200ms |
| 公开工具页 | 可用性 | 99.95% |
| 监控调度 | 任务延迟 P95 | 5s |
| 告警送达 | 30s 内送达率 | 99% |
| 数据库主库 | 可用性 | 99.95% |
| Verdict 生成 | P95 延迟 | S2 ≤ 90s / S3 ≤ 60s |
| TSA 可用性(主备汇总)| 可用性 | S2 ≥ 99.9% / S3 ≥ 99.95% |
| MCP tool call | P95 延迟 | S3 alpha ≤ 10s / GA ≤ 5s |

错误预算:超出预算冻结发布。

### 7.3 关键 Dashboard(Eng Review Failure Modes 补齐)

`apps/admin/internal/dashboards/` + Grafana JSON `infra/grafana/dashboards/`:

| Dashboard | 用途 | 对应 D / Concern |
|---|---|---|
| **Verdict step-level latency** | P95 / P99 按 step 分解(拉数据 / 多节点 / LLM / PDF / KMS / TSA / 嵌入 / 归档 / 自检)| D4 |
| **KMS 应急时间线** | 演练 + 实际应急时记录每步耗时;基线 vs 实际对比 | D11 |
| **数据污染恢复** | Anchor 偏差告警后的恢复进度 + 节点剔除 + 重算覆盖范围 | D-Concern8 |
| **refund_failed 累积** | Paddle refund 自动退款失败趋势 + 未处理订单数 | D5 |
| **LLM eval 趋势** | prompt 版本 × Provider × 月度 eval 分数(必须 ≥4.0/5)| D8 + D9 |
| **MCP SSE 连接** | 每实例并发连接数(上限 10k)+ token revoke 后断开延迟 | D13 |
| **Agent OTA 灰度** | L1/L2/L3 错误率 vs baseline + 自动回滚事件 | K-架构 |

### 7.4 错误追踪 & 审计

- 所有 admin 操作写 `audit_log`(包括 Verdict 工单 / KMS 应急 / Agent 强制下线 / refund 手动处理)
- 6 个月保留;Verdict 报告 6 年(合规)
- KMS 调用 100% 审计(`key_id + key_version + caller + report_id + idempotency_key`)
- `key_ceremony_log` 表只增不删 + 双人审批写入

---

## 8. 故障排查 SOP

### 8.1 常见场景速查表

| 现象 | SOP 入口 |
|---|---|
| 用户付费 Verdict 但拿不到报告 | `docs/RUNBOOKS/verdict-failed.md` + 检查 `attestation_record` WAL last step + admin dashboard refund_failed |
| KMS 应急(怀疑泄露)| `docs/RUNBOOKS/kms-emergency.md` + 12 §20 + 11 §15.5 + Backup HSM 加速路径 |
| 节点失窃 / 异常上报 | `docs/RUNBOOKS/node-compromise.md` + 10 §6.5 + CRL/OCSP 撤销 + Anchor 偏差告警 |
| MCP token 异常突增 | 12 §22 + 用户控制台一键撤销 + Redis broadcast 断 SSE |
| Verdict 自检失败 | `docs/RUNBOOKS/verdict-self-verify-fail.md` + **立即停止新生成** + P0 |
| Agent OTA 灰度失败 | 自动回滚已触发,P1;查 `apps/scheduler/internal/ota/` 灰度日志 |
| TSA 三家全挂 | P0 + 报告生成暂停;切 NTSC(若 S3 已接入);通知用户延迟 |
| LLM 复盘 eval 跌破 4.0 | 暂停该 (provider, version) prompt;回退上版本;调查数据集 |

### 8.2 RUNBOOK 目录(`docs/RUNBOOKS/`)

- `verdict-failed.md` — 单订单失败处理(关联 D4 + D5)
- `kms-emergency.md` — 主路径 12h + Backup HSM 4h SOP(关联 D11)
- `node-compromise.md` — 节点失窃 1h 完全踢出(关联 K-架构)
- `tsa-all-down.md` — 三家 TSA 全挂应急
- `mcp-token-abuse.md` — token 异常突增
- `key-ceremony.md` — 首次部署 + 周期性 sign key 轮换
- `agent-mass-rollback.md` — OTA 灰度大面积失败
- `data-poisoning-recovery.md` — Anchor 偏差检出后数据污染恢复

→ 详细故障 SOP 索引在 `docs/TROUBLESHOOTING.md`

---

## 9. 备份与容灾

### 9.1 备份策略

| 数据 | 策略 |
|---|---|
| PostgreSQL(三 schema)| 每日全量 + WAL 每 5 分钟 + 异地 R2 / OSS |
| Redis | 每日 RDB + AOF(非关键,重启可重建)|
| 对象存储(R2)| 跨区域冗余(R2 默认 + 阿里云 OSS 备份)|
| 配置 / 密钥 | KMS + 离线副本(保险柜)|
| **Verdict 归档(S3/WORM,6 年)** | 只增不删 + 跨区 + 季度抽样自检 |
| `key_ceremony_log` | 只增不删 + 双人审批写入 + 异地物理备份 |

恢复演练:**每季度一次**。

### 9.2 故障级别

| 级别 | 场景 | RTO | RPO |
|---|---|---|---|
| L1 | 单实例挂 | 自动切换 | < 1s |
| L2 | 单可用区挂 | 跨可用区切换 | < 30s |
| L3 | 主区域挂 | 跨区切换(半自动)| < 5 min |
| L4 | 数据库灾难 | 异地恢复 | < 5 min |

---

## 10. 配置 & 秘密管理

### 10.1 环境层级

- `local`(开发机)— `.env.local`(不入 git)
- `staging`(预生产)— GitHub Actions secrets + Vault staging instance
- `prod`(生产)— Vault prod + IAM role

### 10.2 配置项分类

| 类别 | 存储 | 例 |
|---|---|---|
| 环境差异 | env vars + Vault | `DATABASE_URL`,`KMS_KEY_ID`,`PADDLE_API_KEY` |
| 业务参数 | PG(后台运营配置)| 限速阈值 / 配额 / 价格 / Verdict 模板列表 |
| 功能开关 | Feature flag service | `mcp_enabled` / `attest_enabled` / `agent_ota_l1_pct` |

### 10.3 秘密管理铁律

- **不入 git**(任何文件 / commit / branch)
- **CI 中 GitGuardian 扫描**(TODO-10)
- **KMS / Vault 集中托管**
- **应用通过 IAM 临时凭证获取**(STS / 阿里云 RAM)
- **Verdict 签名密钥**:Shamir 切片物理分离(D11)

---

## 11. 安全架构(实施视角)

### 11.1 边界
- Cloudflare WAF + Turnstile + Bot Score
- 按需国家级 IP 屏蔽

### 11.2 应用层
- 所有 endpoint 经过 Gateway(统一鉴权 / 限速 / 黑名单)
- SSRF 防护(解析 → 白名单 → 连接)
- 输入验证(go-playground/validator + zod)+ 输出转义
- CSP / Same-Site Cookie / CSRF Token

### 11.3 数据层
- PG / Redis 仅内网
- 静态加密(磁盘 + 备份)
- 主密钥由 KMS 托管,应用启动时拿 DEK 解密
- 敏感字段(2FA secret / channel config / admin TOTP)AES-256-GCM 应用层加密

### 11.4 Agent 层
- mTLS + **短期 7-30d 客户端证书 + 自动 renewal + CRL/OCSP 撤销**(K-架构)
- 任务签名校验 + 硬编码任务白名单 + 上报水印
- **Anchor 偏差实时检测**(D10 + D-Concern8 向前回溯审查机制)
- 失窃节点 1 小时内完全踢出(Gateway 拒绝连接 + 节点池剔除)

### 11.5 Verdict 信任根层(v2 NEW)
- KMS:云 KMS 起步,HSM S4
- Root key:Shamir 3-of-5 离线 quorum
- Sign key:90d 轮换,过期密钥保留只读用于历史验签
- Backup HSM(D11):冷硬件 1-of-1 应急加速通道
- KMS 调用全审计;revoke 期间 verify 接口仍可用(已发报告仍可被验签)

### 11.6 MCP 凭证安全(v2 NEW)
- **无永久 token**(D2):personal 24h / workspace 90d / service 90d 全自动 renewal
- token hash 存储,前端展示一次后仅显示后 4 位
- Service account token 强制 IP 白名单
- 异常突增告警(24h 调用量 > 历史 P95 × 5)
- GitHub token 扫描自动失活(TODO-10:GitGuardian / 自家正则 / TruffleHog 选型)

详 12-compliance-and-abuse.md(v2 §11 权威测评白名单 / §12 KMS 应急 SOP)。

---

## 12. 与各模块的实施对应关系

| 模块 PRD | 主要 app / package 实施 |
|---|---|
| 01-branding | `apps/web/` 设计 token + `packages/ui/` |
| 02-public-tools | `apps/web/` SSG + `apps/api/internal/handlers/probe/` |
| 03-account-system | `apps/api/internal/handlers/account/` + `packages/auth/` |
| 04-monitoring | `apps/scheduler/internal/probes/` + `apps/aggregator/` + DDL Hypertable |
| 05-alerting | `apps/notifier/` + `packages/notifier/channels/` |
| 06-status-pages | `apps/status/`(Next.js 多租户)+ ACME + LLM 起草 Worker |
| 07-reports | `apps/api/internal/handlers/reports/` + LLM 复盘(`packages/llm/`)|
| 08-open-api | `apps/api/` + `packages/api-spec/` + `packages/sdk-js/` |
| 09-billing | `apps/api/internal/handlers/billing/` + Paddle SDK + KMS |
| 10-nodes-and-agents | `apps/gateway/` + `apps/agent/` + mTLS CA + CRL/OCSP + `infra/terraform/` |
| 11-admin | `apps/admin/` + audit + KMS 仪式后台 + Verdict 工单 |
| 12-compliance | `apps/api/` Gateway 限速 + CF + audit + 权威测评白名单 + KMS 应急 SOP |
| 13-content-seo | `apps/web/` SSG + `/leaderboard` 生成器 |
| **18-evidence (v2)** | `apps/attest-api/` + `apps/attest-worker/` + `apps/attest-verify/` + `apps/attest-refund/` + `packages/kms/` + `packages/tsa/` + S3 WORM |
| **19-ai-agent (v2)** | `apps/mcp-server/` + `packages/mcp-protocol/` + `packages/llm/` + MCP token store |

---

## 13. 阶段交付(实施视角)

> 详细 milestone 在 `docs/prd/17-roadmap.md`。本节给出**实施工程视角**的"S1 / S2 / S3 / S4 必有产物"。

### S1(M1-M4,极简)
- monorepo 骨架 + `make dev-up` 可跑
- core 8 services 中:api / scheduler / aggregator / notifier / gateway / agent 上线
- web(主站工具页 + 首页 SSG)+ docs
- PostgreSQL + TimescaleDB + Redis(单实例)
- Cloudflare 全套 + mTLS CA + Agent enrollment
- Loki + Prometheus + Grafana + Sentry
- CI/CD 基础(lint / test / build / deploy)
- 100+ 节点 IaC(Terraform + Ansible)
- **S1 末**:30 天 Anchor baseline 数据采集报告(TODO-4 输入)

### S2(M5-M8,Evidence + 商业化)
- 控制台 CSR(`apps/web/` 登录后)
- Application Services 拆分(同进程 module)
- Worker Pool 完善(river + asynq)+ Read Replica
- status 多租户 + ACME
- 容器化部署完善
- **v2 NEW**:
  - `apps/attest-*` 全套上线(api + worker + verify + refund)
  - KMS 选型 + 首次 Root key 仪式 + 90d sign key 自动轮换
  - **Backup HSM 采购 + 独立重组演练**(TODO-7 + TODO-2)
  - TSA Client(DigiCert + GlobalSign 双家)
  - Agent 客户端证书短期化(7-30d)+ CRL/OCSP
  - LLM Provider 抽象层 + bootstrap eval 50 条(TODO-5)
  - Agent OTA 3 级灰度上线
- **S2 上线前必演示**:TODO-1(Verdict 失败链路 staging)+ TODO-2(KMS 演练)
- **S2 上线前必完成**:TODO-3(Self-verify 独立部署)+ Anchor 阈值 calibration 报告

### S3(M9-M14,MCP + 规模化)
- 多区主控 + Redis Cluster + Scheduler 分片
- ClickHouse 评估(D14 触发指标命中即启动)/ 接入
- K3s / Nomad 评估
- 服务拆分(按热点)
- **v2 NEW**:
  - `apps/mcp-server/` alpha(M9)→ GA(M12)
  - MCP Auth + Tool Dispatcher + 独立计量
  - `apps/docs-mcp/` 独立子域上线
  - 第三家 TSA(NTSC)接入
  - Agent obs 子系统(M21/M22/M23)
  - 区块链锚定 alpha(可选 add-on)
  - **MCP 兼容性 CI 自动化**(TODO-6,S3 alpha 前完成)
- **S3 末**:TimescaleDB 容量评估报告 + CK PoC 准备

### S4(M15+,企业化)
- 多区域容灾
- **v2 NEW**:HSM 硬件密钥升级 / 白标 Attestation + MCP / M24 Agent Output Quality 监控
- 私有部署版(On-Premises)打包(`tenant_id` 多租户 schema)
- 企业级备份策略 + 高级安全审计

---

## 14. 风险登记与未决事项

### 14.1 已识别风险(实施层)

| 风险 | 缓解 | 跟踪 |
|---|---|---|
| 调度系统单点 | Leader election + 多备 + 数据库持久化任务 | 已规约 |
| PostgreSQL 写入瓶颈 | TimescaleDB Hypertable + D14 触发指标 | D14 |
| Cloudflare 在大陆访问慢 | 国内站走阿里 CDN + 智能 DNS | 14 §11.6 |
| Agent 升级失败大面积掉线 | 3 级灰度 + 自动回滚 + kill switch | K-架构 |
| Redis 内存爆掉 | 监控 + LRU + 限速键 TTL | 14 §11.6 |
| 多区数据一致性 | 主写单点 + 异步复制(接受短暂滞后)| 14 §11.6 |
| **KMS 应急 5 人 1h 联系不上** | Backup HSM 1-of-1 加速通道 + 12h+4h SLA | D11 |
| **Paddle refund API 失败** | retry queue + 30min 道歉邮箱 + refund_failed | D5 |
| **Self-verify bug 互盖** | 独立进程 + 独立 subnet + 独立 KMS 客户端 | D6 |
| **MCP token 泄露** | 90d 过期上限 + GitHub 扫描自动失活 | D2 + Concern 6 |
| **LLM 复盘幻觉/造谣/泄密** | 强制人工审核 + sanitize 字典 + 反馈不发回 LLM Provider | D8 + Concern 7 |
| **Anchor 阈值未数据校准** | S1 后 30d baseline + S2 前 calibration | D10 |

### 14.2 待定(已 deferred,不阻塞 S2)

- Nats / NSQ 事件总线 — S3 评估
- Cloudflare Workers 承担 Edge API — S3 评估
- OpenAI Agents Protocol adapter on mcp.idcd.com — S3 末根据市场份额
- PAdES 签名等级(B-B / B-T / B-LT)— S2 实施时定(TODO-8)
- 区块链锚定具体链(Ethereum / Polygon / Arweave)— S3 评估
- BYOK(企业自带签名密钥)— S4
- Cursor headless CI 集成 — S3 alpha 前(TODO-6)
- M24 Agent Output Quality 监控 — S4

### 14.3 待 CEO / 创始人决策(非工程范畴)

- Pre-1: 三栈并行(S2 Evidence + S3 MCP)1 人 + AI 真能并行 vs 顺序? 影响 17-roadmap 时间线
- Pre-2: KMS sign + TSA + LLM token 月外部依赖成本 $200-1000 acceptable?
- D8: 创始人手动标注 25h 是否优先(影响 S2 上线节奏)?
- D11: Backup HSM 采购 ¥1000+ 是否优先?
- D12: 个人 7×24 P0 响应是否接受?替代方案为 S2 前招第二个 operator

---

## 15. 引用与延伸阅读

| 文档 | 用途 |
|---|---|
| `docs/prd/OVERVIEW.md` | 产品全景骨架(20 模块入口)|
| `docs/prd/DECISIONS.md` | 所有决策汇总(v1 § A-J + v2 §K 9 项 + §L Concern + §M Eng Review 14 项)|
| `docs/prd/14-tech-architecture.md` | PRD 级技术决策(本文实施视角的来源)|
| `docs/prd/15-data-model.md` | 完整 DDL + schema 隔离设计 |
| `docs/prd/16-api-spec.md` | OpenAPI 规范 |
| `docs/prd/17-roadmap.md` | 详细阶段路线(S1 / S2 / S3 / S4 milestone)|
| `docs/prd/18-evidence-and-attestation.md` | Evidence 完整模块 PRD |
| `docs/prd/19-ai-agent-observability.md` | MCP + Agent obs 完整模块 PRD |
| `docs/prd/ENG-REVIEW-REPORT.md` | Eng Review 14 项 D 决策 + verdict |
| `docs/prd/ENG-REVIEW-TODOS.md` | 11 项 TODO + Worktree 并行化 + Effort 汇总 |
| `docs/prd/ER-DIAGRAM.md` | ER 图 |
| `docs/prd/STATE-MACHINES.md` | 关键状态机(verdict_order / monitor / alert)|
| `docs/CONTRIBUTING.md` | 开发流程 / commit 规范 / review 规则 |
| `docs/DEPLOYMENT.md` | 详细部署 SOP |
| `docs/TROUBLESHOOTING.md` | 故障排查索引(指向 RUNBOOKS)|
| `docs/RUNBOOKS/*.md` | 应急 SOP(KMS / TSA / Agent / refund_failed / ...) |

---

> 本文档随 PRD 演进。重大架构变更(新增 service / 新增 schema / 新增信任根)必须在本文 + DECISIONS.md 同步更新。
> 实施中如发现新 issue,追加到 ENG-REVIEW-REPORT D15+ 并同步本文 §3 + §14.1。
