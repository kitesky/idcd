import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { SessionsClient } from "../sessions-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

// ── Default sessions returned by mock fetch ───────────────────────────────────

const MOCK_SESSIONS = [
  {
    id: "sess_current",
    created_at: new Date(Date.now() - 3_600_000).toISOString(),
    is_current: true,
  },
  {
    id: "sess_old",
    created_at: new Date(Date.now() - 86_400_000).toISOString(),
    is_current: false,
  },
]

function mockFetchSuccess(sessions = MOCK_SESSIONS) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ data: { sessions } }),
  } as unknown as Response)
}

function mockFetchEmpty() {
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ data: { sessions: [] } }),
  } as unknown as Response)
}

function mockFetchError() {
  global.fetch = vi.fn().mockRejectedValue(new Error("network error"))
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("SessionsClient", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it("renders the sessions page container", async () => {
    mockFetchSuccess()
    render(<SessionsClient />)
    expect(screen.getByTestId("sessions-page")).toBeInTheDocument()
  })

  it("shows skeleton while loading", () => {
    // Never-resolving fetch so we stay in loading state
    global.fetch = vi.fn().mockReturnValue(new Promise(() => {}))
    render(<SessionsClient />)
    expect(screen.getByTestId("sessions-skeleton")).toBeInTheDocument()
  })

  it("renders session list after successful load", async () => {
    mockFetchSuccess()
    render(<SessionsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("sessions-list")).toBeInTheDocument()
    })
    expect(screen.getByTestId("session-row-sess_current")).toBeInTheDocument()
    expect(screen.getByTestId("session-row-sess_old")).toBeInTheDocument()
  })

  it("shows '当前会话' badge on the current session", async () => {
    mockFetchSuccess()
    render(<SessionsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("badge-current-sess_current")).toBeInTheDocument()
    })
    expect(screen.queryByTestId("badge-current-sess_old")).not.toBeInTheDocument()
  })

  it("revoke button is disabled for the current session", async () => {
    mockFetchSuccess()
    render(<SessionsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("btn-revoke-sess_current")).toBeDisabled()
    })
  })

  it("revoke button is enabled for non-current sessions", async () => {
    mockFetchSuccess()
    render(<SessionsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("btn-revoke-sess_old")).not.toBeDisabled()
    })
  })

  it("removes session from list after successful revoke", async () => {
    mockFetchSuccess()
    global.fetch = vi
      .fn()
      // First call: list sessions
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { sessions: MOCK_SESSIONS } }),
      } as unknown as Response)
      // Second call: DELETE → 204
      .mockResolvedValueOnce({
        ok: true,
        status: 204,
        json: async () => ({}),
      } as unknown as Response)

    render(<SessionsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("btn-revoke-sess_old")).toBeInTheDocument()
    })

    fireEvent.click(screen.getByTestId("btn-revoke-sess_old"))

    await waitFor(() => {
      expect(screen.queryByTestId("session-row-sess_old")).not.toBeInTheDocument()
    })
    expect(screen.getByTestId("session-row-sess_current")).toBeInTheDocument()
  })

  it("shows empty state when no sessions", async () => {
    mockFetchEmpty()
    render(<SessionsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("sessions-empty")).toBeInTheDocument()
    })
  })

  it("shows error message on fetch failure", async () => {
    mockFetchError()
    render(<SessionsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("sessions-error")).toBeInTheDocument()
    })
  })
})
