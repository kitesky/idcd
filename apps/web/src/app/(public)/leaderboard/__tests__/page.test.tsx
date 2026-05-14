import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"

// Tabs from Radix UI uses ResizeObserver — polyfill for jsdom
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

// Mock LeaderboardClient (uses "use client" + Radix internals)
vi.mock("../leaderboard-client", () => ({
  LeaderboardClient: () => (
    <div data-testid="leaderboard-client">
      <div>CDN 响应速度</div>
    </div>
  ),
}))

import LeaderboardPage from "../page"
import { NODE_COUNT, getCurrentMonthLabel } from "../leaderboard-data"

describe("LeaderboardPage", () => {
  it("应该渲染主标题，包含 CDN 关键词", () => {
    render(<LeaderboardPage />)
    const heading = screen.getByRole("heading", { level: 1 })
    expect(heading).toBeInTheDocument()
    expect(heading.textContent).toContain("CDN")
  })

  it("应该在副标题中显示节点数量", () => {
    render(<LeaderboardPage />)
    expect(screen.getByText(String(NODE_COUNT))).toBeInTheDocument()
  })

  it("应该渲染 LeaderboardClient 组件", () => {
    render(<LeaderboardPage />)
    expect(screen.getByTestId("leaderboard-client")).toBeInTheDocument()
  })

  it("应该渲染月份更新标注，包含当前年份", () => {
    render(<LeaderboardPage />)
    const year = new Date().getFullYear().toString()
    const monthLabel = getCurrentMonthLabel()
    const el = screen.getByText((content) => content.includes(monthLabel))
    expect(el).toBeInTheDocument()
    expect(el.textContent).toContain(year)
  })

  it("页面结构应有 main 元素包裹", () => {
    const { container } = render(<LeaderboardPage />)
    expect(container.querySelector("main")).toBeTruthy()
  })
})
