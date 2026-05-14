import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/app/dashboard",
}))

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: {
    children: React.ReactNode
    href: string
    [key: string]: unknown
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}))

const mockSummary = {
  data: {
    monitors: { total: 4, up: 3, down: 1, paused: 0 },
    checks_today: 500,
    avg_uptime_7d: 98.5,
    incidents_open: 1,
    alerts_fired_7d: 2,
    status_pages: 1,
  },
}

const mockPins = { data: { monitor_ids: [] } }
const mockMonitors = { data: { items: [], total: 0, page: 1, limit: 20 } }

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string) => {
      if (url.includes("/v1/dashboard/summary")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockSummary),
        })
      }
      if (url.includes("/v1/dashboard/pins")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockPins),
        })
      }
      if (url.includes("/v1/monitors")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockMonitors),
        })
      }
      return Promise.resolve({ ok: false, json: () => Promise.resolve({}) })
    })
  )
})

import DashboardPage from "../page"

describe("DashboardPage — 真实 API 数据", () => {
  it("渲染不崩溃并显示页面标题", async () => {
    render(<DashboardPage />)
    expect(screen.getByText("总览")).toBeInTheDocument()
  })

  it("6 个统计卡片都存在", async () => {
    render(<DashboardPage />)
    expect(screen.getByTestId("stat-monitors-total")).toBeInTheDocument()
    expect(screen.getByTestId("stat-monitors-up")).toBeInTheDocument()
    expect(screen.getByTestId("stat-checks-today")).toBeInTheDocument()
    expect(screen.getByTestId("stat-uptime-7d")).toBeInTheDocument()
    expect(screen.getByTestId("stat-incidents-open")).toBeInTheDocument()
    expect(screen.getByTestId("stat-status-pages")).toBeInTheDocument()
  })

  it("统计卡片从 API 读取真实数据后渲染", async () => {
    render(<DashboardPage />)
    await waitFor(() => {
      expect(screen.getByText("4")).toBeInTheDocument()
    })
    expect(screen.getByText("3 / 4")).toBeInTheDocument()
    expect(screen.getByText("98.5%")).toBeInTheDocument()
  })

  it("快捷入口链接指向正确路径", () => {
    render(<DashboardPage />)
    expect(screen.getByTestId("link-new-monitor")).toHaveAttribute("href", "/app/monitors/new")
    expect(screen.getByTestId("link-alerts")).toHaveAttribute("href", "/app/alerts")
    expect(screen.getByTestId("link-status-pages")).toHaveAttribute("href", "/app/status-pages")
  })

  it("置顶监控区域存在", () => {
    render(<DashboardPage />)
    expect(screen.getByTestId("pinned-monitors-section")).toBeInTheDocument()
    expect(screen.getByText("置顶监控")).toBeInTheDocument()
  })

  it("置顶为空时显示空状态提示", async () => {
    render(<DashboardPage />)
    await waitFor(() => {
      expect(screen.getByTestId("pinned-empty")).toBeInTheDocument()
    })
    expect(screen.getByText("暂无置顶监控，点击 + 添加")).toBeInTheDocument()
  })

  it("+ 添加按钮存在", () => {
    render(<DashboardPage />)
    expect(screen.getByTestId("open-pin-sheet")).toBeInTheDocument()
  })

  it("快捷入口区域渲染标题", () => {
    render(<DashboardPage />)
    expect(screen.getByText("快捷入口")).toBeInTheDocument()
  })
})
