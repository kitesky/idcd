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

- [ ] **A1** `infra/docker/docker-compose.core.yml`
  - PG 18.3 + TimescaleDB 2.21+ + Redis 7.4+
  - Loki + Prometheus + Grafana + Tempo + Sentry（自托管）
  - 验收：`make dev-up` 本地跑通，`psql` 可连，`redis-cli ping` 返回 PONG
  - *deps: 无* | *lane: A*

- [ ] **A2** monorepo 骨架
  - `go.work`（8 apps + packages/*）+ `pnpm-workspace.yaml` + `Makefile`
  - `make dev-setup` / `make dev-up` / `make test` / `make seed` 四个入口
  - `VERSION` + `CHANGELOG.md` 初始化
  - *deps: 无* | *lane: A*

- [ ] **A3** `packages/db/` — 数据库层
  - `migrations/idcd_main/` 基础表：`users` / `api_keys` / `sessions` / `audit_log`
  - sqlc 配置 + `sqlc generate` 跑通
  - `packages/db/repository/` 基础抽象
  - DDL 规则：**严禁跨 schema REFERENCES**（D1），CI lint 脚本 `scripts/lint-cross-schema-fk.sh`
  - *deps: A1* | *lane: A*

- [ ] **A4** GitHub Actions CI/CD 基础
  - `.github/workflows/ci.yml`：golangci-lint + eslint + tsc + go test + vitest
  - `.github/workflows/deploy.yml`：build docker image → push GHCR → staging deploy
  - lint 规则：`scripts/lint-cross-schema-fk.sh`（D1）+ `scripts/lint-attestation-words.sh`（D-Concern1）
  - *deps: A2* | *lane: A*

- [ ] **A5** Cloudflare 配置
  - DNS：`idcd.com` / `api.idcd.com` / `docs.idcd.com` / `status.idcd.com` / `admin.idcd.com` / `agent-wss.idcd.com`
  - WAF 规则基础 + Bot Score + Turnstile Site Key 申请
  - Full Strict TLS 模式
  - *deps: 无* | *lane: A* | *[👤] 需在 Cloudflare Dashboard 手动操作*

- [ ] **A6** `packages/shared/` 公共包
  - ID 生成：prefix + nanoid（`u_` `m_` `k_` `ar_` 等前缀，见 15-data-model D5）
  - 错误类型 / 日志接口 / 配置读取
  - Redis Streams 写入封装（含 `XADD ... MAXLEN ~ 500000`，D18）
  - *deps: A2* | *lane: A*

- [ ] **A7** `packages/auth/` 认证包
  - JWT 签发 / 验证 / 刷新
  - Session（Redis）存取
  - API Key 哈希存储 + 验证
  - *deps: A3, A6* | *lane: A*

---

### Lane B — 后端服务

- [ ] **B1** `apps/api/` — API Gateway 骨架
  - chi router + middleware 链（request_id / auth / rate_limit / cors / security_headers）
  - 统一错误格式（`{error: {code, message, request_id}}`）
  - 健康检查：`GET /health` + `GET /health/deep`
  - Prometheus metrics endpoint
  - *deps: A3, A6, A7* | *lane: B*

- [ ] **B2** `apps/api/` — 限速模块
  - Redis 滑动窗口，多维度：单 IP / 单用户 / 单目标域名
  - 免登录用户：HTTP 拨测 30/h，Ping 60/h（Turnstile 通过后放宽）
  - 登录 Free 用户：API 100 calls/day
  - *deps: B1* | *lane: B*

- [ ] **B3** `apps/api/` — 账号接口
  - `POST /v1/auth/register`（邮箱 + 密码）
  - `POST /v1/auth/login` / `POST /v1/auth/logout`
  - `POST /v1/auth/verify-email`（6 位 OTP，Redis 10min TTL）
  - `POST /v1/auth/forgot-password` / `POST /v1/auth/reset-password`
  - `GET/PATCH /v1/account/profile`
  - `DELETE /v1/account`（30 天冷静期，软删除）
  - 密码哈希：Argon2id（已决策 §4.11 合规）
  - *deps: B1, A7* | *lane: B*

- [ ] **B4** `apps/api/` — 公开拨测接口（核心 API）
  - `POST /v1/probe/http` — 多地 HTTP/HTTPS 拨测
  - `POST /v1/probe/ping` — 多地 ICMP Ping
  - `POST /v1/probe/tcp` — TCPing
  - `POST /v1/probe/dns` — DNS 解析（含污染对比）
  - `POST /v1/probe/traceroute` — 路由追踪 + ASN 标注
  - `POST /v1/diagnose` — 一键全面诊断（串联以上 + SSL + WHOIS + 备案）
  - 请求参数校验 + SSRF 防护（私有 IP 黑名单）
  - 拨测报告持久化 + 分享 token（30 天过期，未登录用户）
  - *deps: B1, B2, C2* | *lane: B*

- [ ] **B5** `apps/api/` — 网络信息查询接口
  - `GET /v1/info/ip?q=` — IP 归属 + ASN + ISP + 地理
  - `GET /v1/info/whois?q=` — 域名/IP WHOIS
  - `GET /v1/info/dns?q=&type=` — DNS 记录（A/AAAA/MX/TXT/CNAME/NS/CAA/DMARC/SPF）
  - `GET /v1/info/ssl?q=` — SSL 证书链 + 到期 + SAN + 协议
  - `GET /v1/info/icp?q=` — ICP 备案查询（国内特色）
  - *deps: B1, B2* | *lane: B*

- [ ] **B6** `apps/scheduler/` — 调度器骨架（S1 最简版）
  - 接收拨测任务 → 节点筛选 + 打分 → 任务下发 → ack / 完成处理
  - Redis leader election（S1 用 Redis，S2 迁 etcd，见 D16）
  - 优先级队列 P0-P5
  - 超时重试（路由到候补节点）
  - *deps: A3, A6, C1* | *lane: B*

- [ ] **B7** `apps/aggregator/` — 聚合器
  - 消费 `probe.results` Redis Stream
  - 幂等设计（同 task_id 重复处理无副作用）
  - 写 TimescaleDB `probe_result` hypertable
  - 更新诊断报告状态
  - *deps: A3, A6* | *lane: B*

- [ ] **B8** `apps/notifier/` — 通知服务骨架（S1 仅邮件）
  - 邮件通道 adapter（SMTP Postfix + SES 备用）
  - DKIM 配置
  - 验证码邮件 / 欢迎邮件 / 密码重置邮件模板
  - asynq 队列消费
  - *deps: A6* | *lane: B*

---

### Lane C — 节点系统

- [ ] **C1** `apps/agent/` — Agent 1.0 二进制
  - 拨测能力：HTTP/HTTPS + Ping（ICMP）+ TCPing + DNS + Traceroute/MTR
  - mTLS 客户端证书（短期 7-30 天 + 自动 renewal，K-架构）
  - 任务签名校验 + 硬编码任务白名单
  - 上报结果带水印（node_id + task_id + timestamp）
  - **SQLite 本地 24h 缓冲**（D17）：`$AGENT_DATA_DIR/buffer.db`，500MB 上限，进程重启不丢数据
  - systemd service 文件
  - 交叉编译：linux/amd64 + linux/arm64 静态二进制
  - *deps: A2, A6* | *lane: C*

- [ ] **C2** `apps/gateway/` — Agent Gateway
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

- [ ] **C5** 节点目录 API
  - `GET /v1/nodes` — 公开节点列表（ASN / 运营商 / 地理 / 出口 IP）
  - 节点心跳写入 + 自动剔除（5 min 无心跳 → inactive）
  - 节点健康打分（每日 cron）
  - *deps: B1, C2* | *lane: C*

---

### Lane D — 前端站点

- [ ] **D1** `apps/web/` — Next.js 16 骨架
  - App Router + TypeScript strict
  - shadcn/ui 初始化 + blue theme（`docs/DESIGN.md` 锁定配色）
  - Tailwind v4 配置
  - Geist Sans + Geist Mono 字体 + PingFang SC fallback
  - 默认深色模式
  - `packages/ui/` 共享组件库初始化
  - *deps: A2* | *lane: D*

- [ ] **D2** 首页（`/`）
  - 3-hero 布局：一键诊断输入框 + 特性介绍 + 节点地图预览
  - 顶部 Nav（工具 / 节点 / 定价 / 文档 / 登录）
  - Footer（链接 + 备案号 + 隐私协议）
  - 响应式（移动端优先）
  - *deps: D1* | *lane: D*

- [ ] **D3** 工具页 SSG（50 个，`/tools/[slug]`）
  - 路由：`/tools/ping` `/tools/http` `/tools/dns` `/tools/traceroute` `/tools/ssl` `/tools/whois` `/tools/icp` `/tools/ip` `/tools/diagnose` ...（完整列表见 02-public-tools.md）
  - 每页：独立 URL + SSG 构建 + Cloudflare CDN 缓存
  - 组件：拨测表单 + 实时结果展示（SSE 或 polling）+ 节点选择器
  - Turnstile 集成（无登录用户拨测前校验）
  - SEO：`<title>` + `<meta description>` + Schema.org + hreflang
  - *deps: D1, B4, B5* | *lane: D*

- [ ] **D4** 一键诊断（`/tools/diagnose`）
  - 输入域名 → 并发发起：DNS + HTTPS + Ping + Traceroute + SSL + 备案 + WHOIS + 安全头
  - 进度条实时展示（SSE）
  - 诊断报告页面 `/report/<id>`（SSR + OG 卡片）
  - 分享链接（30 天有效，未登录；登录用户永久）
  - PDF 导出按钮（S2 实现，S1 占位即可）
  - *deps: D3, B4* | *lane: D*

- [ ] **D5** 账号页面
  - `/auth/register` `/auth/login` `/auth/logout`
  - `/auth/verify-email`（OTP 输入）
  - `/auth/forgot-password` `/auth/reset-password`
  - `/app/settings/profile`（头像 / 邮箱 / 密码修改）
  - `/app/settings/account`（注销 + 数据导出入口）
  - *deps: D1, B3* | *lane: D*

- [ ] **D6** 公开节点目录（`/nodes`）
  - 节点列表：ASN + 运营商 + 地区 + 出口 IP + 在线状态
  - 按国家 / 运营商筛选
  - 节点地图可视化（ECharts）
  - *deps: D1, C5* | *lane: D*

- [ ] **D7** SEO 基础
  - `sitemap.xml` 动态生成（工具页 + 节点页 + 博客）
  - `robots.txt`（`/legacy/*` noindex）
  - `manifest.json` + favicon
  - Google Search Console / 百度站长 验证文件
  - 帮助中心骨架（`apps/docs/`，Nextra SSG，docs.idcd.com，30 篇初始文档）
  - *deps: D3* | *lane: D*

- [ ] **D8** 辅助工具页（SEO 长尾，各自独立页面）
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
  - *deps: D1* | *lane: D*（纯前端，可最早并行）

---

### Lane E — 反滥用 + 合规底盘

**必须 S1 上线，不能省。**

- [ ] **E1** 拒测黑名单
  - 私有 IP 段（RFC1918）/ 政府 / 银行 / 友站列表
  - 接入 B2 限速中间件，拨测前校验目标
  - 后台可配置黑名单（初期写死配置文件）
  - *deps: B1* | *lane: E*

- [ ] **E2** 测试报告水印
  - 每条拨测结果写入：`node_id + task_id + target + timestamp` 签名
  - 水印可被追溯（abuse 举报时可还原来源）
  - *deps: C1* | *lane: E*

- [ ] **E3** 法律页面（静态页）
  - `/terms` 服务条款
  - `/privacy` 隐私政策（含 PIPL 合规条款）
  - `/aup` 可接受使用政策（明确禁止 DDoS 放大 / 端口扫描 / 漏洞探测）
  - Cookie 同意横幅（底部）
  - `/about` 关于页
  - *deps: D1* | *lane: E* | *[👤] 法律文本需人工起草或购买模板*

- [ ] **E4** 安全头 + CSRF
  - CSP / HSTS / X-Frame-Options / X-Content-Type-Options
  - Same-Site Cookie
  - CSRF Token（表单提交）
  - *deps: B1* | *lane: E*

- [ ] **E5** 可观测性接入
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

- [ ] **F1** 数据库扩展
  - `migrations/idcd_main/` 新增：`monitors` / `monitor_checks` / `probe_tasks` / `probe_results`（TimescaleDB hypertable）
  - `packages/db/repository/monitor.go`
  - *deps: A3* | *可并行*

- [ ] **F2** `apps/api/` — 监控 CRUD 接口
  - `POST/GET/PATCH/DELETE /v1/monitors`
  - 支持类型：HTTP / HTTPS / Ping / TCP / DNS / SSL 到期 / 域名到期 / ICP 备案变更 / 关键字
  - 监控配置：频率（1min/5min/30min）/ 节点选择 / 断言规则
  - 状态机：`active → paused → maintenance → archived`（见 STATE-MACHINES.md）
  - *deps: B1, F1* | *lane: F*

- [ ] **F3** 监控调度集成
  - Scheduler 从 PG 读取活跃监控项 → 定时下发拨测任务
  - Aggregator 收结果 → 判断 quorum（N 个节点失败才告警）→ 写 `monitor_checks`
  - 反误报：连续 N 次失败才变 DOWN；连续 M 次成功才恢复 UP
  - *deps: B6, B7, F1* | *lane: F*

- [ ] **F4** 控制台监控界面（`/app/monitors`）
  - 监控列表：名称 / 状态（UP/DOWN/PAUSED）/ 最后检查时间 / 可用率
  - 监控详情：趋势图（ECharts）/ 节点对比 / 历史事件
  - 新建监控向导：类型选择 → 配置 → 测试 → 保存
  - 批量操作（暂停 / 删除 / 导出 CSV）
  - 实时状态推送（SSE）
  - 维护窗口设置
  - *deps: D1, F2, F3* | *lane: F*

---

### M6 — 告警模块

- [ ] **G1** 数据库扩展
  - `alert_channels` / `alert_policies` / `alert_events` / `alert_notifications`
  - *deps: A3, F1* | *lane: G*

- [ ] **G2** `apps/notifier/` — 告警通道扩展（从邮件扩展到全通道）
  - 通道 adapter 各自独立 Go module，实现统一接口 `Send(payload) -> Result`
  - 邮件（已有）/ Webhook / 企业微信机器人 / 钉钉机器人 / 飞书机器人
  - 微信（自家服务号模板消息 + Server酱 fallback）/ Telegram Bot / Slack / Discord
  - 失败重试（asynq dead letter queue）
  - *deps: B8, G1* | *lane: G*

- [ ] **G3** `apps/api/` — 告警策略接口
  - `POST/GET/PATCH/DELETE /v1/alert-channels`（通道管理 + 测试发送）
  - `POST/GET/PATCH/DELETE /v1/alert-policies`（策略：延迟 N 分钟 / 升级 / 抑制 / 静音）
  - `GET /v1/alert-events`（历史事件）
  - Acknowledge / 解决（resolve）操作
  - *deps: B1, G1* | *lane: G*

- [ ] **G4** 控制台告警界面（`/app/alerts`）
  - 通道管理：添加 / 测试 / 删除各通道
  - 策略配置：规则绑定 / 升级策略 / 静音时段
  - 事件历史：时间线 / 通知记录 / 恢复时间
  - *deps: D1, G3* | *lane: G*

---

### M7 — 计费 + 状态页 + Evidence 准备

- [ ] **H1** 数据库扩展
  - `subscriptions` / `invoices` / `payments` / `status_pages` / `status_page_events`
  - *deps: A3, F1*

- [ ] **H2** Paddle 支付接入
  - Paddle SDK 集成（MoR 模式，含微信/支付宝 via Paddle）
  - `POST /v1/billing/subscribe`（订阅 Pro/Team/Business）
  - `POST /v1/billing/cancel` / `POST /v1/billing/upgrade`
  - Webhook 处理：`subscription.activated` / `subscription.cancelled` / `payment.succeeded` / `payment.failed`
  - 发票：Paddle 自动出具，`GET /v1/billing/invoices`
  - *deps: B1, H1* | *[👤] 需 Paddle 账号 + 海外主体注册*

- [ ] **H3** 配额执行
  - 订阅档位限制：监控数量 / 频率 / 节点数 / API 调用量
  - 超额提醒（邮件 + 控制台 banner）
  - 自动降级（超额后限制高频功能）
  - *deps: B1, H2* | *lane: H*

- [ ] **H4** `apps/status/` — 状态页（Next.js 多租户）
  - 子域：`<slug>.status.idcd.com`（泛域名 SSL，Cloudflare 管理）
  - ISR + CDN 缓存
  - 自定义域名（CNAME + ACME Let's Encrypt）
  - 状态页配置：服务分组 / 历史可用率（90/180/365 天）/ 事件公告
  - Free 档：页脚 `Powered by idcd` 水印；Pro 起去水印 + 自定义域
  - *deps: D1, F3* | *lane: H*

- [ ] **H5** 控制台计费 + 状态页界面
  - `/app/billing`：当前订阅 / 用量 / 发票列表 / 升降级
  - `/app/status-pages`：创建 / 编辑 / 绑定监控项
  - `/app/usage`：API 调用量 progress bar（REST API calls / MCP units 独立，D2）
  - *deps: D1, H2, H4* | *lane: H*

- [ ] **H6** 最小管理台（仅 2 个必要功能）
  - `apps/admin/` 节点健康看板（在线 / 离线 / 延迟分布）
  - `apps/admin/` refund_failed 看板（Paddle 退款失败订单，D5 要求）
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
