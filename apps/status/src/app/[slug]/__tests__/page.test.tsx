import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, within } from "@testing-library/react"
import { StatusClient } from "../status-client"
import { MOCK_STATUS_PAGES } from "../mock-data"

// Mock generateUptimeHistory to return deterministic data.
// Dates are generated as real ISO date strings to avoid invalid calendar dates.
vi.mock("../mock-data", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../mock-data")>()
  const base = new Date("2026-02-13")
  const deterministicHistory = Array.from({ length: 90 }, (_, i) => {
    const d = new Date(base)
    d.setDate(base.getDate() - (89 - i))
    return {
      date: d.toISOString().slice(0, 10),
      status: "operational" as const,
      uptime: 99.9,
    }
  })
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
    // Use within() to scope to the monitor row, avoiding ambiguity with the
    // event description that also mentions "API 服务" in affectedServices.
    const apiMonitorRow = screen.getByTestId("monitor-row-mon-api")
    expect(within(apiMonitorRow).getByText("API 服务")).toBeInTheDocument()
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
    expect(screen.getByTestId("monitor-row-mon-web")).toBeInTheDocument()
    expect(screen.getByTestId("monitor-row-mon-api")).toBeInTheDocument()
    expect(screen.getByTestId("monitor-row-mon-auth")).toBeInTheDocument()
    expect(screen.getByTestId("monitor-row-mon-db")).toBeInTheDocument()
    expect(screen.getByTestId("monitor-row-mon-cache")).toBeInTheDocument()
    expect(screen.getByTestId("monitor-row-mon-cdn")).toBeInTheDocument()
    expect(screen.getByTestId("monitor-row-mon-dns")).toBeInTheDocument()
    expect(screen.getByTestId("monitor-row-mon-email")).toBeInTheDocument()
  })
})
