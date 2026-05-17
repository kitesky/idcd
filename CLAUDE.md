# idcd — CLAUDE.md

> 项目级 AI 协同指南。**在做任何代码 / UI / 决策 之前必读本文件 + 引用文档**。

---

## 单一真实源(SSOT)

| 维度 | 文档 |
|---|---|
| 产品全景 / scope | `docs/prd/OVERVIEW.md` |
| 所有锁定决策 | `docs/prd/DECISIONS.md`(A0-A8 / B / K / M) |
| 品牌(idcd 锁定) | `docs/prd/01-branding.md` |
| 设计系统(shadcn/ui + zinc + OKLCH) | `docs/DESIGN.md` |
| 技术架构(PRD 级) | `docs/prd/14-tech-architecture.md` |
| 项目实施架构 | `docs/ARCHITECTURE.md` |
| API 规范 | `docs/prd/16-api-spec.yaml`(OpenAPI 3.1) |
| 数据模型 ER 图 | `docs/prd/ER-DIAGRAM.md` |
| 状态机集中 | `docs/prd/STATE-MACHINES.md` |
| 工程审查 14 项 D 决策 | `docs/prd/ENG-REVIEW-REPORT.md` |
| 实施 TODO | `docs/prd/ENG-REVIEW-TODOS.md` |
| 路线图 | `docs/prd/17-roadmap.md` |

---

## 设计系统

**始终先读 `docs/DESIGN.md` 再做任何视觉 / UI 决策。**

- 完整采用 shadcn/ui 官方体系（new-york style，不自定义间距/圆角/字体）
- **主题唯一入口 = `apps/web/src/styles/theme.css`**（OKLCH 色彩空间，改这里换全局主题）
- Base Color = **Zinc**（深色偏蓝灰），shadcn/ui 官方预设
- 字体 = Geist Sans + Geist Mono（Next.js / shadcn 默认）
- 中文字体 fallback = PingFang SC / Microsoft YaHei
- 默认深色模式（`next-themes` 动态控制，不硬编码 `className="dark"`）
- 业务语义色扩展：success / warning / info（在 theme.css 定义）
- `/app/*` 后台使用 shadcn `Sidebar` 组件（`SidebarProvider` + `AppSidebar` + `SidebarInset`）
- idcd 特定组件通过 composition，不重写 shadcn 已有组件

### 组件使用强制规则

**必须使用 shadcn/ui 组件，禁止手写 div + className 拼接 UI。**

| 场景 | 做法 |
|---|---|
| 按钮 | `<Button>` 而非 `<div onClick>` 或裸 `<button>` |
| 卡片/面板 | `<Card>` + `<CardContent>` 而非 `<div className="rounded border">` |
| 表单字段 | `<Form>` + `<FormField>` + `<Input>` / `<Select>` 而非裸 input |
| 提示/状态 | `<Alert>` / `<Badge>` 而非自写色块 |
| 移动端菜单/抽屉 | `<Sheet>` 而非自写 fixed panel |
| 后台侧边栏 | `<Sidebar>` + `<SidebarProvider>` 而非自写 aside |
| 面包屑 | `<Breadcrumb>` 系列组件 |
| Toast 通知 | `<Toaster>`（Sonner）而非自写 |
| 间距/布局 | Tailwind spacing utilities + shadcn 布局，不造容器组件 |

**允许裸 div 的唯一情形**：纯布局容器（flex/grid wrapper）且无视觉样式（无 border/bg/shadow/rounded）。

在 code review 和 QA 阶段 flag 任何绕过 shadcn 组件手写 UI 的代码。

---

## 关键 v2 决策摘要(详 DECISIONS.md §M)

实施时必须遵守:

- **D1 跨 schema 不写 FK**:`15-data-model §4.X` 所有 v2 表 DDL 无 cross-schema REFERENCES;走 Repository 应用层 join
- **D2 Token 90d 上限**:所有 MCP token(personal 24h / workspace 90d / service 90d auto_renewal),**无永久 token**;MCP units 与 API 配额完全独立池
- **D4 Verdict WAL**:`attestation_record` 充当 WAL;step-level `UNIQUE(report_id, action)`;KMS sign 必传 idempotency token
- **D5 Refund retry queue**:聚合支付 refund 失败 5min/30min retry;30min 内强制发用户道歉邮箱;`refund_failed` 状态入 admin dashboard + P0
- **D6 Self-Verify 独立**:Self-Verify Worker 独立进程 / 独立 VPC subnet / 独立 KMS 客户端;仅调 `attest.idcd.com/verify` 公开接口
- **D11 KMS 应急 SOP**:**12h Shamir 3-of-5 单路径**(Pre-4 调整);Backup HSM 加速通道**推迟 S4**;S2 上线前必演练 12h 路径;接受 SLA 偶尔滑至 24h+ 现实风险
- **D12 3 档 SLA**:Verdict 失败纯自动 / 1h 仅 P0(KMS / 节点失窃)/ 24h 常规客服
- **D13 MCP SSE**:业务 stateless + 连接 stateful;LB sticky session 必要;10k SSE/实例

---

## 测试门禁（强制）

**写代码必须写测试用例，不满足以下要求不得提交。**

### Go 后端

- 每个新包/函数必须有对应 `_test.go`，行覆盖率目标 ≥ 90%
- 纯计算函数（ID 生成、错误类型、Duration 解析等）目标 100%
- 数据库 / Redis 操作用 **miniredis**（stream）/ **pgx mock** 或测试库隔离，不依赖真实环境
- 测试命令：`go test ./...`（在项目根运行，通过 go.work 覆盖所有 module）

### 前端（Next.js / TypeScript）—— 唯一前端项目 `apps/web`

**所有前端内容统一在 `apps/web` 一个 Next.js 项目中，无单独子应用。**

路由分组架构（不影响 URL 路径）：
- `app/(public)/` — 公开页面（带 Nav + Footer）：首页、工具、文档入口
- `app/app/` — 用户后台（需登录，Sidebar 布局）
- `app/auth/` — 登录 / 注册等认证页
- `app/admin/` — 内部管理后台（仅 VPN 可访问，独立 header，无公开 Nav）
- `app/status/[slug]/` — 状态页（支持自定义域名，通过 middleware 路由）
- `app/docs/` — 产品文档（自建 sidebar 布局，shadcn/ui 样式）

测试：
- 每个 utility 函数必须有 Vitest 单元测试
- 组件测试用 Testing Library（关键交互路径）
- 测试命令：`pnpm --filter @idcd/web test`

### 提交前检查清单

```
□ go test ./... 全绿（无 FAIL）
□ 新文件有配套 _test.go（或 .test.ts）
□ scripts/lint-cross-schema-fk.sh 通过（DB 迁移改动时）
□ scripts/lint-attestation-words.sh 通过（probe 模块改动时）
```

---

## 并发 Agent 派发规则

使用 `Agent(isolation: "worktree", run_in_background: true)` 并发多个 agent 时，必须遵守：

1. **文件集合互不相交**：规划任务时列出每个 agent 修改的文件，确认零重叠再派发
2. **不写绝对路径**：prompt 里写 `文件: apps/web/...`（相对路径），而非 `/Volumes/Workspace/code/idcd/...`，否则 isolation 的 worktree 隔离失效，agent 直接写主 repo
3. **同一文件只能一个 agent 改**：需要多处修改同一文件时，合并成单个 agent 或顺序执行
4. **sidebar-data.ts 是反例**：批次五两个 agent 都改了它，靠运气（后者先读再追加）没丢数据，但不可依赖

---

## Skill routing

When the user's request matches an available skill, invoke it via the Skill tool. When in doubt, invoke the skill.

Key routing rules:
- Product ideas/brainstorming → invoke /office-hours
- Strategy/scope → invoke /plan-ceo-review
- Architecture → invoke /plan-eng-review
- Design system/plan review → invoke /design-consultation or /plan-design-review
- Full review pipeline → invoke /autoplan
- Bugs/errors → invoke /investigate
- QA/testing site behavior → invoke /qa or /qa-only
- Code review/diff check → invoke /review
- Visual polish → invoke /design-review
- Ship/deploy/PR → invoke /ship or /land-and-deploy
- Save progress → invoke /context-save
- Resume context → invoke /context-restore

---

## 5 个 Pre-condition 已决(2026-05-13)

详 DECISIONS.md §N:

- **Pre-1**: ✅ **B) S2 Evidence 主 + S3 MCP 顺序 + CC 并行助手** — 14 个月可交付 v2 全量
- **Pre-2**: ✅ **C) 混合 — KMS+TSA 商业 / LLM 自家底层(阿里通义+DeepSeek)** — 月成本 ¥300-500,节省 ~70%
- **Pre-3 (D8)**: ✅ **A) 创始人 M5-M6 手动标注 25h** — 50 条 eval bootstrap 数据集
- **Pre-4 (D11)**: ✅ **D) Backup HSM 推迟 S4** — S2 走 12h Shamir 单路径,接受 SLA 偶尔滑至 24h+ 风险
- **Pre-5 (D12)**: ✅ **B) 创始人担 + Emergency Contact List** — S2 上线前制定 Backup 联系人名单
