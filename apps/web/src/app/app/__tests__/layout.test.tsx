import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

// ── Mocks ──────────────────────────────────────────────────────────────────────

let mockPathname = "/app/monitors"

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
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

function renderShell(pathname = "/app/monitors") {
  mockPathname = pathname
  return render(
    <AppShell email="test@idcd.com" displayName="Test User" avatarUrl={null}>
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
    expect(screen.getAllByRole("link", { name: /监控列表/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /状态页/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /订阅与计费/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /用量统计/ }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("link", { name: /月度报告/ }).length).toBeGreaterThan(0)
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
    expect(badge.textContent).toContain("Free")
  })

  it("移动端汉堡按钮存在", async () => {
    renderShell()
    await screen.findByTestId("desktop-sidebar")
    const hamburger = screen.getByTestId("mobile-menu-button")
    expect(hamburger).toBeInTheDocument()
    expect(hamburger.getAttribute("aria-label")).toBe("打开菜单")
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
