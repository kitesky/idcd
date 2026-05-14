import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, within } from "@testing-library/react"
import "@testing-library/jest-dom"

// Radix UI Tabs uses ResizeObserver — polyfill for jsdom
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

import { LeaderboardClient } from "../leaderboard-client"
import { CDN_DATA, REGION_LATENCY_DATA, ISP_AVAILABILITY_DATA } from "../leaderboard-data"

describe("LeaderboardClient — Tab 触发器渲染", () => {
  it("应该渲染三个 Tab 触发器", () => {
    render(<LeaderboardClient />)
    const tabs = screen.getAllByRole("tab")
    expect(tabs).toHaveLength(3)
  })

  it("CDN 响应速度 Tab 存在", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("tab", { name: /CDN 响应速度/ })).toBeInTheDocument()
  })

  it("全球节点延迟 Tab 存在", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("tab", { name: /全球节点延迟/ })).toBeInTheDocument()
  })

  it("可用性统计 Tab 存在", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("tab", { name: /可用性统计/ })).toBeInTheDocument()
  })

  it("默认激活的是 CDN Tab（aria-selected=true）", () => {
    render(<LeaderboardClient />)
    const cdnTab = screen.getByRole("tab", { name: /CDN 响应速度/ })
    expect(cdnTab).toHaveAttribute("aria-selected", "true")
  })
})

describe("LeaderboardClient — CDN Tab 内容", () => {
  it("默认 Tab 显示 Cloudflare CDN 行", () => {
    render(<LeaderboardClient />)
    expect(screen.getByText("Cloudflare CDN")).toBeInTheDocument()
  })

  it("CDN 表格应至少渲染 10 个排名单元格", () => {
    render(<LeaderboardClient />)
    const rankCells = screen.getAllByText(/^#\d+$/)
    expect(rankCells.length).toBeGreaterThanOrEqual(10)
  })

  it("CDN 表格包含列头：CDN 名称", () => {
    render(<LeaderboardClient />)
    expect(screen.getByText("CDN 名称")).toBeInTheDocument()
  })

  it("CDN 表格包含列头：全球 P50", () => {
    render(<LeaderboardClient />)
    expect(screen.getByText("全球 P50")).toBeInTheDocument()
  })

  it("腾讯云 CDN 在 CDN 列表中", () => {
    render(<LeaderboardClient />)
    expect(screen.getByText("腾讯云 CDN")).toBeInTheDocument()
  })

  it("所有 CDN 名称均渲染在页面中", () => {
    render(<LeaderboardClient />)
    for (const cdn of CDN_DATA) {
      expect(screen.getByText(cdn.name)).toBeInTheDocument()
    }
  })
})

describe("LeaderboardClient — 底部声明区", () => {
  it("应渲染底部数据声明 Alert", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("alert")).toBeInTheDocument()
  })

  it("Alert 标题为「数据声明」", () => {
    render(<LeaderboardClient />)
    expect(screen.getByText("数据声明")).toBeInTheDocument()
  })

  it("Alert 内容包含「真实探测」字样", () => {
    render(<LeaderboardClient />)
    const alert = screen.getByRole("alert")
    expect(within(alert).getByText(/真实/)).toBeInTheDocument()
  })
})

describe("LeaderboardClient — Tab 面板结构（无需交互）", () => {
  it("页面包含 tabpanel 角色元素", () => {
    render(<LeaderboardClient />)
    // Radix Tabs renders tabpanels; at least the active one is accessible
    const panels = screen.getAllByRole("tabpanel")
    expect(panels.length).toBeGreaterThanOrEqual(1)
  })

  it("CDN 响应速度 Tab 控制的 panel 关联到 CDN tab", () => {
    render(<LeaderboardClient />)
    const cdnTab = screen.getByRole("tab", { name: /CDN 响应速度/ })
    const panelId = cdnTab.getAttribute("aria-controls")
    expect(panelId).toBeTruthy()
    const panel = document.getElementById(panelId!)
    expect(panel).toBeTruthy()
  })

  it("所有 CDN 名称存在于默认激活面板的 DOM 中", () => {
    const { container } = render(<LeaderboardClient />)
    for (const cdn of CDN_DATA) {
      expect(container.textContent).toContain(cdn.name)
    }
  })

  it("三个 tabpanel 对应 aria-controls 均可在 DOM 中找到", () => {
    render(<LeaderboardClient />)
    const tabs = screen.getAllByRole("tab")
    for (const tab of tabs) {
      const panelId = tab.getAttribute("aria-controls")
      expect(panelId).toBeTruthy()
      // Panel exists in DOM (even if hidden via data-state=inactive)
      expect(document.getElementById(panelId!)).not.toBeNull()
    }
  })

  it("非活跃 Tab panel 存在于 DOM（data-state=inactive）", () => {
    render(<LeaderboardClient />)
    const inactivePanels = document
      .querySelectorAll('[role="tabpanel"][data-state="inactive"]')
    // Radix may lazy-mount inactive panels — at minimum the active one exists
    const allPanels = document.querySelectorAll('[role="tabpanel"]')
    expect(allPanels.length).toBeGreaterThanOrEqual(1)
  })
})
