# 全量代码审查 — 2026-05-16

> 上线前自动化审查 Round 1。由 6 个并行 agent + Go/前端 build/test/lint 产出。
> 本文档：**已修复**清单（push 即可） + **未修复**清单（按 P0/P1/P2 排序，要逐项手工跟进）。

---

## 一、本轮已修复（Round 1）

均通过 build + test 验证。

### A. 路由挂载 — 上线 blocker 级机械错误

`apps/api/internal/server/server.go` 缺挂十几条路由，前端调用全部 404。

| Method | Path | handler | 影响 |
|---|---|---|---|
| GET  | /v1/probe/tasks/{taskId}    | `probeH.TaskResult`           | T1 — async 拨测结果查询；前端 SSE/poll 全失败 |
| POST | /v1/probe/mtr               | `probeH.MTR`                  | T12 — MTR 工具页 404 |
| POST | /v1/probe/smtp              | `probeH.SMTP`                 | T13 — SMTP 工具页 404 |
| POST | /v1/probe/ntp               | `probeH.NTP`                  | T14 — NTP 工具页 404 |
| POST | /v1/probe/speedtest         | `probeH.Speedtest`            | N4 — 带宽测速工具页 404 |
| GET  | /v1/info/rdns               | `infoH.RDNS`                  | T5 |
| GET  | /v1/info/mx                 | `infoH.MX`                    | T6 |
| GET  | /v1/info/spf                | `infoH.SPF`                   | T7 |
| GET  | /v1/info/dmarc              | `infoH.DMARC`                 | T8 |
| GET  | /v1/info/dkim               | `infoH.DKIM`                  | T9 |
| GET  | /v1/info/asn                | `infoH.ASN`                   | T10 |
| GET  | /v1/info/bgp                | `infoH.BGP`                   | T11 |
| POST | /v1/diagnose/reports        | `diagReportH.SaveReport`      | T2 — 一键诊断保存 |
| GET  | /v1/diagnose/reports/{id}   | `diagReportH.GetReport`       | T2 — 一键诊断查看 |
| DELETE | /v1/alert-groups/{id}     | `noiseH.DeleteGroup`          | 告警分组删除 |
| POST,GET | /internal/admin/upgrade-rollouts | `nodeUpgradeH.Create/List` | N3 — agent OTA 灰度 |
| PATCH | /internal/admin/upgrade-rollouts/{id} | `nodeUpgradeH.Update` | N3 |
| PATCH | /v1/admin/node-applications/{id}/review | `communityH.AdminUpdate` | K8 — 前端调 PATCH /review，缺别名 |

### B. SQL 表名 / 列名机械错误（5xx in production）

| 文件 | 位置 | 原错 | 修正 |
|---|---|---|---|
| admin.go | 84,92,210,218 | `FROM users` | `FROM "user"`（schema 是带引号的单数） |
| admin.go | 237,252,336 | `FROM users u` | `FROM "user" u` |
| admin.go | 113,120 | `FROM nodes` + `status='online'` | `FROM enrolled_nodes` + `status='active'` |
| beta_invitation.go | 209 | `FROM users` | `FROM "user"` |
| dashboard.go | 100 | `mc.checked_at` | `mc.check_at`（schema 列叫 check_at） |
| auth.go | 293 | `user.Status == "suspended"` | `user.Status != "active"`（schema 枚举是 active/locked/pending_deletion/deleted；"suspended" 不存在，locked 用户原来能登录） |
| admin_test.go |多处 | 测试 mock 的旧名 | 同步更新 |

### C. middleware / context key 错误

- `probe.go:238,244` 用 `r.Context().Value("user_id")` 裸 string key，但 middleware 用 `contextKey("user_id")` 自定义类型 → `probe_task.initiated_by/api_key_id` 列**永远** NULL，无法做用户审计 / 配额扣减。
  改为走 `middleware.UserIDFromContext(ctx)`。
- `aggregator/internal/config/config.go:89` `if err := c.Config.Database.Main.DSN; err == ""`（变量名 err 但赋值 string）；改为 `if c.Config.Database.Main.DSN == ""`。

### D. 跨进程 / 跨服务流水线

- **新增 migration** `lib/db/migrations/idcd_main/00038_probe_result.sql`：缺失的 hypertable，导致 aggregator 每条 probe 结果 INSERT 都失败 → 监控数据完全不落库。
- **新增** `apps/scheduler/internal/scheduler/monitorstore.go` + 在 main.go 接入 `PGMonitorStore` → `MonitorStore` 从未注入，monitorPoller 死循环跳过，**所有监控的定时拨测从未触发**。
- **aggregator dedup race**：`processor.go` 原来"先标 dedup，后写库"——transient DB 故障时 24h 窗内永远跳过；改为先 `IsDuplicate` 检查，写库成功后才 `MarkProcessed`。

### E. 安全 / SSRF

- `status_subscription.go:134` — `isPrivateIP(req.Endpoint)` 把整个 URL 传进 `net.ParseIP`（必返 nil），SSRF 校验完全 bypass；改为：IP 字面量 → 直接判断；hostname → 解析 + 逐 IP 检查私网 / metadata。
- `oauth.go:172,325` — OAuth 回调把 JWT 拼到 URL `?token=...`，会被 access log / Referer / 浏览器历史泄漏；改为只设 HttpOnly cookie + 不带 token 的 redirect。
- `gateway/handler/ws.go` — WebSocket 升级后**没有 `SetReadLimit`**，恶意 agent 单 frame 100MB 即可 OOM gateway；加 `SetReadLimit(64*1024)`。
- `auth.go setAuthCookie` — `SameSite=Strict` 在 OAuth 跨站回调时被丢弃；改 `SameSite=Lax`（仍防 CSRF，但允许 top-level navigation 带 cookie）。

### F. 配置加载

- `apps/notifier/cmd/notifier/main.go:20` — `config.MustLoad("config/dev.env.yaml")` 硬编码 dev 路径，生产容器永远读 dev；改为 `config.DefaultPath()`（honour `IDCD_CONFIG` 环境变量）。

### G. 前端 typecheck / 测试 5 个错

| 文件 | 修正 |
|---|---|
| `app/app/alerts/alerts-client.tsx:1178,1193` | `res.data.channels/policies` → `res.data.items`（与 type 声明 & 后端实际响应对齐，否则 alerts 页 17 个测试 fail + 上线后 channels/policies 永远空） |
| `i18n/getT.ts:25` | `replaceAll` → `replace(/regex/g, ...)`（lib 目标兼容） |
| `tsconfig.json` | `target: ES2017 / lib: es6` → `ES2022 / es2022`（消除 lib target 警告） |
| `i18n/__tests__/i18n.test.ts:61,65` | 类型 cast 加 `as unknown as ...` |
| `test/setup.ts:33` | value 类型补 undefined |
| `components/__tests__/nav.test.tsx` | mock fetch + waitFor — NavUserMenu 现在异步加载 profile |
| `app/app/alerts/__tests__/alerts.test.tsx` | mock fixture `channels`/`policies` → `items`（与 API 一致） |

### H. 前端 quick wins

- `app/layout.tsx` — 挂载 `<Toaster />`（之前 10+ 个文件调 `toast.error/.success` 全静默无效）。
- `app/admin/layout.tsx:28` 移除硬编码 `dark` className（违反 CLAUDE.md + next-themes 冲突）。
- `app/status/layout.tsx` 同上。
- `proxy.ts` CSP `connect-src` 从硬编码 `https://api.idcd.com` 改为读 `NEXT_PUBLIC_API_URL`（preview/staging 不再被 CSP 阻塞）。
- `auth/login/page.tsx` — DingTalk/飞书登录链接 `/api/v1/auth/*` → `${NEXT_PUBLIC_API_URL}/v1/auth/*`（之前指向不存在的 Next.js route → 404）。
- `auth/oauth-callback/page.tsx` — 删 localStorage 写入死代码（与 cookie 流冲突，本来就没人读）。
- `admin/page.tsx` — `NEXT_PUBLIC_ADMIN_TOKEN` 客户端泄露 → 改 server action `admin/actions.ts`。
- `auth.go authResponse` — Register/Login/MFA verify 三处 `AccessToken` 字段始终空；填上 JWT，非浏览器客户端（CLI / mobile / server-to-server）可用。

### I. 测试基线（修复后）

```
Go:        1655 个测试，全绿（apps/api 907 + scheduler 32 + aggregator 49 + gateway 46 + notifier 85 + agent 132 + mcp 21 + cli 11 + lib/auth 143 + lib/shared 107 + lib/ratelimit 13 + sdk-go 9）
Frontend:  739 个测试，全绿
Build:     全部 14 个 Go module + apps/web 都 Pass
Typecheck: 0 error
```

---

## 二、未修复（待 Round 2/3 跟进）

> 按"上线 blocker"程度排，每项标 P0/P1/P2。

### P0 — 上线**必须**修

#### 1. PAT / APIKey middleware 完全缺失（功能形同虚设）
- `apps/api/internal/handler/pat_handler.go` 能 Create/List/Delete personal_access_token，但 `internal/middleware/authn.go` 没有任何分支检查 `idcd_pat_` 前缀；用户创建的 PAT 完全用不了。
- `apikey_handler.go` 同样：能造 API key 但没有 middleware 拿来鉴权。
- CLI / server-to-server / 第三方集成全部死路。
- **影响范围**：所有"用 API key 调 idcd"的场景 0 工作。

#### 2. ACME 客户端死代码（自定义域名状态页拿不到证书）
- `apps/api/internal/acme/manager.go` 完整实现 + 单测 OK，但 `cmd/api/main.go` 和 `server.go` 完全没 import。
- 状态页绑定 `xxx.example.com` 自定义域名 → 永远拿不到证书。
- **影响范围**：M11 自定义域名状态页（K8 ✅ 标完成但实际 broken）。

#### 3. D5 退款重试链路（Paddle webhook → 5min/30min retry）
- `billing.go:442` webhook 收到 `refund_failed` 只把状态写库，**没入 asynq queue 重试**。
- `admin_billing.go RetryRefund` 直接置 `refunded` 但不调 Paddle —— 对账撕裂。
- DECISIONS §D5 锁定决策完全未实施。

#### 4. CSRF 双提交 token 跨域不工作
- 后端 set `csrf_token` cookie 只在 `api.idcd.com` host（host-only）。
- 前端 `idcd.com` 的 `document.cookie` 读不到 → `X-CSRF-Token` 头永远空。
- 后端 CSRF 中间件要么放行（=防御失效），要么拦截（=每个 mutating 请求都 403）。
- 修法：后端把 `csrf_token` 显式 `Domain=.idcd.com` 写入（不是 HttpOnly，前端可读）；并复核所有 CSRF 测试 fixture。

#### 5. CORS 凭据 + Allow-Origin echo 未配
- 前端 `idcd.com` 跨域 fetch `api.idcd.com` 带 `credentials: 'include'`。
- 后端必须返 `Access-Control-Allow-Credentials: true` + `Access-Control-Allow-Origin: https://idcd.com`（不能 `*`）。
- 现在的 middleware/cors 实现需要逐一确认。

#### 6. agent SSRF 目标过滤（攻击面：cloud metadata）
- `apps/agent/cmd/agent/main.go:317-329` 收到 task 后直接传 `task.Target` 给 net.Lookup / HTTP / SMTP。
- 失陷的 gateway / 恶意管理员可让所有 agent 同时扫 `169.254.169.254`（AWS/GCP/阿里云 metadata token）。
- 修法：新建 `lib/shared/netfilter`，在 agent 探测入口拒绝私网/metadata；速度测试增加目标白名单。
- 同时 `apps/api/internal/handler/info.go` DNS 类端点也需走 denylist（公开 endpoint 接受任意域名查询）。

#### 7. MCP server 完全没鉴权（D13 锁定决策违反）
- `apps/mcp/internal/protocol/server.go` SSE + /messages 没 Authorization 检查。
- 任何人都能 POST /messages 调 tools/call → 走共享 `IDCD_API_KEY` → 租户隔离丢失。
- `Access-Control-Allow-Origin: *` + 无 CSRF token → XSRF。
- `io.ReadAll(r.Body)` 无大小限制。
- **建议**：v1 上线如不暴露 MCP 服务可推迟，但不应公开访问。

#### 8. Scheduler 死代码 + leader 切换 race
- `apps/scheduler/internal/scheduler/scheduler.go:155` workers 从 `scheduler:tasks` ZSET 取任务，**没有任何 producer 写入这个 key** → workers 永远空转。
- `monitorPoller` 已通过本轮修复接到 DB（见 D 节），但通用的 ad-hoc tool probe 仍直接走 `probe.tasks` stream，绕过 scheduler。
- leader 切换：`renewLeadership` 失败时 `return`，workers 仍按 1Hz ticker 跑 30s 后才察觉 → split-brain 窗口。
- 决策：要么把 api/probe handler 改成 enqueue 到 scheduler:tasks（保留 P0–P5 优先级），要么把 worker pool + queue.Queue 删掉。

#### 9. enrolled_nodes 生命周期 + ad-hoc probe 选不到节点
- `node_enrollment_handler.go:185` 插入 status='pending'。
- 没有自动化路径把 pending 变 active（实际靠 agent 连接时 gateway 设 active —— 但路径不显式）。
- `probe.go:145-148, 200-203` `_ = pool.QueryRow(...).Scan(&nodeID)` 静默丢错，没有 active 节点时返空 → task 入流但永无 node 接收 → 前端永远 pending。
- 修法：probe handler 在 `nodeID == ""` 时 503；同时显式 enrollment → activate 流程或文档说明。

#### 10. WebAuthn 完全没有真实 CBOR 签名验证
- `lib/auth/webauthn/webauthn.go:102-148` `ParseAttestationResponse` 把 rawID 当公钥存（`"pk:" + rawID`）。
- 后续 `ParseAssertionResponse` 也没真做 ES256/RS256 验签 → 等价于"持有 credentialID 即可登录"。
- 修法：替换实现（go-webauthn/webauthn 或 fxamacker/cbor 自解 attestationObject）；S2 前如不下线该入口必须修。

### P1 — 上线后第一周必修

#### 11. JWT refresh 无 jti / replay 防护
- `lib/auth/jwt/jwt.go:144` Refresh 直接基于旧 claims 签新 token，没把旧 jti 加黑名单。
- token 泄露后无法吊销旧链。

#### 12. session refresh 非原子 + 静默失败
- `lib/auth/session/session.go:104-111` Get 内的 LastSeenAt 写回 race + 写失败匿名吞掉。

#### 13. Argon2 / bcrypt 参数
- 密码 hash 存储格式没 PHC 元数据，未来调整参数会导致全表 rehash。
- maxLength=72 是 bcrypt 时代遗留，Argon2 不需要。

#### 14. notifier 退避不符合 D5
- `apps/notifier/internal/worker/worker.go:49-59` 通用 1s/4s/16s/cap 60s 退避，不匹配 5min/30min 节奏。
- SES sender 完全是 stub —— 配 SES 不配 SMTP 邮件全失败。
- email 模板 `p.URL` 未 escape → href javascript: 风险。

#### 15. aggregator scaling broken
- `consumer.go` 硬编码 `ConsumerName="aggregator-0"`，多副本部署 XREADGROUP 当成一个 consumer，消息分发错乱；`cfg.ConsumerCount` 读了但没用。
- reclaim 只在启动时跑一次，无周期性 reclaim + DLQ。
- poison message 永远在 PEL 转。

#### 16. Cloudflare Workers 部署兼容
- `wrangler.toml` 缺 `[env.production]` 段，`wrangler deploy --env production` 行为不可预期。
- Workers env vars：`INTERNAL_API_URL` / `ADMIN_TOKEN` 没在 wrangler.toml 配置，admin 服务端组件全部 fallback localhost。
- 诊断 SSE route 在 Workers 5min CPU 限制下行为不明（`app/api/diagnose/stream/route.ts` 长轮询 60s）。
- `infra/nginx/nginx.conf:47` `proxy_pass http://127.0.0.1:8080` —— nginx 在容器里，127.0.0.1 是 nginx 容器本身不是 api 容器；上线后全站 502。

#### 17. docker-compose -config flag 不识别
- `infra/docker/docker-compose.prod.yml` 用 `command: ... -config /config/prod.env.yaml` 启动各服务。
- main.go 都没 flag.Parse —— flag 完全被忽略；api/aggregator/scheduler 走 `config.DefaultPath()` → fallback dev.env.yaml → fatal。
- 修法：每个服务 environment 块加 `IDCD_CONFIG=/config/prod.env.yaml`；或加 flag 解析。

#### 18. Dockerfile 全部以 root 跑 + 无 HEALTHCHECK
- 6 个服务的 Dockerfile 都没 `USER`；容器逃逸 = 宿主 root。
- 只有 api 有 healthcheck，其他全裸；compose 不会重启僵尸进程。

#### 19. 4/6 backend 服务没暴露 /metrics
- `aggregator/notifier/scheduler/gateway` config 里有 `prometheus_port`，main.go 完全没起 metrics listener；Grafana 抓不到。

### P2 — 长期改进

#### 20. WebSocket 单 agent 多连接覆盖
- `hub/hub.go:69` Register 覆盖旧 Connection 但不 Close 旧的；attacker 用泄露 token 反复连可让正节点失联。
- 修法：generation token + 旧 readPump 检测后退出。

#### 21. 响应 shape 不统一
- 各 handler 有的返 `{items:[]}`、有的裸数组、有的 `{channels:[]}`/`{policies:[]}` 等等；前端 `.data.items` 假设处处失败。
- 上面已修了 alerts 这一块，其他可统一规范走一遍。

#### 22. lib/db 缺 TxRunner / WithTx
- multi-step 操作没事务边界，user.Create + audit_log.Create 半成功记录。

#### 23. lib/ratelimit Lua member 用 math.random
- 高并发偶尔碰撞，少计一次 → SLO 漂移。

#### 24. 多项 OpenAPI spec 中的 endpoint 完全没实现（v2 模块）
- `/verdict/*` `/attest/*` `/compliance/*` `/mcp/tokens` `/agent-obs/monitors` `/agent-obs/events` `/heartbeat/{token}` `/plans` `/webhook-endpoints` `/leaderboard/optout` — Verdict/Attest/Compliance/MCP token 五大 v2 模块零代码。
- 见 `docs/prd/16-api-spec.yaml` vs `apps/api/internal/server/server.go`。
- S2 路线图，S1 上线可不阻塞，但需 stakeholder 对齐。

### 文档不一致 / 过时（独立 PR 清理）

- `docs/ARCHITECTURE.md` 描述了 `apps/admin/` `apps/status/` `apps/docs/` `apps/attest-*` `apps/mcp-server` `packages/kms/` `packages/tsa/` `packages/llm/` … 一堆**不存在的目录**；实际 apps/ 只有 9 个，packages/ 只有 2 个。
- `TASKS.md` 多项标 `[x]` 但实际由于上面 A 节路由未挂、B 节 SQL 错而不可用（T1-T14、N3、N4、K8）。
- `README.md` 不存在（ARCHITECTURE.md 列了）。
- `CHANGELOG.md` 停留在 A1/A2（2026-05-13），后续 100+ 个 task 完全没记录。
- `infra/nginx/nginx.conf` 只配 api.idcd.com，缺 gateway/status-page custom domain/admin 子域。

### 部署缺失清单

- 无 TLS 证书自动化（certbot/acme.sh），nginx.conf 期望手放证书。
- 无 DB backup（pg_dump → S3/OSS 定时）；丢盘 = 数据全无。
- 无 Sentry / 错误聚合客户端。
- 无 Loki promtail / grafana-agent；compose.core 起了 Loki 但没 producer。
- 无 image tag pinning；compose 用 `:latest` 不能 rollback。
- 无 `infra/terraform/`；DNS/Cloudflare/服务器 配置全手动。
- 无 WAF / DDoS rate limit at edge。

---

## 三、Round 2 建议（下一轮跑）

1. **修 P0 #1**（PAT/APIKey middleware 完全缺失）— 把 PAT/APIKey 鉴权分支加进 `middleware/authn.go`，prefix → SHA256 → DB lookup → 注入 context。
2. **修 P0 #6**（agent SSRF）— 写 `lib/shared/netfilter`，所有 probe 入口共用。
3. **修 P0 #5**（CORS allow-origin echo + credentials）— 单测 + 真实跨域验证。
4. **修 P0 #4**（CSRF token Domain=.idcd.com）。
5. **修 P0 #2**（ACME 接线）— 让自定义域名状态页能拿证书。

完成后再跑一遍本轮的并行 agent + Codex 对抗 review 才能称"可上线"。
