# Design System — idcd

> 状态:**已锁定(2026-05-13)**
> 关联:`01-branding.md`(品牌)/ `14-tech-architecture.md`(技术栈)
> Preview:`/tmp/idcd-design-preview.html`(本次会话生成,可重新生成)

---

## 1. 核心原则

> **采用 shadcn/ui 官方设计体系完整版,仅配色用官方 `blue` theme。**

idcd 不自定义间距、字体、圆角、组件 token。所有这些**直接采用 shadcn/ui 默认值**。设计系统的唯一自由度是**配色 = shadcn/ui blue theme**。

理由:
- 极简维护成本(shadcn 官方维护 token,我们不重复)
- 组件 plug-and-play(`pnpm dlx shadcn add button` 即可,无需 patch)
- 升级路径清晰(shadcn 更新时,直接 pull 官方)
- 与 Next.js 16 + Tailwind v4 生态默认绑定

---

## 2. 技术栈(已锁定,DECISIONS §B)

| 维度 | 选型 | 来源 |
|---|---|---|
| 框架 | Next.js 16 (App Router) | 14 §3.2(2026-05 锁定 latest) |
| UI 库 | shadcn/ui (latest) | 14 §3.2 |
| 底层 | Radix UI primitives | shadcn 依赖 |
| CSS | Tailwind CSS v4 | 14 §3.2 |
| 字体 | Geist Sans + Geist Mono | Next.js 16 默认,shadcn 推荐 |
| 包管理 | pnpm | 14 §3.2 |
| 图表 | ECharts + Recharts(备选) | 14 §3.2 |

---

## 3. 配色 — shadcn/ui blue theme(官方默认)

### 3.1 完整 token 集

用 shadcn/ui CLI 初始化时选择 `blue`:

```bash
pnpm dlx shadcn@latest init
# Style: New York(推荐)
# Base color: Blue          ← 关键决策
# CSS variables: Yes
# RSC: Yes(Next.js App Router)
```

生成的 `app/globals.css`:

```css
@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --card: 0 0% 100%;
    --card-foreground: 222.2 84% 4.9%;
    --popover: 0 0% 100%;
    --popover-foreground: 222.2 84% 4.9%;
    --primary: 221.2 83.2% 53.3%;             /* blue-500 #3B82F6 */
    --primary-foreground: 210 40% 98%;
    --secondary: 210 40% 96.1%;
    --secondary-foreground: 222.2 47.4% 11.2%;
    --muted: 210 40% 96.1%;
    --muted-foreground: 215.4 16.3% 46.9%;
    --accent: 210 40% 96.1%;
    --accent-foreground: 222.2 47.4% 11.2%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 210 40% 98%;
    --border: 214.3 31.8% 91.4%;
    --input: 214.3 31.8% 91.4%;
    --ring: 221.2 83.2% 53.3%;
    --chart-1: 12 76% 61%;
    --chart-2: 173 58% 39%;
    --chart-3: 197 37% 24%;
    --chart-4: 43 74% 66%;
    --chart-5: 27 87% 67%;
    --radius: 0.5rem;
  }

  .dark {
    --background: 222.2 84% 4.9%;
    --foreground: 210 40% 98%;
    --card: 222.2 84% 4.9%;
    --card-foreground: 210 40% 98%;
    --popover: 222.2 84% 4.9%;
    --popover-foreground: 210 40% 98%;
    --primary: 217.2 91.2% 59.8%;             /* blue-400 #60A5FA */
    --primary-foreground: 222.2 47.4% 11.2%;
    --secondary: 217.2 32.6% 17.5%;
    --secondary-foreground: 210 40% 98%;
    --muted: 217.2 32.6% 17.5%;
    --muted-foreground: 215 20.2% 65.1%;
    --accent: 217.2 32.6% 17.5%;
    --accent-foreground: 210 40% 98%;
    --destructive: 0 62.8% 30.6%;
    --destructive-foreground: 210 40% 98%;
    --border: 217.2 32.6% 17.5%;
    --input: 217.2 32.6% 17.5%;
    --ring: 224.3 76.3% 48%;
    --chart-1: 220 70% 50%;
    --chart-2: 160 60% 45%;
    --chart-3: 30 80% 55%;
    --chart-4: 280 65% 60%;
    --chart-5: 340 75% 55%;
  }
}
```

### 3.2 idcd 业务语义色扩展

shadcn 官方只提供 `destructive`(错误)。idcd 需要 `success` / `warning` / `info` 三个额外语义色,从 Tailwind 调色板取:

```css
@layer base {
  :root {
    /* 业务语义色扩展(非 shadcn 官方,但 Tailwind 调色板取) */
    --success: 142.1 76.2% 36.3%;       /* green-600 #16A34A */
    --success-foreground: 355.7 100% 97.3%;
    --warning: 32.1 94.6% 43.7%;        /* orange-600 #DC8500 */
    --warning-foreground: 0 0% 100%;
    --info: 221.2 83.2% 53.3%;          /* 同 primary blue-500 */
    --info-foreground: 210 40% 98%;
  }
  .dark {
    --success: 142.1 70.6% 45.3%;       /* green-500 */
    --warning: 47.9 95.8% 53.1%;        /* yellow-500 */
    --warning-foreground: 26 83.3% 14.1%;
  }
}
```

### 3.3 业务场景使用

| 场景 | Token | 颜色 |
|---|---|---|
| 监控 UP 状态 | `success` | green-600 / green-500 |
| 监控 DEGRADED | `warning` | orange-600 / yellow-500 |
| 监控 DOWN | `destructive` | red-500 / red-700 |
| Verdict verified badge | `success` | green-600 |
| Verdict refund_failed | `destructive` | red-500 |
| CTA / 链接 / 数据强调 | `primary` | blue-500 / blue-400 |
| MCP token type badge | `info`(同 primary) | blue-500 |
| 节点 ASN 标签 | `muted-foreground` | gray-500 |

---

## 4. 字体(shadcn / Next.js 默认)

直接采用 Next.js 16 + shadcn/ui 默认推荐:

```ts
// app/layout.tsx
import { Geist, Geist_Mono } from "next/font/google";

const geistSans = Geist({
  subsets: ["latin", "latin-ext"],
  variable: "--font-sans",
});

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
});

// tailwind.config 已通过 shadcn init 自动配 fontFamily.sans / fontFamily.mono
```

### 中文字体 fallback

Geist 主要支持拉丁字符,中文走系统字体 fallback:

```css
body {
  font-family:
    var(--font-sans),
    -apple-system, BlinkMacSystemFont,
    "PingFang SC", "Microsoft YaHei",  /* 中文优先 */
    "Hiragino Sans GB", sans-serif;
}
```

### 字体使用规则

- **Display + Body**:`font-sans`(Geist)
- **数据 / 代码 / 数字**:`font-mono`(Geist Mono),必加 `tabular-nums`(数字等宽)
- **不引入第三方字体** — 减少 FOUT、降低 LCP

---

## 5. 间距 / 圆角 / 阴影(shadcn 默认)

**全部直接用 Tailwind 默认值 + shadcn `--radius: 0.5rem`(8px)**。

无需在 DESIGN.md 列出 — 使用时直接调:

```tsx
// 间距:Tailwind 默认(gap-2 = 8px / p-4 = 16px / space-y-6 = 24px ...)
<div className="space-y-4 p-6 rounded-lg border">...</div>

// 圆角:shadcn 自动从 --radius 计算
//   rounded-sm = calc(var(--radius) - 4px) = 4px
//   rounded-md = calc(var(--radius) - 2px) = 6px
//   rounded-lg = var(--radius)             = 8px
//   rounded    = 0.25rem(Tailwind 默认 4px)

// 阴影:Tailwind 默认 shadow-sm / shadow / shadow-md / shadow-lg
```

---

## 6. 组件(shadcn/ui 全套)

### 6.1 推荐安装清单

按 idcd 控制台需要的组件:

```bash
# 基础
pnpm dlx shadcn@latest add button card badge alert input label
pnpm dlx shadcn@latest add tabs select dropdown-menu

# 数据
pnpm dlx shadcn@latest add table pagination

# 反馈
pnpm dlx shadcn@latest add toast dialog sheet popover tooltip
pnpm dlx shadcn@latest add progress skeleton

# 表单
pnpm dlx shadcn@latest add form switch checkbox radio-group textarea
pnpm dlx shadcn@latest add command(搜索 / palette)

# 导航
pnpm dlx shadcn@latest add breadcrumb navigation-menu

# 高级
pnpm dlx shadcn@latest add chart  # 内置 Recharts wrapper
pnpm dlx shadcn@latest add data-table  # TanStack Table wrapper
pnpm dlx shadcn@latest add resizable  # 双窗格布局
```

### 6.2 idcd 特定组件(基于 shadcn 扩展,不重写)

| 组件 | 用途 | 实施方式 |
|---|---|---|
| `StatusDot` | 监控 UP/DOWN/PARTIAL 指示 | 基于 `Badge` + Tailwind |
| `MonitorTable` | 监控列表 | 基于 `data-table` + sparkline SVG |
| `VerdictSignatureBadge` | Verdict 报告签名标识 | 基于 `Badge variant=success` |
| `UsageProgressBar` | 双 progress(API + MCP units D2) | 基于 `Progress` × 2 |
| `AnchorDeviationIndicator` | Anchor 偏差告警 | 基于 `Alert variant=warning/destructive` |

**禁止**:不自己实现 shadcn 已有的组件。需要扩展时通过 composition,不通过重写。

---

## 7. 主题切换(深色 / 浅色)

```bash
pnpm add next-themes
pnpm dlx shadcn@latest add mode-toggle  # 自带 sun/moon icon 按钮
```

```tsx
// app/layout.tsx
import { ThemeProvider } from "next-themes";

<html lang="zh-CN" suppressHydrationWarning>
  <body className={`${geistSans.variable} ${geistMono.variable}`}>
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem>
      {children}
    </ThemeProvider>
  </body>
</html>
```

**默认深色模式**(技术品牌 + 开发者偏好 + dashboard 长时间观看)。

---

## 8. 数据可视化

### 8.1 图表库

- **主**:shadcn `Chart`(Recharts wrapper,与 shadcn 主题集成)
- **复杂时序 / 节点地图**:ECharts(直接用,但读 shadcn CSS 变量做配色)

### 8.2 图表配色

直接用 shadcn 内置 `--chart-1` 至 `--chart-5`:

```tsx
<LineChart>
  <Line stroke="hsl(var(--chart-1))" />
  <Line stroke="hsl(var(--chart-2))" />
</LineChart>
```

数据高亮(选中点 / 强调线)用 `--primary`:

```tsx
<Line stroke="hsl(var(--primary))" strokeWidth={2} />
```

---

## 9. idcd 关键页面 UI 规范

详 OVERVIEW §5.1 IA。每个关键页面对应 shadcn 组件组合:

| 页面 | shadcn 组件组合 |
|---|---|
| 首页 | `Button`(CTA)+ 自定义 hero 布局 |
| `/tools/*` 工具页 | `Card` + `Input` + `Button` + `Table` |
| `/verdict/<id>` 报告分享 | `Card` + `Badge variant=success` + `Alert variant=muted`(免责) |
| `/verify` 公开验签 | `Card` + 自定义 dropzone + `Badge` |
| `/leaderboard` | `data-table` + `Tabs` + `Chart` |
| `/transparency` | `Card` + `Chart` + `data-table` |
| `/app/monitors` | `data-table` + `Input` + `Button` + StatusDot + sparkline |
| `/app/verdict` | `data-table` + `Badge` + `Dialog`(下单) |
| `/app/compliance` | `Card` + `Form` + `Switch` |
| `/app/mcp` | `data-table` + `Dialog`(签发 token)+ `Tabs`(三态) |
| `/app/usage` | 双 `Progress` + `Chart` + `data-table` |
| `/admin/refund-failed`(D5) | `data-table` + `Alert variant=destructive` |

---

## 10. 反 AI slop 清单

设计实施时避免:

- ❌ 紫色 / violet 渐变(idcd 是 blue,不要混用)
- ❌ 3-column SaaS icon grid(典型 AI slop)
- ❌ 居中一切(用左对齐 + grid)
- ❌ 圆形 CTA 按钮(用 `rounded-md` / `rounded-lg`)
- ❌ 渐变填充按钮(用 `bg-primary` 纯色)
- ❌ 全 system-ui 字体(用 Geist)
- ❌ 营销大词(`Develop. Preview. Ship.` 风格 → 用 idcd `Probe. Verdict. Observe.`)
- ❌ 装饰性 blob / 模糊光斑

---

## 11. Logo(待 Phase 5 设计)

详 `01-branding.md` §3。基于本设计系统(blue + Geist + minimal)实施:

- **风格**:Geist Mono 字形 "idcd" + 单色 + 一个简单几何元素(可能是 dot 或 underline)
- **颜色**:深色背景上 white,浅色背景上 black;hover / accent 状态用 `--primary`
- **变体**:wordmark 主用,小尺寸 icon 备用

待 AI 生成 5 个草图 → 用户选定 → SVG 矢量化。

---

## 12. 决策日志

| 日期 | 决策 | 理由 |
|---|---|---|
| 2026-05-13 | 完整采用 shadcn/ui 默认体系 | 极简维护成本 + 组件 plug-and-play + 升级路径清晰 |
| 2026-05-13 | 配色用 shadcn/ui blue theme | 用户决策 |
| 2026-05-13 | 字体 Geist Sans + Geist Mono | Next.js 16 默认,shadcn 推荐,与生态绑定 |
| 2026-05-13 | 业务语义色扩展 success/warning/info | shadcn 官方只有 destructive,idcd 监控业务需 success/warning |
| 2026-05-13 | 默认深色模式 | 技术品牌 + 开发者偏好 + dashboard 长时间观看 |
| 2026-05-13 | idcd 特定组件通过 composition,不重写 | 保持 shadcn 升级兼容 |

---

## 13. 实施 checklist(M1 起步)

- [ ] `pnpm dlx shadcn@latest init`(Style: New York, Base: Blue, CSS Variables: Yes)
- [ ] 在 `globals.css` 增加业务语义色扩展(success / warning / info)
- [ ] `pnpm add next-themes`
- [ ] `pnpm dlx shadcn@latest add mode-toggle button card badge alert input table dialog`(基础包)
- [ ] `app/layout.tsx` 配 Geist + Geist_Mono + ThemeProvider(默认 dark)
- [ ] 中文字体 fallback(PingFang SC / Microsoft YaHei)写入 globals.css
- [ ] 实施 `StatusDot` / `VerdictSignatureBadge` 等 idcd 特定组件(基于 shadcn composition)
- [ ] Storybook(可选,S2 上线后)— 内部组件库 review

---

## 14. 维护与更新

- 任何配色 / 字体改动 → 更新本文件 + commit
- shadcn/ui 版本升级 → 跑 `pnpm dlx shadcn@latest add <component>` 更新各组件
- 业务语义色冲突时 → 优先保留 shadcn 官方 token,自定义放命名空间(如 `--idcd-*`)
