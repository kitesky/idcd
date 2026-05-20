# Design System — idcd

> 状态: **已更新(2026-05-15)**  
> 关联: `01-branding.md`(品牌) / `14-tech-architecture.md`(技术栈)

---

## 1. 核心原则

> **完整采用 shadcn/ui 官方组件体系 + Tailwind v4 OKLCH 主题系统。**

idcd 不自定义间距、字体、圆角、组件 token。所有这些**直接采用 shadcn/ui 默认值**。  
设计系统的唯一自由度是：修改 `src/styles/theme.css` 里的 OKLCH 值即可改变整体风格。

理由：
- 极简维护成本（shadcn 官方维护 token，我们不重复）
- 组件 plug-and-play（`pnpm dlx shadcn add button` 即可，无需 patch）
- 升级路径清晰（shadcn 更新时，直接 pull 官方）
- 与 Next.js 16 + Tailwind v4 生态默认绑定

---

## 2. 技术栈

| 维度 | 选型 |
|---|---|
| 框架 | Next.js 16 (App Router) |
| UI 库 | shadcn/ui (new-york style) |
| 底层 | Radix UI primitives |
| CSS | Tailwind CSS v4 |
| 色彩空间 | **OKLCH**（Tailwind v4 标准，非 HSL） |
| 字体 | Geist Sans + Geist Mono |
| 包管理 | pnpm |
| 图表 | Recharts（shadcn Chart wrapper） |

---

## 3. 主题系统（唯一入口：`src/styles/theme.css`）

### 3.1 文件结构

```
frontend/src/
├── app/
│   └── globals.css          ← CSS 入口（@import theme.css，@layer base 等）
└── styles/
    └── theme.css            ← 唯一主题配置文件（改这里换主题）
```

### 3.2 `theme.css` 结构

```css
/* 亮色模式变量（OKLCH 格式） */
:root {
  --radius: 0.5rem;
  --primary: oklch(0.21 0.006 285.885);   /* ← 改这里换主色 */
  --background: oklch(1 0 0);
  /* ... 其他 token ... */
}

/* 暗色模式覆盖 */
.dark {
  --background: oklch(0.141 0.005 285.823);
  --primary: oklch(0.92 0.004 286.32);
  /* ... */
}

/* Tailwind v4 token 映射（使 bg-primary、text-foreground 等 class 生效） */
@theme inline {
  --color-primary: var(--primary);
  --color-background: var(--background);
  /* ... */
}
```

### 3.3 当前默认主题

| 参数 | 值 |
|---|---|
| Style | new-york |
| Base Color | **Zinc**（偏蓝灰，深色模式效果最佳） |
| 色彩空间 | OKLCH |
| 默认模式 | 亮色（`defaultTheme="light"`），用户可手动切换暗色或跟随系统 |

### 3.4 修改主题的方法

**只需修改 `src/styles/theme.css`**：
1. 改 `:root` 里的 OKLCH 值 → 改亮色主题
2. 改 `.dark` 里的 OKLCH 值 → 改暗色主题
3. 改 `--radius` → 改全局圆角
4. 刷新浏览器即可看到效果，无需重启

**从官方获取预设主题**：访问 https://ui.shadcn.com/create 选择配色后「Get Code」复制 CSS 变量。

---

## 4. 颜色 Token 系统

### 4.1 标准 token 对（背景/前景配对）

| Token | 用途 |
|---|---|
| `background` / `foreground` | 页面底色和正文 |
| `card` / `card-foreground` | 卡片、面板 |
| `popover` / `popover-foreground` | 弹层、下拉菜单 |
| `primary` / `primary-foreground` | 主色、品牌色、CTA 按钮 |
| `secondary` / `secondary-foreground` | 次色、辅助按钮 |
| `muted` / `muted-foreground` | 弱化文字、描述、占位符 |
| `accent` / `accent-foreground` | 悬停/激活高亮 |
| `destructive` | 危险操作、删除、错误 |
| `border` | 默认边框 |
| `input` | 表单控件边框 |
| `ring` | 焦点环 |

### 4.2 Sidebar 专属 token

| Token | 用途 |
|---|---|
| `sidebar` / `sidebar-foreground` | 侧边栏背景和文字 |
| `sidebar-primary` / `sidebar-primary-foreground` | 侧边栏主色 |
| `sidebar-accent` / `sidebar-accent-foreground` | 侧边栏悬停高亮 |
| `sidebar-border` / `sidebar-ring` | 侧边栏边框 |

### 4.3 idcd 业务语义色扩展

在 `theme.css` 的 `:root`/`.dark` 和 `@theme inline` 中定义：

| Token | 亮色 OKLCH | 暗色 OKLCH | 用途 |
|---|---|---|---|
| `success` | `oklch(0.527 0.154 150)` | `oklch(0.627 0.194 146)` | 监控 UP / verified |
| `warning` | `oklch(0.769 0.188 70)` | `oklch(0.828 0.189 84)` | 监控 DEGRADED |
| `info` | `oklch(0.623 0.214 260)` | `oklch(0.623 0.214 260)` | MCP token / 信息提示 |

### 4.4 图表调色盘

使用 `--chart-1` 至 `--chart-5`，在 `theme.css` 统一定义：

```tsx
// 在 Recharts 组件中引用
<Line stroke="var(--chart-1)" />
<Bar fill="var(--chart-2)" />
```

---

## 5. 字体

- **Display + Body**: Geist Sans（`--font-sans`）
- **代码 / 数字 / 等宽**: Geist Mono（`--font-mono`），配合 `tabular-nums`
- **中文 fallback**: PingFang SC → Microsoft YaHei → Hiragino Sans GB

---

## 6. 组件使用规则

### 已安装组件（33 个）

**基础**: Button, Input, Label, Textarea, Badge, Separator, Skeleton, Progress, Slider  
**卡片**: Card (+ Header/Footer/Title/Description/Content), Avatar  
**表单**: Form, Checkbox, Select, Switch, RadioGroup, InputOTP  
**导航**: Breadcrumb, Tabs, Sidebar (+ 所有子组件)  
**反馈**: Alert, AlertDialog, Dialog, Sheet, Sonner (Toast), Tooltip, Popover  
**数据**: Table, ScrollArea, Command  
**折叠**: Collapsible, DropdownMenu  

### 使用强制规则

| 场景 | 正确做法 | 禁止 |
|---|---|---|
| 按钮 | `<Button>` | `<div onClick>` 或裸 `<button>` |
| 卡片面板 | `<Card>` | `<div className="rounded border">` |
| 表单字段 | `<Form>` + `<FormField>` + `<Input>` | 裸 `<input>` |
| 弹窗 | `<Dialog>` / `<AlertDialog>` | 自写 overlay |
| 抽屉/移动菜单 | `<Sheet>` | 自写 fixed panel |
| 后台侧边栏 | `<Sidebar>` + `<SidebarProvider>` | 自写 aside + state |
| 面包屑 | `<Breadcrumb>` | 自写 span 链接 |
| Toast 通知 | `<Sonner>` / `<Toaster>` | 自写 toast |

**允许裸 div 的唯一情形**：纯布局容器（flex/grid wrapper），无视觉样式（无 border/bg/shadow/rounded）。

---

## 7. 后台布局（`/app/*`）

使用 shadcn 官方 `Sidebar` 组件：

```tsx
// frontend/src/app/app/layout.tsx
<SidebarProvider>
  <AppSidebar email={email} plan={plan} />
  <SidebarInset>
    <header>  {/* SidebarTrigger + Breadcrumb */}
    <main>{children}</main>
  </SidebarInset>
</SidebarProvider>
```

关键组件文件：
- `src/components/layout/app-sidebar.tsx` — 侧边栏入口
- `src/components/layout/nav-group.tsx` — 导航分组（支持折叠子菜单）
- `src/components/layout/nav-user.tsx` — 底部用户菜单
- `src/components/layout/sidebar-data.ts` — 导航配置（改这里加减菜单项）

---

## 8. 主题切换

由 `next-themes` 控制，支持亮色/暗色切换：

```tsx
// 在任意 Client Component 中
import { useTheme } from "next-themes"
const { theme, setTheme } = useTheme()
setTheme("dark")   // 切换暗色
setTheme("light")  // 切换亮色
setTheme("system") // 跟随系统
```

---

## 9. 数据可视化

- 主：shadcn `Chart`（Recharts wrapper，与 shadcn 主题集成）
- 复杂时序/地图：ECharts（读 CSS 变量配色）

图表颜色引用：
```tsx
// 不要硬编码颜色，要引用 CSS 变量
stroke="var(--chart-1)"    // ✅
stroke="#3B82F6"            // ❌
```

---

## 10. 反 AI slop 清单

- ❌ 紫色/violet 渐变（idcd 是 zinc 中性色）
- ❌ 3-column SaaS icon grid
- ❌ 居中一切（用左对齐 + grid）
- ❌ 圆形 CTA 按钮（用 `rounded-md`）
- ❌ 渐变填充按钮（用 `bg-primary` 纯色）
- ❌ 硬编码颜色值（用 CSS 变量）

---

## 11. 决策日志

| 日期 | 决策 | 理由 |
|---|---|---|
| 2026-05-13 | 完整采用 shadcn/ui 官方体系 | 极简维护 + plug-and-play + 升级路径清晰 |
| 2026-05-13 | 默认深色模式 | 技术品牌 + 开发者偏好 + dashboard 长时间观看 |
| 2026-05-17 | 默认改回亮色模式 | 公开页 / 营销页对新访客可读性更高；用户仍可手动切换暗色或跟随系统 |
| 2026-05-13 | idcd 特定组件通过 composition，不重写 | 保持 shadcn 升级兼容 |
| 2026-05-15 | 迁移到 OKLCH 色彩空间 | Tailwind v4 标准，感知均匀，支持透明度 |
| 2026-05-15 | Base Color 改为 Zinc（原 Blue） | 深色模式偏蓝灰更自然，shadcn-admin 同款 |
| 2026-05-15 | 主题配置集中到 theme.css | 单一入口，改一处换全局 |
| 2026-05-15 | /app/* 后台使用 shadcn Sidebar 组件 | 替代手写 div 侧边栏，collapsible + icon mode |
| 2026-05-15 | 删除 tailwind.config.ts | Tailwind v4 用 @theme inline，不需要 config 文件 |
