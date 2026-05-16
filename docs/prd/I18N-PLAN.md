# idcd 国际化（i18n）落地计划

> 版本：v3.0 · 2026-05-16
> 状态：决策已锁定，待执行
> 上一版：v1.0（已废弃，单文件 JSON / 后端不做 i18n / 二元判断）

---

## 决策摘要（已锁定）

| # | 决策 | 选择 |
|---|---|---|
| D1 | URL 策略 | 默认中文不带前缀，英文带 `/en/` 前缀（`localePrefix: 'as-needed'`） |
| D2 | Admin 后台 | 架构走 i18n key，但**不交付 EN 翻译**，缺失 key 走 fallback 链 |
| D3 | MCP / API | **需要翻译**，按请求 locale 返回错误消息（cn/en） |
| D4 | 复数处理 | 启用 next-intl ICU MessageFormat，后端 catalog 同步支持 |
| D5 | 状态页 | `status_page` 表新增 `default_locale` 字段，owner 可指定 |
| D6 | 持久化 | 本文档作为 SSOT，所有 Phase 引用此处 |
| D7 | Locale code | 内部短码 `cn` / `en`，registry 同步标准 BCP 47 副字段 (`zh-CN` / `en-US`) |
| D8 | 后端 i18n | 自写极简 catalog（~150 行），不引入 `go-i18n` |
| D9 | 初始范围 | S1 上线只交付 cn + en，但所有基建按 N 种语言设计 |

**核心原则**：未来加一种语言 = 在 registry 加 1 行 + 添加 messages 目录，**零代码改动**。任何 `if locale === 'en'` / `locale === 'cn' ? a : b` / `locale.startsWith('en')` 类二元判断 = CI 失败。

---

## 一、现状诊断

### 1.1 已有基础（可复用）

| 资产 | 文件 | 说明 |
|---|---|---|
| 路由 / middleware | `apps/web/src/proxy.ts` | `/en/*` rewrite + 注入 `x-locale: en` header |
| 翻译目录 | `apps/web/src/i18n/messages/{zh,en}/*.json` | 16 namespace，需重命名 `zh` → `cn` |
| 配置 | `apps/web/src/i18n/routing.ts` | locales=['zh','en']，需改 ['cn','en'] |
| 客户端 hook | `useTranslations()` | next-intl 3.26.0 已接入 |
| 服务端 helper | `apps/web/src/i18n/getT.ts` | 服务端组件按 namespace 加载 |
| 语言切换 UI | `apps/web/src/components/nav.tsx:304-391` LangToggle | URL 前缀 + cookie |
| Tool SEO 元数据 | `apps/web/src/i18n/en-tools-meta.ts` | 21 工具 title/description/schemaName |

### 1.2 缺口

| 缺口 | 规模 | 优先级 |
|---|---|---|
| Locale Registry（单一真实源） | 0 → 1 文件 | P0 |
| 后端 i18n catalog（API + notifier） | 0 → ~150 行 | P0 |
| 前后端 errcode 体系统一 | ~123 条 | P0 |
| 前端硬编码中文清理 | 212 文件 / ~2690 条 | P0~P2 |
| 后端硬编码字符串清理 | 24 文件 / ~123 条 | P0 |
| 邮件模板双语化 | 6+ 模板 × 2 | P0 |
| SEO hreflang + sitemap 多 locale | 全公开页 | P1 |
| CI lint（key 完整性 + 二元判断检测 + binary map 检测） | 0 → 1 脚本 | P1 |
| 加新语言操作手册 | 0 → 1 文档 | P2 |

---

## 二、核心架构

### 2.1 Locale Registry（单一真实源）

**`config/locales.json`** —— 前后端启动时同时读取，编译期 generate type-safe 常量。

```json
{
  "default": "cn",
  "locales": [
    {
      "code": "cn",
      "bcp47": "zh-CN",
      "label": "简体中文",
      "nativeLabel": "简体中文",
      "baseLanguage": "zh",
      "acceptLanguageAliases": ["zh", "zh-CN", "zh-Hans", "zh-SG"],
      "dir": "ltr",
      "fontStack": "cjk",
      "fallback": []
    },
    {
      "code": "en",
      "bcp47": "en-US",
      "label": "English",
      "nativeLabel": "English",
      "baseLanguage": "en",
      "acceptLanguageAliases": ["en", "en-US", "en-GB", "en-AU"],
      "dir": "ltr",
      "fontStack": "latin",
      "fallback": []
    }
  ]
}
```

字段语义：
- `code`：内部短码，URL / API / 数据库存储用
- `bcp47`：标准代码，用于 HTML `lang` / `hreflang` / `Intl.*Format` / Accept-Language negotiation
- `acceptLanguageAliases`：协商 fallback 时匹配的别名（如 `zh-Hans` 命中 `cn`）
- `fallback`：当某 key 在该 locale 缺失时的查找顺序（base language → default 由代码自动追加）

**未来加日语**：追加一行 entry + 新建 `messages/ja/` 目录，**零代码改动**。

### 2.2 Locale 解析协议（前 → 后端）

来源优先级（用列表迭代，禁止 `if-else`）：

```
1. X-Locale header（前端 API client 自动注入）
2. JWT claim "locale"（登录后从 user.locale 写入 token）
3. Accept-Language header（未登录场景，走 RFC 4647 best-match）
4. registry.default
```

后端 `LocaleMiddleware` 解析后写入 `ctx.Locale`，所有 handler / response / notifier 任务 enqueue 用同一份 locale。

### 2.3 错误响应格式

```go
type ErrorDetail struct {
    Code      string         `json:"code"`        // 错误码（稳定契约）
    Message   string         `json:"message"`     // 后端按 locale 翻译的兜底文案
    Params    map[string]any `json:"params,omitempty"` // 插值参数
    RequestID string         `json:"request_id"`
}
```

前端处理顺序：`t('errors.${code}', params)` → 后端 `message` 兜底 → `errors.UNKNOWN` 兜底。

### 2.4 后端 i18n 实现

**自写极简 catalog**（~150 行），位于 `apps/api/internal/i18n/`：

```go
type Catalog struct {
    messages map[string]map[string]string // locale → key → message
}

func Load(dir string) (*Catalog, error) {
    c := &Catalog{messages: map[string]map[string]string{}}
    entries, _ := os.ReadDir(dir)
    for _, e := range entries {
        if !e.IsDir() { continue }
        locale := e.Name()
        c.messages[locale] = loadAllJSON(filepath.Join(dir, locale))
    }
    return c, nil
}

func (c *Catalog) T(locale, key string, params map[string]any) string {
    for _, loc := range registry.FallbackChain(locale) {
        if msg, ok := c.messages[loc][key]; ok {
            return interpolate(msg, params)
        }
    }
    return key
}
```

`registry.FallbackChain("en")` 返回 `["en", "cn"]`（自己 → base language → default）。

**ICU plural 支持**：自写 catalog 增加 ~30 行 `{count, plural, one {...} other {...}}` 解析（仅支持 plural 语法，不需要 select/selectordinal）。

### 2.5 文件组织

```
config/locales.json                                # 共享 registry

apps/web/src/i18n/
├── registry.ts                                    # 从 locales.json generate
├── routing.ts                                     # next-intl，locales 来自 registry
├── request.ts                                     # getRequestConfig
├── api-client.ts                                  # 注入 X-Locale header
├── api-error.ts                                   # translateApiError(err)
└── messages/
    ├── cn/
    │   ├── common.json
    │   ├── nav.json
    │   ├── errors.json
    │   ├── monitors.json
    │   ├── alerts.json
    │   ├── billing.json
    │   ├── auth.json
    │   ├── settings.json
    │   ├── status.json
    │   ├── dashboard.json
    │   ├── tools.json
    │   ├── leaderboard.json
    │   ├── nodes.json
    │   ├── pricing.json
    │   ├── home.json
    │   ├── admin.json
    │   └── enums.json
    └── en/
        └── ...（同结构）

apps/api/internal/i18n/
├── registry.go                                    # 读 config/locales.json
├── catalog.go                                     # ~150 行
├── locale.go                                      # ParseLocale + negotiate
├── middleware.go                                  # LocaleMiddleware
└── messages/
    ├── cn/
    │   └── errors.json
    └── en/
        └── errors.json

apps/api/internal/errcode/
├── codes.go                                       # const 块，全量错误码
└── http.go                                        # HTTP status mapping

apps/notifier/internal/i18n/
├── registry.go                                    # 共享同一份 registry
└── catalog.go                                     # 复用 API 的 catalog 逻辑

apps/notifier/internal/template/
├── verify_email.cn.html
├── verify_email.en.html
├── alert_notification.cn.html
├── alert_notification.en.html
├── refund_failed.cn.html
├── refund_failed.en.html
└── selector.go                                    # TemplateFor(name, locale) fallback chain
```

### 2.6 数据库字段调整

- `users.locale`：现有 TEXT DEFAULT 'zh-CN' → 改为 `cn` / `en`，需 migration 转换历史数据
- `status_page.default_locale`：新增 TEXT DEFAULT 'cn'
- `email_subscriptions.locale`：新增 TEXT DEFAULT 'cn'（订阅者提交时的语言）
- `notifications.i18n_key` + `notifications.i18n_params` (JSONB)：新增字段，**不存翻译后的 text**，渲染时按访问者 locale 翻译

---

## 三、分阶段实施计划

### Phase 0 — 决策锁定 + Registry 落地（0.5 天）

**目标**：把 D1-D9 写入本文档，建立共享 registry。

**交付物**：
- ✅ `docs/prd/I18N-PLAN.md`（本文档）
- `config/locales.json`
- `apps/web/src/i18n/registry.ts`（generate from JSON）
- `apps/api/internal/i18n/registry.go`
- `apps/notifier/internal/i18n/registry.go`
- 同步脚本 `scripts/sync-locale-registry.ts`（CI 检查 hash 一致）

---

### Phase 1 — 后端 i18n 基础设施（1 天）

#### 1.1 Catalog 实现
新建 `apps/api/internal/i18n/catalog.go` + ICU plural 支持 + 单元测试。

#### 1.2 Locale Middleware
插入 Chi 路由链路开头，解析后写入 `ctx.Locale`。

#### 1.3 错误码集中
`apps/api/internal/errcode/codes.go`：

```go
const (
    AuthRequired             = "AUTH_REQUIRED"
    AuthInvalidToken         = "AUTH_INVALID_TOKEN"
    AuthExpired              = "AUTH_EXPIRED"
    ValidationFailed         = "VALIDATION_FAILED"
    MonitorNotFound          = "MONITOR_NOT_FOUND"
    AlertRuleConflict        = "ALERT_RULE_CONFLICT"
    BillingInsufficientCredits = "BILLING_INSUFFICIENT_CREDITS"
    RateLimitExceeded        = "RATE_LIMIT_EXCEEDED"
    InternalError            = "INTERNAL_ERROR"
    // ... 全量列表
)
```

HTTP status 映射 `errcode/http.go`：

```go
var statusMap = map[string]int{
    AuthRequired:        401,
    AuthInvalidToken:    401,
    ValidationFailed:    400,
    MonitorNotFound:     404,
    RateLimitExceeded:   429,
    InternalError:       500,
}
```

#### 1.4 response.go 改造
`response.Error(ctx, code, params)` 内部从 ctx 取 locale + 查 catalog 翻译 message + 写 HTTP status。

#### 1.5 JWT locale claim
登录 / 刷新 / 注册 时把 `user.locale` 写入 token，所有后续请求自动携带。

---

### Phase 2 — 后端字符串清理（1.5 天）

#### 2.1 API handler 错误消息（~51 条）
扫 `errors.New / fmt.Errorf / http.Error / response.Error` 调用，逐个改成 `response.Error(ctx, errcode.X, params)`。

#### 2.2 notifier 邮件模板双语化（D3 + 邮件维度）

按 `name.{locale}.html` 命名约定，并新增 `selector.go`：

```go
func TemplateFor(base, locale string) (string, error) {
    for _, loc := range registry.FallbackChain(locale) {
        candidate := fmt.Sprintf("%s.%s.html", base, loc)
        if templateExists(candidate) {
            return candidate, nil
        }
    }
    return "", fmt.Errorf("no template for base=%s", base)
}
```

**邮件全维度国际化**：
- ✅ Subject（每个模板配套 `subject.cn` / `subject.en` key 在 `apps/notifier/internal/i18n/messages/{locale}/email.json`）
- ✅ From name（`"IDCD 通知"` / `"IDCD Notifications"`）
- ✅ Footer / unsubscribe 法律声明
- ✅ 邮件中日期时间走 `formatDateTime(t, locale, tz)`
- ✅ 链接 URL 带 locale 前缀：cn 走 `https://idcd.com/app/...`，en 走 `https://idcd.com/en/app/...`

#### 2.3 notifier task payload 带 locale
所有 Asynq 任务 payload schema 增加 `locale string`，发送方（API handler）从 `user.locale` 取值。监控告警按 `monitor.creator.locale` 决定语言（不是访问者 locale）。

#### 2.4 notifier 自身错误消息（~72 条）
notifier 内部错误（worker 不对外）改为英文 + structured log；用户可见的错误走 errcode + catalog。

---

### Phase 3 — 前端协议对齐（0.5 天）

#### 3.1 API client 自动注入 X-Locale

```ts
// apps/web/src/lib/api-client.ts
const locale = await getCurrentLocale();  // 从 next-intl 或 cookie
headers.set('X-Locale', locale);
```

#### 3.2 错误处理统一

```ts
// apps/web/src/lib/api-error.ts
export function translateApiError(err: ApiError, t: TFunction): string {
  if (err.code && hasKey(`errors.${err.code}`)) {
    return t(`errors.${err.code}`, err.params);
  }
  return err.message || t('errors.UNKNOWN');
}
```

全站 toast / form / inline error 统一调这个 helper。

#### 3.3 错误码 key 集合校验
CI lint：扫 `apps/api/internal/errcode/codes.go` 的常量名，断言每个 errcode 都在 `apps/web/src/i18n/messages/{locale}/errors.json` 中存在。

#### 3.4 LangToggle 改造
现有 `LangToggle` 改为遍历 `registry.locales`：

```tsx
{registry.locales.map(loc => (
  <button key={loc.code} onClick={() => switchLocale(loc.code)}>
    {loc.nativeLabel}
  </button>
))}
```

`switchLocale(target)` 保留当前 path + query + hash，路由用 `router.replace`。

#### 3.5 Cookie 规范
- name：`idcd_locale`
- 属性：`Path=/; Max-Age=31536000; SameSite=Lax; Secure`（生产）
- 同步 `user.locale`（登录用户优先服务端，未登录 cookie 主导）

---

### Phase 4 — 前端硬编码清理（按批次并行）

四批独立 PR，每批走完整测试门禁。

| 批次 | 范围 | 文件数 | 字符串估算 |
|---|---|---|---|
| **4a** | 用户后台 `app/app/*` + `components/layout/*` | ~70 | ~600 |
| **4b** | 认证 + 管理后台 `app/auth/*` `app/admin/*` | ~30 | ~250 |
| **4c** | 公开页 `app/(public)/*` | ~100 | ~1200 |
| **4d** | 文档 + 工具 `app/docs/*` `lib/tool-functions.ts` | ~30 | ~640 |

#### 4a — 用户后台（1 天）

重点文件：
- `apps/web/src/components/layout/nav-user.tsx`（行 61-125 全部硬编码）
- `apps/web/src/app/app/monitors/*`
- `apps/web/src/app/app/alerts/*`
- `apps/web/src/app/app/billing/*`
- `apps/web/src/app/app/settings/*`
- `apps/web/src/app/app/dashboard/*`
- `apps/web/src/app/app/status-pages/*`
- `apps/web/src/app/app/incidents/*`

#### 4b — 认证 + 管理后台（0.5 天）

- 认证页（`auth/login`、`auth/register`、`auth/reset-password`、`auth/verify-email`）—— 完整 EN
- 管理后台（`admin/*`）—— **架构走 i18n key，EN 翻译缺失走 fallback 链回退到中文**。CI lint 在 admin 目录 whitelist 跳过"未翻译 key 必须存在"检查。

#### 4c — 公开页（2 天）

- `app/(public)/page.tsx`（首页 Hero、特性、CTA）
- `app/(public)/about`、`app/(public)/aup`、`app/(public)/privacy`
- `app/(public)/agent`、`app/(public)/transparency`
- `app/(public)/leaderboard`、`app/(public)/nodes`、`app/(public)/pricing`
- `app/(public)/tools/[slug]/probe-client.tsx`、`tools-config.ts`
- `components/nav.tsx`、`components/footer.tsx`

#### 4d — 文档 + 工具（1.5 天）

**长文档采用分语言 MDX**（避免巨型 JSON 维护负担）：

```tsx
// apps/web/src/app/docs/[slug]/page.tsx
async function loadContent(slug: string, locale: string) {
  for (const loc of registry.fallbackChain(locale)) {
    try {
      return await import(`@/content/docs/${slug}/${loc}.mdx`);
    } catch {}
  }
  throw new Error(`No content for ${slug}`);
}
```

文档目录结构：

```
apps/web/src/content/docs/
├── getting-started/
│   ├── cn.mdx
│   └── en.mdx
├── monitors/
│   ├── cn.mdx
│   └── en.mdx
└── ...
```

短 UI 文案（按钮、标题、目录）仍走 `messages/{locale}/docs.json`。

工具相关：
- `tools-config.ts` 改为只存 slug / category / route，名字 / 描述 / metaTitle 从 `messages/{locale}/tools.json` 读
- `lib/tool-functions.ts` 中的错误消息走 i18n key

---

### Phase 5 — 工程化（0.5 天）

#### 5.1 CI lint 脚本 `scripts/lint-i18n.ts`

强制规则：

1. **完整性**：每个 registry locale 在 `messages/{locale}/` 必须有完整 namespace（admin 除外，admin 走 whitelist）
2. **Key 一致性**：所有 locale 的 key 集合必须等于 default locale（admin namespace 除外）
3. **前后端 errcode 对齐**：`errcode/codes.go` ↔ `messages/{locale}/errors.json` key 集合相等
4. **禁止二元 locale 判断**：
   ```bash
   git grep -nE "locale\s*[!=]==?\s*['\"](cn|en|ja|ko|zh)" -- '*.ts' '*.tsx' '*.go'
   git grep -nE "locale\s*===\s*['\"]" -- '*.ts' '*.tsx'
   git grep -nE "\.startsWith\(['\"](en|cn|zh)['\"]\)" -- '*.ts' '*.tsx' '*.go'
   ```
   任何匹配 = 失败
5. **禁止 binary locale map**：
   ```bash
   git grep -nE "\{\s*(cn|en|zh)\s*:\s*['\"][^'\"]+['\"]\s*,\s*(cn|en|zh)\s*:" -- '*.ts' '*.tsx'
   ```
6. **HTML lang 必须 BCP 47**：扫 JSX `<html lang=...>`，断言传入是 `bcp47Of(locale)` 而非 `locale`
7. **Intl.* 调用必须 BCP 47**：扫 `new Intl.DateTimeFormat(`、`new Intl.NumberFormat(`、`new Intl.RelativeTimeFormat(`，第一参数必须是 `bcp47Of(...)`
8. **未翻译字符串扫描**：检测 `.tsx` 中含中文字符的字符串字面量（注释、测试 fixture、admin 目录排除）

#### 5.2 Type 安全

- next-intl `global.d.ts` 声明 messages 类型，`t('xxx')` 编译期校验
- Go errcode 用 `type Code string` + const + linter 检查 message 字面量不直接出现在 handler

#### 5.3 翻译覆盖率脚本 `scripts/i18n-coverage.sh`

输出每个 locale × namespace 的 key 覆盖率，默认 locale 缺 key = error，其他 locale 缺 key = warning（fallback 处理）。

#### 5.4 Dev 体验

- 缺 key 高亮：dev 模式下未翻译 key 渲染为 `🌐 key.path`（next-intl 内置 onError）
- Dead key 检测：构建期 warning
- 缺失 key 自动 stub：`pnpm i18n:stub` 扫源码中 `t('foo.bar')`，给 messages 文件追加占位 key

---

### Phase 6 — 测试与验收（1 天）

#### 6.1 后端测试
- `catalog_test.go`：fallback chain、locale negotiation、未支持 locale 降级、ICU plural
- handler 测试 table-driven 跑所有 supported locale：
  ```go
  for _, loc := range registry.All() {
      t.Run(loc.Code, func(t *testing.T) {
          req.Header.Set("X-Locale", loc.Code)
          // 断言 message 命中对应翻译
      })
  }
  ```

#### 6.2 notifier 测试
邮件模板 snapshot 测试 × N locale × N 模板。

#### 6.3 前端测试
- Vitest mock locale provider 测两套渲染
- Playwright E2E：每个 locale 跑关键路径
  - 切换 LangToggle → 整页文案变化
  - 后台操作触发错误 → 错误消息按 locale 显示
  - 登录后改 profile locale → 邮件接收语言变化
  - 状态页 owner 设置 default_locale → 访客看到正确语言

#### 6.4 可扩展性验收
**关键 acceptance test**：临时在 `config/locales.json` 加一个虚构 locale（如 `ja`），跑完整测试套件全绿（admin namespace 走 fallback，其他 namespace 用占位翻译）。**无需改任何代码**。

#### 6.5 验收清单

```
□ 切换为 EN 后所有公开页 + 用户后台 + 认证页零中文
□ Admin 后台架构走 i18n key，EN 显示中文 fallback（已知容忍）
□ 任意 API 调用错误 toast 按 locale 显示
□ 注册验证邮件 / 监控告警邮件按 user.locale 发送
□ MCP / API 错误消息按 X-Locale 返回
□ status_page.default_locale 设置 en 后，访客 Accept-Language 为 cn 也优先看 en
□ Pricing 货币按 locale 切换（cn→CNY，en→USD）
□ scripts/lint-i18n.ts 通过（含二元判断检测）
□ 前后端 errcode key 集合完全相等
□ 加虚构 locale ja 到 registry，完整测试套件全绿（零代码改动）
□ go test ./... + pnpm test 全绿
□ Playwright × N locale × 关键路径 全绿
```

---

## 四、横切关注点（贯穿所有 Phase）

### 4.1 SEO

#### hreflang
每个公开页 `<head>` 自动渲染：

```html
<link rel="alternate" hreflang="zh-CN" href="https://idcd.com/about" />
<link rel="alternate" hreflang="en-US" href="https://idcd.com/en/about" />
<link rel="alternate" hreflang="x-default" href="https://idcd.com/about" />
```

helper `generateHreflang(currentPath)` 遍历 registry 输出，`hreflang` 值用 `bcp47Of()`。

#### sitemap.xml
`apps/web/src/app/sitemap.ts` 用 registry 自动生成：每个 URL 输出 N 份 locale 版本（带 `xhtml:link`）。

#### canonical
每个 locale 页面的 canonical 指向自己（cn → `/about`，en → `/en/about`），不互指。

#### generateMetadata
Next.js metadata API 按 locale 生成 title / description / OG / Twitter card。helper：

```tsx
export async function generateMetadata({ params }) {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: 'pricing' });
  return {
    title: t('meta.title'),
    description: t('meta.description'),
    alternates: generateAlternates('/pricing'),
  };
}
```

### 4.2 状态页（公开访问 + 自定义域名）

访客 locale 决策链：

```
1. ?lang=en query 参数
2. status_page.default_locale（owner 配置）
3. Accept-Language negotiation
4. registry.default
```

订阅邮件按订阅者提交时的 locale 发送（`email_subscriptions.locale` 列）。

### 4.3 支付 / 计费

- Pricing 页货币按 locale 默认（cn→CNY，en→USD），UI 允许手动切换
- 调用 Paddle SDK 时传 `locale: registry.bcp47Of(userLocale)`，Paddle 错误消息自动 i18n
- Refund 邮件走 notifier 模板系统
- 发票 PDF：S2 才做，先 placeholder

### 4.4 通知 / Webhook

- 站内通知：`notifications.i18n_key` + `i18n_params` 存数据库，渲染时按访问者 locale 翻译
- Webhook payload：保持原始 key / code，**不翻译**（消费方决定），`error_code` 字段是稳定契约
- MCP / API：错误按请求 locale 返回（D3 决策）

### 4.5 字体 fallback

CSS variable 按 locale 切换：

```css
:root[lang="zh-CN"] {
  --font-stack: "Geist Sans", "PingFang SC", "Microsoft YaHei", sans-serif;
}
:root[lang="en-US"] {
  --font-stack: "Geist Sans", system-ui, sans-serif;
}
```

HTML 根 `<html lang={bcp47Of(locale)}>` 驱动，无 JS 切换开销。

### 4.6 ICU 复数

```json
// messages/en/monitors.json
{ "count": "{count, plural, one {# monitor} other {# monitors}}" }
```
```json
// messages/cn/monitors.json
{ "count": "{count} 个监控" }
```

前端 `t('monitors.count', { count: 5 })` 自动选择形式；后端 catalog 同样支持。

### 4.7 相对时间

```ts
new Intl.RelativeTimeFormat(bcp47Of(locale), { numeric: 'auto' }).format(-5, 'minute')
// en → "5 minutes ago"
// cn → "5 分钟前"
```

封装 `useRelativeTime(date)` hook 全站使用。**禁止手写 `${minutes} 分钟前`**。

### 4.8 表单验证

- zod schema：`zod-i18n-map` + 自定义 messages 走 namespace `validation`
- react-hook-form：rules.message 写 i18n key 而非字符串
- 浏览器原生 validation：用 `setCustomValidity()` 注入翻译文案

### 4.9 状态机 / 枚举值翻译

`monitor.status: pending | running | failed | degraded` 在 UI 显示要翻译：

```ts
// 反例
status === 'failed' ? '失败' : '正常'

// 正例
t(`enums.monitor.status.${status}`)
```

约定 namespace：`enums.{entity}.{field}.{value}`。

### 4.10 时区独立于 locale

`user.timezone` 字段独立存（确认 schema），UI 切换 locale 不影响时区。所有日期渲染 = `formatDateTime(date, locale, timezone)`。

### 4.11 aria-label / 无障碍

所有 `aria-label` / `aria-describedby` / `title` 文本走 i18n。CI lint：`.tsx` 中 `aria-label="..."` 含非 ASCII = 警告。

### 4.12 URL 切换 UX

LangToggle 切换时：
- 保留 path + query string + hash
- `router.replace()` 避免回退栈污染
- 切换前 prefetch 目标 locale 的 messages chunk
- helper：`switchLocaleHref(currentHref, targetLocale)` 转换 URL

### 4.13 报告 / 审计

- 用户可见报告（postmortem）：存原始 i18n 数据（key + params），渲染时按访问者 locale 翻译
- Admin 审计日志：英文 key + raw params 存 DB，渲染时翻译

### 4.14 性能

- next-intl namespace 按需加载（默认行为）
- 每个 locale 单独 chunk
- 公共 namespace（`common`、`errors`）首屏 inline

### 4.15 RTL 准备

虽然初版不上阿语，CSS 一次性改造：用 logical properties (`margin-inline-start` 代替 `margin-left`)。registry 已有 `dir` 字段，HTML `<html dir={...}>` 动态设置。Tailwind RTL 插件按需引入。

---

## 五、翻译文件键名规范

```
模块.子模块.键名

nav.tools.probe              = "拨测工具"
nav.tools.groups.network     = "网络连通"
tools.ping.title             = "多地 Ping 测试"
tools.ping.description       = "全球多节点 ICMP Ping..."
tools.ping.meta.title        = "多地 Ping 测试 - 全球延迟检测 | idcd"
common.actions.save          = "保存"
common.actions.cancel        = "取消"
monitors.list.empty.title    = "暂无监控"
monitors.list.empty.cta      = "创建第一个监控"
enums.monitor.status.failed  = "失败"
errors.AUTH_REQUIRED         = "需要登录"
validation.email.invalid     = "邮箱格式错误"
```

**规则**：
- 全部小写 camelCase（路径段），错误码大写蛇形（值）
- 不超过 4 级嵌套
- key 英文，JSON value 写目标语言文本
- 变量用 `{varName}`（next-intl ICU 默认）
- 共享文案放 `common` namespace（避免每个 namespace 都有 save/cancel/delete）

---

## 六、开发规范

### 6.1 服务端组件

```tsx
import { getTranslations } from 'next-intl/server';

export default async function Page() {
  const t = await getTranslations('monitors');
  return <h1>{t('list.title')}</h1>;
}
```

### 6.2 客户端组件

```tsx
'use client';
import { useTranslations } from 'next-intl';

export function CreateButton() {
  const t = useTranslations('common.actions');
  return <Button>{t('create')}</Button>;
}
```

### 6.3 错误处理

```tsx
import { translateApiError } from '@/lib/api-error';
import { useTranslations } from 'next-intl';

const t = useTranslations();
try { await api.create(...); }
catch (e) { toast.error(translateApiError(e, t)); }
```

### 6.4 后端 handler

```go
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    if err := h.svc.Create(...); err != nil {
        if errors.Is(err, ErrConflict) {
            response.Error(w, r, errcode.AlertRuleConflict, map[string]any{"name": name})
            return
        }
        response.Error(w, r, errcode.InternalError, nil)
        return
    }
    response.OK(w, r, result)
}
```

### 6.5 notifier 任务

```go
type SendEmailTask struct {
    Template string         `json:"template"`  // "verify_email"
    Locale   string         `json:"locale"`    // 来自 user.locale
    To       string         `json:"to"`
    Data     map[string]any `json:"data"`
}
```

worker 处理时：

```go
tmpl := template.TemplateFor(task.Template, task.Locale)
subject := catalog.T(task.Locale, "email."+task.Template+".subject", task.Data)
body := renderTemplate(tmpl, task.Data)
```

---

## 七、执行时间线

| Phase | 内容 | 预估工时 | 依赖 |
|---|---|---|---|
| 0 | 决策 + Registry 落地 | 0.5d | — |
| 1 | 后端 i18n 基础设施 | 1d | Phase 0 |
| 2 | 后端字符串清理 + 邮件双语 | 1.5d | Phase 1 |
| 3 | 前端协议对齐 | 0.5d | Phase 1 |
| 4a | 用户后台 | 1d | Phase 3 |
| 4b | 认证 + 管理后台 | 0.5d | Phase 3 |
| 4c | 公开页 | 2d | Phase 3 |
| 4d | 文档 + 工具 | 1.5d | Phase 3 |
| 5 | 工程化（lint + 类型 + dev 工具） | 0.5d | Phase 4 |
| 6 | 测试与验收 | 1d | Phase 5 |
| **合计** | | **~10 工作日** | |

横切关注点（SEO / 状态页 / 支付 / 邮件维度 / 字体 / 复数 / 相对时间 / 表单 / URL UX / 报告审计）的工作量已分摊到相关 Phase。

---

## 八、并发 Agent 执行策略

遵守 CLAUDE.md "并发 Agent 派发规则"。Phase 4 各批次可并行，但文件集合互不相交。

**批次 4a（用户后台，可并发）**：
- Agent A1：`apps/web/src/app/app/monitors/*` + `apps/web/src/components/layout/nav-user.tsx`
- Agent A2：`apps/web/src/app/app/alerts/*`
- Agent A3：`apps/web/src/app/app/settings/*` + `apps/web/src/app/app/billing/*` + `apps/web/src/app/app/usage/*`
- Agent A4：`apps/web/src/app/app/dashboard/*` + `apps/web/src/app/app/incidents/*` + `apps/web/src/app/app/status-pages/*`

**批次 4c（公开页，可并发）**：
- Agent C1：`apps/web/src/components/nav.tsx` + `apps/web/src/components/footer.tsx`
- Agent C2：`apps/web/src/app/(public)/tools/*` + `apps/web/src/lib/tool-functions.ts`
- Agent C3：`apps/web/src/app/(public)/{about,aup,privacy,agent,transparency}/*`
- Agent C4：`apps/web/src/app/(public)/{leaderboard,nodes,pricing,home}/*`

**消息文件维护（串行）**：
- 始终 1 个 Agent 维护 `messages/{cn,en}/*.json`，避免 JSON 合并冲突。其他 Agent 用 `pnpm i18n:stub` 写占位 key，最后由消息维护者集中翻译。

**禁止**：
- 同一文件 ≥ 2 个 Agent 并发改
- prompt 中使用绝对路径（会绕过 worktree 隔离）

---

## 九、加新语言操作手册（附录 A）

新增 locale 的操作步骤（以日语 `ja` 为例）：

1. **追加 registry 条目**：
   ```json
   {
     "code": "ja",
     "bcp47": "ja-JP",
     "label": "日本語",
     "nativeLabel": "日本語",
     "baseLanguage": "ja",
     "acceptLanguageAliases": ["ja", "ja-JP"],
     "dir": "ltr",
     "fontStack": "cjk",
     "fallback": []
   }
   ```

2. **复制 messages 目录**：
   ```bash
   cp -r apps/web/src/i18n/messages/cn apps/web/src/i18n/messages/ja
   cp -r apps/api/internal/i18n/messages/cn apps/api/internal/i18n/messages/ja
   ```

3. **复制邮件模板**：
   ```bash
   for f in apps/notifier/internal/template/*.cn.html; do
     cp "$f" "${f/.cn./.ja.}"
   done
   ```

4. **复制文档 MDX**：
   ```bash
   for d in apps/web/src/content/docs/*/; do
     cp "$d/cn.mdx" "$d/ja.mdx"
   done
   ```

5. **翻译**：LLM 批量初翻 + 人工 review。

6. **字体 fallback**（如需）：
   ```css
   :root[lang="ja-JP"] {
     --font-stack: "Geist Sans", "Hiragino Sans", "Yu Gothic", sans-serif;
   }
   ```

7. **测试**：
   ```bash
   pnpm lint:i18n
   pnpm --filter @idcd/web test
   go test ./...
   ```

8. **提 PR**：标注 `i18n-new-locale` label，触发 CI 强制 review。

**以上 8 步均不涉及代码逻辑改动**，仅是数据复制 + 翻译。

---

## 十、待决问题（已全部 close，留作历史记录）

| # | 问题 | 决策 |
|---|---|---|
| Q1 | URL 策略（cn 是否带前缀） | cn 不带（D1） |
| Q2 | Admin 是否做 EN | 架构做，翻译不做（D2） |
| Q3 | MCP / API 是否翻译 | 翻译（D3） |
| Q4 | 是否启用 ICU 复数 | 启用（D4） |
| Q5 | 状态页是否支持 owner 指定 default_locale | 支持（D5） |
| Q6 | locale code 用 BCP 47 还是短码 | 短码 cn/en，registry 同步 bcp47 副字段（D7） |
| Q7 | 后端 i18n 选型 | 自写极简 catalog（D8） |
| Q8 | 初始范围 | S1 上 cn + en，架构按 N 种语言（D9） |

---

## 附录 B：关键文件路径汇总

```
config/locales.json                                    # 共享 registry

apps/web/src/
├── middleware.ts                                      # next-intl 路由 + cookie
├── i18n/
│   ├── registry.ts                                    # generate from config/locales.json
│   ├── routing.ts                                     # locales 来自 registry
│   ├── request.ts                                     # getRequestConfig
│   ├── api-client.ts                                  # 注入 X-Locale
│   ├── api-error.ts                                   # translateApiError
│   └── messages/{cn,en}/*.json
├── components/
│   ├── nav.tsx                                        # 改造 LangToggle
│   ├── footer.tsx
│   └── layout/nav-user.tsx
├── lib/
│   └── tool-functions.ts                              # 错误消息走 i18n
└── content/docs/{slug}/{cn,en}.mdx

apps/api/internal/
├── i18n/
│   ├── registry.go                                    # 读 config/locales.json
│   ├── catalog.go                                     # ~150 行 + ICU plural
│   ├── locale.go                                      # negotiate
│   ├── middleware.go                                  # LocaleMiddleware
│   └── messages/{cn,en}/errors.json
├── errcode/
│   ├── codes.go                                       # 全量错误码
│   └── http.go                                        # HTTP status mapping
└── response/response.go                               # 改造：从 ctx 取 locale + 翻译

apps/notifier/internal/
├── i18n/                                              # 复用 API catalog
├── template/
│   ├── verify_email.{cn,en}.html
│   ├── alert_notification.{cn,en}.html
│   ├── refund_failed.{cn,en}.html
│   └── selector.go                                    # TemplateFor() fallback chain
└── i18n/messages/{cn,en}/email.json                   # subject / from / footer

lib/db/migrations/idcd_main/
├── XXXXX_locale_short_codes.sql                       # users.locale: 'zh-CN' → 'cn'
├── XXXXX_status_page_default_locale.sql               # ADD default_locale TEXT
├── XXXXX_email_subscriptions_locale.sql               # ADD locale TEXT
└── XXXXX_notifications_i18n_key.sql                   # ADD i18n_key + i18n_params

scripts/
├── sync-locale-registry.ts                            # CI 检查 hash 一致
├── lint-i18n.ts                                       # 完整性 + 二元判断检测
└── i18n-coverage.sh                                   # 覆盖率报告

docs/prd/
├── I18N-PLAN.md                                       # 本文档
└── 16-api-spec.yaml                                   # 错误码 examples 加 cn/en sample
```
