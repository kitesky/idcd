# Eng Review TODOS — idcd PRD v2.0

> 生成日期:2026-05-13
> 对应:`docs/prd/ENG-REVIEW-REPORT.md`
> 这是 eng review 后生成的"实施前必做"清单,按 Severity 排序

---

## CRITICAL(S2 上线前 100% 完成)

### TODO-1: Verdict 失败链路 staging 演示

**What**:在 staging 环境注入 KMS / TSA / S3 / refund 各 step 失败,验证完整退款链路。

**Why**:CRITICAL GAP 8.1 要求"必须演示"。用户付 ¥299 拿不到报告 + 拿不到退款 = 品牌死亡。

**Pros**:
- 把 PRD 09 §13.5 WAL 状态机从纸上变成可验证产品
- 找出实际间歇性问题(Paddle refund 风控、KMS 限流、S3 写入延迟)
- 演示给企业 due diligence 客户 = 信任凭证

**Cons**:
- 需 staging 环境完整搭建 + 故障注入工具(toxiproxy / chaos engineering)
- 1-2 周工作量

**Context**:对应 ENG-REVIEW-REPORT D4 + D5。staging 注入 50% KMS / TSA 失败率 + Paddle refund 间歇失败,验证 30min 内用户收到道歉邮箱 + DLQ 监控告警触发。

**Depends on**:D4 (attestation_record 充 WAL 实施) + D5 (refund retry queue 实施)

---

### TODO-2: KMS Shamir 应急 SOP 模拟演练

**What**:S2 上线前 1 次完整应急召回演练,实测每步耗时,验证 SLA。

**Why**:CRITICAL GAP 8.2。"6h SLA"在纸上,实际可能 24h+。

**Pros**:
- SLA 是演练过的数据,enterprise due diligence 能说出实测数据
- 发现实际通讯通道问题(律所 / 工程师 C 周日凌晨能否联系)
- 12h 主路径 + 4h Backup HSM 加速通道验证

**Cons**:
- 需 5 个 quorum 持有人协调一致演练时间
- 1-2 天工作量 + 律所合同签订时间

**Context**:对应 ENG-REVIEW-REPORT D11。Backup HSM 采购(¥1000+ SoftHSM 入极简化)+ 5 人合同签订 + 演练日期。

**Depends on**:Backup HSM 采购完成 + 5 人合同到位

---

### TODO-3: Self-verify worker 独立性实施

**What**:Self-verify worker 不同进程 / 不同 VPC subnet / 独立 KMS 客户端实例 / 仅调 attest.idcd.com/verify 公开接口。

**Why**:CRITICAL GAP 8.1 衍生 — bug 互盖 = 自检失效。

**Pros**:
- 部署架构能 demonstrate "签名与验证独立路径"
- attest.idcd.com/verify 公开 SLA(主 worker 挂了 verify 不挂)
- enterprise due diligence 关键证据

**Cons**:
- 额外部署一个服务 + 独立监控
- S2 初期资源动 0.5 day

**Context**:对应 ENG-REVIEW-REPORT D6。Docker compose 配置文件中明确两个 service 隔离。

**Depends on**:14 §4.9 PRD 修订完成

---

## HIGH(S2 上线前 PRD 修订 + 部分实施)

### TODO-4: Anchor 阈值 30 天 baseline 数据校准报告

**What**:S1 上线后采集 30 天 anchor / 节点偏差数据,按区域 / 时段分析,重新校准 ×2/×3/×5 阈值。

**Why**:HIGH D10。当前阈值是猜的,无数据校准 → 误报 / 漏报。

**Pros**:
- 阈值有数据依据,误报率可控
- 不同区域 / 时段差异化阈值,精度更高
- enterprise due diligence "你们怎么设计阈值" 可答复

**Cons**:
- 创始人 ~2 days 分析报告
- 必须 S1 已上线运行 30 天

**Context**:对应 ENG-REVIEW-REPORT D10。SQL: `SELECT region, ASN, p50, p95, p99, max FROM anchor_deviation WHERE created_at > now() - interval '30 days' GROUP BY region, ASN`。

**Depends on**:S1 上线 + 30 天运行数据

---

### TODO-5: LLM 复盘 eval 数据集 bootstrap(50 条标注 ~25h)

**What**:30 公开事故(AWS / Cloudflare / Azure 历史公告)+ 20 内部 dogfood = 50 条标注。

**Why**:HIGH D8。S2 上线时没 50 个事故 = eval 跑不动 = LLM 复盘 ship 不了。

**Pros**:
- S2 上线前 eval pipeline 可跑
- 创始人亲自标注 = 产品质感保障 + prompt 鲁棒性
- 公开事故是公开信息,无隐私风险

**Cons**:
- 25h 创始人手动工作(无法 CC 替代)
- 公开事故与 idcd 自家场景不同,需后期补真实数据

**Context**:对应 ENG-REVIEW-REPORT D8。维度:时间线准确性 / 根因建议合理性 / 改进措施可行性 / 措辞专业性。

**Depends on**:无,可立即启动

---

### TODO-6: MCP 兼容测试自动化方案研究

**What**:研究 Cursor / Claude Code / Codex 是否有 headless / CI 模式;如都不提供,自家 Python/TS SDK 模拟 MCP 客户端跑 smoke test。

**Why**:HIGH CRITICAL GAP 8.5。每发布前 smoke test 是 K3 决策的硬要求。

**Pros**:
- 发布前自动化兼容测试 = 不靠人手
- 客户端版本支持窗口政策清晰
- spec 演化时回归测试基础

**Cons**:
- 研究 + 验证 1-2 周
- 如各家都不提供 CI 模式,需自家 SDK 作 mock client(额外工作量)

**Context**:对应 ENG-REVIEW-REPORT 8.5。建议先看 Cursor 是否提供 headless / CI 模式(GitHub issues 搜索)。

**Depends on**:无,可立即启动

---

### ~~TODO-7~~: Backup HSM 采购 — **2026-05-13 Pre-4 推迟 S4**

**状态**:**取消 / 推迟到 S4**。Pre-4 决策:S2 不采购 Backup HSM,仅走 12h Shamir 单路径,接受 SLA 偶尔滑至 24h+ 的现实风险。

**S4 启用条件**:
- 企业客户 due diligence 要求"4h 加速"
- 或 月 Verdict 量超 500 份/月

**届时采购**:YubiHSM2(¥3000)+ 离线保险柜 + 1-of-1 重组流程设计

**对 S2 影响**:
- 11 §15.5 / 12 §20.2 已修订为 12h 单路径
- 17-roadmap M4 移除 Backup HSM 采购里程碑
- 17-roadmap S4 新增"Backup HSM 加速通道"里程碑

---

## MEDIUM(S3 GA 前完善)

### TODO-8: PAdES 等级选择研究

**What**:选 B-B(基础)/ B-T(含 TSA)/ B-LT(长期归档)中一档。

**Why**:K-OPEN-5。影响验签独立性。

**Pros**:
- B-LT 适合 6 年合规归档
- B-T 适合大多数场景且 TSA 已含

**Cons**:
- B-LT 文件体积大、复杂
- 选错需 v2 上线后改

**Context**:S2 实施时定。建议 B-T 起步,S3 评估升级 B-LT。

**Depends on**:无

---

### TODO-9: LLM Provider 抽象层 prompt 跨平台测试

**What**:同一个 prompt 在 Claude / GPT / Gemini 上跑 50 条测试事故,验证 schema 一致性 + eval 分数。

**Why**:MEDIUM D9。Per-Provider eval 是真实要求。

**Pros**:
- 验证 Provider 抽象层不是 silver bullet
- 企业用户接入自家 LLM 时有 baseline 数据

**Cons**:
- ~$50-100 LLM API 测试费
- 1 day 测试 + 报告

**Context**:对应 ENG-REVIEW-REPORT D9。

**Depends on**:TODO-5 eval 数据集

---

### TODO-10: GitHub token 扫描 service 选型

**What**:研究 GitGuardian / 自家正则 / TruffleHog 等 service,选一个集成。

**Why**:MEDIUM Concern 6。MCP token 泄露在 GitHub 仓库是真实威胁。

**Pros**:
- 自动检测 + 失活,不需人工监控
- 企业用户信任度提升

**Cons**:
- GitGuardian 商业版可能费用高
- 自家正则误报率高

**Context**:对应 ENG-REVIEW-REPORT Concern 6 答复。

**Depends on**:无

---

## LOW(S4 评估)

### TODO-11: LLM sanitize 字典构建

**What**:构建 LLM 输出后处理的禁用词清单 / 替换规则。

**Why**:LOW Concern 7。"AWS" 是合法描述还是甩锅?需 lint 规则。

**Pros**:
- 防 LLM 输出引战 / 误指责任方
- 法律边界自动校验

**Cons**:
- 字典维护成本
- 误检率会损用户体验

**Context**:对应 ENG-REVIEW-REPORT Concern 7 答复。

**Depends on**:无

---

## 监控 Dashboard 缺口(对应 Report Failure Modes 章节)

需补 `11-admin §15` 新增以下 dashboard:

| Dashboard | 用途 | 优先级 |
|---|---|---|
| Verdict step-level latency | 排查 P95 90s 哪一 step 慢 | HIGH |
| KMS 应急时间线 | 演练 + 实际应急时记录每步耗时 | HIGH |
| 数据污染恢复 | Anchor 偏差告警后恢复进度 | MEDIUM |
| refund_failed 累积 | Paddle refund 自动退款失败趋势 | MEDIUM |
| LLM eval 趋势 | prompt 版本 → eval 分数历史 | MEDIUM |

---

## PRD 修订清单(对应 Report 14 项决定)

按 Report Worktree Lane 顺序:

### Lane A(数据模型)
- [ ] 15 §4.X 序言:重述"三 schema 可独立 cluster"
- [ ] 15 §4.X.1-§4.X.14 所有 DDL:去除跨 schema FK
- [ ] 15 §4.X.1 verdict_order:status enum 添加 `refund_failed`;新增 `refund_attempt_count`
- [ ] 15 §4.X.2 verdict_report:新增 `report_type=observation_only`
- [ ] 15 §4.X.3 attestation_record:新增 `status` + `UNIQUE(report_id, action)` 约束
- [ ] 15 §4.X.7 mcp_tool_call:新增 `idx_mcp_tool_call_session_time`
- [ ] 15 §4.X.9 agent_obs_monitor:明确"原子 UPDATE"

### Lane B(状态机 / 流程 / SLA)
- [ ] 09 §13.5:WAL 流程重画 + refund retry queue + 道歉邮箱
- [ ] 11 §15.4:工单分类 SLA 表(纯自动 / 1h 仅 P0 / 24h 常规)
- [ ] 11 §15.5:KMS 应急 SOP(12h 主 + 4h Backup HSM,演练记录章节)
- [ ] 12 §20 §21:P0 应急流程同步新 SLA

### Lane C(架构 / 子域)
- [ ] 14 §4.9:Attestation Service 描述新增"Self-Verify Worker 独立部署"
- [ ] 14 §4.10:MCP Server 描述新增"业务 stateless + SSE stateful + LB sticky"
- [ ] 14 §4.11:LLM Provider 抽象层"接口统一但 prompt 不保证跨 Provider 一致"
- [ ] 14 §6.2:CK 切换触发指标 + 7 天 PoC + 部署条件
- [ ] 14 §8.1:`mcp.idcd.com/docs` → `docs.mcp.idcd.com`(302 redirect)
- [ ] 14 §9.1:性能基准表新增 "MCP SSE 连接 10k/实例"
- [ ] 14 §H5:影响传导新增"实施层 join 由 Repository 抽象"

### Lane D(MCP)
- [ ] 19 §3.1:架构图标注 LB sticky session
- [ ] 19 §3.4:token 三种形态明确(personal 24h / workspace 90d / service 90d 全自动 renewal)
- [ ] 19 §3.5:表格新增 "MCP units/day" 与 "API calls/day" 分开
- [ ] 19 §3.7:明确部署 `docs.mcp.idcd.com` 子域
- [ ] 19 §6.1:与 §3.4 一致
- [ ] 19 §6.3:新增"失败 case 用户可选 + 项目件授权下 临时存 7 天原 payload"

### Lane E(Evidence)
- [ ] 18 §3.2:数据流图反映 step-level idempotency + attestation_record WAL
- [ ] 18 §3.3:Backup HSM 描述新增"可独立重组"
- [ ] 18 §3.5:Self-verify 独立性边界明确
- [ ] 18 §6:公开 API 返回 report_type 字段
- [ ] 18 §7.1:revoke 期间 "已发报告仍可被验签(old public key)" 明确

### Lane F(LLM 复盘 / Eval)
- [ ] 04 §3.15-3.18:M21-M24 配置规约保持
- [ ] 07 §6.3:Prompt 按 Provider 独立版本 + sanitize 字典
- [ ] 07 §6.4:bootstrap 数据集来源 + S2 上线前完成
- [ ] 07 §6.5:回流数据隐私边界声明

### Lane G(Anchor / 节点)
- [ ] 10 §6.5:阈值标记 placeholder + 30 天 baseline 必须 + "向前回溯审查"机制
- [ ] 12 §11:自动 LLM lint 检查贬损措辞
- [ ] 12 §22:GitHub token 扫描 service 选型

### Lane H(Roadmap / 里程碑)
- [ ] 17-roadmap S1 末:30 天 baseline 报告
- [ ] 17-roadmap S2 初:Anchor 阈值 calibration 报告
- [ ] 17-roadmap S2 上线前:eval 数据集 50 条标注完成
- [ ] 17-roadmap S2 上线前:KMS 应急 SOP 模拟演练
- [ ] 17-roadmap S2 上线前:Verdict 失败链路 staging 演示
- [ ] 17-roadmap S3 末:TimescaleDB 容量评估 + CK PoC

---

## Estimated Effort 汇总

| Category | Human Effort | CC Effort | 阶段 |
|---|---|---|---|
| PRD 修订(8 Lanes)| ~5-7 days | ~6-8 hours | S2 |
| TODO-1 (Verdict staging 演示)| ~1 week | ~2 days | S2 |
| TODO-2 (KMS 演练)| ~2 days | ~30 min | S2 |
| TODO-3 (Self-verify 独立部署)| ~0.5 day | ~2 hours | S2 |
| TODO-4 (Anchor baseline 报告)| ~2 days | ~4 hours | S1末-S2初 |
| TODO-5 (eval 数据集 50 条) | ~25 hours | 无法替代 | S2 |
| TODO-6 (MCP 兼容 CI 研究)| ~1-2 weeks | ~1 day | S3 alpha 前 |
| ~~TODO-7 (Backup HSM 采购)~~ | ~~已推迟 S4~~ | — | S4 |
| **Total to S2 上线**| **~3-5 weeks** | **~3-4 days** | — |

注:S2 上线前必须完成 TODO-1 至 TODO-6。TODO-7 已取消(推迟到 S4)。TODO-8 至 TODO-11 是 S3 GA 前的工作。
