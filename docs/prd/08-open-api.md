# 08 · 开放 API 平台(v2)

> v2 新增:§12a Verdict / Attestation API(attest.idcd.com 独立子域);§12b MCP Server(mcp.idcd.com 独立子域)+ Agent obs API + 三档 token 鉴权
> 详细规格分别见 18-evidence §3+§6 和 19-ai-agent §3+§5

> 关联：OVERVIEW.md §4.7、03 API Key、09 计费、02 公开工具
> 阶段主体：S2 beta 内测，S3 正式开放 + SDK + CLI
> 品牌名占位：`idcd`

---

## 1. 模块定位

API 平台是 `idcd` 面向 **DevOps / 开发者** 的核心收入路径。对标：
- ipinfo.io（IP 数据 API）
- Globalping（拨测 API + CLI）
- UptimeRobot API（监控管理 API）
- crt.sh / RDAP（网络信息 API）

定位：**"全栈网络数据 + 拨测 + 监控管理"的统一开放 API**。

### 设计原则

1. **简单**：90% 的请求 1 个 endpoint + 1 个参数搞定
2. **稳定**：版本化（v1, v2 ...），破坏性变更慎重
3. **可观测**：请求 ID、调用统计、错误清楚
4. **可计量**：每次调用准确计入配额 / 计费
5. **开发者友好**：交互式文档 + SDK + CLI + 示例

### 关键指标

| 指标 | S2 末 | S3 末 |
|---|---|---|
| API 月调用量 | 100k | 50M |
| API 付费用户 | 20 | 300 |
| API 收入占比 | — | ≥ 15% |
| API P95 延迟（不含拨测执行） | ≤ 100ms | ≤ 50ms |
| API 可用性 | ≥ 99.9% | ≥ 99.95% |

---

## 2. API 总览

### 2.1 命名空间

```
https://api.idcd.com
├── /v1
│   ├── /probe/*            一次性拨测 API（同步 / 异步）
│   ├── /diagnose           一键诊断
│   ├── /ip/*               IP 信息
│   ├── /asn/*              ASN 信息
│   ├── /whois/*            WHOIS / RDAP
│   ├── /dns/*              DNS 查询
│   ├── /ssl/*              SSL 证书
│   ├── /icp/*              ICP 备案
│   ├── /report/*           诊断报告
│   ├── /monitors/*         监控管理（CRUD）
│   ├── /alerts/*           告警与事件
│   ├── /channels/*         通知通道
│   ├── /status-pages/*     状态页管理
│   ├── /nodes/*            节点目录
│   ├── /heartbeat/*        心跳接收（M10）
│   ├── /webhook/*          Webhook 接收（反向）
│   └── /me                 当前 token 信息
└── /openapi.json           完整 OpenAPI 3.1 规范
```

### 2.2 公开 vs 私有 API（**决策 D1 锁定**）

| 类别 | 鉴权 | 限速 | 用途 |
|---|---|---|---|
| **自家工具页前端** | Cloudflare Turnstile + 临时 session | 严格 IP 限速 | idcd.com 工具页 |
| **登录用户 API** | session cookie | 用户档位（Free/Pro/Team/Business） | 控制台前端 |
| **API Key 私有 API** | Bearer Token | API Key 档位 | 第三方开发 |
| **特殊公开 endpoint** | 无 | IP 60/min | 仅 `/v1/status/<slug>/*`（状态页公开数据）和 `/v1/nodes`（节点目录） |

> **决策 D1**：第三方开发者**不开放匿名 API**，所有非自家工具页的 API 调用必须有 API Key。
> **例外**：状态页公开数据和节点目录是营销与透明度需求，保持匿名可访问。
> 同一 endpoint 在不同鉴权下限速档不同；endpoint 设计统一。

---

## 3. 鉴权

### 3.1 API Key 格式
```
Authorization: Bearer idc_live_a1b2c3d4e5f6g7h8...
```

或：
```
X-API-Key: idc_live_a1b2c3d4e5f6g7h8...
```

### 3.2 Key 前缀语义
- `idc_live_*` 生产 Key
- `idc_test_*` 测试 Key（沙箱环境，不计费）
- `idc_pub_*` 仅公开 API 用（不含敏感 scope）

### 3.3 OAuth 2.0（S4）
- 用于 OAuth 应用集成（"用 `idcd` 登录" 场景）
- 标准 Authorization Code Flow + PKCE

### 3.4 Webhook 签名
- 反向：我们调用用户 Webhook 时签名
- 正向：用户调用我们的某些回调 endpoint 需带签名
- 算法：HMAC-SHA256

---

## 4. 通用约定

### 4.1 请求

- Content-Type：`application/json`
- 字符编码：UTF-8
- 时间格式：ISO 8601（如 `2026-05-12T15:30:00Z`）
- 分页：`?page=1&page_size=20`（max 200）
- 排序：`?sort=created_at&order=desc`
- 字段筛选：`?fields=id,name,status`

### 4.2 响应

```json
{
  "data": {...},
  "meta": {
    "request_id": "req_xxx",
    "took_ms": 42,
    "rate_limit": {
      "limit": 5000,
      "remaining": 4982,
      "reset": "2026-05-12T16:00:00Z"
    },
    "pagination": { "page": 1, "page_size": 20, "total": 357 }
  }
}
```

错误：
```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Too many requests, please retry after 30s",
    "details": { "retry_after": 30 }
  },
  "meta": { "request_id": "req_xxx" }
}
```

### 4.3 HTTP 状态码

| Code | 含义 |
|---|---|
| 200 | 成功 |
| 201 | 创建成功 |
| 202 | 已接受（异步任务）|
| 204 | 无内容（删除）|
| 400 | 参数错误 |
| 401 | 未鉴权 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 冲突 |
| 422 | 业务规则不通过 |
| 429 | 限速 |
| 500 | 服务器错误 |
| 503 | 服务不可用 |

### 4.4 错误码体系

错误码语义化、稳定：
- `unauthorized`、`invalid_api_key`、`api_key_revoked`
- `forbidden`、`scope_required`
- `validation_error`、`invalid_target`、`target_in_denylist`
- `rate_limit_exceeded`、`quota_exceeded`
- `not_found`、`already_exists`、`conflict`
- `internal_error`、`service_unavailable`

### 4.5 请求 ID
- 每个请求自动生成 `req_xxx`
- 也接受客户端传 `X-Request-ID`
- 出现在响应、错误、日志中（便于排障）

### 4.6 Idempotency Key
- 重要的写操作（创建监控 / 充值 / 退款）支持 `Idempotency-Key` 头
- 同 key 24h 内重复请求只生效一次

### 4.7 版本演进

- 主版本：URL 路径（`/v1` `/v2`）
- 旧版本至少维护 12 个月
- 破坏性变更需提前 90 天公告
- 增量字段：默认不破坏

---

## 5. 拨测 API（一次性）

### 5.1 同步 vs 异步

| 模式 | 适用 |
|---|---|
| 同步 | 简单拨测（HTTP/Ping/DNS），最长 30s 内返回 |
| 异步 | Trace / 长任务 / 多节点大并发：返回 task_id，轮询或 webhook 回调 |

### 5.2 主要 endpoint

#### POST /v1/probe/http
```json
{
  "url": "https://example.com",
  "method": "GET",
  "headers": {},
  "body": "",
  "timeout_ms": 10000,
  "follow_redirect": true,
  "ip_version": "auto",
  "nodes": {
    "mode": "pool",
    "size": 5,
    "regions": ["CN", "US"],
    "isps": ["CT", "CU", "CM"]
  },
  "wait": "sync" | "async",
  "callback_url": "https://my.app/webhook"  // 异步时
}
```

响应（同步）：
```json
{
  "data": {
    "task_id": "pt_xxx",
    "target": "https://example.com",
    "summary": {
      "success_count": 5, "fail_count": 0,
      "avg_response_ms": 234,
      "p95_response_ms": 312
    },
    "node_results": [
      {
        "node_id": "nd_jp_tk_01",
        "node": { "country": "JP", "city": "Tokyo", "isp": "Vultr" },
        "success": true,
        "status_code": 200,
        "response_ms": 198,
        "tls": { "version": "1.3", "cipher": "..." },
        "dns_ms": 12, "tcp_ms": 45, "tls_ms": 78, "ttfb_ms": 60,
        "response_size": 12345,
        "final_url": "https://example.com/",
        "headers": {...}
      },
      ...
    ]
  },
  "meta": {...}
}
```

#### 其他拨测 endpoint（类似结构）
- `POST /v1/probe/ping`
- `POST /v1/probe/tcping`
- `POST /v1/probe/dns`
- `POST /v1/probe/traceroute`
- `POST /v1/probe/mtr`
- `POST /v1/probe/udp` (S2)
- `POST /v1/probe/speedtest` (S2)

#### 异步任务查询
- `GET /v1/probe/tasks/<task_id>`：查询任务状态 + 结果
- `DELETE /v1/probe/tasks/<task_id>`：取消任务（如还在执行）

#### Webhook 回调（异步）
- 完成时 POST 用户提供的 callback_url
- 含签名：`X-Brand-Signature: t=<ts>,v1=<hmac>`

---

## 6. 一键诊断 API

#### POST /v1/diagnose
```json
{
  "target": "example.com",
  "checks": ["dns", "http", "ssl", "ping", "trace", "whois", "icp", "headers"],
  "create_report": true,
  "report_visibility": "private"
}
```

响应（202 异步）：
```json
{
  "data": {
    "diagnosis_id": "dg_xxx",
    "report_id": "r_xxx",
    "report_url": "https://idcd.com/report/r_xxx",
    "status": "running"
  }
}
```

随后 webhook / 轮询拿完整报告。

#### GET /v1/report/<id>
- 完整结构化报告
- 支持 `?format=json | pdf | html | markdown`

---

## 7. 网络信息 API

### 7.1 IP 信息

#### GET /v1/ip/<ip>
```json
{
  "data": {
    "ip": "8.8.8.8",
    "version": 4,
    "asn": 15169,
    "asn_org": "Google LLC",
    "isp": "Google",
    "country": "US",
    "country_code": "US",
    "region": "California",
    "city": "Mountain View",
    "latitude": 37.42, "longitude": -122.08,
    "timezone": "America/Los_Angeles",
    "rir": "ARIN",
    "is_anycast": true,
    "is_vpn": false,
    "is_proxy": false,
    "abuse_contact": "..."
  }
}
```

#### GET /v1/ip/<ip>/whois
- IP 的 RIR WHOIS / RDAP

#### POST /v1/ip/batch
- 批量查询（最多 100 个/次）
- Business 档以上

### 7.2 ASN

- `GET /v1/asn/<asn>` 基础信息
- `GET /v1/asn/<asn>/prefixes` IP 段
- `GET /v1/asn/<asn>/peers` 对等连接

### 7.3 域名

- `GET /v1/whois/<domain>`
- `GET /v1/dns/<domain>?type=A`（或 ALL）
- `GET /v1/ssl?host=<host>&port=443`
- `GET /v1/icp/<domain>` 备案
- `GET /v1/headers?url=<url>` HTTP Header

### 7.4 缓存策略
- 大部分网络信息 API 有缓存（IP 信息 24h / WHOIS 7d / ICP 7d）
- 响应头含 `X-Cache: HIT|MISS` 和 `X-Cache-Age: 1234`
- 用户可加 `?fresh=true` 强制刷新（消耗更多配额）

---

## 8. 监控管理 API（控制台等价）

### 8.1 监控 CRUD

- `GET /v1/monitors` 列表
- `POST /v1/monitors` 创建
- `GET /v1/monitors/<id>` 详情
- `PATCH /v1/monitors/<id>` 修改
- `DELETE /v1/monitors/<id>` 删除
- `POST /v1/monitors/<id>/pause` 暂停
- `POST /v1/monitors/<id>/resume` 恢复
- `POST /v1/monitors/<id>/check_now` 立即检查（不计入历史）

### 8.2 监控数据

- `GET /v1/monitors/<id>/checks?from=...&to=...` 历史拨测
- `GET /v1/monitors/<id>/uptime?days=30` 可用率
- `GET /v1/monitors/<id>/events` 告警事件

### 8.3 批量
- `POST /v1/monitors/batch` 批量创建 / 修改
- `GET /v1/monitors/export?format=csv|json` 批量导出

---

## 9. 告警 API

- `GET /v1/alert-policies` 策略列表
- `POST /v1/alert-policies` 创建
- `GET /v1/alert-events?status=open` 事件列表
- `POST /v1/alert-events/<id>/acknowledge` Ack
- `POST /v1/alert-events/<id>/resolve` 解决
- `GET /v1/channels` 通道列表
- `POST /v1/channels` 添加通道
- `POST /v1/channels/<id>/test` 测试通道

---

## 10. 状态页 API

- `GET /v1/status-pages` 列表
- `POST /v1/status-pages` 创建
- `GET /v1/status-pages/<id>/components`
- `POST /v1/status-pages/<id>/incidents` 创建事件
- `PATCH /v1/status-pages/<id>/incidents/<id>` 更新事件
- `POST /v1/status-pages/<id>/maintenance` 计划维护

公开访问：
- `GET /v1/status/<slug>/summary` 摘要（无需鉴权）
- `GET /v1/status/<slug>/components`
- `GET /v1/status/<slug>/incidents`
- `GET /v1/status/<slug>/uptime?days=90`

---

## 11. 心跳 API（M10）

### POST /v1/heartbeat/<token>

- 用户的客户端定时调用此 URL，告诉 `idcd`"我还活着"
- token 是用户在控制台创建 heartbeat monitor 时生成的
- 不需要 API Key（token 本身是鉴权）
- 限速：单 token 60 次/分钟

可带 payload：
```json
{ "status": "ok", "metrics": { "processed": 12345 } }
```

---

## 12. 节点目录 API

- `GET /v1/nodes` 列表（公开）
- `GET /v1/nodes/<id>` 详情
- `GET /v1/nodes/<id>/health` 健康指标

返回数据按 10-nodes-and-agents.md §7 公开字段策略脱敏。

---

## 12a. Verdict / Attestation API(v2 NEW, 详见 18-evidence §6)

> 独立子域 `attest.idcd.com`,与主 `api.idcd.com` 鉴权 / 计量 / SLA 解耦。本节列出端点摘要;详细 schema 见 16-api-spec.md。

### 12a.1 下单与查询

- `POST /v1/verdict/quote` — 预估价格 + 数据可用性预检
- `POST /v1/verdict/orders` — 创建订单(返回 聚合支付 checkout url)
- `GET  /v1/verdict/orders/<id>` — 订单状态(SSE 推进度可选)
- `GET  /v1/verdict/reports/<id>` — 报告详情 + PDF 下载链接(签名 URL,1 小时有效)
- `POST /v1/verdict/reports/<id>/share` — 生成分享 token(可设过期 / 密码)
- `DELETE /v1/verdict/orders/<id>` — 取消未付款的订单

### 12a.2 公开验签(不需登录)

- `POST attest.idcd.com/verify` — 上传 PDF / JSON 验签,返回 ✅/❌ + 详细信息
- `GET  attest.idcd.com/key/<key_id>` — 查询公钥 + 元数据(JWK / PEM)
- `GET  attest.idcd.com/ceremony` — 密钥仪式公开记录(transparency)

### 12a.3 Compliance 年订 API

- `POST /v1/compliance/subscriptions` — 创建年订
- `GET  /v1/compliance/subscriptions/<id>/reports` — 年订生成的周/月度报告列表
- `POST /v1/compliance/reports/<id>/regenerate` — 触发重新生成(需在配额内)

### 12a.4 申诉

- `POST attest.idcd.com/dispute/<report_id>` — 被报告对象提交申诉
- `GET  attest.idcd.com/disputes` — 公开 transparency 申诉记录

### 12a.5 限速与配额(独立池,不与 v1 共用)

| 档位 | Verdict 件价 / 月 | Verify 公开调用 / IP / min |
|---|---|---|
| 未登录 | — | 30 |
| Free | 0 件价 + 1 测试报告/月 | 60 |
| Pro | 不限件价 | 200 |
| Team | 不限件价 + 5 份免费(每月) | 500 |
| Compliance Starter | + 0 件价免费(¥3k 包月度报告) | 同上 |
| Compliance Pro | + 5 件价/年 免费 | 1000 |
| Compliance Enterprise | + 不限件价 + MCP 优先 | 不限 |

> Verdict 件价**不走 v1 API 配额**;独立计量。所有 `attest.*` 调用进 attestation_record 审计表。

---

## 12b. MCP Server(v2 NEW, 详见 19-ai-agent §3)

> 独立子域 `mcp.idcd.com`,sub-product 阵型;协议为 Anthropic MCP spec(JSON-RPC over stdio + HTTP+SSE)。本节列摘要,详细 schema 见 16-api-spec.md 和 mcp.idcd.com/docs。

### 12b.1 协议

- **stdio 模式**:Cursor / Claude Code / Codex 本地 spawn `idcd-mcp` 二进制,通过 stdin/stdout 通信
- **HTTP+SSE 模式**:`https://mcp.idcd.com/v1/sessions` 创建 session,Server-Sent Events 推 tool call 事件;适合远程 / web 集成

### 12b.2 Tools 列表(详 19 §3.3)

13 个 tool 全量:
- `idcd_ping` / `idcd_http_probe` / `idcd_dns_resolve` / `idcd_traceroute`
- `idcd_ssl_cert` / `idcd_diagnose` / `idcd_ip_info` / `idcd_whois` / `idcd_icp`
- `idcd_create_monitor` / `idcd_check_monitor`
- `idcd_generate_verdict`(配 Verdict 配额或扣余额)
- `idcd_list_nodes`

每个 tool 的 schema 在 mcp.idcd.com/docs;OpenAPI-like 定义。

### 12b.3 鉴权(详 19 §3.4)

- Personal token(1h-7d):用户在控制台手动签发,适合开发者本地
- Workspace token(1-90d):团队管理员签发,绑定 team subscription
- Service account token(长期 + 强制 IP 白名单):生产 Agent 服务

签发 API:
- `POST /v1/mcp/tokens` — 签发(返回 token,只展示一次)
- `DELETE /v1/mcp/tokens/<id>` — 撤销
- `GET  /v1/mcp/tokens` — 列表(展示后 4 位)

### 12b.4 计量(独立)

- 每个 tool call 计 units(详 19 §3.3 计费表)
- 用量查询:`GET /v1/mcp/usage?period=2026-05`
- 80%/95%/100% 三级提醒;用户可设硬上限自动停服

### 12b.5 Agent Observability API

- `POST /v1/agent-obs/monitors` — 创建 Agent obs 监控(M21/M22/M23)
- `GET  /v1/agent-obs/events?monitor_id=...&since=...` — SSE 事件流
- `GET  /v1/agent-obs/monitors/<id>/stats` — 统计聚合

### 12b.6 客户端兼容(详 19 §3.6)

每发布前 smoke test:
- Claude Code(stdio + http+sse)
- Cursor(stdio)
- Codex CLI(stdio)
- 自家 SDK:`idcd-mcp-py`(pypi)/ `idcd-mcp-ts`(npm),MIT 开源

---

## 13. 限速与配额

### 13.1 配额维度（按 API Key）

| 档位 | 日配额 | 月配额 | 秒级峰值 |
|---|---|---|---|
| Free | 100 | 3,000 | 10 |
| Pro | 5,000 | 150,000 | 30 |
| Team | 30,000 | 900,000 | 100 |
| Business | 200,000 | 6,000,000 | 300 |

### 13.2 不同 endpoint 权重

| Endpoint | 权重 |
|---|---|
| `/v1/ip/*`、`/v1/whois/*`（缓存命中） | 0.1 |
| `/v1/ip/*`（缓存未命中） | 1 |
| `/v1/probe/ping`、`/probe/tcping` | 2 |
| `/v1/probe/http` | 3 |
| `/v1/probe/traceroute`、`/probe/mtr` | 5 |
| `/v1/diagnose`（完整检查） | 20 |
| `/v1/probe/speedtest` | 30 |
| `/v1/monitors/*` 管理类 | 1 |

> 配额扣减 = 调用次数 × 权重

### 13.3 限速返回
- HTTP 429
- `Retry-After` 头
- `X-RateLimit-Limit/Remaining/Reset` 头
- `X-Quota-Limit/Remaining/Reset-Date`（月配额）

### 13.4 突发与平滑
- 令牌桶：允许小突发（burst = 2× rate）
- 持续超限 → 严格阻断

### 13.5 软超额（按量付费）
- 用户开启"允许超额按量"：
  - 月配额用完 → 自动按 ¥0.5 / 1k 次计费扣余额
  - 余额不足 → 返 429 quota_exceeded

---

## 14. OpenAPI 文档站

### 14.1 入口
- `docs.idcd.com` 子域
- Swagger / Redoc / Mintlify 风格（选 Mintlify，最美）

### 14.2 内容结构

```
Quick Start
  Authentication
  First Request
  SDK 安装
Endpoints
  按命名空间展开
  每个 endpoint 含：
    - 描述、参数、响应示例、错误码
    - "Try it" 交互式调试（用沙箱 Key）
    - 多语言代码示例：curl / JS / Go / Python / Java
Guides
  错误处理
  分页
  Webhook 接收
  幂等性
  批量请求
SDK Reference
  JS / Go / Python 三套
CLI Reference
  全命令文档
Changelog
  版本变更记录
Migration
  v1 → v2 迁移指南（远期）
Status
  跳转到 status.idcd.com
```

### 14.3 沙箱
- 文档内置"Try it"按钮 → 用临时 demo Key 调用
- 沙箱环境（test_ Key）：不计费、不消耗节点资源
- 沙箱有限制（不能调用真实 target，返回固定 mock）

### 14.4 OpenAPI 规范文件
- `GET /openapi.json` 完整 3.1 规范
- 公开，可用工具自动生成 client

---

## 15. SDK

### 15.1 官方 SDK（S3）

| 语言 | 主要场景 |
|---|---|
| JavaScript / TypeScript | 前端 + Node.js |
| Go | 后端 / CLI / 集成 |
| Python | 脚本 / 数据 |
| Java / Kotlin | 企业 |
| PHP（远期） | LAMP 站长 |

### 15.2 SDK 设计原则

- 自动重试（指数退避）
- 自动分页（迭代器）
- 错误类型化（区分网络错误 / 业务错误）
- 透明限速（429 → 等 Retry-After 后重试）
- Webhook 验证 helper

### 15.3 例：JavaScript

```typescript
import { Client } from '@idcd/sdk';

const client = new Client({ apiKey: 'idc_live_xxx' });

// 一次性拨测
const result = await client.probe.http({
  url: 'https://example.com',
  nodes: { mode: 'pool', size: 5 }
});

// 监控管理
const monitor = await client.monitors.create({
  name: 'My API',
  type: 'http',
  url: 'https://api.my.com/health',
  interval: 60
});

// 异步任务 + Webhook
const task = await client.probe.diagnose(
  { target: 'example.com' },
  { async: true, callback: 'https://my.app/webhook' }
);

// 自动分页
for await (const event of client.alertEvents.list({ status: 'open' })) {
  console.log(event);
}
```

### 15.4 例：Go

```go
client := brand.NewClient(brand.WithAPIKey("idc_live_xxx"))

result, err := client.Probe.HTTP(ctx, &brand.HTTPProbeRequest{
    URL: "https://example.com",
    Nodes: &brand.NodeSelection{Mode: "pool", Size: 5},
})

monitor, err := client.Monitors.Create(ctx, &brand.MonitorCreate{
    Name: "My API",
    Type: "http",
    URL:  "https://api.my.com/health",
    Interval: 60,
})
```

### 15.5 包管理
- npm `@idcd/sdk`
- Go module
- PyPI

---

## 16. CLI 工具

### 16.1 目标

- 类似 `gp`（Globalping CLI）：开发者直接终端拨测
- 兼具：管理监控、查事件、看节点

### 16.2 安装

```
# Homebrew (macOS)
brew install idcd/tap/cli

# Scoop (Windows)
scoop bucket add idcd ... && scoop install idcd

# curl 一行
curl -fsSL https://get.idcd.com | sh

# Docker
docker run idcd/cli ping example.com
```

### 16.3 命令

```bash
# 认证
idcd login              # 浏览器跳转 OAuth
idcd auth status

# 拨测
idcd ping example.com --from CN,US
idcd http example.com --method GET --from JP
idcd dns example.com --type A --from CN
idcd diagnose example.com
idcd mtr 1.1.1.1 --from US-west

# 监控管理
idcd monitors list
idcd monitors create --name "API" --url ... --interval 60
idcd monitors pause <id>
idcd monitors delete <id>

# 事件
idcd alerts list --status open
idcd alerts ack <event_id>

# 节点
idcd nodes list
idcd nodes info nd_jp_tk_01

# 报告
idcd report show <report_id> --format pdf > report.pdf

# 配置
idcd config set api_key <key>
idcd config set output json
```

### 16.4 输出格式
- 默认彩色 table（终端美观）
- `--output json | yaml | csv`
- `--quiet`、`--verbose`

---

## 17. Webhook（反向）

### 17.1 我们调用用户 Webhook 的场景

- 异步拨测完成
- 监控告警 / 恢复
- 状态页事件创建
- 订阅 / 账单事件
- API Key 异常 / 撤销

### 17.2 Payload 结构

```json
{
  "id": "evt_xxx",
  "type": "monitor.down",
  "created_at": "2026-05-12T15:00:00Z",
  "data": {
    "monitor": { "id": "...", "name": "..." },
    "event": { "id": "...", "type": "down", "started_at": "..." }
  },
  "metadata": { "api_version": "v1" }
}
```

### 17.3 签名

```
X-Brand-Event-ID: evt_xxx
X-Brand-Signature: t=1715515200,v1=<hmac_sha256(t.body, secret)>
```

接收方验证：
```
expected = HMAC-SHA256(secret, "{t}.{raw_body}")
actual   = v1
```

5 分钟过期 + 签名比对（防重放）。

### 17.4 重试

- 5xx / 超时 / 网络错误：重试
- 重试策略：5s, 30s, 2m, 10m, 1h, 6h（共 6 次）
- 4xx：不重试（除 408 / 429）
- 重试记录展示在通道详情页

### 17.5 死信
- 6 次失败 → 死信队列
- 用户可手动"重发"

---

## 18. 计量与计费集成

### 18.1 调用计量

每个 API 调用记录：
```
usage_event:
  ts, api_key_id, owner_id,
  endpoint, http_method,
  weight, status_code,
  duration_ms, response_bytes
```

### 18.2 实时聚合
- 秒级：用于限速
- 分钟级：用于看板
- 小时级：用于配额展示
- 日级：用于计费

### 18.3 用户可视化
- `/app/api-keys/<id>` 详情：调用曲线、错误率、TOP endpoint
- `/app/billing/usage` 全局用量

### 18.4 计费集成
- 每月 1 号根据上月用量生成账单
- 超额费用自动从余额扣
- 详见 09-billing.md

---

## 19. 关键流程

### 19.1 第三方开发者集成（一次性拨测）

```
开发者注册 → 创建 API Key (scope: probe) → 复制
  → 在自家代码中 POST /v1/probe/http
       Header: Authorization: Bearer idc_live_xxx
  → 收到结果（同步）或 task_id（异步）
  → 异步：监听 webhook 或轮询 task_id
  → 拿结果展示在自家产品
```

### 19.2 通过 API 创建监控

```
脚本 / Terraform / CI / CD → POST /v1/monitors
  → 校验 scope: monitor:write
  → 校验配额（监控数 + 频率）
  → 创建 monitor
  → 立即返回 monitor 对象（含 id）
  → 后台异步触发首次检查
```

### 19.3 心跳监控集成（M10）

```
用户在控制台创建 heartbeat monitor → 拿 URL：
  https://api.idcd.com/v1/heartbeat/hb_xxx

用户在自家 cron / batch / IoT 设备中：
  - 每天凌晨 2:00 跑完任务后 curl 该 URL
  - idcd 收到后更新 monitor.last_heartbeat_at
  - 若超过期望间隔（如 24h + 宽限期）未收到 → 异常 → 告警
```

---

## 20. 数据模型

```
api_request_log
  id, ts, api_key_id, owner_id,
  endpoint, method,
  request_id, idempotency_key,
  client_ip, user_agent,
  status_code, weight, latency_ms,
  response_bytes, error_code

api_quota_usage
  owner_id, period (day|month),
  bucket_at, weighted_total

webhook_endpoint
  id, owner_id, name, url, secret,
  events (text[]),
  is_active, last_delivery_at, last_status

webhook_delivery
  id, endpoint_id, event_id, event_type,
  attempt, request_payload, response_status,
  response_body, latency_ms,
  next_retry_at, delivered_at, failed_at

sandbox_key
  id, user_id, prefix, created_at, expires_at
```

---

## 21. 安全要点（与 12 合规模块呼应）

- API Key 哈希存储（创建后不可恢复）
- IP / Origin 白名单
- API Key 出现在 GitHub 公开 repo → 自动失活 + 通知
- SSRF 防护：所有用户控制的 URL / host 经过严校
- 高敏目标 + 高敏 endpoint 单独限速更严
- 异常行为模型（流量突变、地理突变）触发降级

---

## 22. 与其他模块接口

| 模块 | 接口 |
|---|---|
| `02-public-tools.md` | 公开 API 实现一致（同 endpoint） |
| `03-account-system.md` | API Key 来自账号系统 |
| `04-monitoring.md` | 监控管理 API |
| `05-alerting.md` | 告警事件 API |
| `06-status-pages.md` | 状态页 API |
| `09-billing.md` | 配额 + 计费 |
| `10-nodes-and-agents.md` | 节点目录 API |
| `12-compliance-and-abuse.md` | 限速 + SSRF + 滥用检测 |

---

## 23. 阶段交付清单

### S2（4–8 月）
- v1 API：所有公开工具 + 监控管理 CRUD + 告警事件 + 状态页 + 节点目录
- API Key 系统（管理 + 限速 + 配额）
- 心跳监控接收
- 异步任务 + Webhook 回调
- OpenAPI 文档站（Mintlify 风格）
- "Try it" 交互式调试
- 沙箱环境（test Key）
- 计量与配额展示
- **v2 NEW: attest.idcd.com 独立子域 + Verdict / Attestation API**
- **v2 NEW: 公开验签接口 /verify**
- **v2 NEW: Compliance 年订 API**

### S3（8–14 月）
- 官方 SDK：JS / Go / Python
- CLI 工具完整
- Webhook 接收端（用户给我们 webhook，反向）
- 批量 endpoint（/ip/batch、/monitors/batch）
- "按量纯付费"档（开发者只用 API 不订阅）
- API 用量异常告警
- **v2 NEW: mcp.idcd.com 独立子域 + MCP Server alpha (M9-M11) → GA (M12)**
- **v2 NEW: MCP 13 tools 全量 + 三档 token + Agent obs API**
- **v2 NEW: 自家 MCP SDK: idcd-mcp-py + idcd-mcp-ts(MIT 开源)**
- **v2 NEW: 提交 Anthropic MCP gallery**

### S4（14+ 月）
- v2 API（如有破坏性变更）
- OAuth 2.0（应用集成）
- 大客户专属 endpoint
- 私有 API 网关（企业自部署）
- **v2 NEW: OpenAI Agents Protocol 接入(视市场份额评估)**
- **v2 NEW: 白标 Attestation API + 白标 MCP server**

---

## 24. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| API Key 泄露 → 滥用 | GitHub 扫描自动失活 + 异常模式告警 + Origin / IP 白名单 |
| 文档与代码不同步 | 自动生成 OpenAPI + CI 校验 + Mintlify 自动部署 |
| SDK 版本碎片化 | 严格语义化版本 + LTS 维护策略 |
| 限速过严影响体验 | 配额可观测 + 按量软超额 + 升级引导 |
| 沙箱被恶意用 | 沙箱内置 mock，不能调真实 target |
| API 滥用做 SSRF | 强 SSRF 防护 + 黑名单 + Origin 检查 |
| **v2 NEW: MCP token 泄露(Cursor 配置被偷)** | 短期 token + IP 白名单 + 异常突增告警 + 一键撤销(详 12 §22) |
| **v2 NEW: MCP 调用计量与主 API 配额混淆** | 独立计量池 + 用户控制台分开展示 + Agent Pro 独立档 |
| **v2 NEW: Verdict API 被用做"竞品诬告"** | 目标黑名单 + 所有权验证 + 滥用申诉通道(详 12 §3.5) |
| **v2 NEW: Verify API 被滥用(海量伪造 PDF 验签耗资源)** | IP 限速 + Cloudflare WAF + 自家轻量服务独立部署不阻塞主 Attestation |

---

## 25. 决策记录（已锁定，见 DECISIONS.md）

### v1.0
- ✅ **D1** 匿名 API：**不开放**（仅状态页 + 节点目录例外）
- ✅ **D2** CLI 形态：**仅命令行**（仿 gh / gp），不做 TUI
- ✅ **D3** 沙箱 test Key：**计入日志**（不计费，便于排障）
- ✅ **D4** Webhook 重试：**6 次**（5s/30s/2m/10m/1h/6h）
- ✅ **B3** 文档站：**自建 Nextra / Fumadocs**
- ✅ **C5** 按量纯付费档：**S3 推出**

### v2.0 (K 节, 2026-05-12)
- ✅ **K1** Verdict / Attestation API:独立子域 attest.idcd.com,与 v1 API 鉴权 / 计量 / SLA 分离
- ✅ **K1** MCP Server:独立子域 mcp.idcd.com,sub-product 阵型
- ✅ **K3** MCP 鉴权三档:personal / workspace / service,service 强制 IP 白名单
- ✅ **K-API verify 公开**:不需登录,独立轻量部署,纯只读
- ✅ **K-API 调用计量独立**:Verdict 件价 / Compliance 年订 / MCP units 三套独立计量

### 待定（不紧迫）

- [ ] GraphQL endpoint：S4 评估
- [ ] **v2 NEW** OpenAI Agents Protocol 接入:S3 末根据市场份额评估
- [ ] **v2 NEW** 白标 MCP server(企业自家域名):S4 评估
