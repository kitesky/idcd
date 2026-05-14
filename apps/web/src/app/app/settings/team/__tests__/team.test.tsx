import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { TeamClient } from "../team-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe("TeamClient", () => {
  it("renders without crashing", () => {
    render(<TeamClient />)
    expect(screen.getByTestId("team-page")).toBeInTheDocument()
  })

  it("renders the members table", () => {
    render(<TeamClient />)
    expect(screen.getByTestId("members-table")).toBeInTheDocument()
  })

  it("shows correct role badges for mock members", () => {
    render(<TeamClient />)
    expect(screen.getByTestId("role-badge-owner")).toBeInTheDocument()
    expect(screen.getByTestId("role-badge-admin")).toBeInTheDocument()
    expect(screen.getAllByTestId("role-badge-member").length).toBeGreaterThanOrEqual(1)
  })

  it("invite button is present", () => {
    render(<TeamClient />)
    expect(screen.getByTestId("btn-invite-member")).toBeInTheDocument()
  })

  it("shows member emails in the table", () => {
    render(<TeamClient />)
    expect(screen.getByText("alice@acme.com")).toBeInTheDocument()
    expect(screen.getByText("bob@acme.com")).toBeInTheDocument()
    expect(screen.getByText("carol@acme.com")).toBeInTheDocument()
  })

  it("shows pending invitations card", () => {
    render(<TeamClient />)
    expect(screen.getByTestId("pending-invitations-card")).toBeInTheDocument()
  })

  it("opens invite dialog on button click", () => {
    render(<TeamClient />)
    fireEvent.click(screen.getByTestId("btn-invite-member"))
    expect(screen.getByTestId("input-invite-email")).toBeInTheDocument()
    expect(screen.getByTestId("select-invite-role")).toBeInTheDocument()
  })

  it("shows team name in header", () => {
    render(<TeamClient />)
    expect(screen.getByTestId("team-name")).toHaveTextContent("Acme Corp")
  })
})
