# 代码审查修复计划

> 生成日期：2026-05-13  
> 覆盖范围：提交 B4 / C5 / D3–D7 / E1 / E4 / E5  
> 来源：安全审查 + Go 后端审查 + 前端 TypeScript 审查（三路并行）

---

## 总体评级

| 维度 | 评级 | 核心问题 |
|---|---|---|
| 安全 | **HIGH RISK — 不可上线** | 4 Critical：账号接管链 + SSRF + 无限流 |
| Go 后端 | **BLOCK** | DNS rebinding + WS 无来源验证 + nil panic |
| 前端 | **构建损坏** | lucide-react Github 导入不存在 |

---

## Phase 1 — 紧急（当天修复，阻断任何公开流量）

### F1-1 构建损坏：删除不存在的 `Github` 图标导入
- **文件**：`apps/web/src/app/about/page.tsx:4`
- **问题**：lucide-react v1.14 已移除 `Github`，TypeScript 编译失败
- **修复**：改用 `react-icons/fa` 的 `FaGithub`，或改用纯文字链接

### F1-2 账号接管链（三个问题必须同步修复）

**F1-2a SEC-C3：ForgotPassword 把 `otp_id` 直接返回给调用方**
- **文件**：`apps/api/internal/handler/auth.go:299`
- **风险**：攻击者知道任意邮箱 → 获取 otp_id → 爆破 100 万种 6 位 OTP（无限流保护）
- **修复**：响应体中删除 `otp_id` 字段；otp_id 仅随 OTP code 一起发到邮件，前端从邮件中获取

**F1-2b SEC-C4：认证接口零速率限制**
- **文件**：`apps/api/internal/server/server.go:127`
- **风险**：`RateLimit` 中间件已实现但从未注册到 `/v1/auth` 路由
- **修复**：在 `/v1/auth` subrouter 注册 `middleware.RateLimit`，策略：`forgot-password`/`login` 5 次/IP/分钟

**F1-2c SEC-C2：Logout 不撤销 session，JWT 退出后仍有效 24 小时**
- **文件**：`apps/api/internal/handler/auth.go:209`
- **修复**：从 JWT claims 中取 `SessionID`，调用 `sessSvc.Delete(ctx, claims.SessionID)`

### F1-3 DNS rebinding SSRF（denylist TOCTOU）
- **文件**：`apps/api/internal/denylist/denylist.go:68`
- **风险**：攻击者控制 DNS TTL → 第一次解析返回公网 IP 通过校验 → 第二次解析返回 `169.254.169.254`
- **修复（两处）**：
  1. `resolveHost` 改为检查**所有**返回 IP，不只 `ips[0]`
  2. probe agent 直接 dial 预验证的 IP，不再做第二次 DNS 解析

### F1-4 WebSocket CheckOrigin 永远返回 true
- **文件**：`apps/gateway/internal/handler/ws.go:20`
- **风险**：任意网站可发起跨站 WebSocket 劫持
- **修复**：改为白名单校验 `origin == "https://idcd.com" || origin == "https://app.idcd.com"`

### F1-5 `/v1/info/ssl` 无 SSRF 校验直接 TLS dial
- **文件**：`apps/api/internal/handler/info.go:327`
- **风险**：用户传入内网 IP → 服务器探测内网主机（端口扫描）
- **修复**：在 `tls.DialWithDialer` 前调用 `denylist.CheckTarget(query)`；DNS、Whois、ICP handler 同理

### F1-6 Telemetry 错误被吞，defer 调用 nil 函数导致 panic
- **文件**：所有 `cmd/*/main.go`（aggregator, api, gateway, notifier, scheduler）:35–45
- **风险**：`Init` 失败后 `shutdownTelemetry` 为 nil，进程退出时 panic
- **修复**：`Init` 失败改为 `log.Fatalf`，或保证 `Init` 失败时返回 no-op 函数

---

## Phase 2 — 高优先级（本周内，上线前必须完成）

### F2-1 JWT 存 localStorage，XSS 即可窃取
- **文件**：`apps/web/src/app/auth/login/page.tsx:55`，`apps/web/src/lib/api.ts:4`
- **风险**：任意 XSS → token 被盗 → 结合 F1-2c 的修复前：退出无效，24 小时窗口
- **修复**：改为 `HttpOnly; Secure; SameSite=Strict` cookie，access token 用内存存储 + HttpOnly refresh-token

### F2-2 CSRF token 非常数时间比较
- **文件**：`apps/api/internal/middleware/csrf.go:107`
- **修复**：`subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) == 1`

### F2-3 CSP 仅 Report-Only + 含 unsafe-inline/eval
- **文件**：`apps/api/internal/middleware/security_headers.go:11`
- **修复**：
  1. 同时下发 `Content-Security-Policy`（强制）和 `Content-Security-Policy-Report-Only`
  2. 移除 `unsafe-inline` / `unsafe-eval`，改用 nonce-based CSP（Next.js middleware 支持）

### F2-4 JSON 拼接注入
- **文件**：`apps/gateway/internal/handler/ws.go:251`
- **修复**：`inner, _ := json.Marshal(map[string]string{"task_id": result.TaskID})`；禁止字符串拼接构造 JSON

### F2-5 `info.go` 的 isPrivateIP 比 denylist 版本弱
- **文件**：`apps/api/internal/handler/info.go:405`
- **修复**：提取 `denylist` 包中的 `isPrivateIP` 为共享逻辑，删除 `info.go` 中的重复实现

### F2-6 Prometheus MustRegister 在测试中重复注册 panic
- **文件**：`apps/api/internal/server/server.go:80`
- **修复**：使用 `prometheus.NewRegistry()` 注入，或改 `Register` 并处理 `AlreadyRegisteredError`

### F2-7 Telemetry shutdown 使用无超时 context，进程无法正常退出
- **文件**：所有 `cmd/*/main.go:55`
- **修复**：`shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)`

### F2-8 Cookie Banner X 按钮不写入同意记录（GDPR 违规）
- **文件**：`apps/web/src/components/cookie-banner.tsx:61,69`
- **风险**：PIPL/GDPR 合规要求关闭行为等同于拒绝，必须持久化
- **修复**：X 按钮调用 `handleConsent("essential")`；移除链接 onClick 中的 `setIsVisible(false)`

### F2-9 X-Forwarded-For 盲信，绕过速率限制
- **文件**：`apps/api/internal/middleware/rate_limit.go:82`，`handler/probe.go:261`
- **修复**：仅当 `r.RemoteAddr` 属于可信代理 CIDR 时才信任 `X-Forwarded-For`；使用最右侧可信 IP

### F2-10 `/metrics` 无认证暴露在公网端口
- **文件**：`apps/api/internal/server/server.go:108`
- **修复**：改为独立内部端口（如 `:9090`），或添加 IP 白名单/共享密钥中间件

---

## Phase 3 — 合规 & 中优先级（上线 GA 前）

### F3-1 法律页面含"待法务审核"占位符已上线（PIPL 合规风险）
- **文件**：`apps/web/src/app/privacy/page.tsx`、`terms/page.tsx`、`aup/page.tsx`
- **问题**：ICP 备案号 `京ICP备XXXXXXXX号` 为假号；隐私政策明文写"待填充"
- **修复**：填充真实法律文本后上线，或上线前设置 noindex + 前端路由保护

### F3-2 OTP 用裸 SHA-256（无盐、无 HMAC key）
- **文件**：`apps/api/internal/handler/auth.go:439`
- **修复**：改用 `HMAC-SHA256(serverSecret, code)` 或 bcrypt；6 位纯数字彩虹表可秒破

### F3-3 ForgotPassword timing side-channel 泄露邮箱是否注册
- **文件**：`apps/api/internal/handler/auth.go:284`
- **修复**：邮箱不存在时 sleep 等同于 `issueOTP` 的耗时，保持响应时间一致

### F3-4 localStorage 类型断言不校验（cookie-banner）
- **文件**：`apps/web/src/components/cookie-banner.tsx:19`
- **修复**：
  ```ts
  const valid: ConsentType[] = ["all", "essential"]
  const parsed = valid.includes(savedConsent as ConsentType) ? (savedConsent as ConsentType) : null
  ```

### F3-5 AlertTitle ref 类型声明错误
- **文件**：`packages/ui/src/components/alert.tsx:37`
- **修复**：`HTMLParagraphElement` 改为 `HTMLHeadingElement`

### F3-6 CORS Allow-Credentials 无条件下发
- **文件**：`apps/api/internal/middleware/cors.go:31`
- **修复**：将 `Access-Control-Allow-Credentials: true` 移入 origin 校验通过的分支

### F3-7 OTel 所有服务使用硬编码空 endpoint，telemetry 配置不走 config
- **文件**：aggregator/gateway/notifier/scheduler/agent 的 `main.go`
- **修复**：从各服务的 config.go 读取 `OTLPEndpoint` 和 `Enabled`

### F3-8 deprecated OTel semconv 属性名
- **文件**：`packages/shared/telemetry/telemetry.go:101`
- **修复**：引入 `semconv/v1.21.0`，替换 `http.method` → `http.request.method` 等

### F3-9 页脚内部链接使用 `<a>` 而非 `<Link>`
- **文件**：`apps/web/src/components/footer.tsx`
- **修复**：`import Link from "next/link"`，替换所有内部 `<a href>`

### F3-10 initSentry 导出但从未调用（死代码）
- **文件**：`apps/web/src/lib/sentry.ts`
- **修复**：在 layout.tsx 通过 Client Component wrapper 调用，或删除文件等 S2

### F3-11 新增页面缺少 canonical URL
- **文件**：`about/aup/privacy/terms/page.tsx`
- **修复**：各页面 Metadata 添加 `alternates: { canonical: "https://idcd.com/xxx" }`

---

## Phase 4 — 低优先级（GA 后迭代）

| ID | 问题 | 文件 |
|---|---|---|
| F4-1 | denylist CIDR 每次调用重新 ParseCIDR，热路径性能浪费 | `denylist/denylist.go:100` |
| F4-2 | `blocklist.txt` 提交但从未被读取 | `denylist/blocklist.txt` |
| F4-3 | `go.mod` 声明 `go 1.26`（不存在的版本） | `shared/go.mod:3` |
| F4-4 | `t.Sub(time.Now())` 改为 `time.Until(t)` | `middleware/rate_limit.go:72` |
| F4-5 | 测试用裸 string 做 context key（SA1029） | `handler/probe_test.go:73` |
| F4-6 | responseWriter 在 server.go 和 telemetry.go 中重复定义 | 两处 |
| F4-7 | 页脚"隐私"链接重复出现两次 | `footer.tsx` |
| F4-8 | Sentry beforeSend 缺少 PII 过滤钩子（S2 激活前添加） | `lib/sentry.ts` |
| F4-9 | Cookie Banner 出现延迟硬编码 1 秒 | `cookie-banner.tsx:23` |
| F4-10 | User-Agent 未净化直接写入结构化日志 | `middleware/logger.go:58` |
| F4-11 | Permissions-Policy 缺少 interest-cohort / browsing-topics | `next.config.ts` |

---

## 修复执行顺序建议

```
Phase 1（今天）→ Phase 2（本周）→ Phase 3（下周 / 灰度前）→ Phase 4（迭代）
```

Phase 1 中 **F1-2（三件套）必须同步提交**，单独修复任何一个都不能阻断账号接管链。

---

## 问题数量汇总

| 来源 | Critical | High | Medium | Low |
|---|---|---|---|---|
| 安全审查 | 4 | 7 | 7 | 4 |
| Go 后端 | 3 | 4 | 7 | 4 |
| 前端 TS | 0 | 5 | 7 | 3 |
| **合计（去重后）** | **6** | **13** | **16** | **9** |
