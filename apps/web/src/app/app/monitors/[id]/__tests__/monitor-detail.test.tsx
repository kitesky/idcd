import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import "@testing-library/jest-dom"
import type { Monitor } from "../../types"

// Mock next-intl — return the key itself so tests are locale-agnostic
vi.mock("next-intl", () => ({
  useTranslations: () => (key: string, params?: Record<string, unknown>) => {
    if (params) {
      return Object.entries(params).reduce(
        (acc, [k, v]) => acc.replace(`{${k}}`, String(v)),
        key,
      )
    }
    return key
  },
  useLocale: () => "zh",
}))

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

    // Find the edit button by its text (i18n key: actions.edit)
    const editButton = screen.getByRole("button", { name: /actions\.edit/ })
    expect(editButton).toBeInTheDocument()

    fireEvent.click(editButton)

    // Dialog title should appear (i18n key: edit.title)
    await waitFor(() => {
      expect(screen.getByText("edit.title")).toBeInTheDocument()
    })

    // Input field is pre-filled with monitor name
    expect(screen.getByDisplayValue("idcd.com 主站")).toBeInTheDocument()
  })

  it("点击删除按钮打开确认 Dialog", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    const deleteButton = screen.getByRole("button", { name: /actions\.delete/ })
    expect(deleteButton).toBeInTheDocument()

    fireEvent.click(deleteButton)

    // AlertDialog confirmation text should appear (i18n key: confirm.deleteTitle)
    await waitFor(() => {
      expect(screen.getByText("confirm.deleteTitle")).toBeInTheDocument()
    })

    // The monitor name is mentioned in the dialog description
    const matches = screen.getAllByText(/idcd\.com 主站/)
    expect(matches.length).toBeGreaterThan(0)

    // Confirm and cancel buttons should be present
    expect(screen.getByRole("button", { name: "confirm.confirmDelete" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "bulk.cancel" })).toBeInTheDocument()
  })

  it("告警历史区块存在", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    // The alert-history-section card should be in the DOM immediately
    expect(screen.getByTestId("alert-history-section")).toBeInTheDocument()

    // After loading, should show empty state (i18n key: detail.noAlertHistory)
    await waitFor(() => {
      expect(screen.getByText("detail.noAlertHistory")).toBeInTheDocument()
    })
  })

  it("告警策略按钮链接正确", async () => {
    render(<MonitorDetailClient monitor={MOCK_MONITOR} monitorId="mon_001" />)

    // Wait for async effects to settle, then assert on the link
    await waitFor(() => {
      const alertPolicyLink = screen.getByRole("link", { name: /actions\.alertPolicy/ })
      expect(alertPolicyLink).toBeInTheDocument()
      expect(alertPolicyLink.getAttribute("href")).toContain("/app/alerts")
    })
  })
})
