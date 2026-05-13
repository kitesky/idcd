# 19 · AI Agent 可观测 与 MCP Server

> 关联:OVERVIEW.md §1.2 长版定位、§4.14;DECISIONS.md §K1/§K3/§K4;14-tech-architecture.md(独立子域 mcp.idcd.com);08-open-api.md(MCP 接入指南);04-monitoring.md(Agent/LLM 监控类型)
> 阶段主体:S3 上线(M9-M11 alpha,M12-M14 GA);MCP server 与 Agent obs 同步
> 品牌名占位:`idcd`

---

## 1. 模块定位

2026 年 AI Agent 与 MCP 协议爆发,网络可观测产品的形态正在从"人类应用 dashboard"向"Agent 工作流 observability"演化。本模块定义 `idcd` 如何抓住这个红利:

1. **AI Agent Observability** — 监控 Agent 工作流的"出口稳定性",而非应用本身的稳定性
2. **idcd-mcp server** — 把 `idcd` 所有拨测 / 监控 / Evidence 能力**包装为 MCP 协议接口**,任何 MCP 客户端(Claude / Cursor / Codex / 自家 SDK)可直接 import

### 1.1 一句话

> "你的 Agent 用 idcd-mcp 验证它访问的每个工具/API/RAG 端点都活着,而不是把失败传染到下一个 step。"

### 1.2 为什么 `idcd` 适合做这个

- 100 节点 = 天然的"全球出口模拟器",任意 Agent 工作流的出口稳定性都可以从 N 国视角验证
- 已有 Verdict / 签名报告能力 = Agent 失败可被回溯证据化
- 全栈拨测/监控能力 = 不需要从零搭建数据层

### 1.3 关键指标

| 指标 | S3 alpha | S3 GA |
|---|---|---|
| MCP 月活客户端 | 500 | 5,000 |
| MCP tool call 月调用量 | 100k | 5M |
| Agent obs 监控数 | 1,000 | 10,000 |
| MCP tool call P95 latency | ≤ 10s | ≤ 5s |
| MCP 收入(独立 SKU) | ¥0(免费 alpha) | ¥10k/月 |

---

## 2. 子模块 A:AI Agent Observability

### 2.1 监控类型(参与 04-monitoring §4 新增)

| 类型 | 代码 | 监控对象 | 拨测形态 |
|---|---|---|---|
| LLM Endpoint Health | M21 | OpenAI / Anthropic / Bedrock / 自家 LLM | HTTPS + 健康 prompt + 响应解析 |
| Tool/API Endpoint | M22 | Agent 调用的工具 / API(MCP server / 自家 webhook) | HTTPS + payload 校验 + 状态码 |
| RAG/Vector Store | M23 | Qdrant / Pinecone / pgvector / Elastic | 健康查询 + 近似响应时间 |
| Agent Output Quality | M24(S4) | Agent 端到端 prompt → output 的 LLM 评分 | 评估器评分 |

### 2.2 数据维度

```
agent_observability_event
  timestamp,
  agent_id,              # 用户给 Agent 起的名(如 customer_support_v3)
  step,                  # Agent 工作流的 step name
  endpoint_url,          # 调用的目标
  endpoint_type,         # llm | tool | rag | other
  region,                # 从哪个节点验证
  latency_ms,
  success (bool),
  failure_class,         # timeout | 4xx | 5xx | malformed | semantic
  failure_detail (jsonb),
  trace_id (可选)
```

### 2.3 告警维度(配合 05-alerting)

- "LLM endpoint 在 3 个国家持续不可达 > 5 分钟"
- "Agent 出口 P99 > 10s 连续 3 个采样"
- "Tool endpoint 返回 4xx 比例 > 10% / 5 分钟"
- "Agent 工作流 step 失败率突增 > 基线 2x"

### 2.4 故障复盘自动起草(配合 07-reports)

PRD v1 原定 P3/S4 → 现提至 **P1/S2**(见 04 §4 K4)。逻辑:
- 故障触发后 30 分钟,LLM 用结构化输入(时间线 + 节点表现 + 路由变化)生成"事故公告草稿 + 根因建议"
- 输出**强制人工审核** + "AI 草稿"水印 + sanitize 后才能发到 status page / Webhook
- 离线 eval:每周抽 50 个真实事故,人工打分 ≥ 4.0/5 才允许该版本 prompt 上线

---

## 3. 子模块 B:idcd-mcp server

### 3.1 定位与架构(决策 §K1 + v2 D13 状态边界)

**独立子域** `mcp.idcd.com`,独立计量 / 计费 / SLA。**不**与 `api.idcd.com` 共享同栈,理由:
- MCP 是面向 Agent 的产品,定价 / SLA / 文档 / 鉴权模型都与 REST API 不同
- 未来可独立白标卖给企业(企业自家域名挂 `idcd` 后端)
- 协议演化时双轨维护成本低,新协议(OpenAI Agents Protocol / 其他)可加新 server 而不影响 REST

**状态边界(v2 D13)**:业务逻辑 **stateless**(可水平扩展);但 SSE 连接 **stateful**,需 LB sticky session(基于 token_id 哈希)。

```
                       Cloudflare WAF + LB (sticky session by token_id)
                                     │
                          mcp.idcd.com (MCP Server, multi-instance)
                                     │
                        ┌────────────┴────────────┐
                        │                         │
                  MCP Auth Service          MCP Tool Dispatcher
                  (短期 token 最长 90d,     (按 tool 路由到内部)
                   IP 白名单, D2)
                        │                         │
                        └────────────┬────────────┘
                                     │
                                     ▼
                  Internal RPC to Probe / Monitor / Verdict / Account services
                                     │
                              Metering + Billing
                              (MCP units 独立计量,与 API 配额池完全独立, D2)

                              ─── SSE Layer ───
                  /v1/agent-obs/events: long-lived SSE,LB sticky 必要
                  单实例 10k 并发连接(详 14 §9.1)
                  多实例横向扩展走 Redis Streams 跨实例广播

                              ─── Docs ───
                  docs.mcp.idcd.com(独立子域,Cloudflare Pages,D3)
                  mcp.idcd.com/docs → 302 redirect → docs.mcp.idcd.com
```

### 3.2 协议支持

- **主**:Anthropic MCP spec(JSON-RPC over stdio + HTTP+SSE)
- **次**(S3 后期):OpenAI Agents Protocol(若市场份额超 30% 则接入)

不主动跟进所有协议;以 Anthropic MCP 为锚,其他协议**只接入有付费需求的**。

### 3.3 暴露给 Agent 的 Tools(初版)

| Tool 名 | 描述 | 计费 |
|---|---|---|
| `idcd_ping` | 多地 Ping(指定/默认节点选择) | 1 unit/call |
| `idcd_http_probe` | 多地 HTTP/HTTPS 拨测 | 2 unit/call |
| `idcd_dns_resolve` | 多地 DNS 解析 + 污染检测 | 2 unit/call |
| `idcd_traceroute` | 多地路由追踪 | 3 unit/call |
| `idcd_ssl_cert` | SSL 证书查询 + 安全评分 | 1 unit/call |
| `idcd_diagnose` | 一键全面诊断 | 10 unit/call |
| `idcd_ip_info` | IP 归属 / ASN / ISP 查询 | 1 unit/call |
| `idcd_whois` | WHOIS 查询 | 1 unit/call |
| `idcd_icp` | ICP 备案查询 | 1 unit/call |
| `idcd_create_monitor` | 创建监控项(需 API key 权限) | 0 unit(免费,触发计费档)|
| `idcd_check_monitor` | 查询监控历史 | 1 unit/call |
| `idcd_generate_verdict` | 触发 Verdict 报告生成(需账户 Verdict 配额或扣余额) | 0 unit(报告本身计费)|
| `idcd_list_nodes` | 列出可用节点(透明度) | 0 unit |

> 1 unit ≈ ¥0.002(便宜让 Agent 用)。Cursor / Claude Code 普通会话调用 5-20 unit/次,月活用户 ¥0.5-5/月,**真正的盈利来自企业 Agent 团队 + Compliance 档绑定**。

### 3.4 鉴权模型(v2 D2 锁定:无永久 token + 三种形态)

> **v2 D2 原则**:所有 token 都有过期日,**最长 90 天**;不存在"永久 / 长期" token。
> auto_renewal 机制:workspace / service token 在过期前 24h 自动续期(基于上次使用时间;超 30 天未用不续期 → 等于自动撤销)。
> personal access token 24h 短期,开发者每天首次使用时由 OAuth-like flow 自动 refresh。

- **三种凭证形态**:

| 形态 | 有效期 | Renewal | IP 白名单 | 使用场景 |
|---|---|---|---|---|
| **Personal access token** | 24 小时 | OAuth-like 自动 refresh | 可选 | 开发者本地 Cursor / Claude Code 会话 |
| **Workspace token** | 90 天 | auto_renewal(过期前 24h)| 可选 | 团队成员共享,绑定 Team 订阅 |
| **Service account token** | 90 天 | auto_renewal(过期前 24h)| **强制** | 生产 Agent 服务,无白名单不签发 |

- **凭证泄露应急(详 12 §22)**:
  - 异常突增告警(24h 调用量 > 历史 P95 × 5 倍)
  - 用户控制台一键撤销 + 通知 + 操作审计
  - GitHub token 扫描自动失活(S3 alpha 前选定 GitGuardian / 自家正则 / TruffleHog,详 12 §22.3)
  - revoke 后该 token 关联的 active SSE 连接立即断开(via Redis pub-sub broadcast)

### 3.5 限速与配额(v2 D2 锁定:MCP units 独立池,与 API 配额完全独立)

> **v2 D2 锁定**:MCP units 配额池与 API 调用配额池**完全独立**。用户控制台看到两条独立量表:`MCP units/day` 和 `API calls/day`,不混合。
> Free / Pro / Team / Business 各档都有独立的 MCP units 额度,Agent Pro 是 MCP units 加大独立 SKU,不影响 API 配额。

| 档位 | MCP units / day(独立池)| API calls / day(详 09 §2.3) | concurrent SSE | priority |
|---|---|---|---|---|
| Free | 100 | 100 | 2 | 普通 |
| Pro | 5,000 | 5,000 | 10 | 普通 |
| Team | 30,000 | 30,000 | 50 | 普通 |
| Business | 200,000 | 200,000 | 200 | 普通 |
| **Agent Pro(MCP 专档,S3 GA)** | **1M(独立加大)** | 同订阅档 | 500 | 优先 |
| Compliance Enterprise | 不限 | 议价 | 议价 | 优先 |

> **重要**:Pro 用户 5000 MCP units/day 和 5000 API calls/day 是**两个独立池**,不互相消耗。用户在 /app/usage 看到两条独立 progress bar。
> 详 09 §2.8 Agent Pro 档。

### 3.6 客户端兼容性

| 客户端 | 状态 | 测试 |
|---|---|---|
| Claude Code(stdio + http+sse) | S3 alpha 起支持 | 每发布前 smoke test |
| Cursor(stdio) | S3 alpha 起支持 | 同上 |
| Codex CLI(stdio) | S3 GA | 同上 |
| Anthropic Console(MCP gallery)| S3 GA 提交 | — |
| 自家 SDK(idcd-mcp-py / idcd-mcp-ts)| S3 GA | npm + pypi 发布 |

### 3.7 文档站(v2 D3 独立子域)

- **独立子域 `docs.mcp.idcd.com`**(Cloudflare Pages,Nextra SSG)
- `mcp.idcd.com/docs` URL 走 **302 redirect → docs.mcp.idcd.com**
- 设计理由:MCP server 是 dynamic Go/Node 服务,文档站是 SSG;独立部署避免互相影响(MCP server 挂了文档仍可访问;文档发布不需重启 MCP server)
- 内容:5 分钟接入 → Tool 参考 → 鉴权 → 示例(Cursor / Claude Code / Codex / 自家 SDK)
- 风格参考 Anthropic MCP docs(开发者熟悉的范式)

---

## 4. 数据模型(参与 15 模块)

```
mcp_session
  id (mcps_xxx), token_id, client_id (Cursor|ClaudeCode|Codex|other),
  client_version, ip,
  started_at, last_activity_at, ended_at,
  total_tool_calls, total_units

mcp_tool_call
  id, session_id, owner_id,
  tool_name, request_payload_hash, response_payload_hash,
  units_charged, status (success|failure|timeout),
  latency_ms, error_class, error_detail,
  created_at

mcp_token
  id (mcpt_xxx), owner_id, type (personal|workspace|service),
  scope (jsonb: tools[], regions[]),
  ip_whitelist (jsonb), revoked (bool),
  expires_at, created_at, last_used_at

agent_observability_monitor
  id (m_xxx), owner_id, agent_name, step_name,
  endpoint_type (llm|tool|rag|other), endpoint_url,
  frequency_seconds, expected_latency_ms,
  failure_threshold (jsonb: latency, status_codes, payload_assertions),
  notification_channels (jsonb),
  created_at
```

---

## 5. API 端点(参与 16 模块)

```
POST   /v1/mcp/tokens              签发 MCP token
DELETE /v1/mcp/tokens/:id          撤销 token
GET    /v1/mcp/tokens              列出我的 tokens

POST   /v1/agent-obs/monitors      创建 Agent obs 监控项
GET    /v1/agent-obs/events        查询 Agent obs 事件流(SSE)

GET    /v1/mcp/usage               MCP 用量 + 计费明细
```

MCP 协议本身的端点(`mcp.idcd.com`)走 MCP 标准,不在 REST API 路径中暴露。

---

## 6. 安全 / 合规

### 6.1 凭证安全(v2 D2 锁定)
- **所有 MCP token 都有过期日,最长 90 天**(personal 24h auto refresh / workspace 90d auto_renewal / service 90d auto_renewal + IP 白名单强制)
- **不签发"永久 / 长期" token**(D2 严格原则)
- token 哈希存储,前端展示一次后只显示后 4 位
- token 调用全审计 + 异常突增告警(详 12 §22)
- service account 强制 IP 白名单(无白名单不签发)

### 6.2 滥用防控
- MCP 调用走与 REST 同一套限速 + 黑名单(12 模块)
- 每个 tool call 仍走 12 §3 目标黑名单
- Free 档 Anti-DDoS:concurrent 2 + 100/day 是为了防 Cursor 用户被攻陷后做"代理 DDoS"
- LLM endpoint monitoring 不允许"频繁调用昂贵 LLM 直到爆账单"(用户自家 LLM 端点不限,但目标白名单需 verify)

### 6.3 数据隐私(v2 D7 增强:失败 case 临时存原 payload)
- MCP tool call 的 request/response payload **默认哈希存储,不存原文**(避免 Agent 用户的 prompt / 数据被 `idcd` 看见)
- **例外(v2 D7)**:用户可在 /app/mcp/settings 开启"**失败 case 临时存 7 天原 payload**"
  - 默认**关**
  - 开启后:`status=failure` 的 tool call 会写入 `request_payload_raw` / `response_payload_raw` jsonb 字段
  - 保留期 7 天(基于 `payload_retain_until = created_at + 7d`),过期自动 cron 清理
  - 用途:用户报障 / 自查时可查到具体失败 payload
  - **隐私边界**:仅用户本人可见;`idcd` 客服 review 需用户授权(走 /app/mcp/troubleshoot/grant 流程)
- Agent obs 失败采样可选存原文(用户在 monitor 设置中可勾选,默认关)
- 6 个月留存 + 自助导出 + 一键删除

### 6.4 边界
- MCP server 不替 Agent 执行业务逻辑;仅提供"可观测能力"
- 不存储 Agent 端的对话 / 用户 prompt
- 不参与 Agent 的决策

---

## 7. 阶段交付清单

### S3 alpha(M9-M11)
- mcp.idcd.com 独立部署
- 8 个核心 tool(ping/http/dns/trace/ssl/diagnose/ip/whois)
- Personal token + 控制台签发/撤销
- Cursor + Claude Code 兼容性测试
- mcp.idcd.com/docs 文档站
- 邀请制 alpha(100 开发者)

### S3 GA(M12-M14)
- 13 个 tool 全量
- Workspace + Service account token
- Codex CLI 兼容
- 自家 SDK(idcd-mcp-py / idcd-mcp-ts)
- 提交 Anthropic MCP gallery
- Agent obs 监控(M21-M23)上线
- LLM 故障复盘自动起草(P1 提前后,先于 M5-M6 落地)
- MCP 独立计费 + 用量看板

### S4(M15+)
- OpenAI Agents Protocol(若有付费需求)
- 白标 MCP server(企业自家域名)
- M24 Agent Output Quality 监控
- 与 Compliance 档绑定(企业 Agent 团队套餐)

---

## 8. 关键风险

| 风险 | 缓解 |
|---|---|
| MCP 协议快速演化,Anthropic / OpenAI 分叉 | 主跟 Anthropic;次跟 OpenAI 只有付费需求时;不主动适配所有 |
| Cursor / Claude Code 客户端版本不兼容 | 每发布前 smoke test + 版本兼容矩阵公开;旧客户端降级提示 |
| Agent 端 token 泄露(用户 Cursor 配置被偷)| 短期 token + IP 白名单 + 异常突增告警 + 一键撤销 |
| MCP tool call 计费意外(Agent 死循环爆账单) | Free 100/day 硬上限;Pro 起 80%/95%/100% 三级提醒 + 自动停服选项 |
| LLM endpoint monitoring 被滥用爆别人账单 | 12 §3 目标黑名单 + 目标所有权可验证 |
| LLM 复盘草稿幻觉/谩骂/泄密 | Prompt 约束 + 必须人工审核 + AI 标识 + 输出 sanitize + 离线 eval ≥ 4.0/5 |
| Anthropic 自己出 MCP gallery 把流量收回去 | 接受;`idcd` 优势在 100 节点 + Verdict + Compliance,不靠 Anthropic 渠道 |

---

## 9. 决策记录(已锁定,见 DECISIONS.md §K)

- ✅ **K1** MCP 独立子域 mcp.idcd.com,sub-product 阵型
- ✅ **K3** MCP 鉴权:短期 token + 三种形态(personal/workspace/service)+ 可选 IP 白名单
- ✅ **K4** LLM 故障复盘自动起草从 P3/S4 提至 P1/S2
- ⏳ **K-OPEN-3** OpenAI Agents Protocol 是否接入 — S3 末根据市场份额评估
- ⏳ **K-OPEN-4** Agent Output Quality 监控(M24)的评估器选型 — S4 评估

### 待定(非紧迫)
- [ ] MCP 调用结果是否可生成 Verdict 报告(把 Agent 验证证据化)— S4 评估
- [ ] 是否提供 "MCP 调用回放" 功能给开发者排障 — S4 评估
- [ ] 自家 SDK 是否开源(MIT)— S3 末决,默认开
