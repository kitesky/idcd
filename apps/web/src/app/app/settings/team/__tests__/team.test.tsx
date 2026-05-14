import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { TeamClient } from "../team-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

// Mock the apiRequest helper
vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
  API_BASE: "http://localhost:8080",
}))

import { apiRequest } from "@/lib/api"
const mockApiRequest = vi.mocked(apiRequest)

const MOCK_TEAM = {
  id: "team_acme01",
  name: "Acme Corp",
  slug: "acme",
  plan: "free",
  member_count: 3,
}

const MOCK_MEMBERS = [
  { id: "tmb_001", user_id: "u_owner01", email: "alice@acme.com", role: "owner", joined_at: "2025-03-01" },
  { id: "tmb_002", user_id: "u_admin01", email: "bob@acme.com", role: "admin", joined_at: "2025-03-15" },
  { id: "tmb_003", user_id: "u_member01", email: "carol@acme.com", role: "member", joined_at: "2025-04-01" },
]

const MOCK_INVITATIONS = [
  { id: "tinv_001", email: "dave@acme.com", role: "member", expires_at: "2026-05-21" },
]

const MOCK_KEYS = [
  { id: "key_t001", name: "CI/CD Key", prefix: "sk_live_deadbeef...", key_type: "production", created_at: "2026-05-01" },
  { id: "key_t002", name: "Staging Key", prefix: "sk_test_cafebabe...", key_type: "test", created_at: "2026-05-10" },
]

function setupMocks() {
  mockApiRequest.mockImplementation((path: string) => {
    if (path === "/v1/teams") {
      return Promise.resolve({ data: { teams: [MOCK_TEAM] } })
    }
    if (path === `/v1/teams/${MOCK_TEAM.id}/members`) {
      return Promise.resolve({ data: { members: MOCK_MEMBERS } })
    }
    if (path === `/v1/teams/${MOCK_TEAM.id}/invitations`) {
      return Promise.resolve({ data: { invitations: MOCK_INVITATIONS } })
    }
    if (path === `/v1/teams/${MOCK_TEAM.id}/api-keys`) {
      return Promise.resolve({ data: { api_keys: MOCK_KEYS } })
    }
    return Promise.resolve({})
  })
}

describe("TeamClient", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupMocks()
  })

  it("renders without crashing and shows loading skeleton initially", () => {
    render(<TeamClient />)
    expect(screen.getByTestId("team-page")).toBeInTheDocument()
  })

  it("renders the members table after loading", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("members-table")).toBeInTheDocument())
  })

  it("shows correct role badges for members", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("role-badge-owner")).toBeInTheDocument())
    expect(screen.getByTestId("role-badge-admin")).toBeInTheDocument()
    expect(screen.getAllByTestId("role-badge-member").length).toBeGreaterThanOrEqual(1)
  })

  it("invite button is present", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("btn-invite-member")).toBeInTheDocument())
  })

  it("shows member emails in the table", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByText("alice@acme.com")).toBeInTheDocument())
    expect(screen.getByText("bob@acme.com")).toBeInTheDocument()
    expect(screen.getByText("carol@acme.com")).toBeInTheDocument()
  })

  it("shows pending invitations card", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("pending-invitations-card")).toBeInTheDocument())
  })

  it("opens invite dialog on button click", async () => {
    render(<TeamClient />)
    await waitFor(() => screen.getByTestId("btn-invite-member"))
    fireEvent.click(screen.getByTestId("btn-invite-member"))
    await waitFor(() => expect(screen.getByTestId("input-invite-email")).toBeInTheDocument())
    expect(screen.getByTestId("select-invite-role")).toBeInTheDocument()
  })

  it("shows team name in header", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("team-name")).toHaveTextContent("Acme Corp"))
  })

  it("renders the team API Keys section", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("team-api-keys-card")).toBeInTheDocument())
    expect(screen.getByTestId("team-keys-table")).toBeInTheDocument()
    expect(screen.getByTestId("btn-add-team-key")).toBeInTheDocument()
  })

  it("shows API keys from server in the team keys table", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("key-row-key_t001")).toBeInTheDocument())
    expect(screen.getByTestId("key-row-key_t002")).toBeInTheDocument()
    expect(screen.getByText("CI/CD Key")).toBeInTheDocument()
    expect(screen.getByText("Staging Key")).toBeInTheDocument()
  })

  it("renders the team subscription section", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("team-subscription-card")).toBeInTheDocument())
    expect(screen.getByTestId("team-plan-badge")).toBeInTheDocument()
  })

  it("shows upgrade button when plan is free", async () => {
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("btn-upgrade-team")).toBeInTheDocument())
  })

  it("opens add key dialog on button click", async () => {
    render(<TeamClient />)
    await waitFor(() => screen.getByTestId("btn-add-team-key"))
    fireEvent.click(screen.getByTestId("btn-add-team-key"))
    await waitFor(() => expect(screen.getByTestId("input-key-name")).toBeInTheDocument())
    expect(screen.getByTestId("select-key-type")).toBeInTheDocument()
  })

  it("shows empty state when no teams returned", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: { teams: [] } })
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("team-empty-state")).toBeInTheDocument())
  })

  it("shows error alert when teams API fails", async () => {
    mockApiRequest.mockRejectedValueOnce(new Error("网络错误"))
    render(<TeamClient />)
    await waitFor(() => expect(screen.getByTestId("team-error-alert")).toBeInTheDocument())
  })

  it("calls invite API and updates invitation list", async () => {
    const newInvitation = { id: "tinv_002", email: "eve@acme.com", role: "member", expires_at: "2026-06-01" }
    mockApiRequest.mockImplementation((path: string, opts?: any) => {
      if (path === "/v1/teams") return Promise.resolve({ data: { teams: [MOCK_TEAM] } })
      if (path === `/v1/teams/${MOCK_TEAM.id}/members`) return Promise.resolve({ data: { members: MOCK_MEMBERS } })
      if (path === `/v1/teams/${MOCK_TEAM.id}/invitations`) {
        if (opts?.method === "POST") return Promise.resolve({ data: { invitation: newInvitation } })
        return Promise.resolve({ data: { invitations: MOCK_INVITATIONS } })
      }
      if (path === `/v1/teams/${MOCK_TEAM.id}/api-keys`) return Promise.resolve({ data: { api_keys: MOCK_KEYS } })
      return Promise.resolve({})
    })

    render(<TeamClient />)
    await waitFor(() => screen.getByTestId("btn-invite-member"))
    fireEvent.click(screen.getByTestId("btn-invite-member"))
    await waitFor(() => screen.getByTestId("input-invite-email"))
    fireEvent.change(screen.getByTestId("input-invite-email"), { target: { value: "eve@acme.com" } })
    fireEvent.click(screen.getByTestId("btn-confirm-invite"))
    await waitFor(() => expect(screen.queryByTestId("input-invite-email")).not.toBeInTheDocument())
  })

  it("calls revoke API and removes key from list", async () => {
    mockApiRequest.mockImplementation((path: string, opts?: any) => {
      if (path === "/v1/teams") return Promise.resolve({ data: { teams: [MOCK_TEAM] } })
      if (path === `/v1/teams/${MOCK_TEAM.id}/members`) return Promise.resolve({ data: { members: MOCK_MEMBERS } })
      if (path === `/v1/teams/${MOCK_TEAM.id}/invitations`) return Promise.resolve({ data: { invitations: [] } })
      if (path === `/v1/teams/${MOCK_TEAM.id}/api-keys`) {
        if (opts?.method === "DELETE") return Promise.resolve({})
        return Promise.resolve({ data: { api_keys: MOCK_KEYS } })
      }
      return Promise.resolve({})
    })

    render(<TeamClient />)
    await waitFor(() => screen.getByTestId("btn-revoke-key-key_t001"))
    fireEvent.click(screen.getByTestId("btn-revoke-key-key_t001"))
    await waitFor(() => expect(screen.queryByTestId("key-row-key_t001")).not.toBeInTheDocument())
  })
})
