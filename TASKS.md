# TASKS — idcd 前期任务规划

> **读我优先**：开新 session 先读本文件 + `CLAUDE.md` + `docs/prd/DECISIONS.md`
> 技术栈锁定：Go 1.26 / Next.js 16 / PG 18.3 + TimescaleDB 2.21+ / Redis 7.4+ / shadcn/ui + Tailwind v4
> 设计系统：`docs/DESIGN.md` | 架构：`docs/ARCHITECTURE.md` | API：`docs/prd/16-api-spec.yaml`

---

## 状态符号

```
[ ]  未开始        [~]  进行中（标注 branch）
[x]  已完成        [!]  阻塞（标注原因）
[-]  推迟          [👤] 需要人工决策/操作，CC 不能独立完成
```

## 并行 Lane 说明

S1 四条 Lane 可同时推进，文件无冲突：

```
Lane A: 基础设施    infra/ + packages/db/
Lane B: 后端服务    apps/api/ + apps/scheduler/ + apps/gateway/
Lane C: 节点系统    apps/agent/ + infra/terraform/ + infra/ansible/
Lane D: 前端站点    apps/web/ + apps/docs/ + packages/ui/
Lane E: 合规底盘    apps/api/internal/middleware/ + static pages
```

---

## S1：落地页 + 工具站（M1–M3，约 3 个月）

> 目标：100 节点稳定运行 + 50 个工具页上线 + 公开发布
> ICP 备案已有，无阻塞。

---

### Lane A — 基础设施

**优先级最高，其他 Lane 依赖此 Lane 完成。**

- [x] **A1** `infra/docker/docker-compose.core.yml`
  - PG 18 + TimescaleDB 2.27.0 + Redis 7.4 已在远端服务器运行（开发不用本地 Docker）
  - docker-compose.core.yml 作为 Production/Staging 参考已建立
  - config/dev.env.yaml（gitignore）+ config/dev.env.example.yaml（tracked）
  - `make dev-up` → `scripts/check-connections.sh` 验证通过，TimescaleDB 2.27.0 ✓
  - *deps: 无* | *lane: A* | *完成 2026-05-13*

- [x] **A2** monorepo 骨架
  - `go.work`（9 modules: 6 apps + 3 packages）+ `pnpm-workspace.yaml` + `Makefile`
  - `make dev-setup` / `make dev-up` / `make test` / `make seed` / `make lint` 已就位
  - `VERSION` + `CHANGELOG.md` 初始化完成
  - 模块前缀：`github.com/kite365/idcd/*`，Go 1.26.2
  - *deps: 无* | *lane: A* | *完成 2026-05-13*

- [x] **A3** `packages/db/` — 数据库层
  - `migrations/idcd_main/` 5 个迁移：extensions / users+otp / session / api_key / audit_log(hypertable)
  - sqlc v1.31.1 生成 `gen/idcdmain/`（models + querier + 4 个查询文件）
  - `packages/db/repository/`：User / Session / APIKey / AuditLog 四个 repository，pgx/v5 驱动
  - DDL 规则 D1 lint 通过，audit_log TimescaleDB hypertable 已验证
  - 迁移已应用到远端 idcd_dev DB，`psql` 验证 8 张表 + 1 个 hypertable
  - *deps: A1* | *lane: A* | *完成 2026-05-13*

- [x] **A4** GitHub Actions CI/CD 基础
  - `.github/workflows/ci.yml`：golangci-lint + eslint + tsc + go test + vitest
  - `.github/workflows/deploy.yml`：build docker image → push GHCR → staging deploy
  - lint 规则：`scripts/lint-cross-schema-fk.sh`（D1）+ `scripts/lint-attestation-words.sh`（D-Concern1）
  - *deps: A2* | *lane: A*

- [ ] **A5** Cloudflare 配置
  - DNS：`idcd.com` / `api.idcd.com` / `docs.idcd.com` / `status.idcd.com` / `admin.idcd.com` / `agent-wss.idcd.com`
  - WAF 规则基础 + Bot Score + Turnstile Site Key 申请
  - Full Strict TLS 模式
  - *deps: 无* | *lane: A* | *[👤] 需在 Cloudflare Dashboard 手动操作*

- [x] **A6** `packages/shared/` 公共包
  - `idgen/`：prefix + nanoid(12, 62字母表)，30+ 实体前缀，APISecret/APIKeyPrefix/Node
  - `apperr/`：9 种错误码 + HTTP 状态映射，Is()/AsError() 链式查找
  - `logger/`：slog 封装，WithRequestID/UserID/TraceID context 注入，Discard()
  - `config/`：YAML 加载，Duration 支持 "7d" 格式，IDCD_CONFIG 环境变量
  - `stream/`：XADD MAXLEN ~ 500000（D18），5 个命名流，miniredis 测试
  - 全部测试通过（idgen×5 / apperr×7 / logger×6 / config×8 / stream×9 = 35 个用例）
  - *deps: A2* | *lane: A* | *完成 2026-05-13*

- [x] **A7** `packages/auth/` 认证包
  - JWT 签发 / 验证 / 刷新（HS256，access 15min / refresh 7d）
  - Session（Redis）存取（miniredis 测试）
  - API Key 哈希存储 + 验证（argon2id，prefix sk_live_）
  - 103 个测试全部通过，覆盖率 ≥ 90%
  - *deps: A3, A6* | *lane: A* | *完成 2026-05-13*

---

### Lane B — 后端服务

- [x] **B1** `apps/api/` — API Gateway 骨架
  - chi v5 router + 中间件链（Recover→RequestID→Logger→SecurityHeaders→CORS）
  - 统一响应格式 JSON()/Error()，apperr code+status 映射
  - GET /health（版本）、GET /health/deep（PG+Redis）、GET /metrics（Prometheus）
  - 49 个测试全部通过，覆盖率 ≥ 90%
  - *deps: A3, A6, A7* | *lane: B* | *完成 2026-05-13*

- [x] **B2** `apps/api/` — 限速模块
  - Redis 滑动窗口，多维度：单 IP / 单用户 / 单目标域名
  - 免登录用户：HTTP 拨测 30/h，Ping 60/h（Turnstile 通过后放宽）
  - 登录 Free 用户：API 100 calls/day
  - *deps: B1* | *lane: B*

- [x] **B3** `apps/api/` — 账号接口
  - `POST /v1/auth/register`（邮箱 + 密码）
  - `POST /v1/auth/login` / `POST /v1/auth/logout`
  - `POST /v1/auth/verify-email`（6 位 OTP，Redis 10min TTL）
  - `POST /v1/auth/forgot-password` / `POST /v1/auth/reset-password`
  - `GET/PATCH /v1/account/profile`
  - `DELETE /v1/account`（30 天冷静期，软删除）
  - 密码哈希：Argon2id（已决策 §4.11 合规）
  - *deps: B1, A7* | *lane: B*

- [x] **B4** `apps/api/` — 公开拨测接口（核心 API）
  - `POST /v1/probe/http` — 多地 HTTP/HTTPS 拨测
  - `POST /v1/probe/ping` — 多地 ICMP Ping
  - `POST /v1/probe/tcp` — TCPing
  - `POST /v1/probe/dns` — DNS 解析（含污染对比）
  - `POST /v1/probe/traceroute` — 路由追踪 + ASN 标注
  - `POST /v1/diagnose` — 一键全面诊断（串联以上 + SSL + WHOIS + 备案）
  - 请求参数校验 + SSRF 防护（私有 IP 黑名单）
  - 拨测报告持久化 + 分享 token（30 天过期，未登录用户）
  - *deps: B1, B2, C2* | *lane: B*

- [x] **B5** `apps/api/` — 网络信息查询接口
  - `GET /v1/info/ip?q=` — IP 归属 + ASN + ISP + 地理
  - `GET /v1/info/whois?q=` — 域名/IP WHOIS
  - `GET /v1/info/dns?q=&type=` — DNS 记录（A/AAAA/MX/TXT/CNAME/NS/CAA/DMARC/SPF）
  - `GET /v1/info/ssl?q=` — SSL 证书链 + 到期 + SAN + 协议
  - `GET /v1/info/icp?q=` — ICP 备案查询（国内特色）
  - *deps: B1, B2* | *lane: B*

- [x] **B6** `apps/scheduler/` — 调度器骨架（S1 最简版）
  - 接收拨测任务 → 节点筛选 + 打分 → 任务下发 → ack / 完成处理
  - Redis leader election（S1 用 Redis，S2 迁 etcd，见 D16）
  - 优先级队列 P0-P5
  - 超时重试（路由到候补节点）
  - *deps: A3, A6, C1* | *lane: B*

- [x] **B7** `apps/aggregator/` — 聚合器
  - 消费 `probe.results` Redis Stream
  - 幂等设计（同 task_id 重复处理无副作用）
  - 写 TimescaleDB `probe_result` hypertable
  - 更新诊断报告状态
  - *deps: A3, A6* | *lane: B*

- [x] **B8** `apps/notifier/` — 通知服务骨架（S1 仅邮件）
  - 邮件通道 adapter（SMTP STARTTLS/TLS + SES stub）
  - 验证码邮件 / 欢迎邮件 / 密码重置邮件模板（响应式 HTML）
  - asynq 队列消费（default/critical 双优先级，指数退避重试）
  - 26 个测试全部通过，go build ✓
  - *deps: A6* | *lane: B* | *完成 2026-05-13*

---

### Lane C — 节点系统

- [x] **C1** `apps/agent/` — Agent 1.0 二进制
  - 5 种 probe：HTTP(TLS/重定向) / Ping(ICMP接口化) / TCP / DNS / Traceroute(30跳)
  - 水印签名 HMAC-SHA256(node_id:task_id:target:timestamp)
  - SQLite 本地缓冲 D17：Cleanup 按 created_at，500MB 上限
  - systemd service 文件，CGO_ENABLED=0 交叉编译 linux/amd64+arm64
  - 102 个测试全部通过
  - *deps: A2, A6* | *lane: C* | *完成 2026-05-13*

- [x] **C2** `apps/gateway/` — Agent Gateway
  - WSS 接入（mTLS）
  - 心跳处理（30s timeout → drain）
  - 任务下发（来自 Scheduler）+ 结果上报（推 Aggregator）
  - 控制消息（drain / upgrade / config push）
  - 单实例承载 5000-10000 Agent 连接
  - *deps: A6, A7* | *lane: C*

- [ ] **C3** `infra/terraform/` — 节点 IaC
  - Hetzner / Vultr / RackNerd / DMIT / BWG 节点模板
  - 变量：厂商 / 地区 / 规格 / tag（tier1_cn / tier1_overseas）
  - `terraform apply` 一键创建 VPS
  - 输出：IP 列表 → 自动写入 Ansible inventory
  - *deps: 无* | *lane: C* | *[👤] 需要各 VPS 厂商 API Key*

- [ ] **C4** `infra/ansible/` — 节点部署 playbook
  - `site.yml`：系统初始化（SSH 加固 + UFW + fail2ban）
  - `agent.yml`：Agent 二进制部署 + systemd 启动 + 证书 enrollment
  - `agent-update.yml`：OTA 更新（L1 1% → L2 10% → L3 100%，K-架构）
  - 目标：`ansible-playbook agent.yml` 30 分钟内完成 100 节点部署
  - *deps: C1, C3* | *lane: C*

- [x] **C5** 节点目录 API
  - `GET /v1/nodes` — 公开节点列表（ASN / 运营商 / 地理 / 出口 IP）
  - 节点心跳写入 + 自动剔除（5 min 无心跳 → inactive）
  - 节点健康打分（每日 cron）
  - *deps: B1, C2* | *lane: C*

---

### Lane D — 前端站点

- [x] **D1** `apps/web/` — Next.js 16 骨架
  - App Router + TypeScript strict，深色模式默认
  - shadcn/ui blue theme CSS 变量，Tailwind v4
  - Geist Sans + Geist Mono + PingFang SC fallback
  - `packages/ui/` 5 个基础组件（Button/Card/Input/Badge/Separator）
  - Vitest 测试 15 个全绿（utils 6 + Button 9）
  - *deps: A2* | *lane: D* | *完成 2026-05-13*

- [x] **D2** 首页（`/`）
  - Hero：诊断输入框 + 一键诊断 + 快捷 Badge 链接
  - Feature Cards（全球节点/实时并发/SSL检测）+ 节点统计 4 卡片 + 工具入口 6 个
  - Nav sticky（汉堡菜单 mobile）+ Footer（三列 + ICP 备案号）
  - 14 个 Vitest 测试全部通过
  - *deps: D1* | *lane: D* | *完成 2026-05-13*

- [x] **D3** 工具页 SSG（50 个，`/tools/[slug]`）
  - 路由：`/tools/ping` `/tools/http` `/tools/dns` `/tools/traceroute` `/tools/ssl` `/tools/whois` `/tools/icp` `/tools/ip` `/tools/diagnose` ...（完整列表见 02-public-tools.md）
  - 每页：独立 URL + SSG 构建 + Cloudflare CDN 缓存
  - 组件：拨测表单 + 实时结果展示（SSE 或 polling）+ 节点选择器
  - Turnstile 集成（无登录用户拨测前校验）
  - SEO：`<title>` + `<meta description>` + Schema.org + hreflang
  - `apps/web/src/app/tools/[slug]/` 动态路由 + `tools-config.ts` 50+ 工具元数据
  - 5 类 client 组件（probe/text/converter/generator/lookup），116 测试 ✓
  - *deps: D1, B4, B5* | *lane: D* | *完成 2026-05-14*

- [x] **D4** 一键诊断（`/tools/diagnose`）
  - 输入域名 → 并发发起：DNS + HTTPS + Ping + Traceroute + SSL + 备案 + WHOIS + 安全头
  - 进度条实时展示（SSE）
  - 诊断报告页面 `/report/<id>`（SSR + OG 卡片）
  - 分享链接（30 天有效，未登录；登录用户永久）
  - PDF 导出按钮（S2 实现，S1 占位即可）
  - *deps: D3, B4* | *lane: D*

- [x] **D5** 账号页面
  - `/auth/register` `/auth/login` `/auth/logout`
  - `/auth/verify-email`（OTP 输入）
  - `/auth/forgot-password` `/auth/reset-password`
  - `/app/settings/profile`（头像 / 邮箱 / 密码修改）
  - `/app/settings/account`（注销 + 数据导出入口）
  - *deps: D1, B3* | *lane: D*

- [x] **D6** 公开节点目录（`/nodes`）
  - 节点列表：ASN + 运营商 + 地区 + 出口 IP + 在线状态
  - 按国家 / 运营商筛选
  - 节点地图可视化（ECharts）
  - *deps: D1, C5* | *lane: D*

- [x] **D7** SEO 基础
  - `sitemap.xml` 动态生成（工具页 + 节点页 + 博客）
  - `robots.txt`（`/legacy/*` noindex）
  - `manifest.json` + favicon
  - Google Search Console / 百度站长 验证文件
  - 帮助中心骨架（`apps/docs/`，Nextra SSG，docs.idcd.com，30 篇初始文档）
  - *deps: D3* | *lane: D*

- [x] **D8** 辅助工具页（SEO 长尾，各自独立页面）
  - JSON / YAML / XML 格式化
  - Base64 / URL / Unicode 编码解码
  - 时间戳转换
  - 哈希计算（MD5 / SHA256 / CRC32）
  - JWT 解码
  - 正则表达式测试
  - Cron 表达式可视化
  - 二维码生成 / 解码
  - IP 段 / CIDR 计算（纯前端）
  - IPv6 检测 / 转换
  - 38 个 Vitest 测试全部通过（含 5 个工具单元测试）
  - *deps: D1* | *lane: D* | *完成 2026-05-13*

---

### Lane E — 反滥用 + 合规底盘

**必须 S1 上线，不能省。**

- [x] **E1** 拒测黑名单
  - 私有 IP 段（RFC1918）/ 政府 / 银行 / 友站列表
  - 接入 B2 限速中间件，拨测前校验目标
  - 后台可配置黑名单（初期写死配置文件）
  - *deps: B1* | *lane: E*

- [x] **E2** 测试报告水印
  - 每条拨测结果写入：`node_id + task_id + target + timestamp` 签名
  - 水印可被追溯（abuse 举报时可还原来源）
  - *deps: C1* | *lane: E*

- [x] **E3** 法律页面（静态页）
  - `/terms` 服务条款
  - `/privacy` 隐私政策（含 PIPL 合规条款）
  - `/aup` 可接受使用政策（明确禁止 DDoS 放大 / 端口扫描 / 漏洞探测）
  - Cookie 同意横幅（底部）
  - `/about` 关于页
  - *deps: D1* | *lane: E* | *[👤] 法律文本需人工起草或购买模板*

- [x] **E4** 安全头 + CSRF
  - CSP / HSTS / X-Frame-Options / X-Content-Type-Options
  - Same-Site Cookie
  - CSRF Token（表单提交）
  - *deps: B1* | *lane: E*

- [x] **E5** 可观测性接入
  - 所有 Go service：Prometheus metrics + OpenTelemetry trace + Loki 日志（含 request_id）
  - Next.js：Sentry 接入（前端错误追踪）
  - Grafana dashboard：RPS / 延迟 / 错误率 / 节点在线数
  - 自家产品 dogfood：主站 / API / Gateway 加监控（吃自家狗粮）
  - *deps: A1, B1, C2* | *lane: E*

---

### S1 验收清单（M4 末）

- [ ] 100+ 节点稳定运行（成功率 > 95%）
- [ ] 50+ 工具页 SSG 上线，页面可访问
- [ ] 一键诊断可生成可分享报告
- [ ] 反滥用底盘上线（黑名单 + 限速 + Turnstile）
- [ ] 邮箱注册 / 登录可用
- [ ] ICP 备案号显示在页面底部（已有，挂上即可）
- [ ] Prometheus / Grafana 监控就位
- [ ] [👤] 公开发布（去除 beta 标签，发公关稿）
- 目标指标：日 UV 1000+ / 注册用户 500+

---

---

## S2：客户中心（M5–M7，约 3 个月）

> 依赖 S1 全部完成。管理台大部分推迟，只做 refund_failed 看板 + 节点健康看板。

---

### M5 — 监控模块

- [x] **F1** 数据库扩展
  - `lib/db/migrations/idcd_main/00007_monitors.sql`：monitors + monitor_checks（TimescaleDB hypertable）
  - `lib/db/queries/idcd_main/monitor.sql`：7 个 sqlc 查询 + `lib/db/repository/monitor.go`
  - *deps: A3* | *完成 2026-05-14*

- [x] **F2** `apps/api/` — 监控 CRUD 接口
  - `POST/GET/PATCH/DELETE /v1/monitors` + pause/resume，7 个 handler，26 测试 ✓
  - SSRF 校验、ownership 检查、Bearer token 鉴权
  - *deps: B1, F1* | *完成 2026-05-14*

- [x] **F3** 监控调度集成
  - Scheduler `monitorPoller` goroutine（30s 轮询 ListActiveMonitorsDue）
  - Aggregator `Process()` 写 monitor_checks + 推进 next_check_at
  - 492 tests ✓（api+scheduler+aggregator+lib/db）
  - *deps: B6, B7, F1* | *完成 2026-05-14*

- [x] **F4** 控制台监控界面（`/app/monitors`）
  - 列表（Table + 4 统计卡片 + 暂停/删除）+ 新建向导（9 种类型，4 步）+ 详情（48 块趋势图 + SSE 占位）
  - 21 tests ✓ | shadcn/ui 全组件
  - *deps: D1, F2, F3* | *完成 2026-05-14*

---

### M6 — 告警模块

- [x] **G1** 数据库扩展
  - `lib/db/migrations/idcd_main/00008_alerts.sql`：alert_channels / alert_policies / alert_events / alert_notifications
  - D1 合规，无 cross-schema FK
  - *deps: A3, F1* | *完成 2026-05-14*

- [x] **G2** `apps/notifier/` — 告警通道扩展
  - `apps/notifier/internal/channel/`：Channel 接口 + Webhook / 企业微信 / 钉钉 / 飞书 四个 adapter
  - `HandleAlertNotification` handler 路由到各 adapter，asynq 队列
  - 48 tests ✓（httptest mock 外部 HTTP）
  - *deps: B8, G1* | *完成 2026-05-14*

- [x] **G3** `apps/api/` — 告警策略接口
  - 11 个 endpoint（channels/policies/events/ack），30 测试 ✓
  - Bearer token 鉴权，`?monitor_id=` 过滤，D1 compliant
  - *deps: B1, G1* | *完成 2026-05-14*

- [x] **G4** 控制台告警界面（`/app/alerts`）
  - 三 Tab：事件历史（firing/resolved/ack）+ 通道管理（Card+Sheet）+ 策略配置（Table+Switch）
  - 32 tests ✓ | shadcn/ui 全组件
  - *deps: D1, G3* | *完成 2026-05-14*

---

### M7 — 计费 + 状态页 + Evidence 准备

- [x] **H1** 数据库扩展
  - `lib/db/migrations/idcd_main/00009_billing.sql`：subscriptions / invoices / payments / status_pages
  - payments 含 refund_retry_count + partial index WHERE status='refund_failed'（D5）
  - *deps: A3, F1* | *完成 2026-05-14*

- [x] **H2** 支付接口层（provider-agnostic stub，待接聚合支付）
  - `apps/api/internal/billing/`：`Provider` 接口 + `StubProvider`（内存模拟）
  - `migration 00010`：paddle_* 字段迁移为通用 ext_* + payment_providers 配置表
  - billing API：POST /v1/billing/subscribe|cancel + GET subscription|invoices + webhook + stub-confirm
  - 22 provider tests + 25 handler tests（534 total ✓）
  - 接聚合支付只需实现 Provider 接口（Subscribe/Cancel/ParseWebhook/RefundPayment）
  - *deps: B1, H1* | *完成 2026-05-14*

- [x] **H3** 配额执行
  - `apps/api/internal/quota/`：`PlanLimits` + `Limits()` + 5 个 Check 函数
  - `APIRateLimiter`：Redis INCR 日限，fail-open，clock 可 mock
  - monitor/alert handler 注入配额检查，超额返回 HTTP 402 + upgrade_url
  - `GET /v1/account/quota`：返回当前用量 JSON
  - `APIQuotaMiddleware`：对认证路由扣 API 调用量，超额 429
  - new-monitor-client 捕获 402 → Alert + 升级按钮
  - 46 quota tests + miniredis tests（842 Go total ✓）
  - *deps: B1, H2* | *完成 2026-05-14*

- [x] **H4** `apps/status/` — 状态页（Next.js 独立 app）
  - `apps/status/` 独立 Next.js 16 app，支持 `<slug>.status.idcd.com`
  - 服务分组 + 90 天 CSS grid 方块图 + 事件公告 + Powered by idcd 水印
  - 9 tests ✓
  - *deps: D1, F3* | *完成 2026-05-14*

- [x] **H5** 控制台计费 + 状态页界面
  - `/app/billing`：定价对比表 4 档 + Paddle 占位 Alert + 空发票列表
  - `/app/status-pages`：列表 Card + 新建 Sheet + Free 升级 Dialog
  - `/app/usage`：4 个 Progress 卡片 + 7 天 CSS 柱状图
  - 20 tests ✓ | shadcn/ui 全组件
  - *deps: D1, H2, H4* | *完成 2026-05-14*

- [x] **H6** 最小管理台（仅 2 个必要功能）
  - `apps/admin/`（独立 Next.js app，port 3001）
  - `/admin/nodes`：节点健康看板（4 统计卡 + 状态 Badge 表格）
  - `/admin/refund-failed`：退款失败看板（GET/POST retry API，440+12 tests ✓）
  - *完成 2026-05-14*
  - 访问：VPN / WireGuard 才可（admin.idcd.com）
  - *deps: B1, C5* | *lane: H*

---

### S2 验收清单（M7 末）

- [ ] 监控类型 M01-M08 可创建并正常拨测
- [ ] 至少 5 个告警通道可用（邮件+企微+钉钉+飞书+Webhook）
- [ ] 状态页可创建 + 自定义域 + ACME 自动证书
- [ ] Paddle 支付可完成订阅（测试环境验证）
- [ ] refund_failed 看板可查
- [ ] [👤] 海外公司主体注册完成（Paddle 收款主体）
- 目标指标：首笔商业订单 / MRR ¥10k+ / 付费用户 200+

---

---

## 长期推迟（不进入当前冲刺）

```
[-] Evidence MVP（KMS + TSA + Verdict）    → S2 中后期，M6 启动准备
[-] MCP Server（mcp.idcd.com）            → S3，M9+
[-] API 公开 GA + SDK + CLI               → S3，M8-M9
[-] 团队 / 多用户 / 角色权限              → S3，M12
[-] 众包节点                              → S3，M13
[-] 管理台完整功能                        → S2 末/S3
[-] 企业版 SSO / 私有部署                 → S4
```

---

## 并行任务分配看板

> 开新 worktree 时在此记录。

| Branch | 负责内容 | 主要目录 | 状态 |
|---|---|---|---|
| `main` | PRD + 架构文档 | `docs/` | 持续维护 |
| — | — | — | — |

---

## 会话日志

| 日期 | 完成 | 遗留/阻塞 |
|---|---|---|
| 2026-05-13 | 架构审查 D17-D21，TASKS.md 建立 | — |
| 2026-05-13 | A1（基础设施）+ A2（monorepo 骨架）完成，TimescaleDB 2.27.0 安装 | — |
| 2026-05-13 | A3（DB 层）完成：5 个迁移 + sqlc 生成 + 4 个 repository | — |
| 2026-05-13 | A6（shared 包）完成：idgen/apperr/logger/config/stream，35 个测试 | A7（auth 包）待开始 |
| 2026-05-13 | A7（auth 包）完成：jwt/session/apikey，103 tests ✓ | — |
| 2026-05-13 | B8（notifier）完成：SMTP+模板+asynq，26 tests ✓ | — |
| 2026-05-13 | D1（Next.js 骨架）完成：App Router + shadcn/ui + packages/ui，15 tests ✓ | D2/D8 可启动 |
| 2026-05-14 | D3（工具页 SSG 50+）完成：[slug] 动态路由 + tool-functions + SSE API，216 tests ✓ | A5/C3/C4 需人工操作 |
| 2026-05-14 | F1/F2/F3（监控模块）+ G1/G2/G3（告警模块）+ H1/H6（计费DB+管理台）并行完成，735 Go tests ✓ | F4/G4/H4/H5 待做 |
| 2026-05-14 | F4（监控UI）+ G4（告警UI）+ H4（状态页app）+ H5（计费UI）并行完成，289+9 前端 tests ✓ | H2/H3 待 Paddle 账号 [👤] |
| 2026-05-14 | H2（支付stub）+ H3（配额执行）+ App Shell（侧边栏）+ Settings（account+api-keys）并行完成，842 Go + 334 前端 tests ✓ | 聚合支付接入待定 |

---

## 快速命令

```bash
# 新 session 开始
/context-restore

# 领取任务并开始（CC 自动创建 worktree）
"读 TASKS.md，领取 Lane A 中第一个未开始的任务，开始工作"

# 并行领取多个任务
"读 TASKS.md，把 Lane D 的 D1 D2 D8 并行执行（D8 无依赖可最早启动）"

# 检查依赖后自动编排
/everything-claude-code:orchestrate

# 结束 session 前
/context-save
# 然后手动更新本文件的状态符号和会话日志
```
