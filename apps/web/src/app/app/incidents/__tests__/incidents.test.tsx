import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

const mockPush = vi.fn()

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/incidents",
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
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

const mockIncidents = [
  {
    alert_event_id: "ev_001",
    monitor_id: "m1",
    monitor_name: "API Gateway",
    status: "resolved",
    severity: "high",
    started_at: "2026-05-13T14:23:00Z",
    resolved_at: "2026-05-13T15:10:00Z",
    has_postmortem: true,
  },
  {
    alert_event_id: "ev_002",
    monitor_id: "m2",
    monitor_name: "Payment Service",
    status: "resolved",
    severity: "critical",
    started_at: "2026-05-12T09:11:00Z",
    resolved_at: null,
    has_postmortem: false,
  },
]

vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
}))

import { apiRequest } from "@/lib/api"
import IncidentsPage from "../page"

const mockedApiRequest = apiRequest as ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.clearAllMocks()
  mockPush.mockReset()
})

describe("IncidentsPage", () => {
  it("renders the page container", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    expect(screen.getByTestId("incidents-page")).toBeInTheDocument()
  })

  it("renders page title 故障记录", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    expect(screen.getByText("故障记录")).toBeInTheDocument()
  })

  it("shows skeleton while loading", () => {
    mockedApiRequest.mockReturnValueOnce(new Promise(() => {}))
    render(<IncidentsPage />)
    expect(screen.getByTestId("incidents-skeleton")).toBeInTheDocument()
  })

  it("renders the incidents table after load", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("incidents-table")).toBeInTheDocument()
    })
  })

  it("renders incident rows from API", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("incident-row-ev_001")).toBeInTheDocument()
      expect(screen.getByTestId("incident-row-ev_002")).toBeInTheDocument()
    })
  })

  it("renders severity badges for each incident", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    await waitFor(() => {
      const badges = screen.getAllByTestId(/^severity-badge-/)
      expect(badges.length).toBeGreaterThan(0)
    })
  })

  it("renders generate buttons for each incident", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    await waitFor(() => {
      const buttons = screen.getAllByTestId(/^generate-btn-/)
      expect(buttons).toHaveLength(2)
      expect(buttons[0]).toHaveTextContent("生成复盘")
    })
  })

  it("shows postmortem status badges", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    await waitFor(() => {
      const statuses = screen.getAllByTestId(/^postmortem-status-/)
      expect(statuses.length).toBeGreaterThan(0)
    })
  })

  it("shows 已生成 badge for incidents with postmortem", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByText("已生成")).toBeInTheDocument()
    })
  })

  it("shows 未生成 badge for incidents without postmortem", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByText("未生成")).toBeInTheDocument()
    })
  })

  it("shows error alert when API fails", async () => {
    mockedApiRequest.mockRejectedValueOnce(new Error("Network error"))
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("incidents-error-alert")).toBeInTheDocument()
    })
  })

  it("shows empty state when no incidents", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: [] } })
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("incidents-empty-state")).toBeInTheDocument()
    })
  })

  it("clicking generate button calls API and redirects", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    mockedApiRequest.mockResolvedValueOnce({ data: { id: "pm_123", title: "Draft" } })
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("generate-btn-ev_001")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("generate-btn-ev_001"))
    expect(screen.getByTestId("generate-btn-ev_001")).toHaveTextContent("生成中...")
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/app/incidents/pm_123")
    })
  })

  it("shows generate error alert when draft API fails", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { incidents: mockIncidents } })
    mockedApiRequest.mockRejectedValueOnce(new Error("生成失败"))
    render(<IncidentsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("generate-btn-ev_001")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("generate-btn-ev_001"))
    await waitFor(() => {
      expect(screen.getByTestId("generate-error-alert")).toBeInTheDocument()
    })
  })
})
