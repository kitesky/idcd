# Eng Review Brief — idcd PRD v2.0

> 生成日期:2026-05-13
> 对应 CEO Plan:`~/.gstack/projects/idcd/ceo-plans/2026-05-12-idcd-prd-v2-expansion.md`
> 这是给后续 `/plan-eng-review` 的输入文档,**不是 PRD 一部分**,而是 reviewer 的 entry point
> 阅读顺序建议:本文档 → OVERVIEW.md → DECISIONS.md §K + §L → 各模块按需

---

## 0. 给 Reviewer 的话

这次 eng review 不是常规的 PRD review。背景:

- PRD v1 是"成熟的国内化监控 SaaS",18 模块完整,40 项决策已锁,可以工程实施
- v2 经过 plan-ceo-review EXPANSION 模式,**叠加 3 栈新业务**:Evidence-as-a-Service / AI Agent Observability + MCP server / 权威排行榜 + LLM 故障复盘提前
- 新增 2 个 PRD 模块(18-evidence / 19-ai-agent),修订 17 个现有文件
- **9 项 K 决策已锁,但工程可行性未经过工程视角的二次校验**
- **8 项 Reviewer Concerns(见 DECISIONS §L)是 CEO 视角抓出的风险点,需 eng 视角拆解为可测可验证的检查项**
- v2 引入的 1 个 CRITICAL GAP(Verdict 付费失败兜底)+ 7 个 high-severity 错误面 + 3 个 high-severity 安全威胁,**实施前必须有工程答复**

**你的任务**:不是再创造方案,是把 v2 PRD 的所有"假设"压到地面,告诉创始人 "这些工程可不可行 / 哪些假设有问题 / 哪些 SLO 是空中楼阁"。

**Output 期望**:
- 每个高风险面的"可不可行"判断 + 一线工程师能立刻执行的修改建议
- 容量 / 性能 / 延迟 数字的现实校准(PRD 里的目标 P95 是猜的还是有数据?)
- 应急 SOP 的"时间线可执行性"(KMS quorum 1 小时召回是不是空话?)
- 8 项 Reviewer Concerns 逐一答复 / 拆解 / 升级

**posture 建议**:HOLD SCOPE 为主。v2 scope 决策已经在 CEO review 里被用户接受;eng review 不需要重新挑战 scope。除非发现"该 scope 工程根本不可行"才升到 SCOPE REDUCTION,但应该极少。

---

## 1. v2 引入的所有架构改动(对比 v1 baseline)

### 1.1 新增独立子域 / 服务

| 组件 | 子域 | 部署 | 风险面 | 必查项 |
|---|---|---|---|---|
| Attestation Service | `attest.idcd.com` | 独立 Docker compose;独立 KMS 调用 | 信任根失窃 / 报告生成失败 / 自检失败 | KMS 集成端到端 / 签名 → 时间戳 → 归档 → 自检的状态机完整性 |
| MCP Server | `mcp.idcd.com` | 独立部署;JSON-RPC + HTTP+SSE 双协议 | 凭证泄露 / 客户端兼容性 / 计费混乱 | 客户端兼容矩阵 / token 撤销链路 / 计量与 API Key 隔离 |
| Public Verify | `attest.idcd.com/verify` | 与 Attestation Worker 解耦的轻量服务 | DDoS 放大 / 主服务挂时 verify 也挂 | 独立部署能否在主 Attestation 离线时仍可用 |
| MCP 文档站 | `mcp.idcd.com/docs` | Nextra SSG | 一般风险 | 与主 docs 站的内容一致性 |

### 1.2 新增外部信任依赖

| 依赖 | 角色 | 失败影响 | 容灾 | 必查项 |
|---|---|---|---|---|
| 云 KMS(AWS / 阿里云) | Verdict 签名 | KMS 挂 = 暂停新 Verdict | 同地域 AZ;跨厂商 fallback 未规约 | 选型决策的 latency / 可用性数据 / 成本 / due diligence(若用 AWS,国内收款主体合规如何?) |
| RFC3161 TSA | 报告时间戳 | 主备三家全挂 = 暂停新 Verdict | DigiCert + GlobalSign(S2)+ NTSC(S3) | TSA 历史可用率数据 / 切换响应时间 / 费用 |
| LLM Provider | 复盘起草 + Verdict 解读 | LLM 失败 = 降级模板 | 多 Provider 抽象层 | 抽象层实际接口设计 / prompt 跨 Provider 一致性 / 出海 LLM 国内访问 |
| Blockchain Anchor(可选) | 报告链上锚定 | 链拥堵 = 跳过锚定标记 | S3 add-on,默认 RFC3161 已够 | S3 评估时再说,目前不阻塞 |

### 1.3 新增数据库表(14 张)

详 15-data-model §4.X.1-§4.X.14:

| 表 | schema | 类型 | 高风险点 |
|---|---|---|---|
| `verdict_order` | idcd_attest | OLTP | 状态机 enum 完整性;Paddle 关联 |
| `verdict_report` | idcd_attest | OLTP | content_hash / signature / TSA 字段不可空 |
| `attestation_record` | idcd_attest | append-only | 索引设计;100k+/日 写入压力 |
| `tsa_response` | idcd_attest | append-only | 同上 + 二进制 blob 存储成本 |
| `key_ceremony_log` | idcd_audit | 只增不删 | 双人审批写入;不能 DELETE / UPDATE |
| `mcp_session` | idcd_mcp | OLTP | last_activity_at 高频更新 |
| `mcp_tool_call` | idcd_mcp | **Hypertable** | 28k+ 写入/秒峰值;CK vs Hypertable 决策点 |
| `mcp_token` | idcd_mcp | OLTP | 撤销路径要 immediate;hash unique 索引 |
| `agent_obs_monitor` | idcd_main | OLTP | budget_per_check_usd 实时累加,并发写 |
| `agent_obs_event` | idcd_main | **Hypertable** | 同 mcp_tool_call;时序压力 |
| `compliance_subscription` | idcd_main | OLTP | 多 team 关联;period_end 续费触发 |
| `leaderboard_report` | idcd_main | OLTP + jsonb | 月度 ~12 行/年,jsonb 大字段查询 |
| `leaderboard_optout_request` | idcd_main | OLTP | 低频 |
| `postmortem` (扩展) | idcd_main | 现有表扩字段 | 迁移策略 |

**必查项**:
- 跨 schema FK 不用的"应用层 join"具体怎么实现?Repository 模式还是 service 拼接?
- mcp_tool_call 28k 写入/秒峰值下,Hypertable vs ClickHouse 的 trade-off 数字 — PRD 说 "S3 评估",但实际何时切换?触发指标?
- `idcd_attest` schema 与 `idcd_main` 是否同 PostgreSQL cluster?如果是,信任根 schema 与业务 schema 共连接池的安全边界?

### 1.4 新增 API 端点(40+)

详 16-api-spec.md(本次 review 也需要 reviewer 起 16 的具体 OpenAPI yaml,目前 16 是 v1 的)。

关键新端点组:
- `/v1/verdict/*`(6 个):订单 / 报告 / 分享
- `attest.idcd.com/verify`(3 个):公开验签 / 公钥查询 / 仪式记录
- `/v1/compliance/*`(3 个):年订管理
- `attest.idcd.com/dispute/*`(2 个):申诉
- `/v1/mcp/tokens/*`(3 个):token 管理
- `mcp.idcd.com/*`(MCP 协议层):session + tool call(JSON-RPC,不是 REST)
- `/v1/agent-obs/*`(4 个):Agent obs 监控 + 事件流(SSE)
- `/v1/mcp/usage`(1 个):MCP 独立用量

**必查项**:OpenAPI 规范完整性;rate limiting key 设计;SSE 长连接资源消耗

### 1.5 新增协议适配

- **Anthropic MCP spec**:JSON-RPC over stdio + HTTP+SSE 双模式;客户端兼容性矩阵(Cursor / Claude Code / Codex)
- **OpenAI Agents Protocol**:S3 末评估(目前不阻塞)
- **PAdES PDF 签名**:报告嵌入签名 + 时间戳的二进制格式

**必查项**:
- PAdES 等级选 B-T 还是 B-LT(详 18 §10 K-OPEN);影响验签独立性
- MCP 协议演化时的双轨成本(过去 6 个月 Anthropic 改了 spec 几次?)
- JSON-RPC over stdio 模式如何承载文件上传 / 大 payload?

### 1.6 新增长流程

| 流程 | 涉及组件 | 关键 SLA | 风险 |
|---|---|---|---|
| Verdict 生成(端到端) | DB → Worker → KMS → TSA → S3 → Self-Verify | P95 ≤ 90s / 100% 自检通过 | **CRITICAL GAP** 失败兜底 |
| KMS 应急撤销 | KMS API → Shamir 重组 → 新 sign key 切换 → 通知 | 6 小时完成 | quorum 召回是否现实 |
| 节点失窃应急 | 检测 → drain → revoke → 数据污染恢复 | 1 小时完成 | 实际节点失窃从未演练 |
| Verdict 工单兜底 | 工单创建 → 操作员介入 → 用户补偿 | 1 小时 SLA | 一人创业 SLA 可达性 |
| Agent OTA 3 级灰度 | L1 1% → L2 10% → L3 100% | 失败自动回滚 5 分钟 | 自动回滚机制实际可用性 |
| LLM 故障复盘起草 | 事件 resolve → LLM 生成 → 人工审核 → 发布 | T+10m 草稿 | LLM 调用费用控制 |

**必查项**:每个 SLA 是否有 monitoring + alerting + dashboard?**没有 metric 的 SLA 等于谎言**。

---

## 2. 8 项 Reviewer Concerns 的工程化拆解(DECISIONS §L)

### Concern 1:Verdict 报告"法定效力"边界(法律风险)

**CEO 视角**:报告文案必须明确"非鉴定结论"。

**Eng 视角必查**:
- [ ] PDF 模板中"非鉴定结论"段落是否硬编码 / 渲染层 / 不可被用户编辑?(防止用户改文案后冒充)
- [ ] Verdict 报告 metadata 中是否有 `report_type=observation_only`(非 `forensic_conclusion`)字段?用于第三方解析
- [ ] 公开验签接口返回的字段是否包含报告类型?(避免误导)
- [ ] 营销 / 文档 / 控制台 / 邮件模板中 "鉴定" / "认定" / "判定" 等词是否有 CI lint 拒绝?

**Severity**:HIGH(法律红线,但有缓解措施)

### Concern 2:Verdict 付费失败 = CRITICAL GAP(产品质量根本)

**CEO 视角**:WAL 状态机 + 自检失败自动退款 + 工单兜底 + P1 告警。

**Eng 视角必查**:
- [ ] **完整状态机 ASCII 图**(09 §13.5 已写但需要 reviewer 验证):
  - `pending → paid → generating → (delivered | failed | refunded)`
  - 每个 transition 是否原子操作?Paddle webhook 失败 / Worker crash / KMS 间歇性失败 各自如何回滚?
- [ ] **idempotency** 保证:同一 order_id 多次进入 queue 不能生成两份报告
- [ ] Paddle refund API 失败的兜底:**自动退款失败** → 用户两小时拿不到报告 + 拿不到退款 = 灾难。需要"退款 retry queue + 人工兜底"
- [ ] **DLQ 监控**:dead letter queue 任何 message 出现 5 分钟内必须告警
- [ ] 历史失败模拟:在 staging 注入"KMS 间歇失败 / S3 写失败 / TSA 慢响应"等故障,验证 SLA
- [ ] Self-verify 的"独立路径":验签 worker 不能复用 sign worker 的代码 / 配置 / 密钥缓存,否则验证无意义

**Severity**:**CRITICAL**(必须实施前 100% 解决)

### Concern 3:CDN 厂商关系长期博弈

**CEO 视角**:法务备案 + 律所储备 + 退出通道 + 月度沟通。

**Eng 视角必查**:
- [ ] `/leaderboard/optout` 表单的 spam 防护(防止恶意填写假厂商)
- [ ] "本月排行榜"草稿在发布前是否有"自动 LLM lint"检查贬损措辞?
- [ ] 已发布报告的"事后修订"机制:厂商发现数据错误申诉 → 修订流程(不删除原报告,出"勘误公告")
- [ ] 厂商邮件通知的可投递性(48 小时反馈窗口是否真能送达?)

**Severity**:MEDIUM(法律风险长期累积)

### Concern 4:18+19 新模块 PRD 膨胀(组织风险)

**CEO 视角**:S3 中期 PRD 三层重组(Core / Extension / Platform)。

**Eng 视角必查**:
- [ ] 当前 18 模块的工程师 onboarding 时长基线?(没有数据就是猜的)
- [ ] PRD 跨模块引用是否 CI 验证?(如 09 引用 18 §2.6 / 18 引用 12 §3.5,引用断链如何检测)
- [ ] PRD 重组的"工程实施成本":重新组织目录 vs 持续添加章节,哪个总成本低?(reviewer 给出建议)

**Severity**:LOW(可推迟)

### Concern 5:签名密钥泄露应急

**CEO 视角**:revoke + rotate + 通知 + transparency,6 小时完成。

**Eng 视角必查**:
- [ ] **Shamir 3-of-5 quorum 召回 SLA**:5 个持有人(创始人 / 法务公司 A / 律所 B / 工程师 C / Backup HSM)在"周日凌晨 3 点"是否真能 1 小时内联络上?
  - 替代方案:Backup HSM 自动可重组(冷备硬件 + 物理锁)能否缩短到 30 分钟?
- [ ] 通知 "所有历史报告持有者" 的实际机制:
  - 数据库 query:`SELECT DISTINCT owner_id FROM verdict_order WHERE delivered_at < now() AND owner_id IS NOT NULL`
  - 邮件发送速率 / 邮件服务商防 spam 限速
  - 用户反馈渠道(他们验签自检后如何回报?)
- [ ] **revoke 期间**:Attestation Service 切只读,但**已发的报告仍可被验签**(用 old public key)。这条逻辑是否实现?
- [ ] Backup HSM 的物理位置 + 备份频率 + 离线测试演练频率

**Severity**:CRITICAL(信任根失守 = 公司死亡)

### Concern 6:MCP token 凭证泄露

**CEO 视角**:短期 token + IP 白名单 + 异常突增告警 + 一键撤销。

**Eng 视达必查**:
- [ ] 异常突增告警的具体 threshold:
  - "24h 调用量 > 历史 P95 × 5 倍" 这个数字怎么校准?如果用户突然项目上线,正常增长 5x?
  - 替代方案:** 监督学习 + 用户行为画像**(成本高)?还是简单 rule(用户接受偶尔误报)?
- [ ] IP 白名单的实际可用性:用户 ISP 动态 IP / VPN 切换 / 多机房部署 → 白名单维护成本
- [ ] "一键撤销"的执行时间:从用户点击到所有 active session 断开,目标 ≤ 30 秒
- [ ] GitHub 扫描自动失活:具体接哪个 service(GitGuardian? 自家正则?)
- [ ] 短期 token 的 refresh 用户体验:Cursor 用户每 24h 重新生成 token 是否能接受?

**Severity**:HIGH(企业用户的真实威胁)

### Concern 7:LLM 复盘幻觉/造谣/泄密

**CEO 视角**:Prompt 约束 + 人工审核 + AI 标识 + sanitize + eval ≥ 4.0/5。

**Eng 视角必查**:
- [ ] **离线 eval 数据集**:"每月 50 个真实事故"的来源 — 用什么标注?谁标注?标注质量?
  - 冷启动:S2 上线时没有 50 个真实事故可用,如何起步?
  - 替代:用公开事故数据(如 AWS / Cloudflare 历史故障公告)做 bootstrap
- [ ] sanitize 规则:具体禁止哪些词?LLM 输出的"AWS"是否触发?(可能是合法描述,也可能是甩锅)
- [ ] 多 LLM 投票复盘(S3)的成本爆炸:3 个 LLM × 50 个事故 / 月 = 150 次 LLM 调用,单价 + token 估算
- [ ] **强制人工审核**的工程实现:发布按钮 disable 直到 reviewer_id 已设;reviewer_id 与 owner_id 必须不同?(避免一人审自己的)

**Severity**:HIGH(公开发声不可逆)

### Concern 8:Anchor 节点偏差告警未规约

**CEO 视角**:实时偏差告警 + 自动剔除 + Verdict 排除"低置信"。

**Eng 视角必查**:
- [ ] **偏差阈值**(轻 / 中 / 重 / 致命)的数据基础:
  - "中位数 × 2 / 3 / 5" 这些倍数是猜的吗?用历史数据校准是必要的
  - 不同区域 / 时段(白天 vs 凌晨)阈值是否需要差异化?
- [ ] Anchor 节点自身偏差检测的循环:3 个 Anchor 互查,如果 2 个被同时攻陷会怎样?(系统会自动降级正常的那个 Anchor)
- [ ] "数据污染自动恢复":过去 N 分钟标记 low_confidence — N 是固定 5/15/30 分钟还是动态?
  - 动态:由偏差持续时间决定(偏差持续 10 分钟 → 标记前 10 分钟数据)
  - 但攻击者可以"先 8 分钟正常 + 后 2 分钟造假" → 系统只标记后 2 分钟,前 8 分钟造假未被发现
- [ ] Verdict 报告生成时的"low_confidence 节点排除":如果排除后节点数 < 3,如何处理?(报告拒绝生成?降级 confidence label?)

**Severity**:HIGH(数据污染影响 Verdict 信任根)

---

## 3. 关键性能与容量挑战

### 3.1 数据库写入压力

**v2 新增的写入源**:
- `mcp_tool_call`:Agent Pro 档 1M calls/day = 12 qps base;峰值若 5x = 60 qps × 单条 ~1KB = 60 KB/s(不算大)
- 但 S3 GA 后**月调用 5M = 60 qps base 持续**,叠加 v1 的 28k qps 时序写入
- `agent_obs_event`:10k 监控 × 60s 频率 = 167 qps,叠加 LLM endpoint 多区域验证 ×3 = 500 qps
- `attestation_record`:每份 Verdict 报告写 ~10 行 audit + 1 行 verdict_report = 1k 报告/月 = 0.4/min(低)

**叠加总量**:
- v1 时序写入(monitor_check):28k qps 峰值(已在 14 §9.2)
- v2 新增:600-1000 qps 峰值
- 总:~30k qps 时序写入

**Reviewer 必查**:
- TimescaleDB 单实例 30k qps 是否真实可用?(实测数据 vs 文档承诺)
- Hypertable 分区策略(按天 vs 按小时)对查询的影响
- WAL 与 streaming replication 对主从延迟的影响(国内主 + 海外热备)
- 何时切 ClickHouse?触发指标(单日新增 > 10GB / P99 > 100ms)是否合理?

### 3.2 Verdict 端到端延迟

**v2 PRD 假设 P95 ≤ 90s**:

| Step | 估算 latency | Reviewer 必查 |
|---|---|---|
| 数据拉取(TimescaleDB) | 1-5s(取决于时间窗大小) | 30 天窗口 vs 6 年窗口的差异 |
| 多节点交叉验证 | 几乎 0(纯计算) | — |
| LLM 解读(可选) | 5-30s | 出海 LLM 国内访问延迟 |
| PDF 渲染 | 2-10s(取决于报告大小) | Headless Chrome vs 服务端 PDF lib 选型 |
| 内容哈希 SHA-256 | < 1s | — |
| KMS sign API | **0.5-2s**(云 KMS 实测) | AWS KMS vs 阿里云 KMS latency 数据 |
| RFC3161 TSA(主) | **2-10s**(网络往返) | DigiCert / GlobalSign 实测 |
| 嵌入 PDF + S3 上传 | 1-3s | — |
| 自检(重新读 + 验签) | 1-3s | 必须独立路径 |

**累加 P95**:**12-66 秒**(主路径都成功);**+ TSA fallback / KMS retry → 90+ 秒可能**。

**Reviewer 必查**:
- 这 9 个 step 是串行还是部分并行?(LLM 与 PDF 渲染可以并行)
- TSA 选型时是否做过 P95 实测?DigiCert 的中国大陆 P95 vs 美国 P95?
- 90 秒 SLA 在多节点 / 多 LLM provider 的最差路径下是否可达?

### 3.3 MCP 客户端兼容矩阵

| 客户端 | 当前 spec 版本 | 协议模式 | 测试方式 |
|---|---|---|---|
| Claude Code | 2026-05 | stdio + http+sse | 自动化 smoke test |
| Cursor | 2026-04 | stdio | 自动化 smoke test |
| Codex CLI | 2026-03 beta | stdio | 手动验证 |
| 自家 SDK(py) | 自维护 | http+sse | 自家 CI |
| 自家 SDK(ts) | 自维护 | http+sse | 自家 CI |

**Reviewer 必查**:
- "每发布前 smoke test" 的具体实现:CI 中如何 spawn Cursor / Claude Code 客户端?(client headless mode 是否存在?)
- spec 演化频率:过去 12 个月 Anthropic 改了 MCP spec 几次?向后兼容性?
- spec 分叉风险:OpenAI Agents Protocol 真的会出现独立?是否需要 adapter 层?

### 3.4 LLM 调用成本

**v2 引入的 LLM 调用源**:
- LLM 故障复盘起草(P1 S2):每个 resolve 事件 1 次 LLM 调用;若 1000 监控 × 1 事件/月 = 1000 次/月
- LLM endpoint 监控(M21):每用户监控 12-288 次/日 × LLM 调用本身就是 LLM
- Verdict LLM 解读(可选):件价附加,每份报告 1-3 次 LLM 调用
- 公告草稿(配合 06):每事件 1 次 LLM(与复盘草稿可合并)

**月成本估算**:
- 故障复盘:1000 × ~5k token output × $0.015/1k = $75/月(GPT-4-class)
- LLM endpoint 监控:用户自付(M21 设计如此)
- Verdict:¥0.50/份 LLM 成本 × 100-1000 份/月 = ¥50-500/月
- 总:**$100-200/月起,S3 GA 后可能 $500-1000/月**

**Reviewer 必查**:
- LLM Provider 抽象层是否真能切换不同 Provider?(prompt 跨 Provider 一致性是难点)
- 单 prompt token 上限 / 用户档位限制 / 月预算硬上限 的具体实现
- 缓存策略:相似事件能否复用 LLM 结果?

---

## 4. 跨模块依赖与边界

### 4.1 三栈独立但共用 PG 的边界

**架构假设**(14 §H5):
- 三栈虽独立子域,但共用同一 PostgreSQL cluster
- 通过 schema 隔离:`idcd_main` / `idcd_attest` / `idcd_mcp` / `idcd_audit`
- 跨 schema 不用 FK,通过应用层 join

**Reviewer 必查**:
- "应用层 join" 的具体实现:Repository 模式 / service 拼接 / GraphQL?
- 共用连接池的安全边界:Attestation Worker crash 是否影响主业务连接池?
- 跨 schema 事务:同一请求需要读 idcd_main 写 idcd_attest 时如何保证一致性?
- 备份策略:idcd_attest 6 年合规留存 vs idcd_main 5 年,是否独立备份?

### 4.2 09 计费 / 18 Evidence / 19 MCP 计量边界

**v2 的计费维度**:
1. **订阅档(09 §2.1)**:监控数 / 频率 / API 配额 / 状态页数 / 团队成员
2. **Verdict 件价(09 §2.6)**:¥199-999/件,独立非订阅
3. **Compliance 年订(09 §2.7)**:¥3k-30k/年,组织级
4. **Agent Pro 档(09 §2.8)**:¥299/月,MCP units 独立池

**Reviewer 必查**:
- 4 个计费维度共用同一用户账户余额还是各自独立?(09 §10 余额系统 vs 独立)
- Verdict 件价的"Pro+ 9 折"折扣如何与订阅档绑定?
- 用户同时有"Pro 订阅 + Compliance Pro + Agent Pro"时,API 调用走哪个配额池?
- Paddle webhook 多个订单 / 多个 SKU 的关联追溯

### 4.3 12 合规 / 18 Evidence / 13 SEO 的法律边界

**v2 的法律红线**:
- Verdict "非鉴定结论"(18 §1.2 / 12 §3.5)
- 排行榜公共边缘 only(13 §3.6 / 12 §19)
- LLM 复盘不允许指定具体责任方(07 §6.3 / 04 §17 / 06 §5.8)

**Reviewer 必查**:
- 3 个边界是否有"工程 CI lint"自动校验?(光靠人工审核会漏)
- 报告 PDF 模板 / 营销文案 / 控制台 UI 的"禁用词" lint 工具:用什么实现?(grep based 还是 NLP based?)
- LLM 输出的"禁止断言责任方"如何在 prompt 层 + 后处理 sanitize 层双重保证?

---

## 5. 关键决策的工程可行性挑战

### 5.1 KMS 选型(K2)

**CEO 决定**:云 KMS(AWS 或阿里云),根据收款主体地区。

**Reviewer 必查**:
- **AWS KMS**:中国大陆访问 latency P95 ≈ 200-500ms(经香港中转),美国主体合规
- **阿里云 KMS**:国内访问 P95 ≈ 20-50ms,但海外用户访问 P95 可能 200-800ms
- **跨厂商容灾**:若 AWS KMS 区域性事故,如何切到阿里云?(密钥 portability 问题)
- **成本**:Verdict 月 1000 份 = 1000 次 sign API 调用,AWS $0.03/万次 = ¥0.003/月(几乎免费,实际成本在数据传输和审计存储)

**建议**:**起步选阿里云 KMS**(国内主体 + 国内用户为主,latency 优势明显);S4 企业出海时再加 AWS KMS adapter。

### 5.2 TSA 主备(K2 配套)

**CEO 决定**:DigiCert + GlobalSign + NTSC(S3 加)

**Reviewer 必查**:
- DigiCert TSA 商业 SLA?(目前是免费 service 还是付费?年费?)
- GlobalSign TSA 同上
- NTSC(国家授时中心):**国内司法场景认可度高,但 API 稳定性 / 文档完整性 不如商业 TSA**;实际接入难度?
- TSA 切换时间:主失败到切备的真实 latency(网络往返 + 重试)
- TSA 响应 blob 的存储成本:平均 ~5KB/份 × 1000 份/月 × 6 年 = ~360MB,可忽略

**建议**:**S2 选 DigiCert 主 + GlobalSign 备**,S3 NTSC 加入但作为第三备,因为 NTSC 的 API 在工程实施时可能踩坑。

### 5.3 Anchor 偏差告警阈值(§K-节点)

**CEO 决定**:轻度 ×2 / 中度 ×3 / 重度 ×5 / 致命 系统性矛盾。

**Reviewer 必查**:
- **这些倍数是数据校准过的吗?**(回答可能是"no")
- 真实数据中,合法的节点抖动占多少 σ?恶意伪造数据的偏差范围?
- 不同区域 / 时段(白天 vs 凌晨)阈值需要不需要差异化?
- **建议实施**:S1 部署后先**收集 30 天 baseline**,再 calibrate 阈值;不要直接拍 ×2/×3/×5

### 5.4 Verdict 自检独立性

**CEO 决定**:每份报告生成后 self-verify;每日抽样 10 份历史报告独立验签。

**Reviewer 必查**:
- "独立路径"的具体边界:自检 worker 不能复用 sign worker 的代码 / 配置 / 密钥缓存
  - 进一步:运行在不同物理机?不同 VPC?不同语言实现?
- 抽样 10 份/日的"随机性":伪随机算法是否能被攻击者预测后规避?
- 自检失败时的处理:已交付给用户的报告被发现失败,如何召回?(伦理 + 法律)

### 5.5 MCP 客户端兼容性的承诺

**CEO 决定**:每发布前 smoke test Cursor / Claude Code / Codex。

**Reviewer 必查**:
- Cursor 客户端是否提供 "headless / CI" 模式?如果不提供,smoke test 如何自动化?
- "兼容矩阵" 的工程产出:具体哪个 Cursor 版本 × 哪个 idcd-mcp 版本 = 通过/失败?
- 客户端版本上限:用户 1 年没升级 Cursor,idcd-mcp 是否保持兼容?(版本支持窗口)

---

## 6. 失败模式与应急 SOP 的可执行性

### 6.1 KMS 密钥仪式 SOP(11 §15.5)

**CEO 流程**:首次 root 仪式 / 90 天 sign key 轮换 / 应急撤销(6 小时).

**Reviewer 必查**:
- **Shamir 3-of-5 quorum 持有人定位**:
  - 5 人:创始人 / 法务公司 A / 律所 B / 工程师 C / Backup HSM
  - "周日凌晨 3 点" 是否真能 1 小时内联络上 3 人?
  - 替代:Backup HSM 是冷硬件可独立重组(无需人工)→ 等同 1-of-5,但物理获取需要去保险柜
- 加密通道:quorum 持有人提交切片走什么通道?邮件加密?Signal?物理寄回?
- 测试演练:首次仪式后是否每年至少 1 次"模拟应急召回演练"?
- **法律可执行性**:5 个切片的"接收人法律合同"是否签了?如果工程师 C 离职,如何接力?

**建议**:**S2 上线前必做一次"模拟应急召回演练"**,验证 6 小时 SLA;不演练就是空话。

### 6.2 节点失窃应急 SOP(10 §6.6 / 12 §21)

**CEO 流程**:1 小时内完全踢出 + 数据污染恢复.

**Reviewer 必查**:
- 检测路径的覆盖率:OCSP / Anchor / fingerprint / 流量 4 个检测路径,**任一触发即应急**;有没有死角?(如:攻击者拿到节点 fingerprint 但不触发流量异常?)
- "1 小时内完全踢出" 的实际时间分布:
  - CRL/OCSP 推送:1-5 分钟(取决于 CA 的 publish 间隔)
  - Gateway 关闭 active 连接:5 秒(主动 kill connection)
  - 节点池剔除:5 秒(scheduler 配置更新)
  - 数据污染恢复:**这步可能是 30+ 分钟**(需要 Aggregator 重算 + Verdict 报告自检)
- "数据污染恢复" 的成本:1000 份过去 7 天的 Verdict 报告需要重新生成?

**建议**:在 staging 注入"伪造节点 fingerprint" 等场景,实测应急时间。

### 6.3 Verdict 工单兜底 1 小时 SLA(11 §15.4)

**CEO 流程**:工单 1 小时内人工接手 + 自动退款 + 系统性问题告警

**Reviewer 必查**:
- **1 人创业的 SLA 可达性**:你不在线时(睡觉 / 出差 / 假期)谁兜底?
  - 替代:**第二个 Operator**(配偶 / 朋友 / 外包客服)
  - 或:**纯自动化兜底**(失败必退款 + 无人工介入,但用户体验差)
- "系统性问题"自动聚合阈值:>= 5 失败/小时 → P0,这个数字是否过低?(单 KMS 间歇性失败可能瞬间触发 5+)

### 6.4 LLM 故障复盘 eval ≥ 4.0/5 才上线(07 §6.4)

**CEO 流程**:每月 50 个真实事故人工打分.

**Reviewer 必查**:
- **冷启动**:S2 上线时没有 50 个真实事故,如何 ship?
  - 替代:用公开事故数据(AWS / Cloudflare 历史故障)bootstrap 30 个 + 内部 dogfood 20 个
- 标注成本:50 个事故 × ~10 分钟标注 = 8 小时/月,谁标注?(创始人?)
- eval 不达标 ship 后再 fail 的成本:回滚 prompt 容易,但已发的"AI 起草"复盘草稿无法撤回

---

## 7. 测试覆盖度建议

### 7.1 必须建立的常驻 eval(沿用 CEO 审查 §6)

| Eval 套件 | 频率 | 输入 | 通过标准 | 实施 owner |
|---|---|---|---|---|
| Verdict 验签自检 | 每日 | 抽样 10 份历史报告 | 100% 通过 | Attestation Worker |
| LLM 复盘质量 | 每周 | 50 个新事故,人工打分 | ≥ 4.0/5 | 创始人 / 标注外包 |
| MCP 兼容矩阵 | 每发布前 | Cursor / Claude Code / Codex 各 1 遍 | 100% smoke test 通过 | CI pipeline |
| 节点污染检测 | 每分钟 | Anchor 偏差监控 | 实时 |  Aggregator |

**Reviewer 建议**:在 PRD 17-roadmap 中明确这 4 套 eval 的"S2 末必须建立"的硬指标。

### 7.2 红队测试矩阵(必须 S2 前完成)

| 攻击场景 | 预期防御 | 测试方法 |
|---|---|---|
| 节点 client cert 失窃,伪造拨测结果 | Anchor 偏差告警 + 数据污染恢复 | staging 部署"恶意节点"持续 30 分钟 |
| MCP token 失窃,大量爆账单 | 异常突增告警 + 用户硬上限 | 注入"突然 10x 调用量" |
| Verdict 报告被滥用诬告 | 目标黑名单 + 所有权验证 + 申诉 | 模拟"用户 X 短时间内对竞品域名提交 5 个 Verdict 请求" |
| LLM prompt 注入(让 LLM 输出威胁性公告) | sanitize + 人工审核 | 在事件描述中注入对抗性 prompt |
| Verdict PDF 篡改后冒充 | 公开验签拒绝 | 修改 PDF 任一字节后调 /verify |
| Anchor 节点被同时攻陷 | 多 Anchor 交叉 + 异常 Anchor 自动降级 | staging 同时污染 2/3 Anchor |
| KMS API 间歇性失败 | DLQ + retry + 工单兜底 | 注入 50% KMS 失败率 |
| Paddle webhook 漏送 / 重复 | nightly 对账 + idempotency | 模拟 webhook 失败 |

---

## 8. CRITICAL GAP 必须答复的清单

实施前**必须有工程答复 + 可演示原型**:

### 8.1 Verdict 付费失败 WAL 状态机(09 §13.5 + 18 §3.2)

- [ ] 完整状态机实现的代码 review
- [ ] 在 staging 注入 KMS / TSA / S3 / Self-verify 各 step 失败,验证退款链路
- [ ] DLQ 监控的告警通道配置
- [ ] **演示**:从用户付款到自检失败 → 自动退款到账,全流程视频/截图

### 8.2 KMS 应急撤销 SOP 演练(11 §15.5 / 12 §20)

- [ ] 5 个 quorum 持有人定位 + 法律合同
- [ ] 加密通道方案(Signal / 物理寄回 / PGP 邮件)
- [ ] **演练**:在 staging 模拟 sign key 泄露,真实走完 6 小时 SOP,记录每步实际耗时

### 8.3 Anchor 偏差告警阈值数据校准(10 §6.5)

- [ ] S1 部署后 30 天 baseline 数据采集
- [ ] 不同区域 / 时段的阈值差异化
- [ ] 阈值校准报告(数据 + 决策)

### 8.4 LLM 复盘 eval 数据集 bootstrap(07 §6.4)

- [ ] 30 个公开事故 + 20 个 dogfood 事故的标注数据
- [ ] 每月持续标注的 owner 指定
- [ ] eval pipeline CI 集成

### 8.5 MCP 客户端兼容测试自动化(19 §3.6)

- [ ] Cursor / Claude Code / Codex headless 模式或 CI 友好方案
- [ ] 每发布前的兼容矩阵 CI 报告
- [ ] 版本支持窗口政策(支持过去 6 个月发布的客户端版本)

---

## 9. eng review 进入条件检查

### 9.1 必须在 eng review 之前回答的问题

- [ ] **预算**:虽然用户说"暂不考虑",但 v2 引入 KMS / TSA / LLM 三个外部依赖月成本 $200-500;Eng review 需要确认这是 acceptable
- [ ] **人力时间预期**:用户 1 人 + 多 AI 协助,v2 的 Tier 1 ~ Tier 3 增量工作量估算:human 30-50 天 / CC 100-150 小时(扣除 dogfood + iteration)
- [ ] **是否真的同时跑三栈**:S2 Evidence MVP + S3 MCP server + S3 排行榜 — 单人 + AI 能并行?还是顺序?
- [ ] **第一个 Compliance / Verdict 客户来源**:S2 上线后从哪获取?(冷启动客户)

### 9.2 可以在 eng review 中讨论的问题

- 数据库分区 / Hypertable vs ClickHouse 切换时机
- TSA 选型具体厂商
- LLM Provider 选型(Anthropic vs OpenAI vs 自家)
- PAdES 签名等级
- 区块链锚定的具体链选

---

## 10. 输入材料清单 + 推荐审查顺序

### 10.1 必读

1. 本文档(ENG-REVIEW-BRIEF.md)
2. `docs/prd/OVERVIEW.md` §1.2 / §4.13-§4.14 / §11.K
3. `docs/prd/DECISIONS.md` §K(9 项)+ §L(8 项 Concerns)+ §H5 / §H6 影响传导
4. `~/.gstack/projects/idcd/ceo-plans/2026-05-12-idcd-prd-v2-expansion.md`

### 10.2 高优先级模块

按推荐审查顺序:
1. `18-evidence-and-attestation.md` — 信任根核心,最不可逆
2. `09-billing.md` §13.5 — CRITICAL 流程
3. `14-tech-architecture.md` §2 / §4.9-§4.12 / §11 — 架构变更
4. `15-data-model.md` §4.X(14 张新表)
5. `12-compliance-and-abuse.md` §20 / §21 / §22 应急 SOP
6. `19-ai-agent-observability.md` — MCP server 全栈
7. `10-nodes-and-agents.md` §3.4 / §6.5 / §6.6 — 节点安全
8. `04-monitoring.md` §3.15-§3.18 + §6(LLM 复盘提前)
9. `07-reports-and-dashboards.md` §6(LLM 起草工作流)
10. `06-status-pages.md` §5.7-§5.8 + §8.4

### 10.3 中优先级

11. `11-admin.md` §15.4-§15.6 — 运维 SOP
12. `02-public-tools.md` §4.2 — 3-hero IA
13. `03-account-system.md` §5a / §6.8 — MCP token + Compliance org
14. `08-open-api.md` §12a / §12b — Verdict / MCP API
15. `13-content-and-seo.md` §3.6 — 排行榜内容工作流
16. `17-roadmap.md` — 阶段交付与里程碑

### 10.4 低优先级(参考)

17. `05-alerting.md` / `16-api-spec.md` — v1 未实质改动,v2 增量小

---

## 11. eng review 期望产出格式

建议 reviewer 输出以下结构:

```markdown
# Eng Review Report — idcd PRD v2.0

## Overall Verdict
{CLEAR | CLEAR_WITH_CONCERNS | NEEDS_REWORK | BLOCKED}

## Section 1: Architecture
- [v2 三栈 sub-product] {评估 + 必改项}
- [KMS 信任根] ...
- [MCP 协议层] ...

## Section 2: Error & Rescue
- [Verdict CRITICAL GAP] {状态机实施细节 + 必演示项}
- ...

## Section 3-11: 按 plan-eng-review 11 节走

## CRITICAL GAP 答复(8.1-8.5)
- Verdict WAL 状态机:{答复}
- KMS 应急 SOP 演练:{答复}
- ...

## Reviewer Concerns(对应 §L 8 项)
- Concern 1:{答复 + 拆解为可测项}
- Concern 2-8:...

## 推荐修订(按 Severity)
- CRITICAL:{N 项}
- HIGH:{N 项}
- MEDIUM:{N 项}
- LOW:{N 项}

## 未决问题(eng review 之后还需 CEO 决策的)
- ...
```

---

## 12. 给 reviewer 的"反例提醒"

eng review 容易踩的坑:

- ❌ **不要再创造 scope** — v2 scope 已被 CEO 接受;eng review 不挑 scope,只挑工程可行性
- ❌ **不要纸上谈兵** — 任何"SLA / P95 / 阈值"必须有数据来源(实测 / 历史 / 第三方文档),光写数字 = 谎言
- ❌ **不要"S3+ 评估"逃避** — 涉及 S2 上线的工程问题必须 S2 前答复,不能推到 S3
- ❌ **不要假设"人工介入兜底"** — 1 人创业,人工 SLA 不可信;能自动化的必须自动化
- ❌ **不要忽视应急 SOP 演练** — 演练过的 SOP 才是真的;PRD 写的 SOP 默认是空话直到演练
- ✅ **请 push 工程细节** — KMS 选哪家?TSA 哪家备?LLM 哪家 Provider?具体 latency 数字?
- ✅ **请 push 数据校准** — Anchor 阈值 / 异常突增告警 倍数 / SLA 数字 都需要校准

---

**审查准备完毕**。reviewer 可以从本文档 §10.2 开始按顺序读 PRD,然后回到 §2(8 项 Concerns)+ §6(应急 SOP)+ §8(CRITICAL GAP)逐一答复。

预期 eng review 时长:**4-8 小时**(取决于 reviewer 熟悉度);CC 协助下压缩至 **1-2 小时**。
