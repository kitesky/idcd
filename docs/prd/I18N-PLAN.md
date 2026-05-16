# idcd 国际化（i18n）落地计划

> 版本：v1.0 · 2026-05-16  
> 状态：草稿，待确认后执行

---

## 一、现状诊断

### 1.1 已有基础（可复用）

| 资产 | 文件 | 说明 |
|---|---|---|
| 语言配置 | `src/i18n/config.ts` | `locales = ['zh','en']`，`defaultLocale = 'zh'` |
| 翻译文件骨架 | `src/i18n/messages/zh.json` + `en.json` | 覆盖 tools / common / leaderboard，共 85 行 |
| EN 工具 SEO 元数据 | `src/i18n/en-tools-meta.ts` | 21 个工具完整 title/description/schemaName |
| 英文路由 | `/(public)/en/page.tsx` + `/(public)/en/tools/[slug]/` | 手动建立，未连接全局 locale 体系 |
| 依赖包 | `next-intl ^3.26.0` | 已安装，**未接入**（无 middleware.ts，无 useTranslations） |
| 测试 | `src/i18n/__tests__/i18n.test.ts` | 消费 getMessages，验证结构同步 |

### 1.2 缺口

| 缺口 | 规模 | 优先级 |
|---|---|---|
| `next-intl` 中间件未接入 | — | P0 |
| 公开页面 Nav / Footer 硬编码中文 | ~169 处 | P0 |
| `tools-config.ts` 工具名/描述全中文 | 50+ 工具 | P0 |
| 公开页面主体内容（首页、工具页、排行榜、节点、文档） | ~50 页 | P1 |
| App 后台页面（monitors/alerts/billing 等）全中文 | ~40 页 | P2 |
| Auth 页面（login/register/reset）全中文 | ~7 页 | P1 |
| Admin 后台（内部使用，暂不国际化） | — | 豁免 |
| 后端错误 message 字段中英文混杂 | — | P2 |

---

## 二、架构决策

### 2.1 URL 策略（已定，沿用现有方向）

```
https://idcd.com/           → 中文（默认）
https://idcd.com/en/        → 英文
```

**不采用** Next.js `[locale]` 文件夹方案（需移动 ~106 个页面，风险过高）。

**采用** next-intl **Prefix Strategy** with `always`（中文使用根路径，英文使用 `/en/` 前缀）：

```ts
// i18n/routing.ts
export const routing = defineRouting({
  locales: ['zh', 'en'],
  defaultLocale: 'zh',
  localePrefix: 'as-needed'   // zh 不加前缀，en 加 /en/
})
```

### 2.2 App 后台语言切换

App 后台（`/app/*`）已认证，无 SEO 需求，使用**用户偏好 Cookie** 控制语言，与公开页面的 URL 前缀体系共用同一套翻译文件，不改变 URL。

### 2.3 后端策略

后端**不引入 i18n 框架**。统一规则：

- `apperr.Error.Code` 字段 → 机器可读，保持英文（已良好）
- `apperr.Error.Message` 字段 → **统一改为英文**（目前有中文混入），前端按 Code 做本地化映射
- API 响应中**不输出本地化字符串**，所有 UI 文案由前端负责

### 2.4 翻译方案

- **格式**：JSON（`messages/zh.json`、`messages/en.json`）
- **不引入** ICU message 格式或第三方翻译平台（规模不够）
- **变量插值**用 `{varName}` 格式（next-intl 默认）
- **不在代码里写内联翻译**，全部从 JSON 文件引用

---

## 三、分阶段实施计划

### Phase 0 — 基础设施接入（约 0.5 天）

**目标**：让 next-intl 正式接管路由和翻译注入，取代现有手工模块。

**文件变动**：

```
apps/web/
  src/
    middleware.ts                     ← 新建（next-intl routing）
    i18n/
      routing.ts                      ← 新建（defineRouting）
      request.ts                      ← 新建（getRequestConfig，加载 JSON）
      config.ts                       ← 删除（合并进 routing.ts）
      messages.ts                     ← 删除（next-intl 接管）
  next.config.ts                      ← 加 createNextIntlPlugin 包装
```

**具体步骤**：

1. 新建 `src/middleware.ts`：
```ts
import createMiddleware from 'next-intl/middleware'
import { routing } from './i18n/routing'
export default createMiddleware(routing)
export const config = {
  matcher: ['/((?!api|_next|_vercel|.*\\..*).*)']
}
```

2. 新建 `src/i18n/routing.ts`：
```ts
import { defineRouting } from 'next-intl/routing'
export const routing = defineRouting({
  locales: ['zh', 'en'],
  defaultLocale: 'zh',
  localePrefix: 'as-needed'
})
```

3. 新建 `src/i18n/request.ts`：
```ts
import { getRequestConfig } from 'next-intl/server'
import { routing } from './routing'
export default getRequestConfig(async ({ requestLocale }) => {
  const locale = (await requestLocale) ?? routing.defaultLocale
  return {
    locale,
    messages: (await import(`./messages/${locale}.json`)).default
  }
})
```

4. 更新 `next.config.ts` 引入 `withNextIntl`

5. 在根 `layout.tsx` 中注入 `NextIntlClientProvider`

**测试门禁**：`pnpm --filter @idcd/web test`（更新 i18n 测试以使用 routing.ts）

---

### Phase 1 — 翻译文件扩容（与 Phase 2 并行）

**目标**：将所有公开页面和 App 页面字符串提取到 JSON，做到 zh/en 完整覆盖。

**文件结构规划**：

```json
// zh.json 结构（en.json 镜像）
{
  "nav": { ... },
  "footer": { ... },
  "home": { ... },
  "tools": {
    "ping": { "title": "...", "description": "...", "meta": { ... } },
    ...
    "_ui": {
      "run": "开始检测",
      "result": "检测结果",
      "loading": "检测中...",
      "copy": "复制",
      "copied": "已复制",
      "export": "导出",
      "filter": "筛选",
      "nodes": "个节点",
      ...
    }
  },
  "leaderboard": { ... },
  "nodes": { ... },
  "pricing": { ... },
  "auth": {
    "login": { ... },
    "register": { ... },
    "forgotPassword": { ... }
  },
  "app": {
    "common": { "save": "保存", "cancel": "取消", "delete": "删除", ... },
    "dashboard": { ... },
    "monitors": { ... },
    "alerts": { ... },
    "billing": { ... },
    "settings": { ... }
  },
  "errors": {
    "NOT_FOUND": "资源不存在",
    "DUPLICATE": "已存在，请勿重复提交",
    "VALIDATION": "输入内容有误",
    "UNAUTHORIZED": "请先登录",
    "FORBIDDEN": "无访问权限",
    "RATE_LIMIT": "请求过于频繁，请稍后重试",
    "INTERNAL": "服务器内部错误",
    "UNAVAILABLE": "服务暂时不可用"
  }
}
```

**字符串提取策略**：

工具名/描述：`tools-config.ts` 改为引用翻译 key，不再硬编码中文。

```ts
// 改造前
{ name: 'SSL 证书检查', description: '检查域名...' }

// 改造后（tools-config.ts 只存 slug/category/route 元数据）
{ slug: 'ssl', category: 'probe', href: '/tools/ssl' }
// 组件内通过 t('tools.ssl.title') 读取
```

---

### Phase 2 — 公开页面组件国际化（约 2 天）

**优先级 P0**（直接影响英文用户）：

| 组件/页面 | 主要字符串 | 说明 |
|---|---|---|
| `components/nav.tsx` | 导航菜单、工具分类名 | 21.7KB，大量硬编码 |
| `components/footer.tsx` | 链接文字、版权 | 6.7KB |
| `/(public)/en/page.tsx` | 接入翻译，删除内联英文 | 当前为独立维护，需并轨 |
| `/(public)/tools/tools-config.ts` | 50+ 工具名/描述/metaTitle | 改为翻译 key |

**优先级 P1**（影响体验但不阻断）：

| 页面 | 字符串规模 |
|---|---|
| `/(public)/leaderboard` | 中等 |
| `/(public)/nodes` + `[id]` | 中等 |
| `/(public)/pricing` | 小 |
| `/(public)/tools/[slug]` 各工具 UI | 每个工具独立 UI 字符串 |
| `/(public)/transparency` | 小 |
| `/(public)/about` | 小 |
| `auth/*` 所有认证页面 | 中等 |
| `docs/*` 文档页面 | 大（内容页可采用 mdx locale 分支） |

**Locale Switcher（语言切换器）**：

在 Nav 右上角加语言切换入口（地球仪图标已存在）：

```tsx
// 公开页：切换 URL 前缀
function LocaleSwitcher() {
  const locale = useLocale()
  const pathname = usePathname()
  return (
    <Link href={locale === 'zh' ? `/en${pathname}` : pathname.replace(/^\/en/, '')}>
      {locale === 'zh' ? 'EN' : '中文'}
    </Link>
  )
}
```

---

### Phase 3 — App 后台国际化（约 2 天）

**策略**：App 后台 URL 不变（不加 `/en/` 前缀），语言跟随用户偏好 Cookie `locale=en|zh`。

**实现**：

```ts
// middleware.ts 扩展：/app/* 路由从 Cookie 读取 locale
// 写入 Cookie 时机：Nav 语言切换按钮点击 + 用户设置页保存
```

**覆盖范围**（按实际影响用户的频率排序）：

1. `app/monitors/*` — 监控列表/详情/新建
2. `app/alerts/*` — 告警策略/分组/通知渠道
3. `app/dashboard` — 仪表盘
4. `app/settings/*` — 所有设置页
5. `app/billing` + `app/usage`
6. `app/status-pages/*`
7. `app/incidents/*`
8. `app/reports`

**UI 字符串规模估算**：586 处硬编码中文，约 300-400 个唯一字符串（含重复）。

---

### Phase 4 — 后端 Message 英文统一（约 0.5 天）

**目标**：`apperr.Error.Message` 全部改为英文，前端通过 Code 映射到本地化文案。

**改动范围**（grep 确认含中文 message 的 handler）：

- `handler/auth.go` — 登录/注册错误中文消息
- `handler/monitor.go` — 监控错误
- `handler/alert.go` — 告警错误
- `handler/account.go` — 账号错误
- 其他 handler

**示例改动**：

```go
// 改造前
return apperr.Validation("邮箱格式错误", "")

// 改造后
return apperr.Validation("invalid email format", "")
// 前端：t('errors.VALIDATION') → "输入内容有误" (zh) / "Invalid input" (en)
```

---

### Phase 5 — SEO & 元数据完善（约 0.5 天）

**hreflang**：所有公开页面 metadata 加 `alternates.languages`：

```ts
alternates: {
  canonical: locale === 'zh' ? `https://idcd.com${pathname}` : `https://idcd.com/en${pathname}`,
  languages: {
    'zh': `https://idcd.com${pathname}`,
    'en': `https://idcd.com/en${pathname}`,
    'x-default': `https://idcd.com${pathname}`
  }
}
```

**sitemap.ts**：为每个公开页面生成 zh + en 两条 URL。

**robots.ts**：确保 `/en/` 不被意外屏蔽。

---

## 四、翻译文件键名规范

```
模块.子模块.键名

nav.tools.probe          = "拨测工具"
nav.tools.groups.network = "网络连通"
tools.ping.title         = "多地 Ping 测试"
tools.ping.description   = "全球多节点 ICMP Ping..."
tools.ping.meta.title    = "多地 Ping 测试 - 全球延迟检测 | idcd"
tools._ui.run            = "开始检测"
app.monitors.title       = "监控任务"
app.monitors.actions.pause = "暂停"
errors.NOT_FOUND         = "资源不存在"
```

**规则**：
- 全部小写 camelCase
- 不超过 4 级嵌套
- 英文 key，JSON 字符串写目标语言文本
- 变量用 `{varName}`，不用 `%s` 或模板字符串

---

## 五、使用方式（开发规范）

### 服务端组件

```tsx
import { getTranslations } from 'next-intl/server'

export default async function Page() {
  const t = await getTranslations('tools')
  return <h1>{t('ping.title')}</h1>
}
```

### 客户端组件

```tsx
'use client'
import { useTranslations } from 'next-intl'

export function RunButton() {
  const t = useTranslations('tools._ui')
  return <Button>{t('run')}</Button>
}
```

### Metadata（generateMetadata）

```tsx
import { getTranslations } from 'next-intl/server'

export async function generateMetadata({ params }: Props) {
  const { locale } = await params
  const t = await getTranslations({ locale, namespace: 'tools.ping.meta' })
  return { title: t('title') }
}
```

### 错误码本地化

```tsx
import { useTranslations } from 'next-intl'

function ErrorMessage({ code }: { code: string }) {
  const t = useTranslations('errors')
  return <p>{t(code, { fallback: t('INTERNAL') })}</p>
}
```

---

## 六、测试要求

### 新增 i18n 专项测试

```ts
// 1. 键同步测试（已有，需扩展到所有新增 namespace）
it('zh 和 en 所有 key 完全一致', ...)

// 2. 无孤儿 key（JSON 中定义但代码未使用）
// 工具：pnpm i18n:unused（脚本：grep keys in JSON vs. grep t() in source）

// 3. 无内联字符串（组件中直接写中文）
// CI lint rule: eslint-plugin-i18n 或自定义 AST 检查

// 4. locale 切换快照
// 对每个关键页面生成 zh + en 两份快照，防止回归
```

### 提交前检查

```
□ pnpm --filter @idcd/web test 全绿（含 i18n 结构同步测试）
□ zh.json 和 en.json 所有 namespace key 完全对应
□ 无新增内联中文字符串（CI grep 检查）
□ metadata alternates 已设置
```

---

## 七、执行时间线

| Phase | 内容 | 预估工时 | 依赖 |
|---|---|---|---|
| 0 | next-intl 接入 + middleware | 0.5d | — |
| 1 | 翻译文件扩容（JSON 写入） | 1d | Phase 0 |
| 2 | 公开页面组件国际化 | 2d | Phase 1 |
| 3 | App 后台国际化 | 2d | Phase 1 |
| 4 | 后端 message 英文统一 | 0.5d | — |
| 5 | SEO/hreflang/sitemap | 0.5d | Phase 2 |

**总计：约 6.5 工作日**

---

## 八、并发 Agent 执行策略

Phase 2 + Phase 3 可并行执行，但必须遵守文件不重叠规则：

**批次 A（公开页面，并发）**：
- Agent A1：`components/nav.tsx` + `components/footer.tsx`
- Agent A2：`/(public)/tools/tools-config.ts` + `/(public)/tools/[slug]/`
- Agent A3：`/(public)/leaderboard` + `/(public)/nodes`
- Agent A4：`auth/*` 所有认证页面

**批次 B（App 后台，并发）**：
- Agent B1：`app/monitors/*`
- Agent B2：`app/alerts/*`
- Agent B3：`app/settings/*` + `app/billing` + `app/usage`
- Agent B4：`app/dashboard` + `app/incidents` + `app/status-pages`

**批次 C（翻译文件，串行）**：
- 仅 1 个 Agent 维护 `zh.json` + `en.json`（避免 JSON 合并冲突）

---

## 九、待决问题（需确认后执行）

| # | 问题 | 选项 |
|---|---|---|
| Q1 | `/docs/*` 文档页面是否需要国际化？ | A) 暂不国际化（仅中文）B) Phase 2 一起做 C) 独立 Sprint |
| Q2 | App 后台语言切换是否写入用户 Profile（数据库）？ | A) 仅 Cookie（无需后端） B) Cookie + DB 同步 |
| Q3 | 工具 UI（探测结果、节点表格）是否全量国际化？ | A) 是（Phase 2 包含） B) 仅 Nav/Footer/Landing，工具 UI 暂缓 |
| Q4 | Admin 后台是否豁免 i18n？ | A) 是（内部使用，中文足够） B) 也要做 |

---

## 附录：关键文件路径汇总

```
apps/web/
  src/
    middleware.ts                         [新建] next-intl 路由中间件
    i18n/
      routing.ts                          [新建] defineRouting
      request.ts                          [新建] getRequestConfig
      messages/
        zh.json                           [扩容] 从 85 行 → ~600 行
        en.json                           [扩容] 同上
      en-tools-meta.ts                    [保留] SEO 专用，不动
    components/
      nav.tsx                             [改造] 提取所有中文字符串
      footer.tsx                          [改造] 提取所有中文字符串
    app/
      (public)/
        tools/tools-config.ts             [改造] 移除中文内联
        en/                               [删除或合并] 独立英文页面并轨
      app/
        monitors/*                        [改造] useTranslations
        alerts/*                          [改造]
        ...
  next.config.ts                          [更新] withNextIntl 包装

apps/api/
  internal/handler/*                      [Phase 4] message 英文统一
  lib/shared/apperr/apperr.go             [Phase 4] 构造函数 message 英文
```
