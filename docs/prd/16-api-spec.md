# 16 · API 详细规范（OpenAPI 总览）

> 关联：08-open-api.md（产品视角）、15-data-model.md（数据视角）、14-tech-architecture.md（架构）
> 阶段：S2 v1 beta，S3 v1 GA
> 品牌名占位：`idcd`

---

## 1. 文档定位

08-open-api 讲"做什么、怎么定价、为何这么设计"，本文件讲：

1. **规范层面**：HTTP 约定、错误码、版本、限速、签名等可机器化的规则
2. **接口清单**：所有 endpoint 的 path / method / scope / 限速权重 / 错误码
3. **完整 schema**：request / response 字段（OpenAPI yaml 是 source of truth，本文是人类可读总览）
4. **Webhook event 完整目录**

实际 OpenAPI 3.1 yaml 文件维护在 `packages/api-spec/openapi.yaml`，自动生成文档站 `docs.idcd.com` 和 SDK。

---

## 2. 通用约定（细化）

### 2.1 Base URL

```
https://api.idcd.com         生产
https://api-staging.idcd.com 预生产
```

### 2.2 版本

- 主版本 URL 路径：`/v1`、`/v2`
- 旧版本至少维护 12 个月（announced via `Sunset` header + 文档）
- 同版本内字段只增不删（删除算 breaking change）

### 2.3 鉴权

| 方式 | Header | 适用 |
|---|---|---|
| API Key | `Authorization: Bearer idc_live_xxx` 或 `X-API-Key: idc_live_xxx` | 第三方开发 |
| Session Cookie | `Cookie: session=xxx` | 控制台前端 |
| Webhook 接收签名 | `X-Brand-Signature: t=<ts>,v1=<hmac>` | 反向 webhook |
| Heartbeat token | URL path `/v1/heartbeat/<token>` | 心跳监控 |

### 2.4 Headers 约定

**请求头**
- `Content-Type: application/json`
- `Accept: application/json`
- `Idempotency-Key`：写操作幂等（可选）
- `X-Request-ID`：客户端追踪 ID（可选，否则服务端生成）

**响应头**
- `X-Request-ID`：本次请求 ID
- `X-Cache: HIT|MISS`：缓存状态
- `X-Cache-Age`：缓存时长（秒）
- `X-RateLimit-Limit/Remaining/Reset`：限速窗口
- `X-Quota-Limit/Remaining/Reset-Date`：月配额
- `Retry-After`：429/503 时建议等待秒数
- `Sunset: <RFC3339>`：本 endpoint / 版本将下线时间（可选）
- `Deprecation: true`：endpoint 已废弃

### 2.5 响应包络

```json
{
  "data": <object|array|null>,
  "meta": {
    "request_id": "req_xxx",
    "took_ms": 42,
    "rate_limit": { "limit": 5000, "remaining": 4982, "reset": "2026-05-12T16:00:00Z" },
    "pagination": { "page": 1, "page_size": 20, "total": 357, "has_more": true },
    "cache": { "hit": true, "age_sec": 1234 }
  }
}
```

错误：
```json
{
  "error": {
    "code": "string_error_code",
    "message": "人类可读的错误描述",
    "details": { /* 字段级错误等 */ },
    "doc_url": "https://docs.idcd.com/errors/<code>"
  },
  "meta": { "request_id": "req_xxx" }
}
```

### 2.6 分页

#### offset / page 模式（默认）
```
GET /v1/monitors?page=1&page_size=20&sort=created_at&order=desc
```

#### cursor 模式（高基数列表，避免 offset 性能问题）
```
GET /v1/monitor-checks?cursor=eyJ0cyI6...&limit=100
```

返回中：
```json
"meta": {
  "pagination": {
    "next_cursor": "eyJ...",
    "has_more": true
  }
}
```

### 2.7 字段筛选

```
GET /v1/monitors?fields=id,name,status
```

只返回指定字段；嵌套用 `fields=id,owner.id,owner.email`。

### 2.8 时间格式
- 所有时间：ISO 8601 UTC，带 `Z`：`2026-05-12T15:30:00Z`
- 时间区间：`?from=2026-05-01T00:00:00Z&to=2026-05-12T00:00:00Z`
- 也支持相对：`?from=-7d` `?from=-1h`

### 2.9 错误码体系（完整）

| Code | HTTP | 含义 |
|---|---|---|
| `unauthorized` | 401 | 未提供有效凭证 |
| `invalid_api_key` | 401 | API Key 格式错或不存在 |
| `api_key_revoked` | 401 | API Key 已撤销 |
| `api_key_expired` | 401 | API Key 已过期 |
| `forbidden` | 403 | 权限不足 |
| `scope_required` | 403 | 缺少特定 scope |
| `validation_error` | 400 | 字段校验失败（details 含具体）|
| `invalid_target` | 400 | target URL / IP 格式错 |
| `target_in_denylist` | 422 | target 命中拒测黑名单 |
| `target_unresolvable` | 422 | target 域名无法解析 |
| `target_high_risk` | 422 | target 命中敏感名单，需二次确认 |
| `unsupported_protocol` | 400 | 不支持的协议 |
| `not_found` | 404 | 资源不存在 |
| `already_exists` | 409 | 资源已存在 |
| `conflict` | 409 | 资源冲突（并发修改等） |
| `gone` | 410 | 资源已删除 |
| `precondition_failed` | 412 | If-Match 等失败 |
| `request_too_large` | 413 | body 超 32KB |
| `unsupported_media` | 415 | Content-Type 不支持 |
| `rate_limit_exceeded` | 429 | 触发限速 |
| `quota_exceeded` | 429 | 配额耗尽（含 X-Quota） |
| `idempotency_key_mismatch` | 422 | 同 key 不同 payload |
| `payment_required` | 402 | 需付费档位（功能限制） |
| `internal_error` | 500 | 服务端错误 |
| `service_unavailable` | 503 | 维护 / 过载 |
| `node_unavailable` | 503 | 选定节点不可用 |
| `gateway_timeout` | 504 | 节点超时 |

`details` 字段示例：
```json
{
  "code": "validation_error",
  "message": "Invalid request",
  "details": {
    "fields": [
      { "field": "url", "code": "url_format", "message": "Not a valid URL" },
      { "field": "timeout_ms", "code": "range", "message": "Must be 1000-30000" }
    ]
  }
}
```

---

## 3. 限速与配额（细化）

### 3.1 端点权重（与 08 §13.2 同步）

| Endpoint pattern | 权重 |
|---|---|
| `GET /v1/ip/*` (cache hit) | 0.1 |
| `GET /v1/ip/*` (cache miss) | 1 |
| `GET /v1/whois/*`, `/dns/*`, `/icp/*` (cache hit) | 0.1 |
| `GET /v1/whois/*` (cache miss) | 2 |
| `POST /v1/probe/ping`, `/tcping` | 2 |
| `POST /v1/probe/http` | 3 |
| `POST /v1/probe/dns` | 1 |
| `POST /v1/probe/traceroute`, `/mtr` | 5 |
| `POST /v1/probe/speedtest` | 30 |
| `POST /v1/diagnose` | 20 |
| `POST /v1/probe/udp` | 3 |
| `GET /v1/monitors/<id>/checks` | 1 |
| 控制面 CRUD（monitor/alert/channel） | 1 |
| `POST /v1/heartbeat/<token>` | 0.5 |

### 3.2 限速维度

```
Anonymous:
  per IP: 30 req/hour, 5 req/min
  + Turnstile 校验通过

Logged-in (no API key):
  per user: 按订阅档

API Key:
  per key per second: 按档位
  per key per minute: 60×秒级
  per key per day: 配额
  per key per month: 月配额
```

### 3.3 限速命中响应

```
HTTP 429
Retry-After: 30
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 2026-05-12T15:31:00Z

{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Too many requests",
    "details": { "retry_after": 30, "scope": "api_key", "limit_dimension": "per_minute" }
  }
}
```

---

## 4. 详细 endpoint 清单

> 完整 schema 见 OpenAPI yaml。本节列出 method / path / scope / 描述 / 权重。

### 4.1 Account & Auth

| Method | Path | Scope | Weight | 描述 |
|---|---|---|---|---|
| POST | `/v1/auth/register` | — | 1 | 邮箱注册 |
| POST | `/v1/auth/login` | — | 1 | 登录 |
| POST | `/v1/auth/logout` | — | 1 | 登出 |
| POST | `/v1/auth/refresh` | — | 1 | Refresh token |
| POST | `/v1/auth/password/forgot` | — | 1 | 发起密码重置 |
| POST | `/v1/auth/password/reset` | — | 1 | 重置密码 |
| POST | `/v1/auth/email/verify` | — | 1 | 邮箱验证 |
| POST | `/v1/auth/2fa/enable` | — | 1 | 启用 2FA |
| POST | `/v1/auth/2fa/disable` | — | 1 | 关闭 2FA |
| GET | `/v1/me` | * | 0.1 | 当前用户 |
| PATCH | `/v1/me` | — | 1 | 更新资料 |
| GET | `/v1/me/sessions` | — | 0.1 | 会话列表 |
| DELETE | `/v1/me/sessions/<id>` | — | 1 | 撤销会话 |

### 4.2 API Keys

| Method | Path | Scope | Weight |
|---|---|---|---|
| GET | `/v1/api-keys` | — | 0.1 |
| POST | `/v1/api-keys` | — | 1 |
| GET | `/v1/api-keys/<id>` | — | 0.1 |
| PATCH | `/v1/api-keys/<id>` | — | 1 |
| DELETE | `/v1/api-keys/<id>` | — | 1 |
| GET | `/v1/api-keys/<id>/usage` | — | 0.5 |

### 4.3 Teams

| Method | Path | Scope |
|---|---|---|
| GET | `/v1/teams` | — |
| POST | `/v1/teams` | — |
| GET | `/v1/teams/<id>` | admin |
| PATCH | `/v1/teams/<id>` | admin |
| DELETE | `/v1/teams/<id>` | admin |
| GET | `/v1/teams/<id>/members` | admin |
| POST | `/v1/teams/<id>/invitations` | admin |
| DELETE | `/v1/teams/<id>/members/<user_id>` | admin |

### 4.4 Probe（一次性拨测）

| Method | Path | Scope | Weight |
|---|---|---|---|
| POST | `/v1/probe/http` | probe | 3 |
| POST | `/v1/probe/ping` | probe | 2 |
| POST | `/v1/probe/tcping` | probe | 2 |
| POST | `/v1/probe/dns` | probe | 1 |
| POST | `/v1/probe/traceroute` | probe | 5 |
| POST | `/v1/probe/mtr` | probe | 5 |
| POST | `/v1/probe/udp` | probe | 3 |
| POST | `/v1/probe/speedtest` | probe | 30 |
| POST | `/v1/probe/websocket` | probe | 3 |
| GET | `/v1/probe/tasks/<id>` | probe:read | 0.1 |
| DELETE | `/v1/probe/tasks/<id>` | probe | 1 |

### 4.5 Diagnose 与报告

| Method | Path | Scope | Weight |
|---|---|---|---|
| POST | `/v1/diagnose` | probe | 20 |
| GET | `/v1/diagnose/<id>` | probe:read | 0.5 |
| GET | `/v1/report/<id>` | — | 0.5 |
| GET | `/v1/report/<id>.pdf` | — | 5 |
| DELETE | `/v1/report/<id>` | — | 1 |

### 4.6 网络信息查询

| Method | Path | Scope | Weight |
|---|---|---|---|
| GET | `/v1/ip/<ip>` | — | 0.1-1 |
| GET | `/v1/ip/<ip>/whois` | — | 1 |
| POST | `/v1/ip/batch` | — | 0.5×N |
| GET | `/v1/asn/<asn>` | — | 0.5 |
| GET | `/v1/asn/<asn>/prefixes` | — | 1 |
| GET | `/v1/whois/<domain>` | — | 0.1-2 |
| GET | `/v1/dns/<domain>` | — | 0.5 |
| GET | `/v1/dns/<domain>/<type>` | — | 0.5 |
| GET | `/v1/ssl?host=<host>&port=443` | — | 1 |
| GET | `/v1/icp/<domain>` | — | 0.1-1 |
| GET | `/v1/headers?url=<url>` | — | 1 |
| GET | `/v1/security-headers?url=<url>` | — | 1 |

### 4.7 Monitors

| Method | Path | Scope |
|---|---|---|
| GET | `/v1/monitors` | monitor:read |
| POST | `/v1/monitors` | monitor:write |
| GET | `/v1/monitors/<id>` | monitor:read |
| PATCH | `/v1/monitors/<id>` | monitor:write |
| DELETE | `/v1/monitors/<id>` | monitor:write |
| POST | `/v1/monitors/<id>/pause` | monitor:write |
| POST | `/v1/monitors/<id>/resume` | monitor:write |
| POST | `/v1/monitors/<id>/check-now` | monitor:write |
| GET | `/v1/monitors/<id>/checks` | monitor:read |
| GET | `/v1/monitors/<id>/uptime?days=30` | monitor:read |
| GET | `/v1/monitors/<id>/events` | monitor:read |
| POST | `/v1/monitors/batch` | monitor:write |
| GET | `/v1/monitors/export?format=csv` | monitor:read |

### 4.8 Alerting

| Method | Path | Scope |
|---|---|---|
| GET | `/v1/alert-policies` | alert:read |
| POST | `/v1/alert-policies` | alert:write |
| GET/PATCH/DELETE | `/v1/alert-policies/<id>` | alert:read / write |
| GET | `/v1/alert-events` | alert:read |
| GET | `/v1/alert-events/<id>` | alert:read |
| POST | `/v1/alert-events/<id>/acknowledge` | alert:write |
| POST | `/v1/alert-events/<id>/resolve` | alert:write |
| POST | `/v1/alert-events/<id>/comments` | alert:write |
| GET | `/v1/channels` | alert:read |
| POST | `/v1/channels` | alert:write |
| GET/PATCH/DELETE | `/v1/channels/<id>` | alert:read / write |
| POST | `/v1/channels/<id>/test` | alert:write |

### 4.9 Status Pages

| Method | Path | Scope |
|---|---|---|
| GET | `/v1/status-pages` | status:read |
| POST | `/v1/status-pages` | status:write |
| GET/PATCH/DELETE | `/v1/status-pages/<id>` | status |
| POST | `/v1/status-pages/<id>/components` | status:write |
| POST | `/v1/status-pages/<id>/incidents` | status:write |
| PATCH | `/v1/status-pages/<id>/incidents/<id>` | status:write |
| POST | `/v1/status-pages/<id>/maintenance` | status:write |
| GET | `/v1/status-pages/<id>/subscribers` | status:read |
| 公开：|||
| GET | `/v1/status/<slug>/summary` | — |
| GET | `/v1/status/<slug>/components` | — |
| GET | `/v1/status/<slug>/incidents` | — |
| GET | `/v1/status/<slug>/uptime?days=90` | — |

### 4.10 Reports & Dashboards

| Method | Path | Scope |
|---|---|---|
| GET | `/v1/reports/sla?period=2026-04` | — |
| POST | `/v1/reports/export` | — |
| GET | `/v1/dashboards` | — |
| POST | `/v1/dashboards` | — |
| GET/PATCH/DELETE | `/v1/dashboards/<id>` | — |

### 4.11 Heartbeat

| Method | Path |
|---|---|
| POST | `/v1/heartbeat/<token>` |

### 4.12 Nodes（公开）

| Method | Path |
|---|---|
| GET | `/v1/nodes` |
| GET | `/v1/nodes/<id>` |
| GET | `/v1/nodes/<id>/health` |

### 4.13 Webhooks（接收方）

| Method | Path |
|---|---|
| GET | `/v1/webhook-endpoints` |
| POST | `/v1/webhook-endpoints` |
| GET/PATCH/DELETE | `/v1/webhook-endpoints/<id>` |
| POST | `/v1/webhook-endpoints/<id>/test` |
| GET | `/v1/webhook-endpoints/<id>/deliveries` |
| POST | `/v1/webhook-deliveries/<id>/redeliver` |

### 4.14 Billing

| Method | Path | Scope |
|---|---|---|
| GET | `/v1/billing/subscription` | billing:read |
| POST | `/v1/billing/subscription` | billing:write |
| POST | `/v1/billing/subscription/cancel` | billing:write |
| GET | `/v1/billing/orders` | billing:read |
| GET | `/v1/billing/orders/<id>` | billing:read |
| POST | `/v1/billing/orders/<id>/refund` | billing:write |
| GET | `/v1/billing/invoices` | billing:read |
| POST | `/v1/billing/invoices` | billing:write |
| GET | `/v1/billing/credits` | billing:read |
| POST | `/v1/billing/credits/topup` | billing:write |
| GET | `/v1/billing/usage` | billing:read |
| GET | `/v1/billing/coupons/<code>` | — |

### 4.15 Plans（公开）

| Method | Path |
|---|---|
| GET | `/v1/plans` |
| GET | `/v1/plans/<code>` |

---

## 5. 关键 schema 示例

### 5.1 HTTP Probe Request

```yaml
type: object
required: [url]
properties:
  url:           { type: string, format: uri }
  method:        { type: string, enum: [GET,HEAD,POST,PUT,DELETE,OPTIONS], default: GET }
  headers:       { type: object, additionalProperties: { type: string } }
  body:          { type: string, maxLength: 32768 }
  timeout_ms:    { type: integer, minimum: 1000, maximum: 30000, default: 10000 }
  follow_redirect: { type: boolean, default: true }
  ip_version:    { type: string, enum: [v4,v6,auto], default: auto }
  nodes:
    type: object
    properties:
      mode:        { type: string, enum: [pool,fixed], default: pool }
      size:        { type: integer, minimum: 1, maximum: 50, default: 3 }
      regions:     { type: array, items: { type: string } }
      isps:        { type: array, items: { type: string } }
      tags:        { type: array, items: { type: string } }
      ids:         { type: array, items: { type: string } }  # mode=fixed
  wait:          { type: string, enum: [sync,async], default: sync }
  callback_url:  { type: string, format: uri }  # async 必填
```

### 5.2 HTTP Probe Response

```yaml
type: object
properties:
  task_id:       { type: string, example: "pt_xyz" }
  status:        { type: string, enum: [running,completed,failed] }
  target:        { type: string }
  summary:
    type: object
    properties:
      success_count:    { type: integer }
      fail_count:       { type: integer }
      avg_response_ms:  { type: number }
      p95_response_ms:  { type: number }
  node_results:
    type: array
    items:
      type: object
      properties:
        node_id:       { type: string }
        node:
          type: object
          properties:
            country:   { type: string }
            city:      { type: string }
            isp:       { type: string }
            asn:       { type: integer }
        success:       { type: boolean }
        status_code:   { type: integer }
        response_ms:   { type: integer }
        timing:
          type: object
          properties:
            dns_ms:    { type: integer }
            tcp_ms:    { type: integer }
            tls_ms:    { type: integer }
            ttfb_ms:   { type: integer }
            download_ms: { type: integer }
        tls:
          type: object
          properties:
            version:   { type: string }
            cipher:    { type: string }
        response_size: { type: integer }
        final_url:     { type: string }
        headers:       { type: object }
        error:         { type: string, nullable: true }
```

### 5.3 Monitor Create Request

```yaml
type: object
required: [name, type, target, interval_sec]
properties:
  name:          { type: string, maxLength: 200 }
  type:          { type: string, enum: [http,ping,tcping,dns,ssl,domain,icp,keyword,json,heartbeat,browser] }
  target:        { type: string }
  params:        { type: object }
  interval_sec:  { type: integer, enum: [10,30,60,300,900] }
  node_selection: { $ref: '#/components/schemas/NodeSelection' }
  assertions:    { type: array, items: { $ref: '#/components/schemas/Assertion' } }
  trigger_rule:
    type: object
    properties:
      consecutive_fail:  { type: integer, default: 2 }
      fail_node_quorum:  { type: string, default: "2/3" }
  alert_policy_id: { type: string, nullable: true }
  tags:          { type: array, items: { type: string } }
  group_id:      { type: string, nullable: true }
```

### 5.4 Alert Policy

```yaml
type: object
properties:
  id: { type: string }
  name: { type: string }
  rules:
    type: array
    items:
      type: object
      properties:
        match:
          type: object
          properties:
            severity: { type: array, items: { type: string, enum: [critical,warning,info] } }
            tags_any: { type: array, items: { type: string } }
            time_window:
              type: object
              properties:
                timezone: { type: string }
                days:     { type: array, items: { type: string } }
                start:    { type: string, pattern: "^\\d{2}:\\d{2}$" }
                end:      { type: string }
        channels:
          type: array
          items:
            type: object
            properties:
              type:    { type: string }
              channel_id: { type: string }
              to:      { type: array, items: { type: string } }
        delay_sec: { type: integer, default: 0 }
        repeat:
          type: object
          properties:
            enabled:     { type: boolean }
            interval_sec: { type: integer }
            max_count:   { type: integer }
  escalation:    { type: object }
  suppression:   { type: object }
  on_recovery:   { type: object }
```

### 5.5 Webhook Event Payload

```yaml
type: object
required: [id, type, created_at, data]
properties:
  id:        { type: string, example: "evt_xyz" }
  type:      { type: string, example: "monitor.down" }
  created_at: { type: string, format: date-time }
  data:      { type: object }
  metadata:
    type: object
    properties:
      api_version: { type: string, example: "v1" }
      delivery_id: { type: string }
      attempt:     { type: integer }
```

---

## 6. Webhook Event 完整目录

| Event Type | 触发 | Data 关键字段 |
|---|---|---|
| `monitor.down` | 监控状态变 DOWN | monitor, event, affected_nodes |
| `monitor.up` | 监控恢复 UP | monitor, event, duration_sec |
| `monitor.degraded` | 监控进入降级 | monitor, event |
| `monitor.created` | 新增监控 | monitor |
| `monitor.updated` | 监控配置变更 | monitor, changes |
| `monitor.deleted` | 删除监控 | monitor_id |
| `monitor.paused` | 暂停 | monitor |
| `alert.triggered` | 告警事件创建 | event, monitor |
| `alert.acknowledged` | Ack | event, ack_by |
| `alert.resolved` | 解决 | event, resolved_kind |
| `incident.created` | 状态页事件创建 | incident, status_page |
| `incident.updated` | 状态页事件更新 | incident, update |
| `incident.resolved` | 状态页事件解决 | incident |
| `maintenance.scheduled` | 维护已计划 | maintenance |
| `maintenance.started` | 维护开始 | maintenance |
| `maintenance.ended` | 维护结束 | maintenance |
| `probe.completed` | 异步拨测完成 | task, results |
| `probe.failed` | 异步拨测失败 | task, error |
| `ssl.expiring` | SSL 证书快到期 | monitor, days_remaining |
| `domain.expiring` | 域名快到期 | monitor, days_remaining |
| `icp.changed` | ICP 备案变更 | monitor, before, after |
| `subscription.created` | 订阅创建 | subscription |
| `subscription.updated` | 订阅变更（升降级） | subscription, change |
| `subscription.canceled` | 取消 | subscription |
| `subscription.renewed` | 续费成功 | subscription, order |
| `subscription.payment_failed` | 续费失败 | subscription, attempt |
| `invoice.issued` | 发票开具 | invoice |
| `api_key.created` | API Key 创建 | key (脱敏) |
| `api_key.revoked` | API Key 撤销 | key |
| `api_key.leaked_detected` | 检测到 Key 泄露 | key |
| `team.member_added` | 团队加成员 | team, member |
| `team.member_removed` | 团队减成员 | team, member |
| `report.ready` | 报告生成完成 | report |
| `usage.threshold_reached` | 配额到达阈值 | dimension, percent, period |

---

## 7. 幂等性（Idempotency）

### 7.1 适用 endpoint
- 所有写操作（POST / PATCH / DELETE）
- 可选 header `Idempotency-Key`

### 7.2 行为
- 24h 内同 key 同 payload → 返回首次响应
- 24h 内同 key 不同 payload → 422 idempotency_key_mismatch
- 24h 后 key 过期，可复用

### 7.3 推荐场景
- 创建订阅 / 充值
- 创建监控 / 告警策略
- 退款

---

## 8. SDK 生成

### 8.1 流程
- `packages/api-spec/openapi.yaml` 是 source of truth
- 通过 OpenAPI Generator + 自家模板生成各语言 SDK
- CI 自动 PR 到 SDK 仓库
- 人工 review 后发布

### 8.2 SDK 版本
- 跟随 OpenAPI 版本：`@idcd/sdk@1.4.x` ↔ `/v1`
- 破坏性变更：major bump
- 新字段 / endpoint：minor bump
- Bug fix：patch

---

## 9. 沙箱（Sandbox）

### 9.1 测试 Key
- 前缀 `idc_test_xxx`
- 调用真实 endpoint，但：
  - 不计费
  - 不消耗节点资源
  - target 必须是 `*.test.idcd.com`（mock 目标）
- 返回固定 mock 数据 + 真实响应延迟模拟

### 9.2 Mock 目标
- `up.test.idcd.com` → 200 OK
- `slow.test.idcd.com` → 200 OK 但 5s 延迟
- `down.test.idcd.com` → 连接拒绝
- `5xx.test.idcd.com` → 503
- `cert-expired.test.idcd.com` → 过期证书

帮助开发者本地测试错误处理。

---

## 10. Webhook 接收（用户侧）

### 10.1 接收方实现要求

```python
import hmac, hashlib, time

def verify_signature(secret, signature_header, raw_body):
    parts = dict(p.split('=') for p in signature_header.split(','))
    t = int(parts['t'])
    v1 = parts['v1']

    # 5 分钟新鲜度
    if abs(time.time() - t) > 300:
        return False

    expected = hmac.new(
        secret.encode(),
        f"{t}.{raw_body.decode()}".encode(),
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, v1)
```

### 10.2 返回 2xx 表示成功
- 1xx / 4xx 非 408/429：不重试
- 5xx / 超时 / 408 / 429：按指数退避重试 6 次
- 200 也可包含可选 `{"ack": true}`（让用户明确确认）

### 10.3 重试策略
```
attempt 1: 5s
attempt 2: 30s
attempt 3: 2 min
attempt 4: 10 min
attempt 5: 1 hour
attempt 6: 6 hour
死信：手动 redeliver
```

---

## 11. CORS 与浏览器使用

### 11.1 默认策略
- 公开 GET endpoint：`Access-Control-Allow-Origin: *`
- 鉴权 endpoint：必须明确 origin 配置在 API Key 的 `allowed_origins`
- 不允许 cookie 跨域

### 11.2 推荐
- 浏览器场景用 `idc_pub_xxx` 公开 Key（限定 scope + origin）
- 服务器场景用 `idc_live_xxx` 私有 Key（不发到浏览器）

---

## 12. 国际化

### 12.1 接收语言
- 客户端可发 `Accept-Language: zh-CN | en-US`
- 错误消息按语言返回（其他字段保留英文 code）

### 12.2 时区
- 所有时间字段 UTC
- 用户可在 query 加 `?timezone=Asia/Shanghai`，部分聚合接口按用户时区分桶

---

## 13. 监控自家 API

- Prometheus 指标：`http_requests_total{endpoint, status}`
- P50/P95/P99 延迟
- 错误率（5xx / 4xx）
- 慢请求告警
- 单 Key 调用模式异常（突变 / 来源切换）告警

---

## 14. 版本演进规则

### 14.1 Non-breaking（允许在同版本内做）
- 新增字段（响应必须用默认值）
- 新增 endpoint
- 新增可选请求字段
- 放宽校验
- 新增 enum 值（响应；请求侧需小心）

### 14.2 Breaking（必须升 major 或 deprecation）
- 删除字段 / endpoint / enum 值
- 改字段类型
- 改默认值
- 收紧校验

### 14.3 Deprecation 流程
- 公告 90 天前（文档 + 邮件给受影响 Key）
- 响应加 `Deprecation: true` + `Sunset: <date>`
- 到期后返回 410

---

## 15. 测试与契约

### 15.1 契约测试
- OpenAPI schema → CI 跑 Spectral 校验
- Mock server（Prism）跑示例请求
- Postman collection 自动生成

### 15.2 SDK 测试
- 每个 SDK CI 跑 e2e（用 sandbox key 调真实 staging）

### 15.3 API 集成测试
- 主流程 e2e（注册 → 创建监控 → 触发告警 → resolve）

---

## 16. 阶段交付清单

### S2（beta）
- v1 全部公开 + 控制台 API
- OpenAPI yaml + Mintlify 文档站
- "Try it" 沙箱
- 5 种错误码完整
- Webhook 接收 + 签名
- 限速与配额完整
- 幂等性

### S3（GA）
- 官方 SDK：JS / Go / Python
- CLI 工具
- 完整 e2e 测试
- v1 GA + 公告
- Webhook redeliver UI
- Postman / Insomnia collection

### S4
- OAuth 2.0
- GraphQL（评估）
- v2 起草（如需）

---

## 17. 开放决策点

- [ ] 是否同时维护 OpenAPI yaml + AsyncAPI yaml（webhook）？建议 yes
- [ ] 限速维度文档化时是否暴露具体阈值？业内做法两边都有
- [ ] sandbox key 是否仍计入审计？建议 yes（便于开发者排障，不计费）
- [ ] 是否提供 ETag / If-Match 乐观锁？复杂度 vs 收益
- [ ] 自动生成的 SDK 是否独立仓库？建议每个 SDK 一个仓库
