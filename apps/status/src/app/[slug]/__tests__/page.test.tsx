import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { StatusClient } from "../status-client"
import { MOCK_STATUS_PAGES } from "../mock-data"

// Mock generateUptimeHistory to return deterministic data
vi.mock("../mock-data", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../mock-data")>()
  const deterministicHistory = Array.from({ length: 90 }, (_, i) => ({
    date: `2026-02-${String(i + 1).padStart(2, "0")}`,
    status: "operational" as const,
    uptime: 99.9,
  }))
  return {
    ...actual,
    generateUptimeHistory: vi.fn(() => deterministicHistory),
  }
})

const demoData = MOCK_STATUS_PAGES.demo

describe("StatusClient", () => {
  it("renders the status page title", () => {
    render(<StatusClient data={demoData} />)
    expect(screen.getByTestId("status-title")).toHaveTextContent("acme.com 服务状态")
  })

  it("renders overall status badge as operational", () => {
    render(<StatusClient data={demoData} />)
    const overallStatus = screen.getByTestId("overall-status")
    expect(overallStatus).toBeInTheDocument()
    expect(overallStatus.textContent).toContain("全部服务正常")
  })

  it("renders all 3 service groups", () => {
    render(<StatusClient data={demoData} />)
    const groups = screen.getByTestId("service-groups")
    expect(groups).toBeInTheDocument()
    expect(screen.getByText("核心服务")).toBeInTheDocument()
    expect(screen.getByText("数据服务")).toBeInTheDocument()
    expect(screen.getByText("基础设施")).toBeInTheDocument()
  })

  it("renders monitor rows when group is expanded", () => {
    render(<StatusClient data={demoData} />)
    // Groups start expanded, so monitor rows should be visible
    expect(screen.getByTestId("monitor-row-mon-web")).toBeInTheDocument()
    expect(screen.getByText("官网 (acme.com)")).toBeInTheDocument()
    expect(screen.getByText("API 服务")).toBeInTheDocument()
  })

  it("collapses a group when its toggle is clicked", () => {
    render(<StatusClient data={demoData} />)
    const toggle = screen.getByTestId("group-toggle-group-core")
    // Initially expanded — monitor rows visible
    expect(screen.getByTestId("monitor-row-mon-web")).toBeInTheDocument()
    fireEvent.click(toggle)
    // After collapse, monitor rows should be gone
    expect(screen.queryByTestId("monitor-row-mon-web")).not.toBeInTheDocument()
  })

  it("renders 90-day uptime history grid with 90 blocks", () => {
    render(<StatusClient data={demoData} />)
    const historySection = screen.getByTestId("uptime-history")
    expect(historySection).toBeInTheDocument()
    // The grid should contain 90 day blocks
    const grid = screen.getByTestId("uptime-grid")
    expect(grid.children).toHaveLength(90)
  })

  it("renders recent events section with resolved event", () => {
    render(<StatusClient data={demoData} />)
    const eventsSection = screen.getByTestId("recent-events")
    expect(eventsSection).toBeInTheDocument()
    expect(screen.getByText("API 响应延迟升高")).toBeInTheDocument()
    expect(screen.getByText("已解决")).toBeInTheDocument()
  })

  it("renders Powered by idcd branding footer", () => {
    render(<StatusClient data={demoData} />)
    const footer = screen.getByTestId("powered-by")
    expect(footer).toBeInTheDocument()
    expect(footer.textContent).toContain("Powered by idcd")
  })

  it("renders all 8 monitor items across 3 groups", () => {
    render(<StatusClient data={demoData} />)
    // 3 monitors in core + 2 in data + 3 in infra = 8 total
    expect(screen.getByText("认证服务")).toBeInTheDocument()
    expect(screen.getByText("数据库集群")).toBeInTheDocument()
    expect(screen.getByText("缓存服务 (Redis)")).toBeInTheDocument()
    expect(screen.getByText("CDN 分发")).toBeInTheDocument()
    expect(screen.getByText("DNS 解析")).toBeInTheDocument()
    expect(screen.getByText("邮件通知")).toBeInTheDocument()
  })
})
