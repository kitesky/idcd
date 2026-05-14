import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
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
    start_date: "2024-01-01T09:00:00Z",
    team_id: "t_demo",
  },
]

const MOCK_PARTICIPANTS = [
  { id: "p1", schedule_id: "sch_001", user_id: "u_alice", email: "alice@idcd.com", order_index: 0 },
  { id: "p2", schedule_id: "sch_001", user_id: "u_bob", email: "bob@idcd.com", order_index: 1 },
  { id: "p3", schedule_id: "sch_001", user_id: "u_carol", email: "carol@idcd.com", order_index: 2 },
]

function mockFetch(path: string) {
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
})
