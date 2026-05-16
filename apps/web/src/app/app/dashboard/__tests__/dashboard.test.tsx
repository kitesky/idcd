import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/app/dashboard",
}))

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string, params?: Record<string, unknown>) => {
    if (params && typeof params === 'object') {
      return Object.entries(params).reduce<string>(
        (str, [k, v]) => str.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v)),
        key
      )
    }
    return key
  },
  useLocale: () => "cn",
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

function buildFetchMock(
  overrides: {
    summary?: typeof mockSummary["data"]
    downItems?: { id: string; name: string; status: string; last_check_at?: string }[]
    alertEvents?: { id: string; monitorName: string; status: string; startedAt: string }[]
  } = {},
) {
  const summary = overrides.summary ?? mockSummary.data
  const downItems = overrides.downItems ?? []
  const alertEvents = overrides.alertEvents ?? []

  return vi.fn((url: string) => {
    if (url.includes("/v1/dashboard/summary")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ data: summary }),
      })
    }
    if (url.includes("/v1/dashboard/pins")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(mockPins),
      })
    }
    if (url.includes("status=DOWN")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ data: { items: downItems } }),
      })
    }
    if (url.includes("/v1/alert-events")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ data: { items: alertEvents } }),
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
}

beforeEach(() => {
  vi.stubGlobal("fetch", buildFetchMock())
})

import DashboardPage from "../page"

describe("DashboardPage — 真实 API 数据", () => {
  it("渲染不崩溃并显示页面标题", async () => {
    render(<DashboardPage />)
    expect(screen.getByText("title")).toBeInTheDocument()
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
    expect(screen.getByText("pinnedMonitors.title")).toBeInTheDocument()
  })

  it("置顶为空时显示空状态提示", async () => {
    render(<DashboardPage />)
    await waitFor(() => {
      expect(screen.getByTestId("pinned-empty")).toBeInTheDocument()
    })
    expect(screen.getByText("pinnedMonitors.empty")).toBeInTheDocument()
  })

  it("+ 添加按钮存在", () => {
    render(<DashboardPage />)
    expect(screen.getByTestId("open-pin-sheet")).toBeInTheDocument()
  })

  it("快捷入口区域渲染标题", () => {
    render(<DashboardPage />)
    expect(screen.getByText("quickLinks.title")).toBeInTheDocument()
  })

  it("有 down 监控时显示 down 监控快览区块", async () => {
    const downItems = [
      { id: "mon_d1", name: "故障服务 A", status: "down", last_check_at: "2024-01-07T10:00:00Z" },
    ]
    vi.stubGlobal(
      "fetch",
      buildFetchMock({
        summary: { ...mockSummary.data, monitors: { total: 4, up: 3, down: 1, paused: 0 } },
        downItems,
      }),
    )
    render(<DashboardPage />)
    await waitFor(() => {
      expect(screen.getByTestId("down-monitors-section")).toBeInTheDocument()
    })
    expect(screen.getByText("故障服务 A")).toBeInTheDocument()
  })

  it("显示近期告警事件区块", async () => {
    const alertEvents = [
      { id: "evt_1", monitorName: "主站监控", status: "firing", startedAt: "2024-01-07T08:00:00Z" },
      { id: "evt_2", monitorName: "数据库心跳", status: "resolved", startedAt: "2024-01-06T12:00:00Z" },
    ]
    vi.stubGlobal("fetch", buildFetchMock({ alertEvents }))
    render(<DashboardPage />)
    // alert-events-section is always rendered
    expect(screen.getByTestId("alert-events-section")).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.getByText("主站监控")).toBeInTheDocument()
    })
    expect(screen.getByText("数据库心跳")).toBeInTheDocument()
  })

  it("无监控时显示新用户引导 CTA", async () => {
    vi.stubGlobal(
      "fetch",
      buildFetchMock({
        summary: { ...mockSummary.data, monitors: { total: 0, up: 0, down: 0, paused: 0 } },
      }),
    )
    render(<DashboardPage />)
    await waitFor(() => {
      expect(screen.getByText("onboarding.title")).toBeInTheDocument()
    })
    // "createMonitor" button inside the CTA card
    const ctaLinks = screen.getAllByRole("link", { name: /onboarding.createMonitor/ })
    expect(ctaLinks.length).toBeGreaterThan(0)
    expect(ctaLinks[0]).toHaveAttribute("href", "/app/monitors/new")
  })
})
