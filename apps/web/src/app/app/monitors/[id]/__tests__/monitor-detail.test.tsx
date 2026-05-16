import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import "@testing-library/jest-dom"
import type { Monitor } from "../../types"

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  usePathname: vi.fn(() => "/app/monitors/mon_001"),
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

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const MOCK_MONITOR: Monitor = {
  id: "mon_001",
  name: "idcd.com 主站",
  type: "http",
  target: "https://idcd.com",
  status: "UP",
  uptimePercent: 99.8,
  lastCheckedAt: new Date(Date.now() - 60_000).toISOString(),
  intervalSeconds: 60,
  concurrentNodes: 3,
}

function mockFetch(url: string) {
  // monitor detail
  if (url.includes("/v1/monitors/mon_001") && !url.includes("checks") && !url.includes("stream")) {
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ data: MOCK_MONITOR }),
    })
  }
  // alert events
  if (url.includes("alert-events")) {
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ data: [] }),
    })
  }
  // SSE stream — return a non-ok response so apiRequest ignores it gracefully
  if (url.includes("/stream")) {
    return Promise.resolve({
      ok: false,
      status: 404,
      statusText: "Not Found",
      json: () => Promise.resolve({}),
    })
  }
  // check buckets
  return Promise.resolve({
    ok: true,
    json: () => Promise.resolve({ data: { buckets: [] } }),
  })
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string | URL | Request) => {
      const url =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.href
            : input.url
      return mockFetch(url)
    }),
  )
})

import { MonitorDetailClient } from "../monitor-detail-client"

describe("MonitorDetailClient", () => {
  it("渲染监控名称和状态 badge", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    // monitor name renders in heading
    expect(screen.getByText("idcd.com 主站")).toBeInTheDocument()

    // Status badge — there can be multiple "UP" badges (header + stat card); at least one
    await waitFor(() => {
      const badges = screen.getAllByText("UP")
      expect(badges.length).toBeGreaterThan(0)
    })
  })

  it("点击编辑按钮打开编辑 Dialog", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    // Find the edit button by its text
    const editButton = screen.getByRole("button", { name: /编辑/ })
    expect(editButton).toBeInTheDocument()

    fireEvent.click(editButton)

    // Dialog title should appear
    await waitFor(() => {
      expect(screen.getByText("编辑监控")).toBeInTheDocument()
    })

    // Input fields are pre-filled with monitor values
    expect(screen.getByDisplayValue("idcd.com 主站")).toBeInTheDocument()
    expect(screen.getByDisplayValue("https://idcd.com")).toBeInTheDocument()
  })

  it("点击删除按钮打开确认 Dialog", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    const deleteButton = screen.getByRole("button", { name: /删除/ })
    expect(deleteButton).toBeInTheDocument()

    fireEvent.click(deleteButton)

    // AlertDialog confirmation text should appear
    await waitFor(() => {
      expect(screen.getByText("确认删除监控？")).toBeInTheDocument()
    })

    // The monitor name is mentioned in the dialog description
    const matches = screen.getAllByText(/idcd\.com 主站/)
    expect(matches.length).toBeGreaterThan(0)

    // Confirm and cancel buttons should be present
    expect(screen.getByRole("button", { name: "确认删除" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "取消" })).toBeInTheDocument()
  })

  it("告警历史区块存在", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    // The alert-history-section card should be in the DOM immediately
    expect(screen.getByTestId("alert-history-section")).toBeInTheDocument()

    // After loading, should show empty state
    await waitFor(() => {
      expect(screen.getByText("暂无告警记录")).toBeInTheDocument()
    })
  })

  it("告警策略按钮链接正确", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    // Wait for async effects to settle, then assert on the link
    await waitFor(() => {
      const alertPolicyLink = screen.getByRole("link", { name: /告警策略/ })
      expect(alertPolicyLink).toBeInTheDocument()
      expect(alertPolicyLink.getAttribute("href")).toContain("/app/alerts")
    })
  })
})
