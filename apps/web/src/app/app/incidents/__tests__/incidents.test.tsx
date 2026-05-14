import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/incidents",
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

import IncidentsPage from "../page"

describe("IncidentsPage", () => {
  it("renders the page container", () => {
    render(<IncidentsPage />)
    expect(screen.getByTestId("incidents-page")).toBeInTheDocument()
  })

  it("renders page title 故障记录", () => {
    render(<IncidentsPage />)
    expect(screen.getByText("故障记录")).toBeInTheDocument()
  })

  it("renders the incidents table", () => {
    render(<IncidentsPage />)
    expect(screen.getByTestId("incidents-table")).toBeInTheDocument()
  })

  it("renders 5 mock incident rows", () => {
    render(<IncidentsPage />)
    const rows = screen.getAllByTestId(/^incident-row-/)
    expect(rows).toHaveLength(5)
  })

  it("renders severity badges for each incident", () => {
    render(<IncidentsPage />)
    const badges = screen.getAllByTestId(/^severity-badge-/)
    expect(badges.length).toBeGreaterThan(0)
  })

  it("renders generate buttons for each incident", () => {
    render(<IncidentsPage />)
    const buttons = screen.getAllByTestId(/^generate-btn-/)
    expect(buttons).toHaveLength(5)
    expect(buttons[0]).toHaveTextContent("生成复盘")
  })

  it("shows postmortem status badges", () => {
    render(<IncidentsPage />)
    const statuses = screen.getAllByTestId(/^postmortem-status-/)
    expect(statuses.length).toBeGreaterThan(0)
  })

  it("shows 已生成 badge for incidents with postmortem", () => {
    render(<IncidentsPage />)
    const generated = screen.getAllByText("已生成")
    expect(generated.length).toBeGreaterThan(0)
  })

  it("shows 未生成 badge for incidents without postmortem", () => {
    render(<IncidentsPage />)
    const notGenerated = screen.getAllByText("未生成")
    expect(notGenerated.length).toBeGreaterThan(0)
  })

  it("clicking generate button shows generating state", async () => {
    render(<IncidentsPage />)
    const firstBtn = screen.getAllByTestId(/^generate-btn-/)[0]
    fireEvent.click(firstBtn)
    expect(firstBtn).toHaveTextContent("生成中...")
  })
})
