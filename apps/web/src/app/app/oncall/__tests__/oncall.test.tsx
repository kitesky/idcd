import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/oncall",
  useRouter: () => ({ replace: vi.fn() }),
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

const MOCK_SCHEDULES = [
  {
    id: "sch_001",
    name: "工程师值班",
    rotation_type: "weekly",
    rotation_days: 7,
    start_date: "2024-01-01T09:00:00Z",
    team_id: "t_demo",
  },
]

const MOCK_PARTICIPANTS = [
  { id: "p1", schedule_id: "sch_001", user_id: "u_alice", email: "alice@idcd.com", order_index: 0 },
  { id: "p2", schedule_id: "sch_001", user_id: "u_bob", email: "bob@idcd.com", order_index: 1 },
  { id: "p3", schedule_id: "sch_001", user_id: "u_carol", email: "carol@idcd.com", order_index: 2 },
]

const MOCK_ALERT_EVENTS = [
  {
    id: "evt_001",
    monitor_name: "API Health",
    status: "firing",
    fired_at: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
    resolved_at: undefined,
  },
  {
    id: "evt_002",
    monitor_name: "DB Latency",
    status: "resolved",
    fired_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
    resolved_at: new Date(Date.now() - 90 * 60 * 1000).toISOString(),
  },
]

function mockFetch(path: string) {
  if (path.includes("/alert-events")) {
    // Handled via override in individual tests; default to empty
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ data: { items: [] } }),
    })
  }
  if (path.includes("/participants")) {
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ data: { participants: MOCK_PARTICIPANTS } }),
    })
  }
  if (path.includes("/v1/oncall/schedules")) {
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ data: { schedules: MOCK_SCHEDULES } }),
    })
  }
  return Promise.resolve({
    ok: true,
    json: () => Promise.resolve({}),
  })
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    return mockFetch(url)
  }))
})

import OncallPage from "../page"

describe("OncallPage", () => {
  it("renders the page title", async () => {
    render(<OncallPage />)
    expect(screen.getByTestId("oncall-title")).toHaveTextContent("On-Call 排班")
  })

  it("shows current on-call person after load", async () => {
    render(<OncallPage />)
    await waitFor(() => {
      const nameEl = screen.getByTestId("current-oncall-name")
      expect(nameEl.textContent).toBeTruthy()
    })
    const nameEl = screen.getByTestId("current-oncall-name")
    const emails = ["alice@idcd.com", "bob@idcd.com", "carol@idcd.com"]
    expect(emails.some((e) => nameEl.textContent?.includes(e))).toBe(true)
  })

  it("renders 7-day preview list after load", async () => {
    render(<OncallPage />)
    await waitFor(() => {
      expect(screen.getByTestId("preview-list")).toBeInTheDocument()
    })
    const days = screen.getAllByTestId(/^preview-day-\d$/)
    expect(days).toHaveLength(7)
  })

  it("shows hours until handoff after load", async () => {
    render(<OncallPage />)
    await waitFor(() => {
      const el = screen.getByTestId("hours-until-handoff")
      expect(el.textContent).toMatch(/距下次交班还有 \d+ 小时/)
    })
  })

  it("renders create schedule and override buttons", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("create-schedule-button")).toBeInTheDocument()
    expect(screen.getByTestId("override-button")).toBeInTheDocument()
  })

  it("current on-call card is present", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("current-oncall-card")).toBeInTheDocument()
  })

  it("schedule preview card is present", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("schedule-preview-card")).toBeInTheDocument()
  })

  // ── New tests ──────────────────────────────────────────────────────────────

  it("告警记录 Tab trigger exists", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("alerts-tab-trigger")).toBeInTheDocument()
  })

  it("clicking 告警记录 Tab shows 暂无告警记录 when API returns empty array", async () => {
    // Default mockFetch returns empty events
    render(<OncallPage />)
    // Click the alerts tab trigger to make that panel active
    fireEvent.mouseDown(screen.getByTestId("alerts-tab-trigger"))
    // Wait for async API call to resolve and component to render
    await waitFor(() => {
      expect(screen.getByTestId("no-alert-events")).toBeInTheDocument()
    })
    expect(screen.getByTestId("no-alert-events")).toHaveTextContent("暂无告警记录")
  })

  it("clicking 告警记录 Tab shows alert-events-table when API returns events", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
      if (url.includes("/alert-events")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ data: { items: MOCK_ALERT_EVENTS } }),
        })
      }
      return mockFetch(url)
    }))

    render(<OncallPage />)
    fireEvent.mouseDown(screen.getByTestId("alerts-tab-trigger"))
    await waitFor(() => {
      expect(screen.getByTestId("alert-events-table")).toBeInTheDocument()
    })
    expect(screen.getByText("API Health")).toBeInTheDocument()
    expect(screen.getByText("DB Latency")).toBeInTheDocument()
  })

  it("oncall-stats-card renders when schedule and participants are loaded", async () => {
    render(<OncallPage />)
    // Wait for data to load
    await waitFor(() => {
      expect(screen.getByTestId("current-oncall-name")).toBeInTheDocument()
    })
    // Stats card should be visible in the schedules tab (default tab)
    expect(screen.getByTestId("oncall-stats-card")).toBeInTheDocument()
  })

  it("AddParticipantDialog 存在并可打开", async () => {
    render(<OncallPage />)
    // Wait for schedule and participants to load so the dialog trigger renders
    await waitFor(() => {
      expect(screen.getByTestId("add-participant-button")).toBeInTheDocument()
    })
    // Click the trigger button
    fireEvent.click(screen.getByTestId("add-participant-button"))
    // The dialog content should appear
    await waitFor(() => {
      expect(screen.getByTestId("add-participant-dialog")).toBeInTheDocument()
    })
  })

  it("参与人行显示删除按钮", async () => {
    render(<OncallPage />)
    // Wait until participant rows are rendered
    await waitFor(() => {
      expect(screen.getByTestId("schedule-participant-u_alice")).toBeInTheDocument()
    })
    // Each participant row should have a RemoveParticipantButton (data-testid="remove-participant-<userId>")
    expect(screen.getByTestId("remove-participant-u_alice")).toBeInTheDocument()
    expect(screen.getByTestId("remove-participant-u_bob")).toBeInTheDocument()
    expect(screen.getByTestId("remove-participant-u_carol")).toBeInTheDocument()
  })
})
