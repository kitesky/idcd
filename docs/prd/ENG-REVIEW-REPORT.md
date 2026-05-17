# Eng Review Report — idcd PRD v2.0

> 生成日期:2026-05-13
> 对应 CEO Plan: `~/.gstack/projects/idcd/ceo-plans/2026-05-12-idcd-prd-v2-expansion.md`
> 对应 Eng Review Brief: `docs/prd/ENG-REVIEW-BRIEF.md`
> Reviewer:Claude(`/plan-eng-review` skill,4 Sections × 14 AskUserQuestion 全交互决策)
> Mode: HOLD SCOPE(v2 scope 已 CEO accept,工程视角校验可行性,不挑 scope)

---

## Overall Verdict

**CLEAR_WITH_CONCERNS** — v2 PRD 整体设计扎实,但有 **3 个 CRITICAL GAP** 必须 S2 上线前 100% 解决,**7 个 HIGH** 必须 S2 上线前规约,**5 个 MEDIUM** 可 S3 GA 前完善。所有 14 项 reviewer 提出的工程化决策点用户均选择最完整路径(选项 A),工程方向清晰。

**关键阻塞项**:
- Verdict CRITICAL GAP 三件套(WAL 状态机 / 聚合支付 refund / Self-verify 独立性)必须实施前演示
- KMS Shamir 应急 SOP 必须 S2 上线前演练 12h 主路径(**Pre-4 调整:Backup HSM 推迟 S4**)
- Anchor 偏差阈值必须 S1 后 30 天 baseline 数据校准

---

## Step 0: Scope Challenge

复杂度评估:v2 引入 4 个新服务(attest.idcd.com / mcp.idcd.com / Public Verify / docs.mcp.idcd.com)+ 14 张新表 + 40+ 新端点 + 3 个外部信任依赖(KMS / RFC3161 TSA × 3 / LLM Provider),**相比 v1 baseline 18 模块,工程量扩张 50-70%**。

**HOLD SCOPE 决策**:brief 明确 scope 已 CEO accept,eng review 不挑 scope。

但有两个**先决条件**必须在 eng review 之前回答(brief §9.1):

| # | 问题 | 状态 |
|---|---|---|
| Pre-1 | 三栈并行(S2 Evidence + S3 MCP + S3 排行榜)1 人 + AI 真能并行还是顺序? | **✅ 已锁定** — 见 `DECISIONS.md §N.1`：S2 Evidence 主推 + S3 MCP 顺序 + CC 并行助手 |
| Pre-2 | KMS sign + TSA + LLM token 月外部依赖成本 $200-1000 acceptable? | **✅ 已锁定** — 见 `DECISIONS.md §N.2`：混合路径(KMS+TSA 商业 / LLM 自家底层),月成本 ¥300-1000 |

---

## Section 1: Architecture(3 个决策,全部锁 A)

### D1 — 跨 schema FK 不一致 [HIGH][9/10]

- **问题**:15-data-model §4.X 序言说"避免 cross-schema FK,应用层 join",但 DDL 中 `verdict_order.owner_id REFERENCES user(id)`、`mcp_session.owner_id REFERENCES user(id)`、`leaderboard_report.verdict_report_id REFERENCES verdict_report(id)` 全是跨 schema FK。
- **决策 D1.A**:删除跨 schema FK,预留 idcd_attest / idcd_main / idcd_mcp 独立 cluster 能力。Repository 模式应用层 join。
- **PRD 必改**:
  - `15 §4.X` 全部 v2 表 DDL 去除跨 schema `REFERENCES user(...)` 与 `REFERENCES verdict_report(...)`
  - `15 §4.X` 序言重述"三 schema 可独立 cluster 部署"
  - `14 §H5` 影响传导表新增"实施层 join 由 Repository 抽象提供"
- **Effort**:human ~2 days / CC ~3 hours

### D2 — MCP 鉴权 + 计费三处矛盾 [HIGH][8/10]

- **问题**:`19 §3.4` 说 token "1h"; `19 §6.1` 说"1h-90d"; `19 §3.4` 又说 service "长期 token"。`19 §3.5` 说"与 09 共用配额池" vs `09 §2.8` 说"Agent Pro 独立计量"。
- **决策 D2.A**:二条原则锁住:(1)所有 token 最长 90d 自动 renewal,不存在永久/长期;(2)MCP 计量与 API 配额独立,Free/Pro/Team/Business 都有 MCP units 额度,Agent Pro 是 MCP 额度加大独立 SKU。
- **PRD 必改**:
  - `19 §3.4`:明确 personal 24h / workspace 90d / service 90d(全自动 renewal + IP 白名单强制);删除"长期 token"措辞
  - `19 §3.5`:表格新增"MCP units/day"列与"API calls/day"分开
  - `09 §2.8`:Agent Pro 明确"MCP units 1M/day 独立池,不占 API 配额"
  - `19 §6.1` / `03 §6.8`:与上述一致
- **Effort**:human ~1 day / CC ~1 hour

### D3 — MCP 文档站 hosting 路径矛盾 [MEDIUM][8/10]

- **问题**:`19 §3.7` 与 `14 §8.1` 都说 `mcp.idcd.com/docs` 是 Nextra SSG,但 mcp.idcd.com 是动态 MCP server,SSG 如何挂在动态服务子路径未明确。
- **决策 D3.A**:独立子域 `docs.mcp.idcd.com`(Cloudflare Pages),`mcp.idcd.com/docs` 走 302 redirect。
- **PRD 必改**:
  - `14 §8.1` 域名表:`mcp.idcd.com/docs` → `docs.mcp.idcd.com`(添加 302 redirect 注释)
  - `19 §3.7`:明确部署 `docs.mcp.idcd.com` 子域
- **Effort**:human ~1 hour / CC ~10 min

---

## Section 2: Code Quality / CRITICAL GAP(4 个决策,全部锁 A)

### D4 — Verdict 状态机不是真 WAL [CRITICAL][9/10]

- **问题**:`09 §13.5` 说"WAL 化状态机",但实际只是 `verdict_order.status` 字段切换。KMS 签名成功 + TSA 失败重试时会重复签名,污染 KMS audit log。**违反信任根可审计性原则**。
- **决策 D4.A**:`attestation_record` 充当 WAL,每 step 完成写一条 `result=success + external_id`;worker 续跑时先查 attestation_record 跳过已成功 step;KMS sign 启用 idempotency token(AWS KMS 支持)。
- **PRD 必改**:
  - `09 §13.5`:流程图重画,标记每 step 写 `attestation_record(result=success)`;worker 续跑从 last record 续跑
  - `18 §3.2`:同步反映 step-level idempotency
  - `15 §4.X.3` `attestation_record`:增加 `status` 字段 + `UNIQUE(report_id, action)` 约束
- **必演示项**:在 staging 注入 KMS / TSA / S3 / Self-verify 各 step 失败,验证 worker crash 后重启续跑无重复 sign
- **Effort**:human ~1.5 days / CC ~2 hours

### D5 — 聚合支付 refund API 失败无兑底 [CRITICAL][8/10]

- **问题**:`09 §13.5` 写"失败 → 自动 聚合支付 refund",但 聚合支付 refund API 本身可能失败(网络 / 风控)。用户拿不到报告 + 拿不到退款 = 品牌灾难。
- **决策 D5.A**:refund 走 retry queue(5min retry → 30min retry);30min 失败后**立即**发送用户道歉邮箱 + P0 告警;PRD 增加 `verdict_order.status = refund_failed` 状态;admin 后台 dashboard 可查所有 refund_failed。
- **PRD 必改**:
  - `09 §13.5`:在"失败路径"块新增 refund retry 子流程 + status=refund_failed
  - `15 §4.X.1` `verdict_order`:status enum 增加 `refund_failed`;新增 `refund_attempt_count int` 字段
  - `11 §15.4`:Verdict 工单分类 SLA 表新增"refund_failed → P0 + 道歉邮箱已发"
- **必演示项**:staging 注入 支付通道 50% refund 失败率,验证 30min 内用户收到道歉邮箱
- **Effort**:human ~0.5 day / CC ~30 min

### D6 — Self-verify 独立性未规约 [CRITICAL][9/10]

- **问题**:`18 §3.5` 说"走和外部用户一样的路径",但没说 self-verify worker 与 sign worker 独立性边界。复用代码 / 配置 / KMS 客户端 → bug 互盖 → 自检失效。
- **决策 D6.A**:明确 self-verify worker 独立进程 / 独立 VPC subnet / 独立 KMS 客户端实例 / 仅调 `attest.idcd.com/verify` 公开接口。
- **PRD 必改**:
  - `18 §3.5`:明确"self-verify worker 不同进程 + 不同 VPC subnet + 不复用 sign worker 的 KMS 客户端实例;走 attest.idcd.com/verify 公开 HTTP 接口,不走内部 RPC 捷径"
  - `14 §4.9`:Attestation Service 组件描述新增"Self-Verify Worker 独立部署,与主 Worker 资源隔离"
- **必演示项**:部署后展示 self-verify worker 与 sign worker 在不同 docker container + 不同 subnet,运行时调用栈记录显示走 verify HTTP 接口
- **Effort**:human ~0.5 day(PRD) + S2 部署额外重量 0.5 day / CC ~1-2 hours

### D7 — 数据模型 3 个 MEDIUM 修正 [MEDIUM][7/10]

- **问题汇总**:
  - 11) `15 §4.X.9` `agent_obs_monitor.total_cost_this_month_usd` 并发 update 无原子保证
  - 12) `15 §4.X.7` `mcp_tool_call` 缺 session_id 索引,按 session 排障会 full scan
  - 15) `19 §6.3` payload 只哈希不存原文,失败 case 无法排障
- **决策 D7.A**:三项一起修。
- **PRD 必改**:
  - `15 §4.X.9`:明确"`total_cost_this_month_usd` 必须用原子 UPDATE `SET total = total + $1` + budget 检查在同一 transaction"
  - `15 §4.X.7`:新增 `CREATE INDEX idx_mcp_tool_call_session_time ON mcp_tool_call(session_id, created_at DESC)`
  - `19 §6.3`:新增"失败 case 用户可选 + 项目件授权下 临时存 7 天原 payload",默认关
- **Effort**:human ~2 hours / CC ~30 min

---

## Section 3: Tests / Evals(2 个决策,全部锁 A)

### D8 — LLM 复盘 eval cold start bootstrap [HIGH][7/10]

- **问题**:`07 §6.4` / `19 §2.4` 说"每月 50 个真实事故 ≥4.0/5 才 ship"。S2 上线(M7-M8)idcd 没 50 个事故。
- **决策 D8.A**:bootstrap = 30 公开事故(AWS / Cloudflare / Azure 历史公告)+ 20 内部 dogfood + S2 上线前创始人手动标注。
- **PRD 必改**:
  - `07 §6.4`:明确 bootstrap 数据集来源 + S2 上线前完成首版数据集
  - `17-roadmap` S2 增加里程碑"S2 上线前 eval 数据集首版完成(创始人手动标注 50 个事故 ~25h)"
- **Effort**:human ~25h(创始人亲自标注)/ CC 无法替代
- **必演示项**:S2 上线前展示 eval pipeline 跑通 + 数据集 50 条已标注

### D9 — LLM Provider prompt 跨平台一致性 [MEDIUM][7/10]

- **问题**:`07 §6.3` / `14 §4.11` 说"LLM Provider 抽象层 + 用户配自家"。但 Claude vs GPT prompt 跨平台不一致,需 per-Provider prompt。
- **决策 D9.A**:per-Provider prompt 版本 + per-Provider eval。baseline 限定 Claude / GPT;其他 Provider 企业用户自行调 prompt + 自行 eval。
- **PRD 必改**:
  - `07 §6.3`:新增"Prompt 按 Provider 独立版本同独立 eval;baseline 仅 Claude + GPT;其他 Provider 企业用户自行 prompt + eval"
  - `14 §4.11`:LLM Provider 抽象层描述同步,明确"接口统一但 prompt 不保证跨 Provider 一致"
- **Effort**:human ~1 day(PRD)+ 2x eval 工作量(后期)/ CC ~30 min PRD

---

## Section 4: Performance / Capacity(5 个决策,全部锁 A)

### D10 — Anchor 偏差阈值未数据校准 [HIGH][9/10]

- **问题**:`10 §6.5` 阈值 ×2/×3/×5 是猜的,无 baseline 数据。
- **决策 D10.A**:S1 上线后 30 天 baseline 收集 + S2 前重调;PRD 标记当前阈值为 placeholder。
- **PRD 必改**:
  - `10 §6.5`:明确"×2/×3/×5 为 S1 placeholder,S2 上线前必须完成 30 天 baseline 数据校准"
  - `17-roadmap`:S1 末新增里程碑"30 天 baseline 报告";S2 初新增里程碑"Anchor 阈值 calibration 报告"
- **Effort**:human ~2 days(分析报告)/ CC ~4 hours

### D11 — KMS Shamir 应急 SOP 演练 [HIGH][9/10]

- **问题**:`18 §3.3` / `11 §15.5` 6h SLA 假设 5 人周日凌晨 1h 内联系上 3 人不现实。
- **决策 D11.A**:S2 上线前 1 次完整演练 + Backup HSM 独立重组通道 + SLA 重调为"12h 主路径 + 4h Backup HSM 加速"。
- **PRD 必改**:
  - `18 §3.3`:Backup HSM 描述新增"可独立重组(冷硬件 1-of-1 临时路径)"
  - `11 §15.5`:SLA 表重写为 12h 主 + 4h Backup,新增"演练 SOP"子章节
  - `12 §20`:应急流程匹配新 SLA
  - `17-roadmap`:S2 上线前里程碑"模拟应急召回演练 完成"
- **Effort**:human ~2 days(演练) + Backup HSM 采购 ~¥1000(SoftHSM 入极简化)/ CC 辅助 ~30 min

### D12 — 1h 工单 SLA 一人创业不可达 [HIGH][8/10]

- **问题**:`09 §13.5` / `11 §15.4` 多处 1h 工单 SLA 一人创业不现实。
- **决策 D12.A**:3 档 SLA:
  - **纯自动**:Verdict 生成失败 → D5 自动 refund + 道歉邮箱(不走工单)
  - **1h 本人响应**:仅限 P0 = KMS 泄露 / 节点失窃 / Backup HSM 失窃(夜间 + 手机告警)
  - **24h 常规**:一般客服问题
- **PRD 必改**:
  - `09 §13.5`:Verdict 失败明确"30min 自动道歉邮箱 + P0 告警,不走工单"
  - `11 §15.4`:新增"工单分类 SLA 表"
  - `12 §20` `12 §21`:P0 仅限 KMS / 节点失窃场景
- **Effort**:human ~3 hours / CC ~30 min

### D13 — MCP SSE 长连接与 stateless 矛盾 [HIGH][7/10]

- **问题**:`14 §4.10` 说 MCP server stateless,但 `19 §5` 用 SSE。
- **决策 D13.A**:明确"业务 stateless + 连接 stateful";LB sticky session 必要;10k SSE/实例 估算(参考 Agent Gateway 同档)。
- **PRD 必改**:
  - `14 §4.10`:MCP Server 描述新增"业务 stateless 可水平扩展;SSE 连接 stateful 需 LB sticky session"
  - `14 §9.1` 性能基准表新增"MCP SSE 连接:10k/实例"
  - `19 §3.1`:架构图标注 LB sticky session
- **Effort**:human ~2 hours / CC ~20 min

### D14 — 30k qps TimescaleDB 单实例边界未 calibration [MEDIUM][7/10]

- **问题**:`14 §6.2` 说"S3 评估切 CK",触发指标不明确。
- **决策 D14.A**:明确 CK 切换触发指标:**单日 monitor_check 新增 > 10GB 或 P99 write latency > 100ms(持续 1 周)** → 启动评估;两项都达到 → 启动部署。
- **PRD 必改**:
  - `14 §6.2`:重写,明确触发指标 + 7 天 PoC + 部署条件
  - `17-roadmap` S3 末:新增"TimescaleDB 容量评估报告 + CK PoC 准备"里程碑
- **Effort**:human ~1 day(PRD)+ S3 末 CK PoC 5 days / CC 辅助 ~30 min

---

## CRITICAL GAP 5 项答复(对应 ENG-REVIEW-BRIEF §8)

### 8.1 Verdict 付费失败 WAL 状态机 — **答复 D4 + D5**

- ✅ WAL 状态机实施 = attestation_record 充 WAL(每 step 写 success + external_id),worker 续跑跳过已成功 step
- ✅ KMS sign 启用 idempotency token(AWS KMS 支持)防重复签名
- ✅ 聚合支付 refund 走 retry queue(5min → 30min)+ 30min 失败后用户道歉邮箱 + P0 告警
- ✅ DLQ 监控:任何 message 出现 5min 内必告警
- ✅ **必演示**:staging 注入 KMS / TSA / S3 / refund 各 step 失败,验证退款链路;30min 内用户收到道歉邮箱

### 8.2 KMS 应急撤销 SOP 演练 — **答复 D11**

- ✅ S2 上线前 1 次完整应急演练 + 记录每步实际耗时
- ✅ Backup HSM 独立重组通道(冷硬件 1-of-1)= 不依赖 5 人 1h 联系上
- ✅ SLA 重调为 12h 主路径 + 4h Backup HSM 加速
- ✅ 5 个 quorum 持有人 + 加密通道(Signal / 物理寄回 / PGP)
- ✅ **必演练**:模拟 sign key 泄露,真实走完 12h SOP,记录每步实际耗时

### 8.3 Anchor 偏差阈值数据校准 — **答复 D10**

- ✅ S1 上线后 30 天 baseline 数据采集
- ✅ 不同区域 / 时段的阈值差异化
- ✅ S2 上线前 calibration 报告
- ✅ 17-roadmap 明确"S2 上线前 Anchor 阈值校准报告"

### 8.4 LLM 复盘 eval 数据集 bootstrap — **答复 D8**

- ✅ 30 公开事故 + 20 内部 dogfood = 50 条标注数据
- ✅ 创始人手动标注 ~25h S2 上线前
- ✅ eval pipeline CI 集成
- ✅ 17-roadmap 明确"S2 上线前 eval 数据集首版完成"

### 8.5 MCP 客户端兼容测试自动化 — **未单独决策**

- ⚠️ **未答复** — 这是工程实施细节,需研究:
  - Cursor 是否有 headless / CI 模式?
  - Claude Code 是否能用于 CI 测试自家 MCP server?
  - Codex CLI 是否提供 batch test 工具?
- **推荐 TODO**:S3 alpha 前研究 Cursor / Claude Code / Codex 的 CI 友好测试方案;如果都不提供,需自家 Python/TS SDK 作 MCP 客户端 mock 来跑兼容性 smoke test。
- **支持窗口政策**:支持过去 6 个月发布的客户端版本

---

## Reviewer Concerns 答复(对应 DECISIONS.md §L 的 8 项)

### Concern 1: Verdict 报告"法定效力"边界 [HIGH]

**Eng 视角已规约**:
- ✅ 18 §1.2 / §7.3 已明确"非鉴定结论 / 一手观测数据"
- ⚠️ **必须补充**:PDF 模板中"非鉴定结论"段落硬编码 + 不可被用户编辑(防 PDF 篡改)
- ⚠️ **必须补充**:`verdict_report.report_type=observation_only` 字段(`15 §4.X.2`)
- ⚠️ **必须补充**:CI lint 检查 PRD / 营销 / 控制台 / 邮件模板中"鉴定 / 认定 / 判定"字样自动拒绝
- **Action**:补 PRD `18 §6` 公开 API 端点返回 report_type 字段 + 12 §3 lint 规则

### Concern 2: Verdict 付费失败 = CRITICAL GAP [CRITICAL]

**Eng 视角已规约**:见 8.1 + D4 + D5 + D6

### Concern 3: CDN 厂商关系长期博弈 [MEDIUM]

**Eng 视角已规约**:
- ✅ /leaderboard/optout 表单已规约
- ⚠️ **必须补充**:`13 §3.6` 增加"已发布报告事后修订机制(不删除原报告,出'勘误公告')"
- ⚠️ **必须补充**:`12 §11` 增加"自动 LLM lint 检查贬损措辞"流程
- **Action**:补 PRD `13 §3.6` + `12 §11`

### Concern 4: 18+19 新模块导致 PRD 膨胀 [LOW]

**Eng 视角**:同意 CEO 决定"S3 中期 PRD 三层重组"
- 当前 onboarding 时长无 baseline 数据,无法量化
- 暂可推迟到 S3

### Concern 5: 签名密钥泄露应急 [CRITICAL]

**Eng 视角已规约**:见 D11
- ⚠️ **必须补充**:revoke 期间 Attestation Service 切只读但**已发报告仍可被验签**(用 old public key)。这条逻辑必须在 `11 §15.5` 明确实施

### Concern 6: MCP token 凭证泄露 [HIGH]

**Eng 视角已规约**:
- ✅ D2 已锁住"所有 token 最长 90d"原则
- ⚠️ **必须补充**:异常突增告警 threshold("24h 调用量 > 历史 P95 × 5")需 S2 数据校准(类似 D10)
- ⚠️ **必须补充**:GitHub 扫描自动失活的具体 service 选型(GitGuardian / 自家正则)
- **Action**:补 PRD `19 §6.1` + `12 §22`

### Concern 7: LLM 复盘幻觉/造谣/泄密 [HIGH]

**Eng 视角已规约**:见 D8 + D9
- ⚠️ **必须补充**:sanitize 规则具体禁止哪些词?LLM 输出"AWS"是合法描述还是甩锅?需 lint 规则集
- ⚠️ **必须补充**:反馈循环回流数据**仅用于内部 eval,不发给 LLM Provider train**(隐私边界)
- **Action**:补 PRD `07 §6.3` sanitize 字典 + `07 §6.5` 隐私边界声明

### Concern 8: Anchor 节点偏差告警未规约 [HIGH]

**Eng 视角已规约**:见 D10
- ⚠️ **必须补充**:"先 8min 正常 + 后 2min 造假"攻击场景应对 — 当前 PRD 只标记最后 2min 数据为 low_confidence,前 8min 未检出。需在 `10 §6.5` 增加"偏差持续时间内向前回溯审查"机制
- ⚠️ **必须补充**:Verdict 报告引用节点数 < 3 时如何处理(拒绝生成 / 降级 confidence label)
- **Action**:补 PRD `10 §6.5` + `18 §3.2`

---

## NOT in scope

以下工程项目本 review 明确**不纳入**当前 PRD v2 修订:

| 项目 | 原因 |
|---|---|
| Conway's Law / 团队拓扑设计 | 1 人 + AI 协同,组织结构不适用 |
| ClickHouse 实际部署 | S3 末评估,D14 已明确触发指标 |
| 区块链锚定具体链选 | K-OPEN-1 deferred to S3 |
| OpenAI Agents Protocol adapter | K-OPEN-3 deferred to S3 末 |
| M24 Agent Output Quality 监控 | K-OPEN-4 deferred to S4 |
| PAdES 等级具体选(B-B / B-T / B-LT) | K-OPEN-5 deferred to S2 实施时 |
| BYOK(企业自带签名密钥) | K-OPEN-6 deferred to S4 |
| 白标 Attestation API / MCP server | S4 评估 |
| Cursor headless CI 集成 | 8.5 未答,建议 TODO |
| HSM 硬件密钥升级 | S4 评估 |

---

## What already exists(v1 已具备的能力,不重复造)

v1 PRD 已具备 v2 所需的底层能力:

| v1 能力 | v2 复用 |
|---|---|
| TimescaleDB Hypertable + 自动连续聚合 | mcp_tool_call / agent_obs_event 直接复用 |
| Redis Streams 事件总线 | verdict_generation_queue 复用 |
| Cloudflare WAF / Turnstile | attest / mcp 子域复用 |
| 聚合支付 支付通道 | Verdict 件价 / Compliance 年订复用 |
| mTLS Agent Gateway + 节点 fingerprint | v2 短期证书 + CRL/OCSP 增量改造 |
| Loki + Prometheus + Grafana 可观测 | v2 新服务直接接入 |
| Anchor 锚定基准 | v2 偏差告警基于现有 anchor 机制扩展 |

---

## TODOS.md 提案

以下项目应加入 TODOS.md(详见 `docs/prd/ENG-REVIEW-TODOS.md`):

1. **[CRITICAL]** Verdict 失败链路 staging 演示(KMS / TSA / S3 / refund 注入)
2. **[CRITICAL]** KMS 应急 SOP 模拟演练(S2 上线前)
3. **[HIGH]** Anchor 阈值 30 天 baseline 数据校准报告(S1 末 → S2 前)
4. **[HIGH]** LLM 复盘 eval 数据集 bootstrap(50 条标注 ~25h)
5. **[HIGH]** MCP 兼容测试自动化方案研究(Cursor / Claude Code / Codex CI)
6. **[HIGH]** Backup HSM 采购 + 独立重组流程设计
7. **[MEDIUM]** PAdES 等级选择研究(B-T 含 TSA vs B-LT 长期归档)
8. **[MEDIUM]** Provider 抽象层 prompt 跨 Claude/GPT 一致性测试
9. **[MEDIUM]** GitHub token 扫描 service 选型(GitGuardian / 自家)
10. **[LOW]** LLM sanitize 字典构建(禁用词清单)

---

## Failure Modes 关键缺口

对照 ENG-REVIEW-BRIEF §1.6 的 6 个新长流程,逐一标注是否有 monitoring + alerting + dashboard:

| 流程 | SLA | Monitoring 缺口 |
|---|---|---|
| Verdict 生成 P95 ≤ 90s | ✅ 自检通过率 100% | ⚠️ 缺"step-level latency dashboard"(P95 / P99 按 step 分解) |
| KMS 应急撤销 12h+4h | ⚠️ 无演练记录 | ⚠️ 缺"应急流程时间线 dashboard"(每步时间打点) |
| 节点失窃应急 1h | ✅ CRL/OCSP + Anchor 偏差 | ⚠️ 缺"数据污染恢复时间 dashboard" |
| Verdict 工单兜底 SLA | 改为 3 档 SLA | ⚠️ 缺"refund_failed 累积 dashboard" |
| Agent OTA 3 级灰度 | 错误率 ×2 自动回滚 | ✅ 已规约 |
| LLM 故障复盘 T+10m 草稿 | ✅ eval ≥4.0 | ⚠️ 缺"prompt 版本 → eval 分数趋势 dashboard" |

**Critical gaps**(都有缺失 monitoring,需补 PRD 11 §15 dashboard 章节):
- Verdict step-level latency dashboard
- KMS 应急时间线 dashboard
- 数据污染恢复 dashboard
- refund_failed 累积 dashboard
- LLM eval 趋势 dashboard

---

## Worktree 并行化策略

PRD 修订的可并行 lanes(对应 14 个 AskUserQuestion 决定):

| Lane | 工作流 | 修订模块 | Depends on |
|---|---|---|---|
| A | 数据模型修正 | 15 §4.X | — |
| B | 状态机 / 流程 / SLA | 09 §13.5, 11 §15.4-15.5, 12 §20-21 | D11 演练前 |
| C | Service 架构 / 子域 | 14 §4.9-4.11, §8.1, §9.1 | — |
| D | MCP 协议 / 鉴权 | 19 §3.4-3.7, §6.1, §6.3 | — |
| E | Evidence 工作流 | 18 §3.2-3.5, §6 | A 完成 |
| F | LLM 复盘 / Eval | 07 §6.3-6.5, 04 §3.15-3.18 | — |
| G | Anchor / 节点 | 10 §6.5 | D10 baseline 数据 |
| H | Roadmap / 里程碑 | 17 (S1, S2, S3) | A-G 都有里程碑插入 |

**执行顺序**:Lane A, C, D, F 可在 4 个 worktree 并行(无冲突);Lane B 在 A 后启动(需 verdict_order 状态枚举更新);Lane E 在 A 后启动(需 attestation_record 表更新);Lane G 在 D10 baseline 数据后启动;Lane H 最后 merge(汇总所有 milestone)。

**冲突风险**:Lane A 与 Lane E 都触动 `15 §4.X.1-§4.X.3`,需要顺序执行。

---

## Completion Summary

- **Step 0: Scope Challenge** — HOLD SCOPE,2 个 Pre-conditions(人力 / 成本)未答 ⚠️
- **Architecture Review** — 3 issues found(D1-D3),全部锁 A
- **Code Quality Review** — 4 issues found(D4-D7),包含 3 CRITICAL,全部锁 A
- **Test Review** — 2 issues found(D8-D9),全部锁 A;eval dataset bootstrap 必需
- **Performance Review** — 5 issues found(D10-D14),全部锁 A
- **NOT in scope**: 写入(10 项 deferred)
- **What already exists**: 写入(v1 7 项底层能力)
- **TODOS.md 提案**: 10 项(2 CRITICAL + 4 HIGH + 3 MEDIUM + 1 LOW)
- **Failure modes**: 5 个 critical monitoring gaps 标记
- **Outside voice**: 跳过(用户已选 B 严格走 plan-eng-review)
- **Parallelization**: 8 lanes,4 并行 + 4 顺序

### Lake Score

14/14 选项全部选完整路径(A) = **Lake Score 14/14 = 100%**

### Verdict 输出

**CLEAR_WITH_CONCERNS**:

1. 3 CRITICAL GAP(D4 / D5 / D6)必须 S2 上线前演示
2. 7 HIGH(D1 / D2 / D8 / D10 / D11 / D12 / D13)必须 S2 上线前 PRD 修订完成
3. 5 MEDIUM(D3 / D7 / D9 / D14 + Concern 1 PDF 模板)S3 GA 前完善
4. 2 Pre-conditions(人力 / 成本)需 CEO / 创始人答复

---

## 待 CEO / 创始人决策(非 eng review 范畴)

- [x] Pre-1: 三栈并行 1 人 + AI 真能并行? **→ ✅ 已锁定: S2 Evidence 主推 + S3 MCP 顺序 + CC 并行助手** (`DECISIONS.md §N.1`)
- [x] Pre-2: KMS + TSA + LLM 月外部依赖成本 $200-1000 acceptable? **→ ✅ 已锁定: 混合路径,月成本 ¥300-1000** (`DECISIONS.md §N.2`)
- [x] D8 创始人手动标注 25h 是否优先? **→ ✅ 已锁定: S2 前完成** (`DECISIONS.md §N.3`)
- [x] D11 Backup HSM 采购 ¥1000+ 是否优先? **→ ✅ 已锁定: S4 才补,接受 12h 单路径风险** (`DECISIONS.md §N.4`)
- [x] D12 个人 7×24 P0 响应是否接受? **→ ✅ 已锁定: 创始人担 + Emergency Contact List** (`DECISIONS.md §N.5`)

---

## 持续维护

- 本 Report 是 v2 PRD 实施的 reference,实施过程中如发现新 issue 需追加 D15+
- 14 项决定均锁 A,实施时严格按"PRD 必改"清单执行,不允许 silently 跳过
- S2 上线前必须完成所有 8 CRITICAL/HIGH 标记的"必演示项"

---

**审查完成**。reviewer 建议:开始按 worktree 并行化策略修订 PRD;同时启动 S1 数据采集(为 D10 / Concern 6 服务);S2 上线前安排 2 个演示(D4 + D11)。
