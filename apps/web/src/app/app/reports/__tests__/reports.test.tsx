import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/app/reports",
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

import ReportsPage from "../page"

describe("ReportsPage — SLA 月报", () => {
  it("渲染不崩溃并显示页面标题", () => {
    render(<ReportsPage />)
    expect(screen.getByText("SLA 月报")).toBeInTheDocument()
  })

  it("显示所有监控名称", () => {
    render(<ReportsPage />)
    expect(screen.getByText("API 网关健康检查")).toBeInTheDocument()
    expect(screen.getByText("idcd.com 主站")).toBeInTheDocument()
    expect(screen.getByText("数据库心跳")).toBeInTheDocument()
    expect(screen.getByText("SSL 证书到期监控")).toBeInTheDocument()
  })

  it("颜色 Badge 逻辑：99.9%+ 显示绿色", () => {
    render(<ReportsPage />)
    const successBadges = screen.getAllByTestId("badge-success")
    expect(successBadges.length).toBeGreaterThan(0)
  })

  it("颜色 Badge 逻辑：<99% 显示红色", () => {
    render(<ReportsPage />)
    const destructiveBadges = screen.getAllByTestId("badge-destructive")
    expect(destructiveBadges.length).toBeGreaterThan(0)
  })

  it("颜色 Badge 逻辑：99% - 99.9% 之间显示黄色", () => {
    render(<ReportsPage />)
    const warningBadges = screen.getAllByTestId("badge-warning")
    expect(warningBadges.length).toBeGreaterThan(0)
  })

  it("显示月份选择器", () => {
    render(<ReportsPage />)
    expect(screen.getByTestId("months-select")).toBeInTheDocument()
  })

  it("表格包含正确的列标题", () => {
    render(<ReportsPage />)
    const monthHeaders = screen.getAllByText("月份")
    expect(monthHeaders.length).toBeGreaterThan(0)
    const uptimeHeaders = screen.getAllByText("在线率")
    expect(uptimeHeaders.length).toBeGreaterThan(0)
  })

  it("显示监控类型 Badge", () => {
    render(<ReportsPage />)
    expect(screen.getByText("http")).toBeInTheDocument()
    expect(screen.getByText("https")).toBeInTheDocument()
    expect(screen.getByText("tcp")).toBeInTheDocument()
  })
})
