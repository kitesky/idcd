# Changelog

All notable changes to idcd will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Security
- **P0#1** PAT / API key 鉴权中间件（`Authn` → `AuthnWithTokens`）+ PAT/APIKey/BillingEnqueuer repo 实现 + server.go 全量 wire（9f9bde2 / 565a398）
- **P0#4** CSRF cookie `Domain=.idcd.com` env 驱动 + `SameSite` Strict → Lax（9f9bde2）
- **P0#5** CORS 移除 `*` fallback + Allow-Credentials + 严格 origin echo + `Vary: Origin`（9f9bde2）
- **P0#6** SSRF `lib/shared/netfilter` 拦截私网 + cloud metadata (`169.254.169.254` / `100.100.100.200` / `192.0.0.192` / `fd00:ec2::254`)，agent task 与 DNS 公开端点接入（9f9bde2）
- **P0#7** MCP server Bearer 鉴权 + CORS allowlist + 1MiB body 上限；新增 `auth_pg.go` DB-backed TokenValidator（v1 fallback PAT 表，v2-S3 切 mcp_token）（9f9bde2 / 565a398）
- **P0#10** WebAuthn 换 `github.com/go-webauthn/webauthn` v0.13.4 真做 ES256/RS256/EdDSA 签名验证 + clientData/origin/RPID/replay 完整检查；handler 切 `NewVerifier.VerifyAttestation`/`VerifyAssertion`（9f9bde2 / 565a398 / cee1310）
- **P1#11** JWT jti 黑名单（`InMemoryBlocklist` + `RedisBlocklist`），Refresh 自动拉黑旧 jti，Verify fail-closed（ec736a1 / 0d053d8 wire）
- **P1#13** 密码 hash 改 Argon2id PHC 格式（`$argon2id$v=19$m=,t=,p=$salt$hash`）+ `NeedsRehash` helper + maxLength 72 → 1024（ec736a1）
- **P1#14** Email href 仅允 http/https scheme + 二次 html-escape（ec736a1）
- Round 1 SSRF：status_subscription `isPrivateIP` 真做 hostname 解析；OAuth 回调不再 URL `?token=` 泄漏；WS `SetReadLimit(64KB)` 防 OOM（be1066e）

### Added
- **P0#2** ACME 接线 `/.well-known/acme-challenge/{token}` HTTP-01 + autocert，env-gated（9f9bde2）
- **P0#3** D5 退款重试链路：webhook 失败 → asynq 5min/30min retry → max 后道歉邮件；notifier worker handler + PaymentHub 适配器（9f9bde2 / 565a398）
- **P0#9** Probe handler 无 active 节点显式 503 + admin `POST /v1/admin/nodes/{id}/activate` + 完整状态机文档（9f9bde2）
- **P1#15** Aggregator scaling：ConsumerName 动态化 `{HOSTNAME}-{idx}`、周期 1min XAUTOCLAIM、`delivery_count > 5` 入 DLQ stream `{name}:dlq`、`/metrics` 五个 metric（ec736a1）
- **P1#16** Cloudflare Workers `[env.staging]` / `[env.production]` + secrets 注释；nginx upstream `api:8080` + admin/cname 子域 + http2（ec736a1）
- **P1#19** Notifier / Scheduler / Gateway `/metrics` listener（端口 9092/9093/9094）+ 业务 metric（ec736a1）
- **P2#20** Gateway WebSocket Register 加 `generation` 计数 + `pendingSkip` 吸收 stale Unregister，防泄漏 token 反复连断正节点（0d053d8）
- **P2#22** `lib/db/tx.go` `WithTx` helper + 自动 SAVEPOINT 嵌套 + ctx-aware rollback + panic 安全（0d053d8）
- **F1** SES sender 实装（aws-sdk-go-v2/sesv2 v1.60.4）+ 错误分类（transient → retry / permanent → 不重试）（0d053d8）
- **F2** server.go `buildJWTService` wire `RedisBlocklist` 进 JWT service（0d053d8）

### Changed
- **P0#8** Scheduler 删 worker pool + `queue.Queue`（无 producer 死代码）；leader 失锁 cancel workCtx，monitor poller 1s 内退；`cfg.Worker.Count` 配置一并清理（9f9bde2 / ec736a1）
- **P1#12** Session `Get` 的 LastSeenAt 写回从 TTL+Set 两步改 `SetArgs(XX, KEEPTTL)` 原子单命令；写失败 WARN log 不再静默（ec736a1）
- **P1#17** docker-compose.prod env 加 `IDCD_CONFIG=/config/prod.env.yaml` 替代被忽略的 `-config` flag（ec736a1）
- **P1#18** 6 个 Dockerfile 改 rootless `USER 10001:10001` + HEALTHCHECK（ec736a1）
- **P2#23** Ratelimit Lua 去 `math.random`，改 `INCR seq_key` 取 Redis 原子单调 counter，ZSET member 零碰撞（0d053d8）
- WebAuthn handler 加 `WithOrigins(CORSOrigins)` 让 dev 环境支持多 origin（cee1310）

### Fixed
- **Round 1** 路由挂载：`/v1/probe/tasks/{taskId}`、`/v1/probe/{mtr,smtp,ntp,speedtest}`、`/v1/info/{rdns,mx,spf,dmarc,dkim,asn,bgp}`、`/v1/diagnose/reports`、`/v1/admin/upgrade-rollouts` 等 17 条 endpoint 之前 404（be1066e）
- **Round 1** SQL 机械错：`users` → `"user"`、`nodes`+`status='online'` → `enrolled_nodes`+`status='active'`、`mc.checked_at` → `check_at`、auth `user.Status != "active"` 替代不存在的 `suspended`（be1066e）
- **Round 1** Probe context key 走 `middleware.UserIDFromContext`（之前 `r.Context().Value("user_id")` 裸 string 拿不到，导致 `initiated_by` 永远 NULL）（be1066e）
- **Round 1** aggregator dedup race：先 `IsDuplicate` 后 `MarkProcessed`（之前 transient DB 故障会让 24h 窗内跳过）（be1066e）
- **Round 1** 新增 `00038_probe_result.sql` hypertable migration + scheduler `PGMonitorStore` 接线（之前 monitor poller 拿不到 store，定时拨测从未触发）（be1066e）
- **Round 1** notifier `MustLoad("config/dev.env.yaml")` 硬编码 → `config.DefaultPath()`（be1066e）
- 前端 alerts 页 `res.data.{channels,policies}` → `res.data.items` 跟后端实际响应对齐（be1066e）
- `app/layout.tsx` 挂 `<Toaster />`（之前 10+ 文件调 `toast.*` 全静默）（be1066e）
- 移除 `admin/layout.tsx` / `status/layout.tsx` 硬编码 `dark` className（违 CLAUDE.md + 跟 next-themes 冲突）（be1066e）

### i18n（并行 session，2026-05-15→16）
- Phase 1: 后端 catalog + middleware + errcode 基础设施（8621070）
- Phase 2a/2b/2c: 后端错误消息全量 i18n + notifier 邮件模板双语化 + DB migrations + wiring（78c974f / 057ede8 / 7d0a4fa）
- Phase 3: 前端协议对齐 — locale 短码 cn/en + registry 驱动（384a5c0）
- Phase 4b/4c: 认证页 / 管理后台 / 公开页 / 主导航 cn+en 完整 + SEO hreflang/sitemap（7737eb9 / 5589812）

### Deferred
- **P2#21** Response shape 统一（跨 20+ handler，前端联动单独 PR + Codex 审）
- **P2#24** v2 模块 OpenAPI gap：verdict / attest / compliance / mcp_token 五大 v2 模块，按 S2-S3 路线图
- ARCHITECTURE.md 列了不存在目录、README 缺、ENG-REVIEW 中部分 D 决策未 audit
- 部署补丁：TLS 证书自动化（certbot/acme.sh）、DB backup（pg_dump → S3/OSS）、Sentry、Loki promtail、image tag pinning、Terraform、edge WAF

## [0.1.0-dev] - 2026-05-13

### Added
- Initial PRD, architecture docs
- Engineering review (D1-D21 decisions)
- TASKS.md planning board
- Monorepo skeleton: go.work + pnpm-workspace + Makefile (A2)
- Dev config system: config/dev.env.yaml (gitignored) + example (A1)
- Docker compose for production observability stack (A1)
- Directory structure for all apps and packages
