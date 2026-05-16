import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
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

const MOCK_SLA_ENTRIES = [
  {
    monitor_id: "mon-001",
    monitor_name: "API 网关健康检查",
    uptime_percent: 99.95,
    total_checks: 4320,
    failed_checks: 2,
    period_start: "2026-03-01T00:00:00Z",
    period_end: "2026-03-31T23:59:59Z",
  },
  {
    monitor_id: "mon-001",
    monitor_name: "API 网关健康检查",
    uptime_percent: 100.0,
    total_checks: 4320,
    failed_checks: 0,
    period_start: "2026-04-01T00:00:00Z",
    period_end: "2026-04-30T23:59:59Z",
  },
  {
    monitor_id: "mon-001",
    monitor_name: "API 网关健康检查",
    uptime_percent: 99.72,
    total_checks: 2160,
    failed_checks: 6,
    period_start: "2026-05-01T00:00:00Z",
    period_end: "2026-05-14T23:59:59Z",
  },
  {
    monitor_id: "mon-002",
    monitor_name: "idcd.com 主站",
    uptime_percent: 100.0,
    total_checks: 8640,
    failed_checks: 0,
    period_start: "2026-03-01T00:00:00Z",
    period_end: "2026-03-31T23:59:59Z",
  },
  {
    monitor_id: "mon-002",
    monitor_name: "idcd.com 主站",
    uptime_percent: 99.98,
    total_checks: 8640,
    failed_checks: 2,
    period_start: "2026-04-01T00:00:00Z",
    period_end: "2026-04-30T23:59:59Z",
  },
  {
    monitor_id: "mon-002",
    monitor_name: "idcd.com 主站",
    uptime_percent: 99.91,
    total_checks: 4320,
    failed_checks: 4,
    period_start: "2026-05-01T00:00:00Z",
    period_end: "2026-05-14T23:59:59Z",
  },
  {
    monitor_id: "mon-003",
    monitor_name: "数据库心跳",
    uptime_percent: 98.61,
    total_checks: 8640,
    failed_checks: 120,
    period_start: "2026-03-01T00:00:00Z",
    period_end: "2026-03-31T23:59:59Z",
  },
  {
    monitor_id: "mon-003",
    monitor_name: "数据库心跳",
    uptime_percent: 99.54,
    total_checks: 8640,
    failed_checks: 40,
    period_start: "2026-04-01T00:00:00Z",
    period_end: "2026-04-30T23:59:59Z",
  },
  {
    monitor_id: "mon-004",
    monitor_name: "SSL 证书到期监控",
    uptime_percent: 100.0,
    total_checks: 1440,
    failed_checks: 0,
    period_start: "2026-03-01T00:00:00Z",
    period_end: "2026-03-31T23:59:59Z",
  },
]

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ data: { entries: MOCK_SLA_ENTRIES } }),
      }),
    ),
  )
})

import ReportsPage from "../page"

describe("ReportsPage — SLA 月报", () => {
  it("渲染不崩溃并显示页面标题", async () => {
    render(<ReportsPage />)
    expect(screen.getByText("SLA 月报")).toBeInTheDocument()
  })

  it("显示所有监控名称", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByText("API 网关健康检查")).toBeInTheDocument()
    })
    expect(screen.getByText("idcd.com 主站")).toBeInTheDocument()
    expect(screen.getByText("数据库心跳")).toBeInTheDocument()
    expect(screen.getByText("SSL 证书到期监控")).toBeInTheDocument()
  })

  it("颜色 Badge 逻辑：99.9%+ 显示绿色", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      const successBadges = screen.getAllByTestId("badge-success")
      expect(successBadges.length).toBeGreaterThan(0)
    })
  })

  it("颜色 Badge 逻辑：<99% 显示红色", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      const destructiveBadges = screen.getAllByTestId("badge-destructive")
      expect(destructiveBadges.length).toBeGreaterThan(0)
    })
  })

  it("颜色 Badge 逻辑：99% - 99.9% 之间显示黄色", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      const warningBadges = screen.getAllByTestId("badge-warning")
      expect(warningBadges.length).toBeGreaterThan(0)
    })
  })

  it("显示月份选择器", () => {
    render(<ReportsPage />)
    expect(screen.getByTestId("months-select")).toBeInTheDocument()
  })

  it("表格包含正确的列标题", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      const monthHeaders = screen.getAllByText("月份")
      expect(monthHeaders.length).toBeGreaterThan(0)
    })
    const uptimeHeaders = screen.getAllByText("在线率")
    expect(uptimeHeaders.length).toBeGreaterThan(0)
  })

  it("空数据时显示 empty state", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ data: { entries: [] } }),
        }),
      ),
    )
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("reports-empty")).toBeInTheDocument()
    })
  })

  it("API 错误时显示错误 Alert", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          statusText: "Internal Server Error",
          json: () => Promise.resolve({ error: { message: "服务器错误" } }),
        }),
      ),
    )
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("reports-error")).toBeInTheDocument()
    })
  })
})

// ── Noise analysis Tab tests ───────────────────────────────────────────────

const MOCK_NOISE_DATA = {
  period: { from: "2024-01-01", to: "2024-01-07" },
  total_firings: 15,
  total_flaps: 3,
  noisiest_monitors: [
    { monitor_id: "mon_001", firings: 10, flaps: 2 },
    { monitor_id: "mon_002", firings: 5, flaps: 1 },
  ],
  daily_trend: [{ date: "2024-01-07", firings: 3, flaps: 0 }],
}

describe("告警噪音分析 Tab", () => {
  function mockFetchForNoise() {
    vi.stubGlobal(
      "fetch",
      vi.fn((url: string) => {
        if (url.includes("/v1/reports/alert-noise")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ data: MOCK_NOISE_DATA }),
          })
        }
        // SLA tab default
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ data: { entries: MOCK_SLA_ENTRIES } }),
        })
      }),
    )
  }

  it("切换到噪音 tab 显示分析内容", async () => {
    mockFetchForNoise()
    render(<ReportsPage />)

    // Click the noise tab trigger (Radix Tabs responds to mouseDown)
    const noiseTab = screen.getByRole("tab", { name: "告警噪音分析" })
    fireEvent.mouseDown(noiseTab)

    await waitFor(() => {
      expect(screen.getByTestId("noise-tab")).toBeInTheDocument()
    })
    // Summary cards should appear
    expect(screen.getByText("总触发次数")).toBeInTheDocument()
    expect(screen.getByText("总抖动次数")).toBeInTheDocument()
  })

  it("显示 top 噪音监控列表", async () => {
    mockFetchForNoise()
    render(<ReportsPage />)

    const noiseTab = screen.getByRole("tab", { name: "告警噪音分析" })
    fireEvent.mouseDown(noiseTab)

    await waitFor(() => {
      expect(screen.getByTestId("noise-row-mon_001")).toBeInTheDocument()
    })
    expect(screen.getByTestId("noise-row-mon_002")).toBeInTheDocument()
    // monitor IDs visible in the table
    expect(screen.getByText("mon_001")).toBeInTheDocument()
    expect(screen.getByText("mon_002")).toBeInTheDocument()
  })

  it("空状态显示无告警噪音", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((url: string) => {
        if (url.includes("/v1/reports/alert-noise")) {
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve({
                data: {
                  ...MOCK_NOISE_DATA,
                  noisiest_monitors: [],
                  daily_trend: [],
                },
              }),
          })
        }
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ data: { entries: MOCK_SLA_ENTRIES } }),
        })
      }),
    )
    render(<ReportsPage />)

    const noiseTab = screen.getByRole("tab", { name: "告警噪音分析" })
    fireEvent.mouseDown(noiseTab)

    await waitFor(() => {
      expect(screen.getByTestId("noise-empty")).toBeInTheDocument()
    })
    expect(screen.getByText("近期无告警噪音")).toBeInTheDocument()
  })
})
