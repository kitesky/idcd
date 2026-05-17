# 关键决策集中清单（v2.0 锁定:v1.0 40 项 + v2.0 EXPANSION 9 项 §K）

> 日期:v1.0 项目启动决策对齐完成;v2.0 2026-05-12 plan-ceo-review EXPANSION 模式新增
> 维护原则：本文是单一真实来源（Single Source of Truth）。任何决策变更必须更新本文 + 对应模块 PRD。
> 状态符：✅ 已锁定 / ⏳ 阶段性 / 🔄 可微调
> v2.0 变更:见 §K(9 项 EXPANSION 决策)、§H5(K1 三栈 sub-product 阵型的影响传导)、§H6(K5 计费档新增的影响传导)

---

## A. 产品定型决策

| # | 决策 | 锁定结论 | 影响模块 |
|---|---|---|---|
| A0 | **品牌名** | ✅ **idcd**(2026-05-13 锁定);4 字母无语义组合,类似 vercel/stripe;域名 idcd.com 不变;详 01-branding.md | 全部 PRD(67 `<Brand>` + 多处 `<brand>` 占位已批量替换) |
| A1 | 免费档配额 | ✅ 5 监控 / 5min / 仅邮件 / 100 API 调用每天 | 09 §2.1、04 §4 |
| A2 | 账号注销冷静期 | ✅ 30 天 | 03 §9.2、12 §14.3 |
| A3 | 2FA 启用门槛 | ✅ Free 也允许 | 03 §3、09 §2.1 |
| A4 | Username 是否必填 | ✅ 不必填，可后补；默认以邮箱前缀作 display name | 03 §4 |
| A5 | Free 档状态页 watermark | ✅ 页脚文字 "Powered by idcd"（带链接） | 06 §3.1 |
| A6 | 一键诊断报告默认过期 | ✅ 30 天（未登录用户）；登录用户可永久 | 02 §3.8 |
| A7 | 公开节点目录暴露 IP | ✅ 完全公开出口 IP | 10 §7.3 |
| A8 | 维护窗口默认计入可用率 | ✅ 不计入分母 | 06 §13.2 |

## B. 启动技术栈决策

| # | 决策 | 锁定结论 | 影响模块 |
|---|---|---|---|
| B0 | **版本锁定** | ✅ **Go 1.26 / Next.js 16 / PostgreSQL 18.3 / TimescaleDB 2.21+ / Redis 7.4+ / Tailwind v4 / shadcn latest**(2026-05-13 锁定 latest,避免反工) | 14 §3.1/3.3, ARCHITECTURE §6.1 |
| B0a | **LLM Provider 主选** | ✅ **国内底层(阿里通义 / DeepSeek)主选,Claude/GPT 备选**(2026-05-13 Pre-2 C 路径锁定);D9 per-Provider eval 数据集必含通义/DeepSeek;企业版可配自家 | 07 §6.3, 14 §4.11, 19 §2.4 |
| B0b | **KMS + TSA** | ✅ **商业服务**(阿里云 KMS 或 AWS KMS + DigiCert TSA + GlobalSign TSA),不省;月成本 ~$50-100 | 18 §3.3, 14 §4.11 |
| B1 | 国内主控云厂 | ✅ 阿里云 | 14 §3.4、§7 |
| B2 | 主站前端部署 | ✅ 自建（Docker + Cloudflare） | 14 §4.8、§19 |
| B3 | 文档站方案 | ✅ 自建 Nextra / Fumadocs | 08 §14.1、14 §19 |
| B4 | DDoS 防护起步 | ✅ Cloudflare Pro / Business（不上阿里云高防）| 14 §13.4、12 §13.1 |

## C. 商业化策略决策

| # | 决策 | 锁定结论 | 影响模块 |
|---|---|---|---|
| C1 | 早鸟终身计划 | ✅ 不做 | 09 §2.5 |
| C2 | 推荐返利比例 | ✅ 30%（激进，仅首次付费触发） | 09 §6.2 |
| C3 | Pro 14 天免费试用 | ✅ S3 中期推出 | 09 §15 / §17 |
| C4 | 首次订阅 7 天无理由退款 | ✅ 提供 | 09 §5.1 |
| C5 | "按量纯付费"档（不订阅） | ✅ S3 推出（¥0.5/1k 调用） | 09 §17、08 §22 |
| C6 | 学生 / NGO 折扣 | ✅ 提供 5 折（.edu 邮箱 / NGO 资质审核） | 09 §2.5 |
| C7 | 短信 / 语音计费 | ✅ 订阅档赠送配额 + 超额按量 | 09 §2.3 |
| C8 | **经营性 ICP 许可证** | ⚠️ **暂不办理**。备选:①聚合支付服务商代办商户;②找有许可证合作方代收;③S3+ 视量自建商户号。**影响自建微信/支付宝商户号开通** | 09 §3、12 §2、17 §8.1 |

## D. API / 开发者决策

| # | 决策 | 锁定结论 | 影响模块 |
|---|---|---|---|
| D1 | 匿名 API 是否开放（不带 Key）| ✅ **不开放匿名 API**，第三方必须登录拿 Key。**例外**：自家公开工具页前端可匿名调用（带 Turnstile） | 08 §2.2、12 §3 |
| D2 | CLI 形态 | ✅ 仅命令行输入输出（仿 gh / gp）；不做 TUI | 08 §16 |
| D3 | 沙箱（test Key）日志 | ✅ 计入审计日志（不计费），便于排障 | 08 §9 |
| D4 | Webhook 重试上限 | ✅ 6 次（5s / 30s / 2min / 10min / 1h / 6h） | 08 §17 |
| D5 | 主键设计 | ✅ text（prefix + nanoid，如 `u_xxx` `m_xxx`） | 15 §2 |

## E. 节点 / 众包决策

| # | 决策 | 锁定结论 | 影响模块 |
|---|---|---|---|
| E1 | T1 众包节点参与 Pro 监控 | ✅ 默认参与（用户可关）；T2/T3 仅 Free | 10 §10.8 |
| E2 | Honey-task 占比 | ✅ 3-5% | 10 §10.10 |
| E3 | 众包 Ban 转人工阈值 | ✅ 1 小时同 ASN ≥ 5 个 | 10 §10.11 |
| E4 | 众包申请门槛 | ✅ 账号 ≥ 7 天 + 邮箱验证 + 基础认证（PRD 默认） | 10 §10.2 |
| E5 | Agent 开源时机 | ✅ S3 与众包同步开源（MIT） | 10 §17 |
| E6 | 节点首批厂商策略 | ✅ 分散 8+ 家厂商（避免同 ASN 集中） | 10 §9.4、§17 |
| E7 | Anchor 锚定基准目标 | ✅ 第三方（baidu / google / cloudflare） | 10 §10、§6.5 |

## F. 告警 / 通知决策

| # | 决策 | 锁定结论 | 影响模块 |
|---|---|---|---|
| F1 | 浏览器级监控（M11）档位 | ✅ Team 起 | 04 §4 |
| F2 | 个人微信 Bot 接入 | ✅ 自家服务号模板消息 + Server酱 / WxPusher 第三方 fallback | 05 §2.2 C03 |
| F3 | 心跳监控 token URL 鉴权 | ✅ Token 本身即鉴权；可选 HMAC payload 签名 | 04 §3.10、08 §11 |

## G. 合规 / 安全决策

| # | 决策 | 锁定结论 | 影响模块 |
|---|---|---|---|
| G1 | 等保 2.0 二级评测 | 🔄 视业务发展定（默认 S3 中期评估，非强制） | 12 §16 |
| G2 | Bug Bounty 计划 | ✅ S3 中期启动 | 12 §7.4、§16 |
| G3 | 客服扮演（Impersonate） | ✅ 默认允许，用户可在设置中关闭 | 11 §4.4 |
| G4 | VPN（内网接入） | ✅ 自建 WireGuard | 11 §13.1、14 |
| G5 | 状态页默认子域 | ✅ `<slug>.status.idcd.com` | 06 §3、OVERVIEW |

---

## H. 重大决策的影响传导

### H1. "经营性 ICP 暂不办理"（C8）的连锁影响

> 这是最重要的策略决策。后续所有 PRD 涉及国内付费收款的章节都需调整。

**影响**：
1. **09 §3 支付通道**：原计划微信 / 支付宝商户号 S2 上线，现需备选方案
2. **09 §13 关键流程**：国内用户付费方式变化
3. **17 §6 里程碑**：商业化路径调整
4. **12 §2 国内合规**：删除"经营性 ICP S2 前必须取得"

**三个可执行的备选路径**：

| 路径 | 描述 | 时间表 |
|---|---|---|
| **路径 A:聚合支付服务商**(已选) | 通过聚合服务商对接微信支付 + 支付宝,商户资质由聚合方代办,~1% 手续费,人民币 T+1 清算 | 立即可行,S2 上线,`packages/payment-go-sdk` 已集成 |
| **路径 B:找合作方代收** | 与已有"经营性 ICP"的公司签代收协议;他们代为开通微信/支付宝商户号;按比例分润(典型 3-5%) | S3+ 视量再评估 |
| **路径 C:自建商户号** | 月流水 > ¥500k 后自办经营性 ICP + 直连微信/支付宝商户号,把通道费率从 ~1% 压到 0.6%/0.55% | S4+ |

**已锁定**:S1-S2 走路径 A(聚合支付主通道),不上海外路线,无需海外公司主体。

### H2. "匿名 API 不开放"（D1）的影响

**影响**：
1. **08 §2.2** 表格中"匿名公开 API"行需修改
2. **02 公开工具** 仍可匿名使用（页面前端调用），但 SEO 抓取 API 接口需要登录
3. **限速策略简化**：不再需要"匿名 IP 维度 30/h"档；最低档变为"登录 Free"
4. **公开节点目录 `/nodes` API** 可保持开放（这是营销页面，非业务 API）

**修改要点**：所有 `/v1/probe/*` `/v1/diagnose` `/v1/monitors/*` 等都需 Key；只有 `/v1/status/<slug>/*`（状态页公开数据）和 `/v1/nodes` 仍开放。

### H3. "前端自建 + 阿里云国内主控"（B1/B2）的影响

**架构调整**：
- 14 §7 部署架构图中"国内主控"明确为阿里云 ECS
- DNS 智能解析：国内走阿里云 / 海外走 Cloudflare
- CDN 双套：国内站走阿里云 CDN + Cloudflare（备用）

### H4. "状态页子域 + 节点厂商分散"叠加效应

- 用户状态页子域 `*.status.idcd.com` 需要泛域名 SSL（Cloudflare 自动管理）
- 每个状态页是独立子域，但共享后端
- 8+ 节点厂商意味着 8+ 不同 ASN 入口；管理后台节点视图需按厂商分组展示

### H5. (v2 NEW) "K1 三栈 sub-product 阵型"的影响传导

> idcd 不再是单一 monolith,而是 Core / Evidence / MCP 三个独立子域 + 独立计量 + 独立 SLA 的 sub-product 阵型。这是 v2 最大架构决策。

**影响**:
1. **14-tech-architecture.md**:架构图需重画,新增 attest.idcd.com + mcp.idcd.com 两个独立部署 + 各自独立的 service / 计量 / 鉴权
2. **15-data-model.md**:新增 verdict_orders / verdict_reports / attestation_records / tsa_responses / key_ceremony_log / mcp_session / mcp_tool_call / mcp_token / agent_observability_monitor 共 9 张表
3. **16-api-spec.md**:新增 `/v1/verdict/*` `/v1/attest/*` `/v1/verify/*` `/v1/mcp/*` `/v1/agent-obs/*` 共 5 个端点组
4. **09-billing.md**:新增独立 SKU(Verdict 件价 + Compliance 年订 + Agent Pro);独立计量但共用账户余额
5. **11-admin.md**:新增"Verdict 工单"+"KMS 仪式后台"+"MCP token 撤销"运维入口
6. **12-compliance-and-abuse.md**:Verdict 滥用举报 + 排行榜厂商退出申诉 + MCP token 滥用应急流
7. **OVERVIEW.md §5.2**:子域列表新增 attest / mcp 两条
8. **DNS 策略**:attest.idcd.com 与 mcp.idcd.com 走 Cloudflare 同泛域名 cert;但部署、监控、告警独立,以便单独宕机/降级

**为什么独立(不是 monolith 子路径)?**
- 计量独立:Verdict 件价 vs MCP 按 unit 计费,与订阅档计量完全不同
- SLA 独立:Verdict 99.95% / MCP 99.9% / Core 99.5%,SLA 对外宣称不同
- 白标可能:S4 企业版可独立白标 Attestation 或 MCP server 卖给企业
- 协议演化双轨:MCP 协议演化时(Anthropic vs OpenAI 分叉)双轨维护成本低

### H6. (v2 NEW) "K5 Verdict + Compliance 计费档"的影响传导

**影响**:
1. **09-billing.md §2**:新增 §2.6 Verdict 件价档 + §2.7 Compliance 年订档 + §2.8 Agent Pro 档
2. **09-billing.md §3**:聚合支付通道支持件价(non-subscription)+ 年订(annual subscription)
3. **09-billing.md §13**:新增"Verdict 件价下单 → 报告生成 → 自检失败自动退款"流程
4. **09-billing.md §16**:新增风险 — Verdict 付费后生成失败的"WAL 状态机 + 工单兜底"流程
5. **OVERVIEW.md §6.1**:收入结构重写,Verdict + Compliance 占 30%
6. **18-evidence-and-attestation.md §2**:产品形态 + 价格
7. **17-roadmap.md S2-S3**:Verdict MVP M7 上线;Compliance 年订 M7 同期;Agent Pro M11-M12

---

## I. 剩余待定（无紧迫性，留待实际推进时再定）

下列决策有低紧迫性，可在对应阶段开始前再敲定：

- [ ] 出海英文版子路径 `/en/` 还是子域 `en.idcd.com`（S3 出海前）
- [ ] 是否做 GraphQL endpoint（S4 评估）
- [ ] 数据库 BYOK（客户密钥）（S4 企业版评估）
- [ ] 视频内容（B 站 / YouTube）启动时机（S3-S4 评估）
- [ ] 工单系统自建 vs Zendesk（M5-M6 实施时定）
- [ ] OAuth 2.0 用于第三方应用（S4 评估）
- [ ] 多步骤事务监控（M12）（S4）

---

## J. 下一步行动

1. ✅ 本文件（DECISIONS.md）作为决策汇总
2. ⏳ 批量更新各模块 PRD：
   - 把"开放决策点"章节中已决策的项目标 ✅ 并写明结论
   - 涉及正文行为变化的章节按 §H 修改
3. ✅ 更新 OVERVIEW.md §11 决策表，引用本文件(v2 已完成)
4. ⏳ 关键变化的快照：经营性 ICP / 匿名 API / 阿里云
5. ⏳ 进入 M1 启动准备

---

## K. v2.0 EXPANSION 决策(2026-05-12 plan-ceo-review 锁定)

> 背景:2026-05-12 用户走 /plan-ceo-review EXPANSION 模式,全部接受 6 项 cherry-pick + 3 项关键架构决策(D4/D5/D6)。本节是 v2 决策的单一真实来源。详细 vision 与影响分析见 `~/.gstack/projects/idcd/ceo-plans/2026-05-12-idcd-prd-v2-expansion.md`。

### K.1 三栈 sub-product 阵型

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K1 | 三栈独立子域 | ✅ Core(`api.idcd.com`)/ Evidence(`attest.idcd.com`)/ MCP(`mcp.idcd.com`)独立子域 + 独立计量 + 独立 SLA + 独立 PRD 模块 | 14, 15, 16, 18, 19, OVERVIEW §5.2 |

### K.2 Evidence-as-a-Service 信任根

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K2 | 签名密钥架构 | ✅ 云 KMS(AWS KMS 或阿里云 KMS) + Shamir 3-of-5 离线 root + 90 天 sign key 轮换;HSM S4 评估 | 18 §3.3, 11(KMS 仪式后台), 12(应急撤销 SOP) |

### K.3 MCP Server 鉴权

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K3 | MCP 鉴权模型 | ✅ 短期 token(1h-90d)+ 三种形态(personal/workspace/service)+ service 强制 IP 白名单 | 19 §3.4, 03(MCP token 管理), 11(撤销后台) |

### K.4 AI 故障复盘提前

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K4 | LLM 故障复盘自动起草 | ✅ 从 P3/S4 提至 P1/S2;强制人工审核 + AI 标识 + sanitize + 离线 eval ≥ 4.0/5 才允许新 prompt 上线 | 04 §4, 07, 06(状态页 incident 工作流), 19 §2.4 |

### K.5 Verdict + Compliance 计费

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K5 | Verdict 件价档 | ✅ 4 档:SLA ¥299 / 故障取证 ¥199 / 合规自证 ¥499 / 争议取证 ¥999 | 09 §2.6, 18 §2.1 |
| K5b | Compliance 年订档 | ✅ 3 档:Starter ¥3k / Pro ¥12k / Enterprise ¥30k(议价) | 09 §2.7, 18 §2.2 |

### K.6 CDN/云厂商排行榜边界

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K6 | 排行榜测试授权 | ✅ 仅测厂商公开发布的"公共边缘 IP / anycast"(参考 Cloudflare Radar / Catchpoint 业界标准);每厂商在 `/leaderboard/optout` 申请退出;每报告含"非鉴定结论 + 仅观测数据"免责声明 | 13 §X(新增内容矩阵), 12 §11(新增权威测评白名单), 11(厂商退出申诉) |

### K.7 司法鉴定所合作

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K7 | 司法鉴定所合作通道 | ⏳ S3 中后期评估;v2 报告**全文不用"鉴定"字样**,定位"一手观测数据 + 第三方背书";高争议场景 Verdict 报告作为输入数据交合作鉴定所兜底 | 18 §1.2, 18 §7.3, 12(法律边界声明) |

### K.8 区块链锚定

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K8 | 区块链锚定 | ⏳ S3 评估;v2 起步 RFC3161 时间戳即满足公证需求,链锚作为可选 add-on(企业法务市场可能要求);若上链优先选 Polygon 或 Arweave(成本低) | 18 §3.2(可选 step 9), 18 §2.3, 18 §10 K-OPEN-1 |

### K.9 PRD 模块扩展

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| K9 | 新增 PRD 模块 | ✅ 18-evidence-and-attestation.md(完整 10 章)+ 19-ai-agent-observability.md(完整 9 章);PRD 模块数 18→20;新员工 onboarding 先读 OVERVIEW + 自己模块 PRD,Evidence/Agent 团队额外读 18/19 | OVERVIEW §10, 17 §11(阶段交付清单新增 E 组), CLAUDE.md (lint 加 PRD 引用一致性) |

### K-OPEN(v2 待定,留待实际推进时定)

- [ ] **K-OPEN-1**:区块链锚定具体链选(Ethereum vs Polygon vs Arweave)— S3 评估时定
- [ ] **K-OPEN-2**:Verdict 报告"过期可读"UI 措辞 — S2 末敲定;已有报告永久可验签(只读)
- [ ] **K-OPEN-3**:OpenAI Agents Protocol 是否接入 — S3 末根据市场份额评估
- [ ] **K-OPEN-4**:Agent Output Quality 监控(M24)评估器选型 — S4 评估
- [ ] **K-OPEN-5**:PAdES 签名等级(B-B / B-T / B-LT)— S2 实施时定;M6 默认 B-T(已含 TSA),S3 评估升 B-LT
- [ ] **K-OPEN-6**:是否允许企业版 BYOK(自带签名密钥)— S4 评估
- [ ] **K-OPEN-7**:自家 MCP SDK 是否开源(MIT) — S3 末决,默认开
- [ ] **K-OPEN-8**:MCP 调用结果是否可生成 Verdict 报告(把 Agent 验证证据化)— S4 评估
- [ ] **K-OPEN-9**(D2 衍生):auto_renewal 失败后用户被动 logout 的 UX 边界 — S3 alpha 后视真实场景调
- [ ] **K-OPEN-10**(D-Concern6 衍生):GitHub token 扫描 service 选型(GitGuardian / 自家正则 / TruffleHog)— S3 alpha 前定

---

## M. Eng Review 后续决策(2026-05-13 plan-eng-review 14 项 D 锁定)

> 背景:2026-05-13 用户走 /plan-eng-review,所有 14 项决策点全部锁定 A(最完整路径)。本节是 v2 eng review 决策的单一真实来源。详细评估见 `docs/prd/ENG-REVIEW-REPORT.md`,实施清单见 `docs/prd/ENG-REVIEW-TODOS.md`。

### M.1 架构

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| D1 | 跨 schema FK | ✅ DDL 中不写跨 schema FK,通过 Repository 抽象层应用层 join;预留 attest / mcp / main 独立 cluster 部署能力 | 15 §4.X(全部 v2 表 DDL)+ 14 §H5 |
| D2 | MCP token + 计量 | ✅ 二条原则:(1)所有 token 必有过期日,最长 90 天 auto_renewal,无永久;(2)MCP units 与 API 配额完全独立池 | 03 §5a, 09 §2.8, 19 §3.4/3.5/6.1, 15 §4.X.8, 12 §22.3 |
| D3 | MCP 文档站 | ✅ 独立子域 docs.mcp.idcd.com(Cloudflare Pages);mcp.idcd.com/docs 走 302 redirect | 14 §8.1, 19 §3.7 |

### M.2 代码质量 / CRITICAL GAP

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| D4 | Verdict WAL 状态机 | ✅ attestation_record 充 WAL(每 step 写 success+external_id+idempotency_key);UNIQUE(report_id, action)防重复;KMS sign 启用 idempotency token | 09 §13.5, 18 §3.2, 14 §4.11, 15 §4.X.3 |
| D5 | 聚合支付 refund 兑底 | ✅ refund retry queue(5min/30min)+ 30min 强制道歉邮箱 + refund_failed 状态 + P0 告警 | 09 §13.5, 09 §16, 11 §15.4, 15 §4.X.1 |
| D6 | Self-verify 独立 | ✅ 独立进程 / 独立 VPC subnet / 独立 KMS 客户端实例 / 仅调 verify HTTPS 公开接口 | 18 §3.5, 14 §4.9 |
| D7 | 数据模型 3 修正 | ✅ 原子 UPDATE budget / +session_id 索引 / 失败 case 临时存 7 天原 payload(用户授权) | 15 §4.X.7/4.X.9, 19 §6.3 |

### M.3 测试 / Eval

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| D8 | LLM eval cold start | ✅ 30 公开事故(AWS/CF/Azure)+ 20 内部 dogfood,S2 上线前创始人手动标注 ~25h | 07 §6.4, 17-roadmap M5 |
| D9 | Provider prompt 一致性 | ✅ per-Provider 独立 prompt + 独立 eval;baseline 仅 Claude+GPT;其他 Provider 企业用户自行调 + eval | 07 §6.3, 14 §4.11 |

### M.4 性能 / 容量

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| D10 | Anchor 阈值 calibration | ✅ ×2/×3/×5 为 S1 placeholder;S1 末 30 天 baseline + S2 前 calibration 报告;向前回溯审查防"渐进式造假" | 10 §6.5, 17-roadmap M4/M5 |
| D11 | KMS 应急 SOP | ✅ **12h Shamir 3-of-5 单路径**(2026-05-13 Pre-4 调整);Backup HSM 加速通道**推迟到 S4 企业越劤时补**;S2 上线前必演练 12h 路径;接受周日凌晨 SLA 可能滑至 24h+ 的现实风险 | 18 §3.3, 11 §15.5, 12 §20, 17-roadmap M6 / S4 |
| D12 | 1h SLA 现实化 | ✅ 三档 SLA:纯自动(Verdict 失败)/ 1h P0(KMS/节点失窃)/ 24h 常规客服 | 09 §13.5, 11 §15.4, 12 §20/21 |
| D13 | MCP SSE 状态边界 | ✅ 业务 stateless + SSE 连接 stateful;LB sticky session 必要;10k SSE/实例 | 14 §4.10/9.1, 19 §3.1 |
| D14 | TimescaleDB → CK 触发 | ✅ 单日 monitor_check > 10GB 或 P99 write > 100ms(持续 1 周)→ 启动评估;两项都到 → 部署 | 14 §6.2, 17-roadmap M14 |

### M.5 第二次架构审查锁定(2026-05-13 plan-eng-review 二轮，5 项 D 新增)

> 背景:2026-05-13 同日二次 /plan-eng-review，聚焦第一轮未覆盖的 5 个架构盲区。全部锁定 A。

| # | 决策 | 锁定结论 | 影响模块 |
|---|------|---------|---------|
| D17 | Agent 24h 缓冲存储 | ✅ SQLite 本地持久化(modernc.org/sqlite cgo-free);进程重启后缓冲不丢失;replay 时 Aggregator ingest 侧按 `(node_id, task_id, timestamp)` 去重 | apps/agent/ |
| D18 | Redis Streams MAXLEN | ✅ `probe.results` / `monitor.events` / `alert.events` 等全部 Streams 设 `XADD ... MAXLEN ~ 500000`;超出丢弃最旧数据,保护 Redis 内存 | 14 §5.3, 10 事件总线 |
| D19 | S1 ECS 规格 | ✅ 主控 ECS 升至 **8C/16G**（原 4C/8G 全栈内存 4.5-6GB+overhead 过紧）;法兰克福热备维持 2C/4G(轻量备份用途) | 14 §7, ARCHITECTURE §4.1 |
| D20 | attest-worker retry 退步 | ✅ 每步重试间隔 **1s → 4s → 16s 指数退步 + ±25% jitter**;AWS KMS 默认 5 TPS/key,立即三连重试必触限速;Go `time.Sleep` 即可 | apps/attest-worker/ |
| D21 | MCP token 续期幂等 | ✅ `INSERT INTO mcp_token ... ON CONFLICT(token_hash) DO UPDATE SET renewed_at=NOW()` + Redis `SETNX` 30s 分布式锁防并发重复续期 | packages/auth/mcp_token.go |

**跨模型审查新增 Gap(无需决策,直接写入文档)**:
- D6 Self-verify 路径确认:保持公开接口路径(D6 原设计胜出)
- OTA 3 级灰度缺 kill switch SOP → 补 `docs/RUNBOOKS/agent-mass-rollback.md`
- LLM failover 触发条件 → 见 14 §4.11 补充

### M.6 Reviewer Concerns 答复

详 ENG-REVIEW-REPORT.md "Reviewer Concerns 答复" 章节,8 项 Concerns 全部规约:
- Concern 1(Verdict 法定效力):verdict_report.report_type=observation_only + verify 接口返回 + 禁用词 CI lint
- Concern 2(Verdict 付费失败):见 D4+D5+D6
- Concern 3(CDN 厂商关系):errata_pdf_url 勘误公告 + 12 §19.5 自动 LLM lint
- Concern 4(PRD 膨胀):S3 中期三层重组(暂推迟)
- Concern 5(签名密钥泄露应急):见 D11 + 18 §7.1 revoke 期间历史验签
- Concern 6(MCP token 泄露):见 D2 + 12 §22.3 GitHub 扫描
- Concern 7(LLM 幻觉/造谣):见 D8/D9 + 07 §6.3 sanitize 字典 + §6.5 隐私边界
- Concern 8(Anchor 偏差):见 D10 向前回溯审查 + 节点 <3 拒生成

---

---

## L. v2.0 进入工程实施前的关键 Concerns(来自 CEO Plan)

> 这些是 plan-ceo-review 审查中识别的 Reviewer Concerns,实施时必须重点关注:

1. **Verdict 报告"法定效力"边界** — 国内司法实践中仅签名 + 时间戳不够,需要司法鉴定所背书。**Verdict 报告所有文案必须明确"非鉴定结论",定位"一手观测数据"**,以避免误导用户 + 触法律红线。详 18 §1.2 / §7.3。

2. **Verdict 付费后生成失败 = CRITICAL GAP** — 用户付了 ¥299 拿不到报告 = 品牌直接死亡。必须 WAL 化状态机 + 自检失败自动退款 + 客服工单兜底。详 18 §3.2 / 09 §13(新增)。

3. **CDN 厂商关系长期博弈** — /leaderboard 持续打分会引起厂商不满。需要法务备案 + 律所储备 + 退出通道明确。详 12 §11(新增) / 13 §X(新增)。

4. **18+19 新模块导致 PRD 膨胀风险** — 模块数 18→20,新员工易迷路。**建议三层重组**:Core 子目录(02-07)/ Extension 子目录(18-19)/ Platform 子目录(03/08-16)。S3 中期 PRD 重组时执行。

5. **签名密钥泄露应急** — sign key 怀疑泄露时需要 revoke + rotate + 通知所有历史报告持有者验签自检 + 公开 transparency 记录。详 18 §7.1 / 11(新增 KMS 应急运维 SOP)。

6. **MCP token 凭证泄露** — Cursor / Claude Code 配置文件被偷的真实威胁。短期 token + IP 白名单 + 异常突增告警 + 一键撤销。详 19 §3.4 / §6.1。

7. **LLM 复盘幻觉/造谣/泄密风险** — Prompt 约束 + 必须人工审核 + AI 标识 + sanitize + 离线 eval ≥ 4.0/5 才允许新版 prompt 上线。详 19 §2.4 / 07(新增工作流)。

8. **Anchor 节点偏差告警未规约** — 100 节点中任何一个被攻陷 = 数据污染。需要 Anchor 偏差实时告警 + 自动剔除阈值 + Verdict 报告排除"低置信节点"。详 10(待补) / 18 §3.2 step 2。

> 以上 8 项 Concerns 必须在 v2 工程实施前补足或显式规约,否则项目存在严重风险。

---

## N. v2.0 Pre-condition 答复(2026-05-13 锁定,M1 启动前必决)

> 2026-05-13 用户走 5 个 Pre-condition 逐一决策,全部锁定。本节是 v2 实施前最终决策门。

### N.1 Pre-1 — 三栈并行 vs 顺序

| 决策 | 锁定 | 影响 |
|---|---|---|
| Pre-1 | ✅ **B) S2 Evidence 主推 + S3 MCP 顺序 + CC 并行助手** | 17-roadmap 维持原 14 个月时间线;创始人主精力在 Evidence(KMS / WAL / D11 演练 / D8 标注),CC 助手承担 S3 MCP / 排行榜代码主体;创始人 1h/周 review MCP 代码 |

### N.2 Pre-2 — 月外部依赖成本

| 决策 | 锁定 | 影响 |
|---|---|---|
| Pre-2 | ✅ **C) 混合 — KMS+TSA 商业 / LLM 自家底层** | KMS(阿里云 / AWS)+ TSA(DigiCert + GlobalSign)商业服务保留(企业 due diligence);LLM 主选阿里通义 + DeepSeek(国内底层),vs Claude/GPT 节省 ~70% 月成本;月成本估算 S2 上线 ¥300-500/月,S3 ¥500-1000/月 |
| 同步影响 | DECISIONS.md §B0a / 14 §4.11 / 07 §6.3 已更新 | per-Provider eval baseline = 阿里通义 + DeepSeek |

### N.3 Pre-3 (D8) — LLM eval cold start bootstrap

| 决策 | 锁定 | 影响 |
|---|---|---|
| Pre-3 | ✅ **A) 创始人 M5-M6 手动标注 ~25h** | 30 个公开事故(AWS / CF / Azure / 阿里云 / 腾讯云历史)+ 20 个内部 dogfood = 50 条 eval bootstrap 数据集;S2 上线前(M7-M8)完成;后期可走外包 |
| 17-roadmap | M5 里程碑"LLM eval 数据集首版 50 条 bootstrap"已写入 | 创始人投入 ~25h |

### N.4 Pre-4 (D11) — Backup HSM 采购时点

| 决策 | 锁定 | 影响 |
|---|---|---|
| Pre-4 | ✅ **D) S4 才补 — S2 不采购 Backup HSM** | KMS 应急 SOP 改为 **12h Shamir 3-of-5 单路径**;Backup HSM 加速通道推迟 S4 企业越劤时补(YubiHSM2 ¥3000);S2 接受 SLA 偶尔滑至 24h+ 的风险(S2 上线初 Verdict ~100 份/月,可控) |
| 已修订文件 | DECISIONS §M / 18 §3.3 / 11 §15.5 / 12 §20 / 17-roadmap M4-M6 / OVERVIEW §9 §11.M / CLAUDE.md / ENG-REVIEW-REPORT / TODOS | 见 Lane W 完整改动汇总 |

### N.5 Pre-5 (D12) — P0 响应方式

| 决策 | 锁定 | 影响 |
|---|---|---|
| Pre-5 | ✅ **B) 创始人担 + Emergency Contact List** | S2 上线初 P0 全部创始人本人(7×24 手机告警);S2 上线前制定"创始人出差 / 假期 P0 Backup 联系人"名单(配偶 / 朋友 / 边际客服);P0 预期 0-3 次/年,可接受;**M11+ 是第二人 Operator 补充作双保险**,不是 S2 强制 |

### N.6 Pre-condition 综合影响

**月成本**:
- S2 上线初:KMS ¥50/月 + TSA 商业 $50/月(¥350)+ LLM 阿里通义 ¥300-500/月 = **~¥800-1000/月**(¥10k/年)
- S3 GA 后:同上 + LLM 量增 = **~¥1500-2500/月**(¥18-30k/年)
- vs 原方案(Claude/GPT 主)节省 ~70% LLM 成本

**人力路径**:
- S2(M5-M8):创始人 = Evidence + KMS 仪式 + 25h eval 标注 + 12h Shamir 演练;CC = MCP server + Agent obs 代码
- S3(M9-M14):创始人 = 1h/周 review MCP + S3 排行榜运营;CC = 代码主体

**SLA 风险**:
- KMS 应急:12h(周日凌晨可能滑至 24h+)— 接受
- P0 响应:1h(创始人在 + 手机畅通)/ 否则 Backup 联系人接手
- Refund / 工单常规:24h — D12 已规约

**升级路径**(S4 企业越劤时):
- 补 Backup HSM(¥3000)→ KMS 应急 4h 加速通道
- 招二人 Operator → P0 7×24 双保险

---
