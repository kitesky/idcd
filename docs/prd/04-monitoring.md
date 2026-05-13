# 04 · 网站监控（核心商业模块）

> 关联：OVERVIEW.md §4.3、§4.14、§6.2 定价档
> 关联(v2):DECISIONS.md §K4(LLM 复盘提前)、19-ai-agent-observability.md §2(Agent obs 类型)、07-reports §X(LLM 复盘工作流)
> 阶段主体：S2 全量上线（这是商业化启动的核心）+ **v2: LLM 故障复盘自动起草上线(从 P3 提前到 P1)**;S3 增量含 Agent obs M21/M22/M23
> 是否登录：**必须登录**
> 品牌名占位：`idcd`

---

## 1. 模块定位

监控模块是 `idcd` 的**主要付费转化点**。把一次性拨测变成持续观察 + 异常告警 + SLA 报表，覆盖 SaaS 订阅、API 调用、状态页托管三条主线收入。

### 设计原则

1. **零误报优先**：宁可漏报 5 秒也不要"一拨没通就告警"
2. **场景驱动**：用户能在 60 秒内完成"添加监控 + 配通道 + 看到第一次结果"
3. **可观测的可观测**：监控自己的监控（节点失败率、调度延迟、告警延迟）
4. **批量友好**：100 个监控的运维成本不应比 5 个高 10 倍

### 关键指标

| 指标 | S2 目标 | S3 目标 |
|---|---|---|
| 付费用户数 | 200 | 1500 |
| 监控总数 | 5,000 | 80,000 |
| 误报率（用户标记） | ≤ 5% | ≤ 1% |
| 告警从异常发生到首次通知 P95 | ≤ 60s | ≤ 30s |
| 监控可用性（节点不可用导致漏检） | ≥ 99.9% | ≥ 99.95% |

---

## 2. 监控类型全集

| ID | 类型 | 说明 | P | S |
|---|---|---|---|---|
| M01 | HTTP/HTTPS | URL + 方法 + 头 + 期望状态码/关键字 | P0 | S2 |
| M02 | Ping | 主机 + 包数 + 丢包率阈值 | P0 | S2 |
| M03 | TCP 端口 | host:port 连通 | P0 | S2 |
| M04 | DNS 解析 | 域名 + 记录类型 + 期望值 | P0 | S2 |
| M05 | SSL 证书到期 | host:port + 提前预警天数 | P0 | S2 |
| M06 | 域名到期（WHOIS） | 域名 + 提前预警天数 | P0 | S2 |
| M07 | ICP 备案变更 | 域名 + 变更类型监测 | P1 | S2 |
| M08 | 关键字监控 | 页面包含/不包含 + 正则 | P0 | S2 |
| M09 | JSON API 监控 | URL + JSONPath 断言 + 期望值 | P1 | S2 |
| M10 | 心跳监控（Heartbeat） | 反向：客户端定时 ping `idcd` | P1 | S2 |
| M11 | 浏览器级监控（Headless Chrome） | 真实浏览器加载 + 截图 + 关键资源 | P2 | S3 |
| M12 | 多步骤事务监控 | 录制 N 步：登录→搜索→下单→支付 | P3 | S4 |
| M13 | 端口扫描组（多端口批量） | 一个 host 多个端口 | P2 | S3 |
| M14 | 邮件服务器健康 | SMTP/IMAP/POP3 + SPF/DKIM/DMARC | P2 | S3 |
| **M21** | **LLM Endpoint 健康(v2 NEW)** | OpenAI/Anthropic/Bedrock/自家 LLM endpoint 拨测 | **P0** | **S3** |
| **M22** | **Tool/API Endpoint 健康(v2 NEW)** | Agent 调用的工具/API/MCP server,payload 校验 | **P0** | **S3** |
| **M23** | **RAG/Vector Store 健康(v2 NEW)** | Qdrant/Pinecone/pgvector/Elastic 健康查询 + 近似响应时间 | **P1** | **S3** |
| M24 | Agent Output Quality(v2 远期) | Agent 端到端 prompt→output LLM 评分 | P3 | S4 |

> M21-M24 详见 19-ai-agent-observability.md §2;监控数据归 Agent obs 独立维度,但调度/告警/状态页流程与 M01-M14 共用。

---

## 3. 详细规格（按类型展开）

### 3.1 M01 HTTP/HTTPS 监控

#### 配置字段

```yaml
name: "官网首页"
url: "https://example.com/"
method: GET
headers:
  User-Agent: "Brand-Monitor/1.0"
body: null              # POST/PUT 时启用
follow_redirect: true
timeout_ms: 10000
ip_version: auto        # v4 / v6 / auto

# 节点选择
node_selection:
  mode: pool            # pool: 节点池随机 N 个；fixed: 指定节点
  pool_size: 3          # 同时拨测节点数
  regions: ["CN", "US", "EU"]    # 国家/区域筛选
  isps: ["CT", "CU", "CM"]       # 国内三网
  tags: ["tier1"]                # 节点标签
  ipv6_enabled: false

# 频率
interval_sec: 60        # 10 / 30 / 60 / 300 / 900（按订阅档限制）

# 断言（任意未通过则视为异常）
assertions:
  - type: status_code
    operator: in
    value: [200, 201, 204, 301, 302]
  - type: response_time_ms
    operator: lt
    value: 3000
  - type: body_contains
    value: "Welcome"
  - type: header
    name: "content-type"
    operator: contains
    value: "text/html"
  - type: ssl_valid
    value: true
  - type: redirect_chain_length
    operator: lt
    value: 4

# 异常判定（防误报 ★关键）
trigger:
  consecutive_fail: 2          # 连续 N 次失败才算异常
  fail_node_quorum: 2/3        # M/N 节点失败才算异常（多节点确认）

# 告警关联
alert_policy_id: "ap_xxx"

# 维护窗口
maintenance_windows:
  - start: "2026-06-01T02:00:00+08:00"
    end:   "2026-06-01T04:00:00+08:00"
    repeat: weekly_mon_02_04   # 可选周期

# 分组与标签
tags: ["prod", "frontend"]
group_id: "g_main"
```

#### 异常判定逻辑（防误报核心）

```
每次拨测：从 pool_size 个节点中拨测
  节点失败定义：assertions 任一未通过 OR 网络错误 OR 超时

单次拨测结果：fail_node_count / total_node
  → 若 fail_node_count / total_node ≥ fail_node_quorum：标记 "本轮异常"

异常状态判定：
  连续 consecutive_fail 轮"本轮异常" → 监控状态变 DOWN → 触发告警
  连续 consecutive_fail 轮"本轮正常" → 监控状态变 UP → 触发恢复告警
```

这套机制保证：
- 单节点偶发抖动不触发误报
- 但如果 3 个节点同时挂了（quorum 触发），且持续 2 轮，则一定告警

### 3.2 M02 Ping 监控

- 字段：host、包数（默认 4）、间隔、IPv4/v6
- 断言：丢包率 < 阈值、平均 RTT < 阈值
- 异常判定逻辑同 M01

### 3.3 M03 TCP 端口监控

- 字段：host、port、握手超时
- 断言：可连接 / 握手时间 < 阈值
- 应用场景：MySQL 3306、Redis 6379、SSH 22 等

### 3.4 M04 DNS 解析监控

- 字段：domain、record_type、指定 DNS（可选）
- 断言：
  - 解析结果 in [...]（白名单）
  - 解析结果 not contains [...]（黑名单，防劫持）
  - 期望 TTL 范围
  - 多 DNS server 一致性（污染检测）
- 应用场景：检测 DNS 被改、检测 DNS 污染

### 3.5 M05 SSL 证书到期监控

#### 配置
- host:port
- SNI（可选）
- 提前预警阶梯：30 天 / 15 天 / 7 天 / 3 天 / 1 天（可自选）
- 每天检查 1 次（不需要高频）

#### 检查项（任一异常告警）
- 证书已过期
- 证书快过期（命中阶梯）
- 证书自签 / 不受信任
- 证书 CN/SAN 不匹配请求 host
- 证书链不完整
- 弱签名算法（SHA-1、MD5）
- 弱密钥（RSA < 2048）
- TLS 1.0/1.1 启用警告
- OCSP 验证失败
- CT 日志未记录（潜在风险，不阻断）

### 3.6 M06 域名到期监控

- 字段：domain、提前预警阶梯（默认 30/15/7/3/1 天）
- 每天检查 1 次
- 数据来源：WHOIS / RDAP，多源容灾
- 异常事件：域名快过期 / 域名状态异常（clientHold、redemptionPeriod 等）

### 3.7 M07 ICP 备案变更监控

- 字段：domain
- 检查项：
  - 备案被注销
  - 备案主体变更
  - 备案号变更
  - 网站名称变更
- 频率：每天 1 次
- 数据源：工信部公示数据（自抓+缓存）

### 3.8 M08 关键字监控

- 字段：URL + method/headers/body + 期望关键字（include / exclude）+ 正则
- 应用：检测页面被篡改、检测 CDN 缓存命中错误、检测客服系统是否在线（页面包含"在线"）

### 3.9 M09 JSON API 监控

#### 配置
- URL、method、headers、body
- 断言：
  - HTTP 状态码
  - JSONPath 断言：`$.code == 0`、`$.data.count >= 10`、`$.users[*].id` 数量
  - JSON Schema 校验
  - 响应时间
- 应用：监控后端 API 是否返回正确业务数据

### 3.10 M10 心跳监控（Heartbeat）

#### 反向监控模式
用户的 cron 任务 / batch 程序定时 ping `idcd` 的回调 URL，如果超时未收到 ping，则认为该任务未运行。

#### 配置
- 心跳 URL：`https://api.idcd.com/heartbeat/<token>`
- 期望间隔：5 min / 1 h / 1 day（可自定义）
- 宽限期：超过期望间隔 N% 才算异常
- 期望 payload（可选）：必须包含某个字段

#### 应用
- 备份任务每天凌晨跑 → ping 一次 → 没 ping 就告警
- 定时报告生成 → ping 一次 → 漏发告警
- IoT 设备 keep-alive

### 3.11 M11 浏览器级监控

#### 配置
- URL
- 视口大小 / 移动端模拟 / UA 切换
- 等待条件：load / DOMContentLoaded / 指定 CSS selector 可见 / 指定 JS 表达式 true
- 性能指标采集：LCP / FCP / TTI / TBT / CLS
- 截图：每次 / 仅失败时
- 关键资源监控：JS/CSS/图片加载耗时

#### 应用
- SPA 应用监控（裸 HTTP 测不出）
- 检测前端报错（控制台 JS 错误）
- 性能回归检测

#### 边界
- 单次成本高，频率最低 5 分钟
- 仅 Team/Business 档可用

### 3.12 M12 多步骤事务监控（企业版 S4）

- 录制器：录浏览器操作序列（点击、输入、跳转）
- 回放：每次按相同序列执行
- 断言：每步是否成功、整个流程时长
- 应用：登录 → 加购 → 结账完整链路

### 3.13 M13 多端口批量

- 一次填入 host + 多个 port（22/80/443/3306/6379...）
- 共用配置，独立结果
- 主要给运维场景：一台服务器同时监控多个服务

### 3.14 M14 邮件服务器健康

- 探测 SMTP/IMAP/POP3 端口连通
- TLS 启用与证书检查
- SPF / DKIM / DMARC 配置校验
- 在 RBL 黑名单中的状态（与 Q21 共用）

### 3.15 M21 LLM Endpoint 健康(v2 NEW)

#### 配置字段
```yaml
name: "OpenAI Chat Completions"
endpoint: "https://api.openai.com/v1/chat/completions"
auth:
  type: bearer
  token_ref: "secret:openai_test_key"  # 引用脱敏 secret,不存明文
health_prompt:
  model: "gpt-4o-mini"
  messages: [{role: user, content: "Ping. Reply with 'pong'."}]
  max_tokens: 10
expected_response:
  contains: "pong"
  schema_valid: true                   # response 必须是有效 JSON
node_selection:                        # 从哪些节点验证
  regions: ["US", "EU", "CN"]
  pool_size: 3
interval_sec: 300                       # LLM 调用贵,默认 5 min
budget_per_check_usd: 0.01             # 单次检查预算上限
```

#### 断言
- HTTP 状态 200
- 响应时间 < 阈值(默认 30s,LLM 长)
- response 体 schema 校验
- response.choices[0].message.content 包含 expected_response.contains
- token usage 在预期范围(防 LLM 异常返回大段重复)
- **每次调用费用 ≤ budget_per_check_usd**(超额自动暂停 + 通知)

#### 应用
- 企业 Agent 团队验证"我们调用的 LLM 现在还活着吗"
- 多区域验证(中国/欧洲访问 OpenAI 的连通性)
- 模型 fallback 决策依据(主 endpoint 挂了切备 endpoint)

#### 边界
- 不允许测试**未经用户验证所有权或公开免费**的 LLM endpoint(目标黑名单 + 防爆账单)
- 用户自家私有 LLM endpoint 不限,需配置 token
- Free 档不支持 M21(防止滥用爆别人账单);Pro+ 起;Agent Pro 档配额加大

### 3.16 M22 Tool/API Endpoint 健康(v2 NEW)

#### 配置字段
```yaml
name: "internal-payment-api"
endpoint: "https://payments.internal.example.com/v1/health"
method: POST
headers:
  X-API-Key: "secret:internal_key"
body:
  test: true
expected_response:
  status_code: 200
  body_jsonpath:
    "$.status": "healthy"
    "$.version": {"matches": "^v\\d+\\.\\d+"}
node_selection:
  regions: ["CN"]                       # Agent 业务在国内才测国内
  pool_size: 3
interval_sec: 60
```

#### 断言
- HTTP 状态码 / 头 / 响应时间
- body JSONPath 断言(配合 04 §3.9 M09 同样语法)
- body schema 校验
- TLS 证书有效性

#### 与 M01/M09 的区别
- M01/M09:面向"我的网站/API",通用
- M22:面向"Agent 工作流中使用的工具/API",有 Agent 上下文(关联 agent_id + step_name);可与 Agent obs 事件流(19 §2.2)聚合分析

### 3.17 M23 RAG/Vector Store 健康(v2 NEW)

#### 配置字段
```yaml
name: "production-qdrant"
type: qdrant                            # qdrant|pinecone|pgvector|elastic|weaviate
endpoint: "https://qdrant.internal:6333"
auth:
  type: api_key
  token_ref: "secret:qdrant_key"
query:
  collection: "knowledge_base"
  vector: [0.1, 0.2, ...]              # 健康查询向量(用户提供)
  top_k: 5
expected:
  min_results: 3                        # 至少返回 3 条
  max_latency_ms: 200
```

#### 断言
- 连通性
- 查询返回数 ≥ min_results
- 返回向量相似度合理(top_k 中 score 不为 0)
- 索引大小变化告警(防意外删库)

### 3.18 M24 Agent Output Quality(v2 远期, S4)

> 详见 19 §2.1 / §7;S4 评估后开放;评估器选型仍 K-OPEN-4 待定。

---

## 4. 频率档位

| 频率 | 名称 | Free | Pro | Team | Business |
|---|---|---|---|---|---|
| 5 min | 标准 | ✅ | ✅ | ✅ | ✅ |
| 1 min | 加密 | ❌ | ✅ | ✅ | ✅ |
| 30 sec | 实时 | ❌ | ❌ | ✅ | ✅ |
| 10 sec | 极速 | ❌ | ❌ | ❌ | ✅ |

> 浏览器级监控（M11）最低 5 分钟；M05/M06/M07 每天 1 次（成本考虑，频率不可改）。
> **v2 NEW:** M21 LLM 默认 5 min;Agent Pro 档可到 1 min;M22 与 M01/M09 同档;M23 默认 5 min。

---

## 5. 节点选择策略

### 5.1 节点池模式（推荐）

用户不指定具体节点，由系统按规则随机/轮换选择：

- **池大小**：3（默认）/ 5 / 10
- **地理筛选**：国家、大区
- **运营商筛选**：CT / CU / CM / 海外大类
- **Tag 筛选**：tier1 / tier2 / 家宽 / IDC
- **节点来源筛选**（决策 E1）：
  - 仅自有节点（默认 Free 档；Pro+ 可选）
  - 自有 + 众包 T1 节点（Pro+ 默认）
  - 仅指定众包节点（高级，不推荐生产监控）
- **轮换策略**：
  - `random`：每次随机
  - `round-robin`：轮询
  - `sticky`：粘性（同一节点连续测，便于看趋势）

> **众包节点参与说明（决策 E1）**：T1 众包节点默认参与 Pro 档监控。用户在监控配置 / 账号设置里可一键关闭"使用众包节点"。Team/Business 档监控默认仅使用自有节点。

### 5.2 指定节点模式

适合高级用户：指定 1-N 个固定节点，便于"对比同一节点的趋势"。

### 5.3 多区域同时（高级）

为多地理位置分别建立"虚拟监控"：
- 北京电信视角
- 上海联通视角
- 美国东岸视角

每个视角是一组节点池，独立判定状态。便于状态页按地区展示。

---

## 6. 监控管理（列表 + 操作）

### 6.1 列表页（`/app/monitors`）

#### 列表视图
- 列：状态 ● / 名称 / 类型 / 目标 / 频率 / 最近响应时间 / 24h 可用率 / 标签 / 操作
- 状态指示：● 绿（UP）、● 红（DOWN）、● 黄（部分异常）、● 灰（暂停）、● 紫（维护中）
- 排序：默认按状态（异常优先），可自定义
- 筛选：状态 / 类型 / 标签 / 分组 / 节点 / 标记关键
- 搜索：名称、URL 模糊匹配
- 批量操作：暂停 / 恢复 / 删除 / 改标签 / 改告警策略 / 导出

#### 卡片视图（可切换）
- 大卡片，含 24h 迷你时序图、当前响应时间、状态色块

#### 看板视图（适合 SRE）
- 按分组聚合，分组内按状态分布展示

### 6.2 详情页（`/app/monitors/<id>`）

#### 顶部
- 名称 + 状态 + 标签
- 关键操作：暂停 / 强制立即测试 / 编辑 / 删除 / 复制

#### Tab：概览
- 当前状态、连续运行/异常时间
- 7d / 30d / 90d 可用率
- 时序图（响应时间，可切换 7d/30d/90d/自定义）
- 节点维度对比图
- 最近 N 次拨测结果列表

#### Tab：事件
- 历史告警事件时间线
- 每个事件可下钻看：触发原因、影响节点、持续时间、恢复时间
- 标记"误报"、添加"事件备注"

#### Tab：节点详情
- 每个节点的成功率、平均响应时间、最近 N 次结果
- 节点对比：横向看哪些节点不稳定

#### Tab：配置
- 完整配置（含 YAML 视图）
- 一键复制、克隆

#### Tab：测试
- 当前配置下手动触发一次"立即测试"，立刻看结果（不计入历史可用率）

### 6.3 新建 / 编辑

#### 创建向导（适合新手）
- Step 1：选择类型（卡片选择）
- Step 2：填写目标（带智能提示）
- Step 3：选择频率 + 节点
- Step 4：设置断言（默认推荐 + 高级自定义）
- Step 5：选择告警策略（默认 = 邮件 + Webhook）
- Step 6：确认 → 立即测试一次

#### 高级模式
- 直接 YAML 配置（开发者友好）
- 表单 + YAML 双向同步

---

## 7. 分组、标签与维护窗口

### 7.1 分组（Group）
- 用户可创建分组（"生产环境"、"测试环境"、"客户 A"）
- 监控属于一个分组
- 状态页可按分组聚合展示

### 7.2 标签（Tag）
- 自由打标签（多对多）
- 用于筛选、批量操作、状态页过滤

### 7.3 维护窗口（Maintenance Window）

#### 一次性窗口
- 开始时间 + 结束时间
- 范围：单个监控 / 一组监控 / 标签匹配
- 行为：窗口内不告警，但仍记录原始数据；状态页显示"维护中"

#### 周期窗口
- Cron 表达式 / 简化 UI（每周一 02:00-04:00）
- 适合定期备份、定期重启

#### 窗口预告
- 窗口开始前可推送通知给团队
- 状态页提前公告

---

## 8. 批量操作

### 8.1 批量导入

#### CSV 模板
- 类型 / 名称 / 目标 / 频率 / 节点池 / 标签 / 告警策略

#### JSON / YAML 模板
- 完整配置导入
- 支持环境变量替换（用户脚本驱动）

#### 来源
- Cloudflare / Vercel 项目导入（高级 S3）
- HAR 文件导入（M11 浏览器监控用）

### 8.2 批量导出
- CSV / JSON
- 含完整配置（脱敏密钥）
- 用于备份 / 迁移 / 团队共享

### 8.3 Terraform Provider（S3）
- 让 DevOps 用户通过 IaC 管理监控
- 这是企业转化关键钩子

---

## 9. 数据保留策略（按订阅档区分）

| 数据类型 | Free | Pro | Team | Business |
|---|---|---|---|---|
| 原始拨测结果 | 7 天 | 30 天 | 90 天 | 180 天 |
| 聚合数据（按小时） | 30 天 | 180 天 | 1 年 | 2 年 |
| 聚合数据（按天） | 1 年 | 2 年 | 3 年 | 5 年 |
| 告警事件 | 30 天 | 1 年 | 3 年 | 5 年 |

> 超出保留期的原始数据被聚合后删除；聚合数据满后归档冷存储或删除。
> 用户可手动导出后再删除，满足合规。

---

## 10. 数据模型概览（详见 15-data-model.md）

```
monitor
  id, owner_id, group_id, name, type, target, params (jsonb),
  interval_sec, node_selection (jsonb),
  assertions (jsonb), trigger_rule (jsonb),
  alert_policy_id, tags (text[]),
  status (up|down|paused|maintenance|unknown),
  current_streak_count, current_streak_start_at,
  last_check_at, last_result_id,
  created_at, updated_at, paused_at

monitor_check
  id, monitor_id, started_at, finished_at,
  result (up|down|partial),
  node_results (jsonb), summary (jsonb),
  triggered_event_id

monitor_check_aggregate_hour / aggregate_day
  monitor_id, bucket_at,
  total, up_count, down_count, partial_count,
  avg_response_ms, p95_response_ms

monitor_event
  id, monitor_id, type (down|up|degraded),
  started_at, ended_at, duration_sec,
  reason, affected_nodes (jsonb),
  acknowledged_by, ack_at, resolved_by,
  notes

maintenance_window
  id, owner_id, name, scope (monitor_ids[]|tags[]|all),
  start_at, end_at, cron_expr, timezone
```

---

## 11. 关键交互流程

### 11.1 添加 HTTP 监控全流程

```
User 进入 /app/monitors/new
  → 选 HTTP 类型 → 填 URL example.com
  → 选频率 1 min（Pro 用户）
  → 选节点池：CN + US + EU，pool_size=3
  → 默认断言：status 2xx-3xx + 响应时间 < 3000ms
  → 选告警策略 ap_default（系统预置：邮件+微信）
  → 点击"创建并测试"
  → 后端：
      • 创建 monitor 记录
      • 立即触发一次拨测（不计入历史）
      • 返回结果
  → UI 展示结果
  → Scheduler 进入正常拨测循环
```

### 11.2 异常 → 告警 → 恢复全流程

```
T0   定时拨测一轮：3 节点全部失败（fail_node_quorum 触发）
       → 本轮异常，连续异常计数 = 1
T1   下一轮：3 节点继续失败
       → 本轮异常，连续异常计数 = 2（达 consecutive_fail 阈值）
       → 触发事件创建 (monitor_event status=open)
       → 通知告警引擎
       → 告警引擎按 alert_policy 派发到通道
       → 用户收到微信 + 邮件
T2   异常持续：每 5 分钟提醒一次（按策略）
T3   恢复一轮：3 节点全部成功
       → 连续恢复计数 = 1
T4   连续恢复计数 = 2 → 状态变 UP
       → monitor_event status=resolved
       → 派发恢复通知
       → 计算总持续时间，写入事件
```

---

## 12. 性能与成本约束

### 12.1 调度容量估算

- 假设 80,000 个监控、平均 1 分钟频率 → 每分钟 80,000 次任务
- 平均每次任务 3 个节点 → 240,000 节点 RPM
- 单节点 RPM 上限假设 1000 → 至少需要 240 个节点（含余量）

### 12.2 优化策略
- 任务批合并：同一节点同一秒多个任务批发送
- 频率抖动：相同 cron 错峰（每个监控分配偏移 0-60s）
- 优先级队列：付费档高优先
- 节点亲和：尽量复用上次成功的节点（减少冷启动）

### 12.3 失败降级
- 节点不响应 → 自动剔除并补一个节点
- 调度器堆积 → 自动延长低优先监控的频率

---

## 13. 反误报机制（关键卖点之一）

| 机制 | 作用 |
|---|---|
| 多节点 quorum（2/3、3/5） | 避免单节点抖动 |
| 连续 N 次失败才告警 | 避免瞬时抖动 |
| 节点能力分级（tier1 权重高） | 避免劣质节点拖累 |
| 节点失败率监控（节点连续失败自动剔除） | 防故障节点污染 |
| 用户可标记"误报"反馈 | 持续优化策略 |
| 状态机：UP → DEGRADED → DOWN（中间态） | 减少剧烈跳变 |

---

## 14. 与其他模块的接口

| 模块 | 接口 |
|---|---|
| `05-alerting.md` | 监控 → 告警引擎：`AlertEvent { monitor_id, type, severity, started_at, context }` |
| `06-status-pages.md` | 监控状态被引用展示 |
| `07-reports-and-dashboards.md` | 月度可用率、SLA 报告 |
| `08-open-api.md` | API CRUD monitors、查询结果 |
| `10-nodes-and-agents.md` | 节点能力 / 路由策略 |
| `09-billing.md` | 监控数量 / 频率受订阅档限制 |

---

## 15. 阶段交付清单

### S2（4–8 月）必须交付
- M01–M08 完整支持
- M09（基础 JSONPath）
- 列表 / 详情 / 创建 / 编辑 / 暂停 / 删除
- 分组 + 标签 + 维护窗口
- 批量导入（CSV）/ 导出
- 反误报机制全套
- 与告警引擎、状态页打通
- API CRUD（基础）
- **v2 NEW: LLM 故障复盘自动起草上线(P1 提前,K4)— eval ≥ 4.0/5 才允许 prompt 上线;强制人工审核 + AI 标识 + sanitize**
- **v2 NEW: Anchor 偏差实时告警(配合 10 模块)— 数据污染早期检测**

### S3（8–14 月）增量
- M10 心跳监控
- M11 浏览器级监控
- M13 多端口批量
- M14 邮件服务器健康
- **v2 NEW: M21 LLM Endpoint 健康监控**
- **v2 NEW: M22 Tool/API Endpoint 健康监控**
- **v2 NEW: M23 RAG/Vector Store 健康监控**
- Terraform Provider
- 高级 JSONPath / JSON Schema
- 监控分析（异常根因建议）
- 多区域虚拟监控
- **v2 NEW: 多 LLM 投票复盘(可选,降低单 LLM 幻觉)**

### S4（14+ 月）增量（企业版）
- M12 多步骤事务监控
- **v2 NEW: M24 Agent Output Quality(LLM 评估器评分)**
- 高级 SLA 报告
- 监控变更审计日志

---

## 16. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 用户配置 10s 频率消耗节点资源 | 严格档位限制 + 单用户 RPS 上限 |
| 浏览器监控成本高 | Headless Chrome 单独 Worker 集群，独立计费 |
| 心跳监控被滥用做 status badge 流量放大 | Token 维度限速，反复触发自动降级 |
| ICP 备案变更监控数据源不稳 | 多数据源 + 用户可关闭该监控类型 |
| 误报率难达成 1% 目标 | S3 推出 ML 异常检测助力 |
| **v2 NEW: M21 LLM 监控被滥用爆别人账单** | 目标黑名单 + 所有权可验证 + Free 档不开 + budget_per_check_usd 单次预算上限 |
| **v2 NEW: M21 单次 LLM 调用费用累积导致用户意外大额** | 80%/95%/100% 三级提醒 + 用户可设硬上限自动停服 + 默认低频(5min) |
| **v2 NEW: LLM 复盘草稿幻觉/造谣/泄密** | Prompt 约束 + 必须人工审核 + AI 标识 + sanitize + 离线 eval ≥ 4.0/5 |
| **v2 NEW: Anchor 节点偏差导致数据污染** | 实时偏差告警 + 自动剔除阈值 + Verdict 报告排除"低置信节点" |

---

## 17. 决策记录（已锁定，见 DECISIONS.md）

### v1.0
- ✅ **F1** 浏览器级监控（M11）：**Team 起**
- ✅ **F3** 心跳监控 token URL：**Token 本身即鉴权**；可选 HMAC payload 签名作为加强项
- ✅ **A8** 维护窗口：**默认不计入可用率分母**

### v2.0 (K 节, 2026-05-12)
- ✅ **K4** LLM 故障复盘自动起草:从 P3/S4 提至 **P1/S2**;强制人工审核 + AI 标识 + sanitize + 离线 eval ≥ 4.0/5 才允许新 prompt 上线
- ✅ **K-监控 M21 档位**:Free 不开,Pro+ 默认 5 min,Agent Pro 1 min
- ✅ **K-监控 budget**:M21 必须配 budget_per_check_usd,超额自动暂停

### 待定（不紧迫）

- [ ] 自定义脚本 probe（JS）：开放度 vs 安全，S4 评估
- [ ] 自由 cron 频率 vs 固定档位：固定档位（PRD 默认）
- [ ] **v2 NEW** Agent Output Quality(M24) 评估器选型 — S4 评估
- [ ] **v2 NEW** 多 LLM 投票复盘 — S3 评估
