import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { APIKeysClient } from "../api-keys/api-keys-client"

// Mock next/navigation (not used directly but imported transitively)
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
  useLocale: () => "cn",
}))

// ── Fixtures ──────────────────────────────────────────────────────────────────

const MOCK_KEYS = [
  {
    id: "key_001",
    name: "生产环境",
    key_prefix: "sk_live_abc...xyz",
    type: "live",
    status: "active",
    created_at: "2025-03-01T00:00:00Z",
    last_used_at: "2025-05-13T00:00:00Z",
    expires_at: null,
  },
  {
    id: "key_002",
    name: "CI/CD 流水线",
    key_prefix: "sk_live_def...uvw",
    type: "live",
    status: "active",
    created_at: "2025-04-15T00:00:00Z",
    last_used_at: null,
    expires_at: null,
  },
]

function makeListResponse(keys = MOCK_KEYS) {
  return {
    ok: true,
    json: async () => ({ data: { api_keys: keys } }),
  }
}

function makeCreateResponse(overrides: Record<string, unknown> = {}) {
  const created = {
    id: "key_new",
    name: "测试 Key",
    key_prefix: "sk_live_new...",
    type: "live",
    status: "active",
    created_at: new Date().toISOString(),
    last_used_at: null,
    expires_at: null,
    key: "sk_live_newkey1234567890",
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

describe("APIKeysClient", () => {
  beforeEach(() => {
    vi.resetAllMocks()
    // Default: GET returns the two mock keys
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(makeListResponse())
    )
  })

  it("renders the api-keys page container", async () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("api-keys-page")).toBeInTheDocument()
    // wait for loading to settle
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
  })

  it("renders the api-keys card", async () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("api-keys-card")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
  })

  it("renders the create API Key button", async () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("btn-create-key")).toBeInTheDocument()
    // With i18n mock, t('apiKeys.create') returns the key
    expect(screen.getByTestId("btn-create-key").textContent).toContain(
      "apiKeys.create"
    )
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
  })

  it("renders the keys table with 2 rows after loading", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("api-keys-table")).toBeInTheDocument()
    )
    expect(screen.getByTestId("key-row-key_001")).toBeInTheDocument()
    expect(screen.getByTestId("key-row-key_002")).toBeInTheDocument()
  })

  it("shows key names in table rows", async () => {
    render(<APIKeysClient />)
    await waitFor(() => expect(screen.getByText("生产环境")).toBeInTheDocument())
    expect(screen.getByText("CI/CD 流水线")).toBeInTheDocument()
  })

  it("shows key prefixes in table rows", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByText("sk_live_abc...xyz")).toBeInTheDocument()
    )
    expect(screen.getByText("sk_live_def...uvw")).toBeInTheDocument()
  })

  it("shows never-used badge for key with no last_used_at", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("key-row-key_002")).toBeInTheDocument()
    )
    const row = screen.getByTestId("key-row-key_002")
    // With i18n mock, t('apiKeys.neverUsed') returns the key
    expect(row.textContent).toContain("apiKeys.neverUsed")
  })

  it("renders revoke button for each key", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-key_001")).toBeInTheDocument()
    )
    expect(screen.getByTestId("btn-revoke-key_002")).toBeInTheDocument()
  })

  it("shows revoke confirmation panel on revoke button click", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-key_001")).toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-revoke-key_001"))
    expect(screen.getByTestId("btn-confirm-revoke-key_001")).toBeInTheDocument()
    expect(screen.getByTestId("btn-cancel-revoke-key_001")).toBeInTheDocument()
  })

  it("cancels revoke and hides confirmation panel", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-key_001")).toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-revoke-key_001"))
    fireEvent.click(screen.getByTestId("btn-cancel-revoke-key_001"))
    expect(
      screen.queryByTestId("btn-confirm-revoke-key_001")
    ).not.toBeInTheDocument()
    expect(screen.getByTestId("btn-revoke-key_001")).toBeInTheDocument()
  })

  it("removes key from list after confirming revoke", async () => {
    const fetchMock = vi.fn()
    // First call: GET list
    fetchMock.mockResolvedValueOnce(makeListResponse())
    // Second call: DELETE
    fetchMock.mockResolvedValueOnce(makeDeleteResponse())
    vi.stubGlobal("fetch", fetchMock)

    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("btn-revoke-key_001")).toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-revoke-key_001"))
    fireEvent.click(screen.getByTestId("btn-confirm-revoke-key_001"))
    await waitFor(() => {
      expect(screen.queryByTestId("key-row-key_001")).not.toBeInTheDocument()
    })
    // key_002 still present
    expect(screen.getByTestId("key-row-key_002")).toBeInTheDocument()
  })

  it("opens create dialog on button click", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-key"))
    expect(screen.getByTestId("create-key-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("input-key-name")).toBeInTheDocument()
  })

  it("create submit button is disabled when key name is empty", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-key"))
    expect(screen.getByTestId("btn-submit-create")).toBeDisabled()
  })

  it("shows new key value after creation and hides form", async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(makeListResponse())
    fetchMock.mockResolvedValueOnce(
      makeCreateResponse({ name: "测试 Key", key: "sk_live_newkey1234567890" })
    )
    vi.stubGlobal("fetch", fetchMock)

    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-key"))
    const input = screen.getByTestId("input-key-name")
    fireEvent.change(input, { target: { value: "测试 Key" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("new-key-reveal")).toBeInTheDocument()
      expect(screen.getByTestId("new-key-value")).toBeInTheDocument()
      expect(screen.getByTestId("btn-copy-key")).toBeInTheDocument()
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
  })

  it("closes create dialog and adds new key to table after done", async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(makeListResponse())
    fetchMock.mockResolvedValueOnce(
      makeCreateResponse({ name: "新 Key" })
    )
    vi.stubGlobal("fetch", fetchMock)

    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-key"))
    const input = screen.getByTestId("input-key-name")
    fireEvent.change(input, { target: { value: "新 Key" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("btn-done-create"))
    expect(screen.queryByTestId("create-key-dialog")).not.toBeInTheDocument()
    expect(screen.getByText("新 Key")).toBeInTheDocument()
  })

  it("renders the security note alert", async () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("api-keys-security-note")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
  })

  // ── key type badge tests ──────────────────────────────────────────────────

  it("renders production badge for live type keys", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("api-keys-table")).toBeInTheDocument()
    )
    const badges = screen.getAllByTestId("badge-production")
    expect(badges.length).toBeGreaterThanOrEqual(2)
  })

  it("shows type select in create dialog with live default", async () => {
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-key"))
    expect(screen.getByTestId("select-key-type")).toBeInTheDocument()
  })

  it("new key value is shown after creation", async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(makeListResponse())
    fetchMock.mockResolvedValueOnce(
      makeCreateResponse({ name: "Prod Key", key: "sk_live_prodkey987654321" })
    )
    vi.stubGlobal("fetch", fetchMock)

    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.queryByTestId("loading-keys-message")).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByTestId("btn-create-key"))
    const input = screen.getByTestId("input-key-name")
    fireEvent.change(input, { target: { value: "Prod Key" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("new-key-value")).toBeInTheDocument()
    })
    const keyValue = screen.getByTestId("new-key-value").textContent ?? ""
    expect(keyValue).toBe("sk_live_prodkey987654321")
  })

  it("shows empty state when no keys are returned", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(makeListResponse([]))
    )
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("empty-keys-message")).toBeInTheDocument()
    )
  })

  it("shows error state when load fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ message: "服务器错误" }) })
    )
    render(<APIKeysClient />)
    await waitFor(() =>
      expect(screen.getByTestId("load-keys-error")).toBeInTheDocument()
    )
  })
})
