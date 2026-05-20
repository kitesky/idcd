import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { TokensClient } from "../tokens-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
  useLocale: () => "cn",
}))

// ── Fixtures ──────────────────────────────────────────────────────────────────

const MOCK_TOKENS = [
  {
    id: "pat_001",
    name: "本地 CLI",
    key_prefix: "idcd_pat_a1b2c3d4",
    scopes: ["read:monitors", "write:monitors"],
    status: "active",
    expires_at: null,
    created_at: "2025-05-01T00:00:00Z",
    last_used_at: null,
  },
  {
    id: "pat_002",
    name: "MCP 集成",
    key_prefix: "idcd_pat_e5f6a7b8",
    scopes: ["read:monitors"],
    status: "active",
    expires_at: "2026-05-01T00:00:00Z",
    created_at: "2025-05-10T00:00:00Z",
    last_used_at: null,
  },
]

function makeListResponse(tokens = MOCK_TOKENS) {
  return {
    ok: true,
    json: async () => ({ data: { tokens } }),
  }
}

function makeCreateResponse(overrides: Record<string, unknown> = {}) {
  const created = {
    id: "pat_new",
    name: "测试 Token",
    key_prefix: "idcd_pat_new12345",
    scopes: [],
    status: "active",
    expires_at: null,
    created_at: new Date().toISOString(),
    last_used_at: null,
    token: "idcd_pat_newtoken1234567890abcdef",
    ...overrides,
  }
  return {
    ok: true,
    json: async () => ({ data: created }),
  }
}

function makeDeleteResponse() {
  return { ok: true, status: 204, json: async () => ({}) }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("TokensClient", () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(makeListResponse()))
  })

  it("renders the tokens page container", async () => {
    render(<TokensClient />)
    expect(screen.getByTestId("tokens-page")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
  })

  it("renders the tokens card with title", async () => {
    render(<TokensClient />)
    expect(screen.getByTestId("tokens-card")).toBeInTheDocument()
    // With i18n mock, t('tokens.title') returns the key
    expect(screen.getByText("tokens.title")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
  })

  it("renders the generate new token button", async () => {
    render(<TokensClient />)
    const btn = screen.getByTestId("btn-create-token")
    expect(btn).toBeInTheDocument()
    // With i18n mock, t('tokens.create') returns the key
    expect(btn.textContent).toContain("tokens.create")
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
  })

  it("renders the tokens table with 2 rows after loading", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.getByTestId("tokens-table")).toBeInTheDocument()
    )
    expect(screen.getByTestId("token-row-pat_001")).toBeInTheDocument()
    expect(screen.getByTestId("token-row-pat_002")).toBeInTheDocument()
  })

  it("shows token names in table rows", async () => {
    render(<TokensClient />)
    await waitFor(() => expect(screen.getByText("本地 CLI")).toBeInTheDocument())
    expect(screen.getByText("MCP 集成")).toBeInTheDocument()
  })

  it("shows token prefixes in table rows", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.getByText("idcd_pat_a1b2c3d4")).toBeInTheDocument()
    )
    expect(screen.getByText("idcd_pat_e5f6a7b8")).toBeInTheDocument()
  })

  it("shows no-expiry badge for token with no expires_at", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.getByTestId("token-row-pat_001")).toBeInTheDocument()
    )
    const row = screen.getByTestId("token-row-pat_001")
    // With i18n mock, t('tokens.noExpiry') returns the key
    expect(row.textContent).toContain("tokens.noExpiry")
  })

  it("renders revoke button for each token", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-pat_001")).toBeInTheDocument()
    )
    expect(screen.getByTestId("btn-revoke-pat_002")).toBeInTheDocument()
  })

  it("shows revoke confirmation on revoke click", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-pat_001")).toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-revoke-pat_001"))
    expect(screen.getByTestId("btn-confirm-revoke-pat_001")).toBeInTheDocument()
    expect(screen.getByTestId("btn-cancel-revoke-pat_001")).toBeInTheDocument()
  })

  it("cancels revoke and hides confirmation", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-pat_001")).toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-revoke-pat_001"))
    fireEvent.click(screen.getByTestId("btn-cancel-revoke-pat_001"))
    expect(
      screen.queryByTestId("btn-confirm-revoke-pat_001")
    ).not.toBeInTheDocument()
    expect(screen.getByTestId("btn-revoke-pat_001")).toBeInTheDocument()
  })

  it("removes token from list after confirming revoke", async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(makeListResponse())
    fetchMock.mockResolvedValueOnce(makeDeleteResponse())
    vi.stubGlobal("fetch", fetchMock)

    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-pat_001")).toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-revoke-pat_001"))
    fireEvent.click(screen.getByTestId("btn-confirm-revoke-pat_001"))
    await waitFor(() => {
      expect(screen.queryByTestId("token-row-pat_001")).not.toBeInTheDocument()
    })
    expect(screen.getByTestId("token-row-pat_002")).toBeInTheDocument()
  })

  it("opens create dialog on button click", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("create-token-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("input-token-name")).toBeInTheDocument()
  })

  it("create submit button is disabled when name is empty", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("btn-submit-create")).toBeDisabled()
  })

  it("renders scopes checkboxes in create dialog", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("scopes-checkboxes")).toBeInTheDocument()
    expect(
      screen.getByTestId("checkbox-scope-read:monitors")
    ).toBeInTheDocument()
    expect(
      screen.getByTestId("checkbox-scope-write:monitors")
    ).toBeInTheDocument()
  })

  it("renders expiry select in create dialog", async () => {
    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("select-expiry")).toBeInTheDocument()
  })

  it("shows new token value after creation", async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(makeListResponse())
    fetchMock.mockResolvedValueOnce(makeCreateResponse({ name: "测试 Token" }))
    vi.stubGlobal("fetch", fetchMock)

    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-token"))
    const input = screen.getByTestId("input-token-name")
    fireEvent.change(input, { target: { value: "测试 Token" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("new-token-reveal")).toBeInTheDocument()
      expect(screen.getByTestId("new-token-value")).toBeInTheDocument()
      expect(screen.getByTestId("btn-copy-token")).toBeInTheDocument()
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
  })

  it("new token value starts with idcd_pat_ prefix", async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(makeListResponse())
    fetchMock.mockResolvedValueOnce(
      makeCreateResponse({ name: "New Token", token: "idcd_pat_newtoken1234567890" })
    )
    vi.stubGlobal("fetch", fetchMock)

    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-token"))
    const input = screen.getByTestId("input-token-name")
    fireEvent.change(input, { target: { value: "New Token" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("new-token-value")).toBeInTheDocument()
    })
    const tokenValue = screen.getByTestId("new-token-value").textContent ?? ""
    expect(tokenValue).toMatch(/^idcd_pat_/)
  })

  it("closes create dialog and adds new token to table after done", async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(makeListResponse())
    fetchMock.mockResolvedValueOnce(makeCreateResponse({ name: "新 Token" }))
    vi.stubGlobal("fetch", fetchMock)

    render(<TokensClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-token"))
    const input = screen.getByTestId("input-token-name")
    fireEvent.change(input, { target: { value: "新 Token" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("btn-done-create"))
    expect(screen.queryByTestId("create-token-dialog")).not.toBeInTheDocument()
    expect(screen.getByText("新 Token")).toBeInTheDocument()
  })

  it("renders the security note alert", async () => {
    render(<TokensClient />)
    expect(screen.getByTestId("tokens-security-note")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByTestId("loading-tokens-message")).not.toBeInTheDocument()
    )
  })
})
