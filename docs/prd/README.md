# idcd PRD 文档导航

> 版本: v2.0 (2026-05-13 锁定)
> 这是所有产品需求文档的入口。任何决策变更必须同步更新 `DECISIONS.md`。

---

## 必读（30 分钟）

| 文档 | 内容 | 时长 |
|---|---|---|
| [`OVERVIEW.md`](OVERVIEW.md) | 产品全景骨架:愿景、定位、功能版图、商业模式、路线图 | 15 min |
| [`DECISIONS.md`](DECISIONS.md) | 所有已锁定决策的单一真实来源(SSOT) | 10 min |
| [`DESIGN.md`](../DESIGN.md) | 设计系统(shadcn/ui blue theme) | 5 min |

---

## 按角色阅读

### 前端 / 设计师
1. [`DESIGN.md`](../DESIGN.md) — 设计系统
2. [`01-branding.md`](01-branding.md) — 品牌与命名
3. [`02-public-tools.md`](02-public-tools.md) — 公开工具与页面

### 后端工程师
1. [`14-tech-architecture.md`](14-tech-architecture.md) — 技术架构
2. [`15-data-model.md`](15-data-model.md) — 数据模型与 DDL
3. [`STATE-MACHINES.md`](STATE-MACHINES.md) — 关键状态机
4. [`16-api-spec.md`](16-api-spec.md) — API 规范

### SRE / 运维
1. [`10-nodes-and-agents.md`](10-nodes-and-agents.md) — 节点与 Agent 系统
2. [`12-compliance-and-abuse.md`](12-compliance-and-abuse.md) — 合规、安全、反滥用
3. [`ARCHITECTURE.md`](../ARCHITECTURE.md) §4-§9 — 部署与可观测

### AI / LLM 工程师
1. [`19-ai-agent-observability.md`](19-ai-agent-observability.md) — MCP Server + Agent 可观测
2. [`07-reports-and-dashboards.md`](07-reports-and-dashboards.md) §6 — LLM 故障复盘
3. [`ENG-REVIEW-REPORT.md`](ENG-REVIEW-REPORT.md) — D8/D9 eval 要求

---

## v2 新增模块（Evidence + MCP）

| 模块 | 内容 | 阶段 |
|---|---|---|
| [`18-evidence-and-attestation.md`](18-evidence-and-attestation.md) | Evidence-as-a-Service: Verdict 报告、KMS、TSA、验签 | S2 |
| [`19-ai-agent-observability.md`](19-ai-agent-observability.md) | MCP Server + Agent 可观测 | S3 |

---

## 实施参考

| 文档 | 用途 |
|---|---|
| [`ARCHITECTURE.md`](../ARCHITECTURE.md) | Monorepo 结构、服务清单、关键决策落地 |
| [`17-roadmap.md`](17-roadmap.md) | 详细里程碑与验收标准 |
| [`ENG-REVIEW-REPORT.md`](ENG-REVIEW-REPORT.md) | 14 项 D 决策 + CRITICAL GAP 答复 |
| [`ENG-REVIEW-TODOS.md`](ENG-REVIEW-TODOS.md) | 11 项 TODO + 工作量估算 |
| [`ER-DIAGRAM.md`](ER-DIAGRAM.md) | 实体关系图 |

---

## 决策快速索引

| 章节 | 内容 |
|---|---|
| DECISIONS §A | 产品定型(品牌/配额/账号) |
| DECISIONS §B | 技术栈选型 |
| DECISIONS §C | 商业化策略 |
| DECISIONS §D | API / 开发者决策 |
| DECISIONS §K | v2 EXPANSION 决策(9 项) |
| DECISIONS §M | Eng Review 14 项 D 决策 |
| DECISIONS §N | 5 个 Pre-condition 最终答复 |

---

> 维护规则: 模块 PRD 与 `OVERVIEW.md`、`DECISIONS.md` 不同步即视为重大问题。
