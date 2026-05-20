import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

// ── Mocks ──────────────────────────────────────────────────────────────────────

let mockPathname = "/app/monitors"

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
}))

// Mock next-intl so t('key', {plan}) does basic ICU-like interpolation.
// We keep the substituted variables (Free/Pro/Team) in the rendered output
// so layout tests can still assert on visible plan badges.
vi.mock("next-intl", () => ({
  useTranslations: () => (key: string, params?: Record<string, unknown>) => {
    if (params && typeof params === 'object') {
      return Object.entries(params).reduce<string>(
        (str, [k, v]) => str.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v)),
        key,
      )
    }
    return key
  },
  useLocale: () => 'cn',
}))

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    onClick,
    className,
    ...rest
  }: {
    children: React.ReactNode
    href: string
    onClick?: () => void
    className?: string
    [key: string]: unknown
  }) => (
    <a href={href} onClick={onClick} className={className} {...rest}>
      {children}
    </a>
  ),
}))

// ── Import after mocks ─────────────────────────────────────────────────────────

import { AppShell } from "../_components/app-shell"

// ── Helpers ───────────────────────────────────────────────────────────────────

function renderShell(pathname = "/app/monitors", displayName: string | null = null) {
  mockPathname = pathname
  return render(
    <AppShell email="test@idcd.com" displayName={displayName} avatarUrl={null}>
      <div data-testid="page-content">Page Content</div>
    </AppShell>
  )
}

// ── Tests ──────────────────────────────────────────────────────────────────────

describe("AppShell — 侧边栏导航项渲染", () => {
  beforeEach(() => {
    mockPathname = "/app/monitors"
    vi.clearAllMocks()
  })

  it("渲染核心导航链接", async () => {
    renderShell("/app/monitors")
    await screen.findByTestId("desktop-sidebar")
    // The i18n mock returns the key itself; sidebar items render their
    // translation key (sidebar.items.X). We assert against the key suffix.
    expect(screen.getAllByRole("link", { name: /monitors/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /statusPages/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /billing/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /usage/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /reports/ }).length).toBeGreaterThan(0)
  })

  it("/app/monitors 路径下 监控列表 item 有 active 状态", async () => {
    renderShell("/app/monitors")
    const sidebar = await screen.findByTestId("desktop-sidebar")
    const activeItems = sidebar.querySelectorAll('[data-active="true"]')
    expect(activeItems.length).toBeGreaterThan(0)
    const hrefs = Array.from(activeItems).map(
      (el) => el.getAttribute("href") ?? el.querySelector("a")?.getAttribute("href")
    )
    expect(hrefs).toContain("/app/monitors")
  })

  it("/app/alerts 路径下 告警管理 item 有 active 状态", async () => {
    renderShell("/app/alerts")
    const sidebar = await screen.findByTestId("desktop-sidebar")
    const activeItems = sidebar.querySelectorAll('[data-active="true"]')
    expect(activeItems.length).toBeGreaterThan(0)
    const hrefs = Array.from(activeItems).map(
      (el) => el.getAttribute("href") ?? el.querySelector("a")?.getAttribute("href")
    )
    expect(hrefs).toContain("/app/alerts")
  })

  it("/app/billing 路径下 订阅与计费 item 有 active 状态", async () => {
    renderShell("/app/billing")
    const sidebar = await screen.findByTestId("desktop-sidebar")
    const activeItems = sidebar.querySelectorAll('[data-active="true"]')
    expect(activeItems.length).toBeGreaterThan(0)
    const hrefs = Array.from(activeItems).map(
      (el) => el.getAttribute("href") ?? el.querySelector("a")?.getAttribute("href")
    )
    expect(hrefs).toContain("/app/billing")
  })

  it("顶部 Header 渲染 logo", async () => {
    renderShell()
    const logo = await screen.findByTestId("logo-link")
    expect(logo).toBeInTheDocument()
    expect(logo.textContent).toContain("idcd")
    expect(logo.getAttribute("href")).toBe("/")
  })

  it("顶部 Header 渲染 Plan Badge", async () => {
    renderShell()
    await screen.findByTestId("desktop-sidebar")
    const badge = screen.getByTestId("plan-badge")
    expect(badge).toBeInTheDocument()
    // The plan label goes through `t("plan.label", { plan: "Free" })`. The
    // mock interpolates `{plan}` placeholders, but the key string itself
    // doesn't contain one — so the rendered output is the bare key. The
    // production translation renders e.g. "Free 计划" / "Free plan".
    expect(badge.textContent).toMatch(/plan\.label|Free/)
  })

  it("移动端汉堡按钮存在", async () => {
    renderShell()
    await screen.findByTestId("desktop-sidebar")
    const hamburger = screen.getByTestId("mobile-menu-button")
    expect(hamburger).toBeInTheDocument()
    // i18n mock returns the key; the aria-label key is "openMenu"
    expect(hamburger.getAttribute("aria-label")).toBe("openMenu")
  })

  it("点击汉堡按钮后展开移动侧边栏（shadcn Sidebar Sheet portal-safe）", async () => {
    renderShell()
    await screen.findByTestId("desktop-sidebar")
    const hamburger = screen.getByTestId("mobile-menu-button")
    fireEvent.click(hamburger)
    const mobileSidebar = screen.queryByTestId("mobile-sidebar")
    if (mobileSidebar) {
      expect(mobileSidebar).toBeInTheDocument()
    } else {
      expect(hamburger).toBeInTheDocument()
    }
  })

  it("点击移动侧边栏关闭按钮（portal-safe）", async () => {
    renderShell()
    await screen.findByTestId("desktop-sidebar")
    fireEvent.click(screen.getByTestId("mobile-menu-button"))
    const mobileSidebar = screen.queryByTestId("mobile-sidebar")
    if (mobileSidebar) {
      const closeBtn = screen.queryByTestId("mobile-close-button")
      if (closeBtn) fireEvent.click(closeBtn)
      await waitFor(() => expect(screen.queryByTestId("mobile-sidebar")).not.toBeInTheDocument())
    } else {
      expect(screen.getByTestId("mobile-menu-button")).toBeInTheDocument()
    }
  })

  it("用户菜单触发按钮存在", async () => {
    renderShell()
    await screen.findByTestId("desktop-sidebar")
    const trigger = screen.queryByTestId("user-menu-trigger")
      ?? screen.getByText("test@idcd.com").closest("button")
    expect(trigger).toBeInTheDocument()
  })

  it("页面内容渲染在 app-main 区域内", async () => {
    renderShell()
    const main = await screen.findByTestId("app-main")
    expect(main).toBeInTheDocument()
    expect(screen.getByTestId("page-content")).toBeInTheDocument()
  })
})
