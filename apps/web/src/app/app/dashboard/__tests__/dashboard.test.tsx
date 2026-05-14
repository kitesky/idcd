import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
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

import DashboardPage from "../page"

describe("DashboardPage — 渲染", () => {
  it("渲染不崩溃并显示页面标题", () => {
    render(<DashboardPage />)
    expect(screen.getByText("总览")).toBeInTheDocument()
  })

  it("6 个统计卡片都存在", () => {
    render(<DashboardPage />)
    expect(screen.getByTestId("stat-monitors-total")).toBeInTheDocument()
    expect(screen.getByTestId("stat-monitors-up")).toBeInTheDocument()
    expect(screen.getByTestId("stat-checks-today")).toBeInTheDocument()
    expect(screen.getByTestId("stat-uptime-7d")).toBeInTheDocument()
    expect(screen.getByTestId("stat-incidents-open")).toBeInTheDocument()
    expect(screen.getByTestId("stat-status-pages")).toBeInTheDocument()
  })

  it("快捷入口链接指向正确路径", () => {
    render(<DashboardPage />)
    const newMonitorLink = screen.getByTestId("link-new-monitor")
    expect(newMonitorLink).toHaveAttribute("href", "/app/monitors/new")

    const alertsLink = screen.getByTestId("link-alerts")
    expect(alertsLink).toHaveAttribute("href", "/app/alerts")

    const statusPagesLink = screen.getByTestId("link-status-pages")
    expect(statusPagesLink).toHaveAttribute("href", "/app/status-pages")
  })

  it("近期告警事件表格渲染", () => {
    render(<DashboardPage />)
    expect(screen.getByTestId("recent-alerts-table")).toBeInTheDocument()
    expect(screen.getByText("时间")).toBeInTheDocument()
    expect(screen.getByText("监控名")).toBeInTheDocument()
    expect(screen.getByText("状态")).toBeInTheDocument()
    expect(screen.getByText("通道")).toBeInTheDocument()
  })

  it("统计卡片数值正确展示", () => {
    render(<DashboardPage />)
    expect(screen.getByText("5")).toBeInTheDocument()
    expect(screen.getByText("4 / 5")).toBeInTheDocument()
    expect(screen.getByText("99.7%")).toBeInTheDocument()
  })

  it("近期告警数据包含 API 网关健康检查", () => {
    render(<DashboardPage />)
    const rows = screen.getAllByText("API 网关健康检查")
    expect(rows.length).toBeGreaterThan(0)
  })

  it("快捷入口区域渲染标题", () => {
    render(<DashboardPage />)
    expect(screen.getByText("快捷入口")).toBeInTheDocument()
    expect(screen.getByText("近期告警事件")).toBeInTheDocument()
  })
})
